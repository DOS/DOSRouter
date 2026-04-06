package router

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// applyPromotions merges active time-windowed promotions into tier configs.
func applyPromotions(
	tierConfigs map[Tier]TierConfig,
	promotions []Promotion,
	profile string,
	now time.Time,
) map[Tier]TierConfig {
	if len(promotions) == 0 {
		return tierConfigs
	}

	mutated := false
	result := tierConfigs

	for _, promo := range promotions {
		start, err1 := time.Parse("2006-01-02", promo.StartDate)
		end, err2 := time.Parse("2006-01-02", promo.EndDate)
		if err1 != nil || err2 != nil {
			continue
		}
		if now.Before(start) || !now.Before(end) {
			continue
		}

		// Check profile filter
		if len(promo.Profiles) > 0 {
			found := false
			for _, p := range promo.Profiles {
				if p == profile {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Shallow-clone on first mutation
		if !mutated {
			result = make(map[Tier]TierConfig, len(tierConfigs))
			for k, v := range tierConfigs {
				fb := make([]string, len(v.Fallback))
				copy(fb, v.Fallback)
				result[k] = TierConfig{Primary: v.Primary, Fallback: fb}
			}
			mutated = true
		}

		// Merge overrides
		for tier, override := range promo.TierOverrides {
			tc, ok := result[tier]
			if !ok {
				continue
			}
			if override.Primary != "" {
				tc.Primary = override.Primary
			}
			if len(override.Fallback) > 0 {
				tc.Fallback = override.Fallback
			}
			result[tier] = tc
		}
	}

	return result
}

// RulesStrategy implements RouterStrategy using the 15-dimension rule-based classifier.
type RulesStrategy struct{}

func (s *RulesStrategy) Name() string { return "rules" }

func (s *RulesStrategy) Route(prompt string, systemPrompt string, maxOutputTokens int, options RouterOptions) RoutingDecision {
	config := options.Config

	// Estimate input tokens (~4 chars per token)
	fullText := systemPrompt + " " + prompt
	estimatedTokens := int(math.Ceil(float64(len(fullText)) / 4))

	// Rule-based classification
	ruleResult := ClassifyByRules(prompt, systemPrompt, estimatedTokens, config.Scoring)

	// Select tier configs based on routing profile
	routingProfile := options.RoutingProfile
	var tierConfigs map[Tier]TierConfig
	var profileSuffix string
	var profile string

	if routingProfile == "eco" && len(config.EcoTiers) > 0 {
		tierConfigs = config.EcoTiers
		profileSuffix = " | eco"
		profile = "eco"
	} else if routingProfile == "premium" && len(config.PremiumTiers) > 0 {
		tierConfigs = config.PremiumTiers
		profileSuffix = " | premium"
		profile = "premium"
	} else {
		// Auto profile: intelligent routing with agentic detection
		agenticScore := ruleResult.AgenticScore
		isAutoAgentic := agenticScore >= 0.5
		isExplicitAgentic := config.Overrides.AgenticMode
		hasToolsInRequest := options.HasTools
		useAgenticTiers := (hasToolsInRequest || isAutoAgentic || isExplicitAgentic) && len(config.AgenticTiers) > 0

		if useAgenticTiers {
			tierConfigs = config.AgenticTiers
			if hasToolsInRequest {
				profileSuffix = " | agentic (tools)"
			} else {
				profileSuffix = " | agentic"
			}
			profile = "agentic"
		} else {
			tierConfigs = config.Tiers
			profile = "auto"
		}
	}

	// Apply time-windowed promotions
	now := time.Now()
	if options.Now != nil {
		now = time.UnixMilli(*options.Now)
	}
	tierConfigs = applyPromotions(tierConfigs, config.Promotions, profile, now)

	agenticScoreValue := ruleResult.AgenticScore

	// Override: large context -> force COMPLEX
	if estimatedTokens > config.Overrides.MaxTokensForceComplex {
		decision := SelectModel(
			TierComplex,
			0.95,
			"rules",
			fmt.Sprintf("Input exceeds %d tokens%s", config.Overrides.MaxTokensForceComplex, profileSuffix),
			tierConfigs,
			options.ModelPricing,
			estimatedTokens,
			maxOutputTokens,
			routingProfile,
			agenticScoreValue,
		)
		decision.TierConfigs = tierConfigs
		decision.Profile = profile
		return decision
	}

	// Structured output detection
	hasStructuredOutput := false
	if systemPrompt != "" {
		structuredRe := regexp.MustCompile(`(?i)json|structured|schema`)
		hasStructuredOutput = structuredRe.MatchString(systemPrompt)
	}

	var tier Tier
	var confidence float64
	method := "rules"
	reasoning := fmt.Sprintf("score=%.2f | %s", ruleResult.Score, strings.Join(ruleResult.Signals, ", "))

	if ruleResult.Tier != nil {
		tier = *ruleResult.Tier
		confidence = ruleResult.Confidence
	} else {
		// Ambiguous -> default to configurable tier
		tier = config.Overrides.AmbiguousDefaultTier
		confidence = 0.5
		reasoning += fmt.Sprintf(" | ambiguous -> default: %s", tier)
	}

	// Apply structured output minimum tier
	if hasStructuredOutput {
		minTier := config.Overrides.StructuredOutputMinTier
		if TierRank(tier) < TierRank(minTier) {
			reasoning += fmt.Sprintf(" | upgraded to %s (structured output)", minTier)
			tier = minTier
		}
	}

	reasoning += profileSuffix

	decision := SelectModel(
		tier,
		confidence,
		method,
		reasoning,
		tierConfigs,
		options.ModelPricing,
		estimatedTokens,
		maxOutputTokens,
		routingProfile,
		agenticScoreValue,
	)
	decision.TierConfigs = tierConfigs
	decision.Profile = profile
	return decision
}

// --- Strategy Registry ---

var registry = map[string]RouterStrategy{
	"rules": &RulesStrategy{},
}

// GetStrategy returns a registered routing strategy by name.
func GetStrategy(name string) (RouterStrategy, error) {
	s, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown routing strategy: %s", name)
	}
	return s, nil
}

// RegisterStrategy adds a custom routing strategy to the registry.
func RegisterStrategy(s RouterStrategy) {
	registry[s.Name()] = s
}
