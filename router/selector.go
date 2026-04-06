package router

import "math"

const (
	baselineModelID        = "anthropic/claude-opus-4.6"
	baselineInputPrice     = 5.0  // per 1M tokens
	baselineOutputPrice    = 25.0 // per 1M tokens
	serverMarginPercent    = 5
	minPaymentUSD          = 0.001
)

// SelectModel picks the primary model for a tier and builds a RoutingDecision.
func SelectModel(
	tier Tier,
	confidence float64,
	method string,
	reasoning string,
	tierConfigs map[Tier]TierConfig,
	modelPricing map[string]ModelPricing,
	estimatedInputTokens int,
	maxOutputTokens int,
	routingProfile string,
	agenticScore float64,
) RoutingDecision {
	tc := tierConfigs[tier]
	model := tc.Primary
	pricing, hasPricing := modelPricing[model]

	var costEstimate float64
	if hasPricing && pricing.FlatPrice != nil {
		costEstimate = *pricing.FlatPrice
	} else {
		var inputPrice, outputPrice float64
		if hasPricing {
			inputPrice = pricing.InputPrice
			outputPrice = pricing.OutputPrice
		}
		costEstimate = float64(estimatedInputTokens)/1_000_000*inputPrice +
			float64(maxOutputTokens)/1_000_000*outputPrice
	}

	// Baseline: Claude Opus cost
	opusPricing, hasOpus := modelPricing[baselineModelID]
	opusIn := baselineInputPrice
	opusOut := baselineOutputPrice
	if hasOpus {
		opusIn = opusPricing.InputPrice
		opusOut = opusPricing.OutputPrice
	}
	baselineCost := float64(estimatedInputTokens)/1_000_000*opusIn +
		float64(maxOutputTokens)/1_000_000*opusOut

	var savings float64
	if routingProfile != "premium" && baselineCost > 0 {
		savings = math.Max(0, (baselineCost-costEstimate)/baselineCost)
	}

	return RoutingDecision{
		Model:        model,
		Tier:         tier,
		Confidence:   confidence,
		Method:       method,
		Reasoning:    reasoning,
		CostEstimate: costEstimate,
		BaselineCost: baselineCost,
		Savings:      savings,
		AgenticScore: agenticScore,
	}
}

// GetFallbackChain returns [primary, ...fallbacks] for a tier.
func GetFallbackChain(tier Tier, tierConfigs map[Tier]TierConfig) []string {
	tc := tierConfigs[tier]
	chain := make([]string, 0, 1+len(tc.Fallback))
	chain = append(chain, tc.Primary)
	chain = append(chain, tc.Fallback...)
	return chain
}

// CalculateModelCost computes cost for a specific model (used for fallback models).
func CalculateModelCost(
	model string,
	modelPricing map[string]ModelPricing,
	estimatedInputTokens int,
	maxOutputTokens int,
	routingProfile string,
) (costEstimate, baselineCost, savings float64) {
	pricing, hasPricing := modelPricing[model]

	if hasPricing && pricing.FlatPrice != nil {
		costEstimate = math.Max(*pricing.FlatPrice*(1+serverMarginPercent/100.0), minPaymentUSD)
	} else {
		var inputPrice, outputPrice float64
		if hasPricing {
			inputPrice = pricing.InputPrice
			outputPrice = pricing.OutputPrice
		}
		raw := float64(estimatedInputTokens)/1_000_000*inputPrice +
			float64(maxOutputTokens)/1_000_000*outputPrice
		costEstimate = math.Max(raw*(1+serverMarginPercent/100.0), minPaymentUSD)
	}

	opusPricing, hasOpus := modelPricing[baselineModelID]
	opusIn := baselineInputPrice
	opusOut := baselineOutputPrice
	if hasOpus {
		opusIn = opusPricing.InputPrice
		opusOut = opusPricing.OutputPrice
	}
	baselineCost = float64(estimatedInputTokens)/1_000_000*opusIn +
		float64(maxOutputTokens)/1_000_000*opusOut

	if routingProfile != "premium" && baselineCost > 0 {
		savings = math.Max(0, (baselineCost-costEstimate)/baselineCost)
	}
	return
}

// GetFallbackChainFiltered returns models that can handle the estimated context.
func GetFallbackChainFiltered(
	tier Tier,
	tierConfigs map[Tier]TierConfig,
	estimatedTotalTokens int,
	getContextWindow func(modelID string) (int, bool),
) []string {
	fullChain := GetFallbackChain(tier, tierConfigs)

	var filtered []string
	for _, modelID := range fullChain {
		ctxWindow, ok := getContextWindow(modelID)
		if !ok {
			// Unknown model - include it
			filtered = append(filtered, modelID)
			continue
		}
		// Add 10% buffer
		if ctxWindow >= int(float64(estimatedTotalTokens)*1.1) {
			filtered = append(filtered, modelID)
		}
	}

	if len(filtered) == 0 {
		return fullChain
	}
	return filtered
}

// FilterByToolCalling filters models to those supporting tool calling.
func FilterByToolCalling(models []string, hasTools bool, supportsToolCalling func(string) bool) []string {
	if !hasTools {
		return models
	}
	var filtered []string
	for _, m := range models {
		if supportsToolCalling(m) {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return models
	}
	return filtered
}

// FilterByVision filters models to those supporting vision inputs.
func FilterByVision(models []string, hasVision bool, supportsVision func(string) bool) []string {
	if !hasVision {
		return models
	}
	var filtered []string
	for _, m := range models {
		if supportsVision(m) {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return models
	}
	return filtered
}

// FilterByExcludeList removes user-excluded models from the list.
func FilterByExcludeList(models []string, excludeList map[string]bool) []string {
	if len(excludeList) == 0 {
		return models
	}
	var filtered []string
	for _, m := range models {
		if !excludeList[m] {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return models
	}
	return filtered
}
