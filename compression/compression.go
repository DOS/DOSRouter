package compression

import "strings"

const (
	// ShouldCompressThreshold is the minimum total character count across
	// all messages before compression is triggered.
	ShouldCompressThreshold = 5000
)

// ShouldCompress returns true if the total text content across all messages
// exceeds ShouldCompressThreshold characters.
func ShouldCompress(messages []NormalizedMessage) bool {
	total := 0
	for _, msg := range messages {
		total += len(msg.GetTextContent())
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Arguments)
		}
		if total > ShouldCompressThreshold {
			return true
		}
	}
	return false
}

// CompressContext applies all enabled compression layers to a slice of
// messages according to the provided config and returns a CompressionResult.
//
// Layer execution order:
//  1. Deduplication (remove duplicate assistant messages)
//  2. Whitespace normalization
//  3. JSON compaction (tool args + tool results)
//  4. Observation compression (truncate large tool results)
//  5. Dictionary compression (static codebook substitution)
//  6. Path compression (common path prefix replacement)
//  7. Dynamic codebook (frequency-based phrase compression)
//
// After all layers run, a codebook header is prepended to the first user
// message so the model can decode any short codes.
func CompressContext(messages []NormalizedMessage, config CompressionConfig) CompressionResult {
	// Deep copy messages so we don't mutate the caller's data.
	msgs := copyMessages(messages)

	// Measure original size.
	originalChars := totalChars(msgs)

	stats := CompressionStats{
		OriginalChars: originalChars,
		LayerSavings:  make(map[string]int),
	}

	// Layer 1: Deduplication
	if config.Dedup {
		var removed int
		msgs, removed = DeduplicateMessages(msgs)
		stats.MessagesRemoved = removed
		savings := originalChars - totalChars(msgs)
		stats.LayerSavings["dedup"] = savings
	}

	// Layer 2: Whitespace
	if config.Whitespace {
		saved := CompressWhitespace(msgs)
		stats.LayerSavings["whitespace"] = saved
	}

	// Layer 3: JSON Compact (run before observation so tool content is clean)
	if config.JSONCompact {
		saved := CompressJSON(msgs)
		stats.LayerSavings["jsonCompact"] = saved
	}

	// Layer 4: Observation compression
	if config.Observation {
		saved := CompressObservations(msgs)
		stats.LayerSavings["observation"] = saved
	}

	// Layer 5: Dictionary compression
	var dictResult DictionaryResult
	if config.Dictionary {
		dictResult = CompressDictionary(msgs)
		stats.LayerSavings["dictionary"] = dictResult.CharsSaved
		stats.DictCodesUsed = dictResult.UsedCodes
	}

	// Layer 6: Path compression
	if config.Paths {
		pathResult := CompressPaths(msgs)
		stats.LayerSavings["paths"] = pathResult.CharsSaved
		stats.PathMappings = pathResult.Mappings
	}

	// Layer 7: Dynamic codebook
	var dynamicResult DynamicCodebookResult
	if config.DynamicCodebook {
		dynamicResult = CompressDynamicCodebook(msgs)
		stats.LayerSavings["dynamicCodebook"] = dynamicResult.CharsSaved
	}

	// Compute final stats.
	compressedChars := totalChars(msgs)
	stats.CompressedChars = compressedChars
	if originalChars > 0 {
		stats.Ratio = float64(compressedChars) / float64(originalChars)
	}

	// Generate codebook header if any codes were used.
	header := ""
	if len(dictResult.UsedCodes) > 0 || len(dynamicResult.Codes) > 0 || len(stats.PathMappings) > 0 {
		extraCodes := make(map[string]string)
		// Add dynamic codes.
		for code, phrase := range dynamicResult.Codes {
			extraCodes[code] = phrase
		}
		// Add path mappings.
		for path, varName := range stats.PathMappings {
			extraCodes[varName] = path
		}
		header = GenerateCodebookHeader(dictResult.UsedCodes, extraCodes)
	}

	return CompressionResult{
		Messages: msgs,
		Stats:    stats,
		Header:   header,
	}
}

// PrependCodebookHeader inserts a codebook header before the content of
// the first user message. This allows the model to decode compressed codes.
func PrependCodebookHeader(messages []NormalizedMessage, header string) []NormalizedMessage {
	if header == "" {
		return messages
	}

	for i := range messages {
		if messages[i].Role == "user" {
			current := messages[i].GetTextContent()
			messages[i].SetTextContent(header + "\n" + current)
			break
		}
	}

	return messages
}

// totalChars sums the character count of all text content and tool_call
// arguments across all messages.
func totalChars(messages []NormalizedMessage) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.GetTextContent())
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Arguments)
		}
	}
	return total
}

// copyMessages creates a deep copy of a message slice.
func copyMessages(messages []NormalizedMessage) []NormalizedMessage {
	result := make([]NormalizedMessage, len(messages))
	for i, msg := range messages {
		result[i] = NormalizedMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
			Name:       msg.Name,
		}

		// Deep copy Parts.
		if len(msg.Parts) > 0 {
			result[i].Parts = make([]ContentPart, len(msg.Parts))
			for j, p := range msg.Parts {
				result[i].Parts[j] = ContentPart{
					Type: p.Type,
					Text: p.Text,
				}
				if p.ImageURL != nil {
					result[i].Parts[j].ImageURL = make(map[string]interface{})
					for k, v := range p.ImageURL {
						result[i].Parts[j].ImageURL[k] = v
					}
				}
			}
		}

		// Deep copy ToolCalls.
		if len(msg.ToolCalls) > 0 {
			result[i].ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			copy(result[i].ToolCalls, msg.ToolCalls)
		}
	}
	return result
}

// init registers a blank import guard so that unused imports in consuming
// packages trigger compile errors. This is a no-op at runtime.
func init() {
	// Ensure strings is used (it is used above, but this silences any
	// potential linter noise).
	_ = strings.Builder{}
}
