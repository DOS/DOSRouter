package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const classifierPrompt = `You are a query complexity classifier. Classify the user's query into exactly one category.

Categories:
- SIMPLE: Factual Q&A, definitions, translations, short answers
- MEDIUM: Summaries, explanations, moderate code generation
- COMPLEX: Multi-step code, system design, creative writing, analysis
- REASONING: Mathematical proofs, formal logic, step-by-step problem solving

Respond with ONLY one word: SIMPLE, MEDIUM, COMPLEX, or REASONING.`

// LLMClassifierConfig controls the LLM fallback classifier behavior.
type LLMClassifierConfig struct {
	Model           string
	MaxTokens       int
	Temperature     float64
	TruncationChars int
	CacheTTLMs      int64
}

type cacheEntry struct {
	tier    Tier
	expires time.Time
}

var (
	llmCache   = make(map[string]cacheEntry)
	llmCacheMu sync.RWMutex
)

// ClassifyByLLM classifies a prompt using a cheap LLM.
// Returns tier and confidence. Defaults to MEDIUM on any failure.
func ClassifyByLLM(prompt string, config LLMClassifierConfig, apiBase string, httpClient *http.Client) (Tier, float64) {
	truncated := prompt
	if len(truncated) > config.TruncationChars {
		truncated = truncated[:config.TruncationChars]
	}

	// Check cache
	cacheKey := simpleHash(truncated)
	llmCacheMu.RLock()
	if entry, ok := llmCache[cacheKey]; ok && time.Now().Before(entry.expires) {
		llmCacheMu.RUnlock()
		return entry.tier, 0.75
	}
	llmCacheMu.RUnlock()

	// Build request
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqBody := struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
		Stream      bool      `json:"stream"`
	}{
		Model: config.Model,
		Messages: []message{
			{Role: "system", Content: classifierPrompt},
			{Role: "user", Content: truncated},
		},
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
		Stream:      false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return TierMedium, 0.5
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Post(
		fmt.Sprintf("%s/v1/chat/completions", apiBase),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return TierMedium, 0.5
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TierMedium, 0.5
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return TierMedium, 0.5
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return TierMedium, 0.5
	}

	content := ""
	if len(result.Choices) > 0 {
		content = strings.TrimSpace(strings.ToUpper(result.Choices[0].Message.Content))
	}

	tier := parseTier(content)

	// Cache result
	llmCacheMu.Lock()
	llmCache[cacheKey] = cacheEntry{
		tier:    tier,
		expires: time.Now().Add(time.Duration(config.CacheTTLMs) * time.Millisecond),
	}
	if len(llmCache) > 1000 {
		pruneCache()
	}
	llmCacheMu.Unlock()

	return tier, 0.75
}

var (
	reReasoning = regexp.MustCompile(`\bREASONING\b`)
	reComplex   = regexp.MustCompile(`\bCOMPLEX\b`)
	reMedium    = regexp.MustCompile(`\bMEDIUM\b`)
	reSimple    = regexp.MustCompile(`\bSIMPLE\b`)
)

func parseTier(text string) Tier {
	if reReasoning.MatchString(text) {
		return TierReasoning
	}
	if reComplex.MatchString(text) {
		return TierComplex
	}
	if reMedium.MatchString(text) {
		return TierMedium
	}
	if reSimple.MatchString(text) {
		return TierSimple
	}
	return TierMedium
}

func simpleHash(s string) string {
	var hash int32
	for _, c := range s {
		hash = (hash << 5) - hash + c
	}
	return fmt.Sprintf("%x", uint32(hash))
}

func pruneCache() {
	now := time.Now()
	for key, entry := range llmCache {
		if now.After(entry.expires) {
			delete(llmCache, key)
		}
	}
}
