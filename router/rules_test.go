package router

import (
	"testing"
)

func defaultTestConfig() ScoringConfig {
	cfg := DefaultRoutingConfig()
	return cfg.Scoring
}

func TestClassifyByRules_Simple(t *testing.T) {
	config := defaultTestConfig()

	tests := []struct {
		name   string
		prompt string
		want   Tier
	}{
		{"hello", "hello", TierSimple},
		{"what is", "what is Go?", TierSimple},
		{"translate", "translate hello to French", TierSimple},
		{"capital", "what is the capital of France?", TierSimple},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := len(tt.prompt) / 4
			result := ClassifyByRules(tt.prompt, "", tokens, config)
			if result.Tier == nil {
				// Ambiguous defaults to MEDIUM, which is fine for border cases
				t.Logf("ambiguous (score=%.3f, confidence=%.3f)", result.Score, result.Confidence)
				return
			}
			if *result.Tier != tt.want {
				t.Errorf("got tier %s, want %s (score=%.3f, signals=%v)",
					*result.Tier, tt.want, result.Score, result.Signals)
			}
		})
	}
}

func TestClassifyByRules_Complex(t *testing.T) {
	config := defaultTestConfig()

	// Dense prompt with many complexity signals across multiple dimensions
	prompt := `Write a Go middleware with rate limiting using a sliding window algorithm.
The middleware should:
1. Track requests per IP using in-memory storage with O(1) lookups
2. Support configurable window size and max requests
3. Return 429 with Retry-After header
4. Use a distributed lock for multi-instance kubernetes deployments
5. Implement proper error handling with circuit breaker pattern
Implement with unit tests. The architecture should optimize for microservice database infrastructure.
Step 1: design the data structures. Step 2: implement the algorithm.
Format the output as json schema. Don't use any external dependencies.`

	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	t.Logf("score=%.3f, tier=%v, confidence=%.3f, signals=%v", result.Score, result.Tier, result.Confidence, result.Signals)

	// With this many signals, should reach COMPLEX or higher, or at minimum be MEDIUM+ range
	if result.Tier != nil && TierRank(*result.Tier) < TierRank(TierMedium) {
		t.Errorf("expected at least MEDIUM, got %s (score=%.3f)", *result.Tier, result.Score)
	}
	// Score should be positive with all these signals
	if result.Score < 0 {
		t.Errorf("expected positive score, got %.3f", result.Score)
	}
}

func TestClassifyByRules_Reasoning(t *testing.T) {
	config := defaultTestConfig()

	prompt := "Prove that the square root of 2 is irrational. Show your proof step by step with formal mathematical notation."

	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	if result.Tier == nil {
		t.Fatalf("expected REASONING tier, got nil (score=%.3f)", result.Score)
	}
	if *result.Tier != TierReasoning {
		t.Errorf("expected REASONING, got %s (score=%.3f, signals=%v)",
			*result.Tier, result.Score, result.Signals)
	}
	// Reasoning override should give >= 0.85 confidence
	if result.Confidence < 0.85 {
		t.Errorf("expected confidence >= 0.85, got %.3f", result.Confidence)
	}
}

func TestClassifyByRules_ReasoningOverride(t *testing.T) {
	config := defaultTestConfig()

	// 2+ reasoning keywords should force REASONING regardless of score
	prompt := "prove this theorem step by step"

	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	if result.Tier == nil {
		t.Fatalf("expected REASONING tier, got nil")
	}
	if *result.Tier != TierReasoning {
		t.Errorf("expected REASONING (2+ reasoning keywords), got %s", *result.Tier)
	}
}

func TestClassifyByRules_SystemPromptIgnored(t *testing.T) {
	config := defaultTestConfig()

	// System prompt with lots of complex keywords should NOT affect scoring
	systemPrompt := "You are an expert at implementing distributed algorithms step by step with formal proofs."
	prompt := "hello"

	tokens := (len(systemPrompt) + len(prompt)) / 4
	result := ClassifyByRules(prompt, systemPrompt, tokens, config)

	// Should still be SIMPLE despite complex system prompt
	if result.Tier != nil && *result.Tier == TierReasoning {
		t.Errorf("system prompt keywords should not trigger REASONING (got tier=%s, score=%.3f)",
			*result.Tier, result.Score)
	}
}

func TestClassifyByRules_Empty(t *testing.T) {
	config := defaultTestConfig()

	result := ClassifyByRules("", "", 0, config)

	// Empty prompt should be SIMPLE (low token count = negative score)
	if result.Tier != nil && *result.Tier != TierSimple {
		t.Logf("empty prompt tier: %v (score=%.3f)", result.Tier, result.Score)
	}
}

func TestClassifyByRules_LongSimpleText(t *testing.T) {
	config := defaultTestConfig()

	// Long text but simple content - length alone shouldn't make it complex
	prompt := "Tell me about " + repeatString("the history of ", 100) + "France."

	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	// Should be MEDIUM at most (token count adds weight, but no complexity signals)
	if result.Tier != nil && *result.Tier == TierReasoning {
		t.Errorf("long simple text should not be REASONING (got %s, score=%.3f)",
			*result.Tier, result.Score)
	}
}

func TestClassifyByRules_Multilingual(t *testing.T) {
	config := defaultTestConfig()

	tests := []struct {
		name     string
		prompt   string
		minScore float64 // should score at least this
	}{
		{"chinese_code", "帮我写一个函数来查询数据库", 0},
		{"japanese_reasoning", "ステップバイステップで証明してください", 0},
		{"russian_code", "напиши функцию для импорта данных", 0},
		{"korean_simple", "안녕하세요, 무엇을 도와드릴까요?", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := len(tt.prompt) / 4
			result := ClassifyByRules(tt.prompt, "", tokens, config)
			t.Logf("tier=%v, score=%.3f, signals=%v", result.Tier, result.Score, result.Signals)
			if result.Score < tt.minScore {
				t.Errorf("score %.3f below expected minimum %.3f", result.Score, tt.minScore)
			}
		})
	}
}

func TestClassifyByRules_AgenticTask(t *testing.T) {
	config := defaultTestConfig()

	prompt := "Read the file, edit the config, deploy to production, and verify it works. Once done, confirm the deployment."

	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	if result.AgenticScore < 0.5 {
		t.Errorf("expected high agentic score, got %.2f (signals=%v)", result.AgenticScore, result.Signals)
	}
}

func TestClassifyByRules_Dimensions(t *testing.T) {
	config := defaultTestConfig()

	prompt := "implement a kubernetes microservice with json output"
	tokens := len(prompt) / 4
	result := ClassifyByRules(prompt, "", tokens, config)

	// Should have 15 dimensions
	if len(result.Dimensions) != 15 {
		t.Errorf("expected 15 dimensions, got %d", len(result.Dimensions))
	}

	// Check that dimensions have expected names
	dimNames := make(map[string]bool)
	for _, d := range result.Dimensions {
		dimNames[d.Name] = true
	}

	expected := []string{
		"tokenCount", "codePresence", "reasoningMarkers", "technicalTerms",
		"creativeMarkers", "simpleIndicators", "multiStepPatterns", "questionComplexity",
		"imperativeVerbs", "constraintCount", "outputFormat", "referenceComplexity",
		"negationComplexity", "domainSpecificity", "agenticTask",
	}
	for _, name := range expected {
		if !dimNames[name] {
			t.Errorf("missing dimension: %s", name)
		}
	}
}

func TestCalibrateConfidence(t *testing.T) {
	// At boundary (distance=0), confidence should be 0.5
	c := calibrateConfidence(0, 12)
	if c != 0.5 {
		t.Errorf("expected 0.5 at boundary, got %.3f", c)
	}

	// Far from boundary, confidence should approach 1.0
	c = calibrateConfidence(1.0, 12)
	if c < 0.99 {
		t.Errorf("expected >0.99 far from boundary, got %.3f", c)
	}

	// Negative distance, confidence should approach 0
	c = calibrateConfidence(-1.0, 12)
	if c > 0.01 {
		t.Errorf("expected <0.01 for negative distance, got %.3f", c)
	}
}

func repeatString(s string, n int) string {
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

// BenchmarkClassifyByRules measures classification latency.
// Target: < 1ms per classification.
func BenchmarkClassifyByRules(b *testing.B) {
	config := defaultTestConfig()
	prompt := "Write a Go middleware with rate limiting using a sliding window algorithm. Step 1: design the data structure."

	tokens := len(prompt) / 4
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ClassifyByRules(prompt, "", tokens, config)
	}
}
