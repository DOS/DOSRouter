// Package router implements a smart LLM request router with 15-dimension
// weighted scoring, sigmoid confidence calibration, and pluggable strategies.
package router

// Tier represents the complexity classification of a request.
// REASONING is distinct from COMPLEX because reasoning tasks need
// different models (o3, gemini-pro) than general complex tasks (gpt-4o, sonnet-4).
type Tier string

const (
	TierSimple    Tier = "SIMPLE"
	TierMedium    Tier = "MEDIUM"
	TierComplex   Tier = "COMPLEX"
	TierReasoning Tier = "REASONING"
)

// TierRank returns the numeric rank of a tier for comparison.
func TierRank(t Tier) int {
	switch t {
	case TierSimple:
		return 0
	case TierMedium:
		return 1
	case TierComplex:
		return 2
	case TierReasoning:
		return 3
	default:
		return 1
	}
}

// ScoringResult holds the output of the rule-based classifier.
type ScoringResult struct {
	Score        float64          // weighted float (roughly [-0.3, 0.4])
	Tier         *Tier            // nil = ambiguous, needs fallback classifier
	Confidence   float64          // sigmoid-calibrated [0, 1]
	Signals      []string         // human-readable signal descriptions
	AgenticScore float64          // 0-1 agentic task score
	Dimensions   []DimensionScore // per-dimension breakdown
}

// DimensionScore is a single dimension's contribution to the overall score.
type DimensionScore struct {
	Name   string  // dimension identifier
	Score  float64 // score in [-1, 1]
	Signal string  // human-readable signal, empty if no signal
}

// RoutingDecision is the final output: which model to use and why.
type RoutingDecision struct {
	Model        string                // selected model ID
	Tier         Tier                  // classified tier
	Confidence   float64               // confidence in classification
	Method       string                // "rules" or "llm"
	Reasoning    string                // human-readable reasoning
	CostEstimate float64               // estimated cost in USD
	BaselineCost float64               // what Claude Opus would cost
	Savings      float64               // 0-1 percentage saved vs baseline
	AgenticScore float64               // 0-1 agentic task score
	TierConfigs  map[Tier]TierConfig   // tier configs used for this decision
	Profile      string                // "auto", "eco", "premium", "agentic"
}

// TierConfig maps a tier to its primary model and fallback chain.
type TierConfig struct {
	Primary  string   `json:"primary"`
	Fallback []string `json:"fallback"`
}

// RouterOptions carries configuration for a single routing call.
type RouterOptions struct {
	Config         RoutingConfig
	ModelPricing   map[string]ModelPricing
	RoutingProfile string // "eco", "auto", "premium"
	HasTools       bool
	Now            *int64 // unix millis override for promotion window checks (testing)
}

// ModelPricing holds per-model pricing info.
type ModelPricing struct {
	InputPrice  float64  // per 1M tokens
	OutputPrice float64  // per 1M tokens
	FlatPrice   *float64 // promo flat price per request (overrides token pricing)
}

// RoutingConfig is the top-level configuration.
type RoutingConfig struct {
	Version      string                  `json:"version"`
	Classifier   ClassifierConfig        `json:"classifier"`
	Scoring      ScoringConfig           `json:"scoring"`
	Tiers        map[Tier]TierConfig     `json:"tiers"`
	AgenticTiers map[Tier]TierConfig     `json:"agenticTiers,omitempty"`
	EcoTiers     map[Tier]TierConfig     `json:"ecoTiers,omitempty"`
	PremiumTiers map[Tier]TierConfig     `json:"premiumTiers,omitempty"`
	Promotions   []Promotion             `json:"promotions,omitempty"`
	Overrides    OverridesConfig         `json:"overrides"`
}

// ScoringConfig controls the weighted dimension scorer.
type ScoringConfig struct {
	TokenCountThresholds struct {
		Simple  int `json:"simple"`
		Complex int `json:"complex"`
	} `json:"tokenCountThresholds"`

	CodeKeywords           []string `json:"codeKeywords"`
	ReasoningKeywords      []string `json:"reasoningKeywords"`
	SimpleKeywords         []string `json:"simpleKeywords"`
	TechnicalKeywords      []string `json:"technicalKeywords"`
	CreativeKeywords       []string `json:"creativeKeywords"`
	ImperativeVerbs        []string `json:"imperativeVerbs"`
	ConstraintIndicators   []string `json:"constraintIndicators"`
	OutputFormatKeywords   []string `json:"outputFormatKeywords"`
	ReferenceKeywords      []string `json:"referenceKeywords"`
	NegationKeywords       []string `json:"negationKeywords"`
	DomainSpecificKeywords []string `json:"domainSpecificKeywords"`
	AgenticTaskKeywords    []string `json:"agenticTaskKeywords"`

	DimensionWeights map[string]float64 `json:"dimensionWeights"`

	TierBoundaries struct {
		SimpleMedium     float64 `json:"simpleMedium"`
		MediumComplex    float64 `json:"mediumComplex"`
		ComplexReasoning float64 `json:"complexReasoning"`
	} `json:"tierBoundaries"`

	ConfidenceSteepness  float64 `json:"confidenceSteepness"`
	ConfidenceThreshold  float64 `json:"confidenceThreshold"`
}

// ClassifierConfig controls the LLM fallback classifier.
type ClassifierConfig struct {
	LLMModel             string  `json:"llmModel"`
	LLMMaxTokens         int     `json:"llmMaxTokens"`
	LLMTemperature       float64 `json:"llmTemperature"`
	PromptTruncationChars int    `json:"promptTruncationChars"`
	CacheTTLMs           int64   `json:"cacheTtlMs"`
}

// OverridesConfig holds override rules.
type OverridesConfig struct {
	MaxTokensForceComplex   int  `json:"maxTokensForceComplex"`
	StructuredOutputMinTier Tier `json:"structuredOutputMinTier"`
	AmbiguousDefaultTier    Tier `json:"ambiguousDefaultTier"`
	AgenticMode             *bool `json:"agenticMode,omitempty"`
}

// Promotion is a time-windowed tier override.
type Promotion struct {
	Name          string                           `json:"name"`
	StartDate     string                           `json:"startDate"`
	EndDate       string                           `json:"endDate"`
	TierOverrides map[Tier]PartialTierConfig       `json:"tierOverrides"`
	Profiles      []string                         `json:"profiles,omitempty"`
}

// PartialTierConfig allows partial override of a TierConfig.
type PartialTierConfig struct {
	Primary  string   `json:"primary,omitempty"`
	Fallback []string `json:"fallback,omitempty"`
}

// RouterStrategy is the interface for pluggable routing strategies.
type RouterStrategy interface {
	Name() string
	Route(prompt string, systemPrompt string, maxOutputTokens int, options RouterOptions) RoutingDecision
}
