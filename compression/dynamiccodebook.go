package compression

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	// MinDynamicFrequency is the minimum number of times a phrase must
	// appear to be considered for the dynamic codebook.
	MinDynamicFrequency = 3
	// MinDynamicPhraseLen is the minimum character length for a phrase
	// to be eligible for dynamic compression.
	MinDynamicPhraseLen = 8
	// MaxDynamicCodes is the maximum number of dynamic codes to generate.
	MaxDynamicCodes = 20
	// DynamicCodePrefix is the prefix for dynamically generated codes.
	DynamicCodePrefix = "$X"
)

// DynamicCodebookResult holds the outcome of dynamic codebook compression.
type DynamicCodebookResult struct {
	Codes      map[string]string // code -> phrase
	CharsSaved int
}

// CompressDynamicCodebook analyzes content across all messages to find
// frequently repeated phrases, assigns short dynamic codes ($X01-$X20),
// and replaces them in the content.
//
// It modifies messages in-place and returns the generated codebook and
// characters saved.
func CompressDynamicCodebook(messages []NormalizedMessage) DynamicCodebookResult {
	// Step 1: Gather all text content.
	var allText strings.Builder
	for _, msg := range messages {
		allText.WriteString(msg.GetTextContent())
		allText.WriteString(" ")
		for _, tc := range msg.ToolCalls {
			allText.WriteString(tc.Function.Arguments)
			allText.WriteString(" ")
		}
	}

	corpus := allText.String()

	// Step 2: Extract candidate phrases via word n-grams (2-4 words).
	phrases := extractFrequentPhrases(corpus)

	if len(phrases) == 0 {
		return DynamicCodebookResult{Codes: nil, CharsSaved: 0}
	}

	// Step 3: Assign codes to top phrases by savings.
	codes := make(map[string]string) // code -> phrase
	phraseToCode := make(map[string]string)
	limit := MaxDynamicCodes
	if len(phrases) < limit {
		limit = len(phrases)
	}

	for i := 0; i < limit; i++ {
		code := fmt.Sprintf("%s%02d", DynamicCodePrefix, i+1)
		codes[code] = phrases[i].text
		phraseToCode[phrases[i].text] = code
	}

	// Step 4: Replace phrases in messages (longest first).
	sortedPhrases := make([]string, 0, len(phraseToCode))
	for phrase := range phraseToCode {
		sortedPhrases = append(sortedPhrases, phrase)
	}
	sort.Slice(sortedPhrases, func(i, j int) bool {
		return len(sortedPhrases[i]) > len(sortedPhrases[j])
	})

	totalSaved := 0

	replaceInText := func(text string) string {
		result := text
		for _, phrase := range sortedPhrases {
			code := phraseToCode[phrase]
			count := strings.Count(result, phrase)
			if count > 0 {
				saved := count * (len(phrase) - len(code))
				if saved > 0 {
					totalSaved += saved
					result = strings.ReplaceAll(result, phrase, code)
				}
			}
		}
		return result
	}

	for i := range messages {
		original := messages[i].GetTextContent()
		compressed := replaceInText(original)
		if compressed != original {
			messages[i].SetTextContent(compressed)
		}

		for j := range messages[i].ToolCalls {
			origArgs := messages[i].ToolCalls[j].Function.Arguments
			compArgs := replaceInText(origArgs)
			if compArgs != origArgs {
				messages[i].ToolCalls[j].Function.Arguments = compArgs
			}
		}
	}

	return DynamicCodebookResult{
		Codes:      codes,
		CharsSaved: totalSaved,
	}
}

// phraseCandidate pairs a phrase with its frequency and estimated savings.
type phraseCandidate struct {
	text    string
	count   int
	savings int
}

// extractFrequentPhrases tokenizes the corpus and finds n-grams (2-4 words)
// that appear at least MinDynamicFrequency times and are long enough.
// Returns candidates sorted by savings descending.
func extractFrequentPhrases(corpus string) []phraseCandidate {
	words := tokenize(corpus)
	if len(words) < 2 {
		return nil
	}

	freq := make(map[string]int)

	// Generate 2-gram, 3-gram, and 4-gram phrases.
	for n := 2; n <= 4; n++ {
		for i := 0; i <= len(words)-n; i++ {
			phrase := strings.Join(words[i:i+n], " ")
			if len(phrase) >= MinDynamicPhraseLen {
				freq[phrase]++
			}
		}
	}

	// Filter by minimum frequency and compute savings.
	var candidates []phraseCandidate
	for phrase, count := range freq {
		if count < MinDynamicFrequency {
			continue
		}
		// Estimate code length as 4 chars ($X01).
		savings := count * (len(phrase) - 4)
		if savings <= 0 {
			continue
		}
		// Skip if the phrase is a substring of a static codebook phrase.
		if _, exists := GetInverseCodebook()[phrase]; exists {
			continue
		}
		candidates = append(candidates, phraseCandidate{
			text:    phrase,
			count:   count,
			savings: savings,
		})
	}

	// Sort by savings descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].savings > candidates[j].savings
	})

	// Remove overlapping phrases: if a shorter phrase is a substring of
	// a longer one already selected, skip it.
	var filtered []phraseCandidate
	for _, c := range candidates {
		overlaps := false
		for _, f := range filtered {
			if strings.Contains(f.text, c.text) || strings.Contains(c.text, f.text) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// tokenize splits text into lowercase words, stripping punctuation.
func tokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}
