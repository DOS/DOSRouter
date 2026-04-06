package router

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ClassifyByRules scores a request across 15 weighted dimensions and maps
// the aggregate score to a tier using configurable boundaries. Confidence
// is calibrated via sigmoid - low confidence triggers the fallback classifier.
//
// Handles 70-80% of requests in < 1ms with zero cost.
func ClassifyByRules(prompt, systemPrompt string, estimatedTokens int, config ScoringConfig) ScoringResult {
	// Score against user prompt only - system prompts contain boilerplate keywords
	// (tool definitions, skill descriptions, behavioral rules) that dominate scoring.
	userText := strings.ToLower(prompt)

	// Score all 15 dimensions against user text only
	dimensions := []DimensionScore{
		// Token count uses total estimated tokens (system + user)
		scoreTokenCount(estimatedTokens, config.TokenCountThresholds.Simple, config.TokenCountThresholds.Complex),
		scoreKeywordMatch(userText, config.CodeKeywords, "codePresence", "code",
			1, 2, 0, 0.5, 1.0),
		scoreKeywordMatch(userText, config.ReasoningKeywords, "reasoningMarkers", "reasoning",
			1, 2, 0, 0.7, 1.0),
		scoreKeywordMatch(userText, config.TechnicalKeywords, "technicalTerms", "technical",
			2, 4, 0, 0.5, 1.0),
		scoreKeywordMatch(userText, config.CreativeKeywords, "creativeMarkers", "creative",
			1, 2, 0, 0.5, 0.7),
		scoreKeywordMatch(userText, config.SimpleKeywords, "simpleIndicators", "simple",
			1, 2, 0, -1.0, -1.0),
		scoreMultiStep(userText),
		scoreQuestionComplexity(prompt),
		// 6 new dimensions
		scoreKeywordMatch(userText, config.ImperativeVerbs, "imperativeVerbs", "imperative",
			1, 2, 0, 0.3, 0.5),
		scoreKeywordMatch(userText, config.ConstraintIndicators, "constraintCount", "constraints",
			1, 3, 0, 0.3, 0.7),
		scoreKeywordMatch(userText, config.OutputFormatKeywords, "outputFormat", "format",
			1, 2, 0, 0.4, 0.7),
		scoreKeywordMatch(userText, config.ReferenceKeywords, "referenceComplexity", "references",
			1, 2, 0, 0.3, 0.5),
		scoreKeywordMatch(userText, config.NegationKeywords, "negationComplexity", "negation",
			2, 3, 0, 0.3, 0.5),
		scoreKeywordMatch(userText, config.DomainSpecificKeywords, "domainSpecificity", "domain-specific",
			1, 2, 0, 0.5, 0.8),
	}

	// Score agentic task indicators - user prompt only
	agenticDim, agenticScore := scoreAgenticTask(userText, config.AgenticTaskKeywords)
	dimensions = append(dimensions, agenticDim)

	// Collect signals
	var signals []string
	for _, d := range dimensions {
		if d.Signal != "" {
			signals = append(signals, d.Signal)
		}
	}

	// Compute weighted score
	var weightedScore float64
	for _, d := range dimensions {
		w := config.DimensionWeights[d.Name]
		weightedScore += d.Score * w
	}

	// Count reasoning markers for override - only check USER prompt
	var reasoningMatches int
	for _, kw := range config.ReasoningKeywords {
		if strings.Contains(userText, strings.ToLower(kw)) {
			reasoningMatches++
		}
	}

	// Direct reasoning override: 2+ reasoning markers = high confidence REASONING
	if reasoningMatches >= 2 {
		dist := weightedScore
		if dist < 0.3 {
			dist = 0.3
		}
		confidence := calibrateConfidence(dist, config.ConfidenceSteepness)
		if confidence < 0.85 {
			confidence = 0.85
		}
		tier := TierReasoning
		return ScoringResult{
			Score:        weightedScore,
			Tier:         &tier,
			Confidence:   confidence,
			Signals:      signals,
			AgenticScore: agenticScore,
			Dimensions:   dimensions,
		}
	}

	// Map weighted score to tier using boundaries
	var tier Tier
	var distanceFromBoundary float64

	sm := config.TierBoundaries.SimpleMedium
	mc := config.TierBoundaries.MediumComplex
	cr := config.TierBoundaries.ComplexReasoning

	if weightedScore < sm {
		tier = TierSimple
		distanceFromBoundary = sm - weightedScore
	} else if weightedScore < mc {
		tier = TierMedium
		distanceFromBoundary = math.Min(weightedScore-sm, mc-weightedScore)
	} else if weightedScore < cr {
		tier = TierComplex
		distanceFromBoundary = math.Min(weightedScore-mc, cr-weightedScore)
	} else {
		tier = TierReasoning
		distanceFromBoundary = weightedScore - cr
	}

	// Calibrate confidence via sigmoid
	confidence := calibrateConfidence(distanceFromBoundary, config.ConfidenceSteepness)

	// If confidence is below threshold -> ambiguous
	if confidence < config.ConfidenceThreshold {
		return ScoringResult{
			Score:        weightedScore,
			Tier:         nil,
			Confidence:   confidence,
			Signals:      signals,
			AgenticScore: agenticScore,
			Dimensions:   dimensions,
		}
	}

	return ScoringResult{
		Score:        weightedScore,
		Tier:         &tier,
		Confidence:   confidence,
		Signals:      signals,
		AgenticScore: agenticScore,
		Dimensions:   dimensions,
	}
}

// --- Dimension Scorers ---

func scoreTokenCount(estimatedTokens, simpleThreshold, complexThreshold int) DimensionScore {
	if estimatedTokens < simpleThreshold {
		return DimensionScore{Name: "tokenCount", Score: -1.0, Signal: fmt.Sprintf("short (%d tokens)", estimatedTokens)}
	}
	if estimatedTokens > complexThreshold {
		return DimensionScore{Name: "tokenCount", Score: 1.0, Signal: fmt.Sprintf("long (%d tokens)", estimatedTokens)}
	}
	return DimensionScore{Name: "tokenCount", Score: 0}
}

func scoreKeywordMatch(text string, keywords []string, name, signalLabel string,
	lowThreshold, highThreshold int,
	noneScore, lowScore, highScore float64) DimensionScore {

	var matches []string
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			matches = append(matches, kw)
		}
	}

	if len(matches) >= highThreshold {
		top := matches
		if len(top) > 3 {
			top = top[:3]
		}
		return DimensionScore{
			Name:   name,
			Score:  highScore,
			Signal: fmt.Sprintf("%s (%s)", signalLabel, strings.Join(top, ", ")),
		}
	}
	if len(matches) >= lowThreshold {
		top := matches
		if len(top) > 3 {
			top = top[:3]
		}
		return DimensionScore{
			Name:   name,
			Score:  lowScore,
			Signal: fmt.Sprintf("%s (%s)", signalLabel, strings.Join(top, ", ")),
		}
	}
	return DimensionScore{Name: name, Score: noneScore}
}

var multiStepPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)first.*then`),
	regexp.MustCompile(`(?i)step \d`),
	regexp.MustCompile(`\d\.\s`),
}

func scoreMultiStep(text string) DimensionScore {
	for _, p := range multiStepPatterns {
		if p.MatchString(text) {
			return DimensionScore{Name: "multiStepPatterns", Score: 0.5, Signal: "multi-step"}
		}
	}
	return DimensionScore{Name: "multiStepPatterns", Score: 0}
}

func scoreQuestionComplexity(prompt string) DimensionScore {
	count := strings.Count(prompt, "?")
	if count > 3 {
		return DimensionScore{
			Name:   "questionComplexity",
			Score:  0.5,
			Signal: fmt.Sprintf("%d questions", count),
		}
	}
	return DimensionScore{Name: "questionComplexity", Score: 0}
}

// scoreAgenticTask returns a dimension score and a separate agentic score (0-1).
func scoreAgenticTask(text string, keywords []string) (DimensionScore, float64) {
	var matchCount int
	var signals []string

	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			matchCount++
			if len(signals) < 3 {
				signals = append(signals, kw)
			}
		}
	}

	sig := strings.Join(signals, ", ")

	if matchCount >= 4 {
		return DimensionScore{Name: "agenticTask", Score: 1.0, Signal: fmt.Sprintf("agentic (%s)", sig)}, 1.0
	} else if matchCount >= 3 {
		return DimensionScore{Name: "agenticTask", Score: 0.6, Signal: fmt.Sprintf("agentic (%s)", sig)}, 0.6
	} else if matchCount >= 1 {
		return DimensionScore{Name: "agenticTask", Score: 0.2, Signal: fmt.Sprintf("agentic-light (%s)", sig)}, 0.2
	}

	return DimensionScore{Name: "agenticTask", Score: 0}, 0
}

// calibrateConfidence maps distance from tier boundary to [0.5, 1.0] confidence.
func calibrateConfidence(distance, steepness float64) float64 {
	return 1.0 / (1.0 + math.Exp(-steepness*distance))
}
