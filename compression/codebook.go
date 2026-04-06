package compression

import (
	"fmt"
	"sort"
	"strings"
)

// STATIC_CODEBOOK maps short codes to their expanded phrases. These codes
// are injected into compressed content and decoded by the receiving end.
var STATIC_CODEBOOK = map[string]string{
	// OpenClaw / agent system
	"$OC01": "openclaw",
	"$OC02": "tool_call",
	"$OC03": "function",
	"$OC04": "arguments",
	"$OC05": "assistant",
	"$OC06": "observation",
	"$OC07": "tool_result",
	"$OC08": "system_prompt",
	"$OC09": "user_message",
	"$OC10": "thought_process",

	// Skills
	"$SK01": "web_search",
	"$SK02": "code_execution",
	"$SK03": "file_operation",
	"$SK04": "knowledge_base",

	// Tool-related
	"$T01": "parameters",
	"$T02": "description",
	"$T03": "required",
	"$T04": "properties",
	"$T05": "string",
	"$T06": "boolean",
	"$T07": "integer",

	// Data types
	"$D01": "undefined",
	"$D02": "null",

	// Instructions / prompts
	"$I01": "instructions",
	"$I02": "conversation",
	"$I03": "context_window",
	"$I04": "token_count",
	"$I05": "max_tokens",

	// Status / state
	"$S01": "successfully",
	"$S02": "completed",
	"$S03": "error_message",

	// JSON / schema
	"$J01": "json_schema",
	"$J02": "object",
	"$J03": "array",

	// HTTP
	"$H01": "request",
	"$H02": "response",

	// Results
	"$R01": "result",
	"$R02": "content",
	"$R03": "message",
	"$R04": "metadata",

	// Errors
	"$E01": "exception",
	"$E02": "traceback",
	"$E03": "error",
	"$E04": "warning",

	// Misc common
	"$M01": "information",
	"$M02": "configuration",
	"$M03": "environment",
	"$M04": "application",
	"$M05": "implementation",
}

// inverseCodebook is lazily built on first access.
var inverseCodebook map[string]string

// GetInverseCodebook returns a map from phrase -> code.
func GetInverseCodebook() map[string]string {
	if inverseCodebook != nil {
		return inverseCodebook
	}
	inverseCodebook = make(map[string]string, len(STATIC_CODEBOOK))
	for code, phrase := range STATIC_CODEBOOK {
		inverseCodebook[phrase] = code
	}
	return inverseCodebook
}

// GenerateCodebookHeader produces a human-readable header listing all codes
// that were actually used during compression plus any extra codes.
func GenerateCodebookHeader(usedCodes []string, extraCodes map[string]string) string {
	if len(usedCodes) == 0 && len(extraCodes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("[Codebook]\n")

	// Sort used static codes for deterministic output.
	sort.Strings(usedCodes)
	for _, code := range usedCodes {
		if phrase, ok := STATIC_CODEBOOK[code]; ok {
			fmt.Fprintf(&b, "%s=%s\n", code, phrase)
		}
	}

	// Append dynamic / extra codes.
	if len(extraCodes) > 0 {
		keys := make([]string, 0, len(extraCodes))
		for k := range extraCodes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, code := range keys {
			fmt.Fprintf(&b, "%s=%s\n", code, extraCodes[code])
		}
	}

	b.WriteString("[/Codebook]\n")
	return b.String()
}

// DecompressContent replaces all codebook tokens in text with their
// expanded phrases. It handles both static and provided extra codes.
func DecompressContent(text string, extraCodes map[string]string) string {
	// Build a combined map for replacement.
	all := make(map[string]string, len(STATIC_CODEBOOK)+len(extraCodes))
	for k, v := range STATIC_CODEBOOK {
		all[k] = v
	}
	for k, v := range extraCodes {
		all[k] = v
	}

	// Sort codes by length descending so longer codes are replaced first
	// (e.g. $OC10 before $OC1).
	codes := make([]string, 0, len(all))
	for k := range all {
		codes = append(codes, k)
	}
	sort.Slice(codes, func(i, j int) bool {
		return len(codes[i]) > len(codes[j])
	})

	result := text
	for _, code := range codes {
		result = strings.ReplaceAll(result, code, all[code])
	}
	return result
}
