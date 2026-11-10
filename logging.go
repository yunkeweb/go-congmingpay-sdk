package congmingpay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// LogEvent contains a request, response, or transport failure. Payload values
// are redacted only when their field names are configured in Config.
type LogEvent struct {
	Event      string `json:"event"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code,omitempty"`
	Payload    any    `json:"payload,omitempty"`
	Error      string `json:"error,omitempty"`
}

// LogHook receives log events synchronously. It must be safe for concurrent
// calls and should not block request processing.
type LogHook func(ctx context.Context, event LogEvent)

// WithLogHook returns a copy of the client with the given hook. Passing nil
// disables logging. The original client remains unchanged.
func (c *Client) WithLogHook(hook LogHook) *Client {
	if c == nil {
		return nil
	}
	cloned := *c
	cloned.logHook = hook
	return &cloned
}

func defaultLogHook(_ context.Context, event LogEvent) {
	b, err := json.Marshal(event)
	if err != nil {
		return
	}
	fmt.Printf("congmingpay %s\n", b)
}

func (c *Client) logPayload(ctx context.Context, event, path string, status int, body []byte) {
	if c.logHook == nil {
		return
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var extra any
	if decoder.Decode(&value) != nil || decoder.Decode(&extra) != io.EOF {
		value = string(body)
	} else {
		value = redactLogValue(value, "", c.logRedactFields)
	}
	c.logHook(ctx, LogEvent{Event: event, Path: path, StatusCode: status, Payload: value})
}

func (c *Client) logFailure(ctx context.Context, path, message string) {
	if c.logHook != nil {
		c.logHook(ctx, LogEvent{Event: "error", Path: path, Error: message})
	}
}

func redactLogValue(value any, key string, fields map[string]struct{}) any {
	if len(fields) == 0 {
		return value
	}
	if _, redact := fields[normalizeLogField(key)]; key != "" && redact {
		return "[REDACTED]"
	}

	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, item := range v {
			result[k] = redactLogValue(item, k, fields)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = redactLogValue(item, "", fields)
		}
		return result
	case json.Number:
		return v
	case string:
		trimmed := strings.TrimSpace(v)
		if len(trimmed) > 1 && (trimmed[0] == '{' || trimmed[0] == '[') {
			var decoded any
			decoder := json.NewDecoder(strings.NewReader(trimmed))
			decoder.UseNumber()
			if decoder.Decode(&decoded) == nil && (isJSONContainer(decoded)) {
				var extra any
				if decoder.Decode(&extra) == io.EOF {
					if encoded, err := json.Marshal(redactLogValue(decoded, "", fields)); err == nil {
						return string(encoded)
					}
				}
			}
		}
		return v
	default:
		return value
	}
}

func isJSONContainer(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func normalizeLogField(key string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(key) {
		if r != '_' && r != '-' {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}
