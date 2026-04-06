package partners

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PartnerToolDefinition is an agent-callable tool backed by a partner service.
type PartnerToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any // JSON Schema object
	Execute     func(args map[string]any) (any, error)
}

// BuildPartnerTools constructs tool definitions for every registered partner
// service, binding each tool's execute function to make authenticated HTTP
// requests using the provided apiKey.
func BuildPartnerTools(apiKey string) []PartnerToolDefinition {
	tools := make([]PartnerToolDefinition, 0, len(PartnerServices))
	for _, svc := range PartnerServices {
		tools = append(tools, buildTool(svc, apiKey))
	}
	return tools
}

// buildTool creates a single PartnerToolDefinition from a service definition.
func buildTool(svc PartnerServiceDefinition, apiKey string) PartnerToolDefinition {
	schema := buildInputSchema(svc.Params)

	return PartnerToolDefinition{
		Name:        svc.ID,
		Description: svc.Description,
		InputSchema: schema,
		Execute: func(args map[string]any) (any, error) {
			return executeService(svc, apiKey, args)
		},
	}
}

// buildInputSchema produces a JSON Schema "object" descriptor from the
// service's parameter list.
func buildInputSchema(params []PartnerServiceParam) map[string]any {
	properties := make(map[string]any, len(params))
	required := make([]string, 0)

	for _, p := range params {
		prop := map[string]any{
			"type":        string(p.Type),
			"description": p.Description,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		properties[p.Name] = prop

		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// executeService calls the partner API and returns the (optionally extracted)
// response body.
func executeService(svc PartnerServiceDefinition, apiKey string, args map[string]any) (any, error) {
	reqURL := buildURL(svc, args)

	req, err := http.NewRequest(string(svc.Method), reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("partners: build request: %w", err)
	}

	// Default auth header.
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	// Apply any service-specific headers.
	for k, v := range svc.Headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("partners: request %s: %w", svc.ID, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("partners: read response %s: %w", svc.ID, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("partners: %s returned HTTP %d: %s", svc.ID, resp.StatusCode, string(body))
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Return raw string if not JSON.
		return string(body), nil
	}

	// Extract resultPath if specified.
	if svc.ResultPath != "" {
		return extractPath(parsed, svc.ResultPath), nil
	}
	return parsed, nil
}

// buildURL constructs the final request URL by substituting path parameters
// (e.g. {address}) and appending query parameters.
func buildURL(svc PartnerServiceDefinition, args map[string]any) string {
	u := svc.BaseURL

	// Substitute path parameters like {address}.
	usedInPath := make(map[string]bool)
	for _, p := range svc.Params {
		placeholder := "{" + p.Name + "}"
		if strings.Contains(u, placeholder) {
			val := paramString(args, p)
			u = strings.ReplaceAll(u, placeholder, url.PathEscape(val))
			usedInPath[p.Name] = true
		}
	}

	// Build query string from remaining params.
	q := url.Values{}
	for _, p := range svc.Params {
		if usedInPath[p.Name] {
			continue
		}
		val := paramString(args, p)
		if val != "" {
			q.Set(p.Name, val)
		}
	}

	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
	}
	return u
}

// paramString returns the string representation of a parameter value from
// args, falling back to the param's default if not supplied.
func paramString(args map[string]any, p PartnerServiceParam) string {
	if v, ok := args[p.Name]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	if p.Default != nil {
		return fmt.Sprintf("%v", p.Default)
	}
	return ""
}

// extractPath walks a parsed JSON value along a dot-separated path.
// For example, "data.items" navigates into {"data": {"items": [...]}}.
func extractPath(v any, path string) any {
	parts := strings.Split(path, ".")
	current := v
	for _, key := range parts {
		if key == "" {
			continue
		}
		m, ok := current.(map[string]any)
		if !ok {
			return current
		}
		current, ok = m[key]
		if !ok {
			return nil
		}
	}
	return current
}
