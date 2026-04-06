// Package router provides smart LLM request routing with 15-dimension
// weighted scoring and sigmoid confidence calibration.
//
// Entry point: Route() classifies a request and returns which model to use.
package router

// Route classifies a request and returns the cheapest capable model.
// Delegates to the registered "rules" strategy by default.
func Route(prompt string, systemPrompt string, maxOutputTokens int, options RouterOptions) (RoutingDecision, error) {
	strategy, err := GetStrategy("rules")
	if err != nil {
		return RoutingDecision{}, err
	}
	return strategy.Route(prompt, systemPrompt, maxOutputTokens, options), nil
}
