package congmingpay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// LogEvent contains a sanitized request, response, or transport failure.
type LogEvent struct {
	Event      string `json:"event"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code,omitempty"`
	Payload    any    `json:"payload,omitempty"`
	Error      string `json:"error,omitempty"`
}

// LogHook receives sanitized events synchronously. It must be safe for concurrent
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
		value = "[invalid JSON redacted]"
	} else {
		value = redactLogValue(value, "")
	}
	c.logHook(ctx, LogEvent{Event: event, Path: path, StatusCode: status, Payload: value})
}

func (c *Client) logFailure(ctx context.Context, path, message string) {
	if c.logHook != nil {
		c.logHook(ctx, LogEvent{Event: "error", Path: path, Error: message})
	}
}

func redactLogValue(value any, key string) any {
	if isSensitiveLogKey(key) {
		return "[REDACTED]"
	}

	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, item := range v {
			result[k] = redactLogValue(item, k)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = redactLogValue(item, "")
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
					if encoded, err := json.Marshal(redactLogValue(decoded, "")); err == nil {
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

func isSensitiveLogKey(key string) bool {
	var normalized strings.Builder
	for _, r := range key {
		if r >= 'a' && r <= 'z' {
			normalized.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			normalized.WriteRune(r + ('a' - 'A'))
		} else if r >= '0' && r <= '9' {
			normalized.WriteRune(r)
		}
	}
	name := normalized.String()
	for _, sensitive := range []string{
		"signature", "secret", "password", "passwd", "token", "authcode",
		"openid", "unionid", "phonenumber", "mobilenumber", "phone", "mobile", "idcard", "cardno",
		"cardnumber", "bankcard", "cvv", "cvc", "authorization", "credential",
	} {
		if name == sensitive || strings.HasSuffix(name, sensitive) {
			return true
		}
	}
	return name == "sign" || strings.HasSuffix(name, "paysign") || name == "code" || name == "key" || strings.HasSuffix(name, "apikey") || strings.HasSuffix(name, "privatekey") || strings.HasSuffix(name, "secretkey") || strings.HasSuffix(name, "shortkey")
}
