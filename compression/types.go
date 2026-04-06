package compression

// ContentPartType identifies the kind of content in a multi-part message.
type ContentPartType string

const (
	ContentPartTypeText     ContentPartType = "text"
	ContentPartTypeImageURL ContentPartType = "image_url"
)

// ContentPart represents a single part of a multi-part message content.
type ContentPart struct {
	Type     ContentPartType        `json:"type"`
	Text     string                 `json:"text,omitempty"`
	ImageURL map[string]interface{} `json:"image_url,omitempty"`
}

// ToolCallFunction holds the function name and arguments for a tool call.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a tool invocation within an assistant message.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// NormalizedMessage is the unified message format used throughout the
// compression pipeline. Content can be either a plain string or a slice
// of ContentPart for multi-modal messages.
type NormalizedMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Parts      []ContentPart `json:"parts,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	Name       string        `json:"name,omitempty"`
}

// GetTextContent returns the text content of the message. For multi-part
// messages it concatenates all text parts.
func (m *NormalizedMessage) GetTextContent() string {
	if m.Content != "" {
		return m.Content
	}
	var text string
	for _, p := range m.Parts {
		if p.Type == ContentPartTypeText {
			text += p.Text
		}
	}
	return text
}

// SetTextContent sets the text content. If the message uses Parts, it
// updates the first text part; otherwise it sets Content directly.
func (m *NormalizedMessage) SetTextContent(text string) {
	if len(m.Parts) > 0 {
		for i := range m.Parts {
			if m.Parts[i].Type == ContentPartTypeText {
				m.Parts[i].Text = text
				return
			}
		}
		// No text part found - prepend one.
		m.Parts = append([]ContentPart{{Type: ContentPartTypeText, Text: text}}, m.Parts...)
		return
	}
	m.Content = text
}

// CompressionConfig controls which compression layers are applied.
type CompressionConfig struct {
	Dedup           bool `json:"dedup"`
	Whitespace      bool `json:"whitespace"`
	Dictionary      bool `json:"dictionary"`
	Paths           bool `json:"paths"`
	JSONCompact     bool `json:"jsonCompact"`
	Observation     bool `json:"observation"`
	DynamicCodebook bool `json:"dynamicCodebook"`
}

// DefaultCompressionConfig returns the default configuration with only
// dedup, whitespace, and jsonCompact enabled.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Dedup:           true,
		Whitespace:      true,
		Dictionary:      false,
		Paths:           false,
		JSONCompact:     true,
		Observation:     false,
		DynamicCodebook: false,
	}
}

// CompressionStats tracks metrics about the compression process.
type CompressionStats struct {
	OriginalChars   int               `json:"originalChars"`
	CompressedChars int               `json:"compressedChars"`
	Ratio           float64           `json:"ratio"`
	LayerSavings    map[string]int    `json:"layerSavings"`
	MessagesRemoved int               `json:"messagesRemoved"`
	DictCodesUsed   []string          `json:"dictCodesUsed,omitempty"`
	PathMappings    map[string]string `json:"pathMappings,omitempty"`
}

// CompressionResult is the output of CompressContext.
type CompressionResult struct {
	Messages []NormalizedMessage `json:"messages"`
	Stats    CompressionStats    `json:"stats"`
	Header   string              `json:"header,omitempty"`
}
