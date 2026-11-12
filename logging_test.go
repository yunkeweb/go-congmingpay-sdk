package congmingpay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type testLogger struct {
	mu      sync.Mutex
	entries []LogEvent
}

func (l *testLogger) hook(_ context.Context, event LogEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, event)
}
func (l *testLogger) snapshot() []LogEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]LogEvent(nil), l.entries...)
}

func TestLogging(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/error":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"result_code":"FAIL","token":"response-secret"}`))
		case "/api/invalid":
			_, _ = w.Write([]byte("{invalid payload"))
		default:
			_, _ = w.Write([]byte(`{"result_code":"success","order_id":"order-123","order_status":"paid","amount":12.5,"note":"paid order","nested":{"token":"response-secret","shop_name":"Example Shop"},"data":"{\"token\":\"response-secret\",\"order_id\":\"order-embedded\"}"}`))
		}
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app-id", ShopID: "shop", Key: "key-secret", HTTPClient: srv.Client(), LogRedactFields: []string{"sign", "auth-code", "token"}})
	if err != nil {
		t.Fatal(err)
	}
	logger := &testLogger{}
	logged := client.WithLogHook(logger.hook)
	params := Params{"order_id": "order-123", "order_status": "paid", "amount": 12.5, "auth_code": "request-secret", "nested": map[string]any{"token": "nested-secret", "shop_name": "Example Shop"}}
	if _, err := client.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if len(logger.snapshot()) != 0 {
		t.Fatal("logging enabled on original client")
	}
	if _, err := logged.Do(context.Background(), "/api/success", params); err != nil {
		t.Fatal(err)
	}
	if params["auth_code"] != "request-secret" || len(params) != 5 {
		t.Fatal("parameters modified")
	}
	_, err = logged.Do(context.Background(), "/api/error", nil)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err = logged.Do(context.Background(), "/api/invalid", nil); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	entries := logger.snapshot()
	if len(entries) != 6 {
		t.Fatalf("got %d entries", len(entries))
	}
	if entries[1].Event != "response" || entries[1].StatusCode != http.StatusOK || entries[3].StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	request := entries[0].Payload.(map[string]any)
	if request["sign"] != "[REDACTED]" || request["auth_code"] != "[REDACTED]" {
		t.Fatalf("request credentials not redacted: %+v", request)
	}
	b, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"request-secret", "nested-secret", "response-secret", "key-secret"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("leaked %q: %s", secret, b)
		}
	}
	for _, expected := range []string{"success", "order-123", "order-embedded", "paid", "12.5", "paid order", "Example Shop", "app-id", "shop", "[REDACTED]", "{invalid payload"} {
		if !strings.Contains(string(b), expected) {
			t.Fatalf("missing %q: %s", expected, b)
		}
	}
}

func TestLoggingTransportError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client(), LogRedactFields: []string{"sign", "auth_code", "token"}})
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()
	logger := &testLogger{}
	if _, err = client.WithLogHook(logger.hook).Do(context.Background(), "/api/test", nil); err == nil {
		t.Fatal("expected transport error")
	}
	entries := logger.snapshot()
	if len(entries) != 2 || entries[1].Event != "error" || entries[1].Error != "request failed" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestDefaultAndDisabledLogHook(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result_code":"success","order_id":"order-default","order_status":"paid","token":"response-secret"}`))
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original; _ = reader.Close(); _ = writer.Close() }()
	_, err = client.Do(context.Background(), "/api/default", Params{"order_id": "order-default", "auth_code": "request-secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.WithLogHook(nil).Do(context.Background(), "/api/disabled", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	result := string(output)
	for _, expected := range []string{`"event":"request"`, `"event":"response"`, `"status_code":200`} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing %q: %s", expected, result)
		}
	}
	for _, expected := range []string{"order-default", "paid", "success", "request-secret", "response-secret"} {
		if !strings.Contains(result, expected) {
			t.Fatalf("missing ordinary value %q: %s", expected, result)
		}
	}
	if strings.Contains(result, "/api/disabled") || strings.Contains(result, "[REDACTED]") {
		t.Fatalf("unexpected disabled log or default redaction: %s", result)
	}
}

func TestLogFieldMatching(t *testing.T) {
	fields := []string{"sub_openid", "token", "商户密钥", "", "_-"}
	client, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key", LogRedactFields: fields})
	if err != nil {
		t.Fatal(err)
	}
	// Configuration is copied when constructing the client.
	fields[0] = "order_id"
	logger := &testLogger{}
	client = client.WithLogHook(logger.hook)
	client.logPayload(context.Background(), "response", "/api/query.do", 200, []byte(`{"subOpenId":"a","SUB-OPENID":"b","token":"c","access_token":"d","token_count":2,"order_id":"order","sub.openid":"e","商户密钥":"f","商户编号":"shop","":"empty"}`))
	payload := logger.snapshot()[0].Payload.(map[string]any)
	for _, key := range []string{"subOpenId", "SUB-OPENID", "token", "商户密钥"} {
		if payload[key] != "[REDACTED]" {
			t.Fatalf("configured field %q not redacted: %#v", key, payload)
		}
	}
	for key, want := range map[string]any{"access_token": "d", "token_count": json.Number("2"), "order_id": "order", "sub.openid": "e", "商户编号": "shop", "": "empty"} {
		if payload[key] != want {
			t.Fatalf("unconfigured field %q modified: got %v, want %v", key, payload[key], want)
		}
	}
}

func TestLogDefaultPayloadValues(t *testing.T) {
	for _, fields := range [][]string{nil, {}} {
		client, err := NewClient(Config{BaseURL: "https://example.com", ProgramID: "app", ShopID: "shop", Key: "key", LogRedactFields: fields})
		if err != nil {
			t.Fatal(err)
		}
		logger := &testLogger{}
		client = client.WithLogHook(logger.hook)
		client.logPayload(context.Background(), "response", "/api/query.do", 200, []byte(`{"sign":"signature","sub_openid":"openid","unionid":"union","token":"token","order_id":202611102033483123,"money":12.30,"data":"{ \"sign\": \"nested-sign\" }"}`))
		payload := logger.snapshot()[0].Payload.(map[string]any)
		want := map[string]any{"sign": "signature", "sub_openid": "openid", "unionid": "union", "token": "token", "order_id": json.Number("202611102033483123"), "money": json.Number("12.30"), "data": `{ "sign": "nested-sign" }`}
		if !reflect.DeepEqual(payload, want) {
			t.Fatalf("default logging changed values: got %#v, want %#v", payload, want)
		}
	}
}

func TestConfiguredLogRedactionFields(t *testing.T) {
	fields := []string{
		"key", "api_key", "privateKey", "secret_key", "short_key", "appSecret", "client_secret",
		"sign", "SIGN", "signature", "paySign", "password", "passwd",
		"token", "access_token", "refreshToken", "id-token", "Authorization", "credential",
		"auth_code", "authCode", "code", "openid", "sub_openid", "unionId",
		"phone", "phone_number", "mobile", "mobileNumber", "id_card", "idCardNo",
		"bank_card", "bank_card_no", "card_no", "cardNumber", "cvv", "cvc",
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			configured := map[string]struct{}{normalizeLogField(field): {}}
			for _, value := range []any{"sensitive", json.Number("123456"), map[string]any{"value": "sensitive"}, []any{"sensitive"}} {
				if got := redactLogValue(value, field, configured); got != "[REDACTED]" {
					t.Fatalf("sensitive field %q not redacted: %v", field, got)
				}
			}
		})
	}
}

func TestLogBusinessFieldsAndNestedValues(t *testing.T) {
	input := map[string]any{
		"order_id": "order-123", "program_id": "app", "shop_id": "shop", "shopId": "shop",
		"order_status": "paid", "result_code": "CUSTOM_STATUS", "error_msg": "order not found",
		"money": json.Number("12.30"), "enabled": true, "empty": "", "missing": nil,
		"design": "normal", "monkey": "normal", "token_count": json.Number("2"),
		"items": []any{map[string]any{"order_id": "child", "accessToken": "secret"}, "ordinary", false, nil},
		"sign":  map[string]any{"value": "signature"},
	}
	want := map[string]any{
		"order_id": "order-123", "program_id": "app", "shop_id": "shop", "shopId": "shop",
		"order_status": "paid", "result_code": "CUSTOM_STATUS", "error_msg": "order not found",
		"money": json.Number("12.30"), "enabled": true, "empty": "", "missing": nil,
		"design": "normal", "monkey": "normal", "token_count": json.Number("2"),
		"items": []any{map[string]any{"order_id": "child", "accessToken": "[REDACTED]"}, "ordinary", false, nil},
		"sign":  "[REDACTED]",
	}
	fields := map[string]struct{}{"sign": {}, "accesstoken": {}}
	got := redactLogValue(input, "", fields)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected log values: got %#v, want %#v", got, want)
	}
	if input["items"].([]any)[0].(map[string]any)["accessToken"] != "secret" {
		t.Fatal("redaction modified the original payload")
	}
}

func TestLogJSONStringsAndNumberPrecision(t *testing.T) {
	logger := &testLogger{}
	client := &Client{logHook: logger.hook, logRedactFields: map[string]struct{}{"accesstoken": {}, "openid": {}}}
	body := []byte(`{"order_id":202611102033483123,"money":12.30,"data":"{\"access_token\":\"nested-secret\",\"money\":12.30,\"order_id\":202611102033483123}","items":"[{\"openid\":\"nested-secret\",\"order_id\":\"child\"}]","description":"unchanged","invalid":"{invalid json}"}`)
	client.logPayload(context.Background(), "response", "/api/query.do", 200, body)
	payload := logger.snapshot()[0].Payload.(map[string]any)
	if payload["order_id"] != json.Number("202611102033483123") || payload["money"] != json.Number("12.30") {
		t.Fatalf("numeric values lost precision: %#v", payload)
	}
	if payload["description"] != "unchanged" || payload["invalid"] != "{invalid json}" {
		t.Fatalf("ordinary strings modified: %#v", payload)
	}
	for _, field := range []string{"data", "items"} {
		value, ok := payload[field].(string)
		if !ok || !json.Valid([]byte(value)) || strings.Contains(value, "nested-secret") || !strings.Contains(value, "[REDACTED]") {
			t.Fatalf("JSON string not sanitized: %s = %v", field, payload[field])
		}
	}
	if !strings.Contains(payload["data"].(string), "202611102033483123") || !strings.Contains(payload["data"].(string), "12.30") {
		t.Fatalf("embedded numeric values lost precision: %v", payload["data"])
	}
}

func TestLogURLQueryRedaction(t *testing.T) {
	link := "https://pay.example/checkout?order_id=order%2f123&Pro-Access_Token=secret-one&pro%5Faccess_token=secret-two&pro_access_token&access_token=keep+value#unchanged"
	want := "https://pay.example/checkout?order_id=order%2f123&Pro-Access_Token=%5BREDACTED%5D&pro%5Faccess_token=%5BREDACTED%5D&pro_access_token=%5BREDACTED%5D&access_token=keep+value#unchanged"
	fields := map[string]struct{}{"proaccesstoken": {}}
	if got := redactLogValue(link, "url", fields); got != want {
		t.Fatalf("unexpected redacted URL: %v", got)
	}
	parsed, err := url.Parse(want)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("Pro-Access_Token") != "[REDACTED]" {
		t.Fatal("redaction marker is not a valid URL query value")
	}
	for _, value := range []string{
		link,
		"https://pay.example/?access_token=keep&token_count=2",
		"https://pay.example/?order_id=order%2f123",
		"ordinary text pro_access_token=keep",
		"{invalid json}",
	} {
		if got := redactLogValue(value, "", nil); got != value {
			t.Fatalf("default log changed: %v", got)
		}
		if !strings.Contains(value, "secret-one") && redactLogValue(value, "", fields) != value {
			t.Fatalf("unselected value changed: %q", value)
		}
	}
	logger := &testLogger{}
	client := &Client{logHook: logger.hook, logRedactFields: fields}
	encoded, err := json.Marshal(map[string]any{"url": link, "nested": []any{link}, "data": `{"url":"` + link + `"}`})
	if err != nil {
		t.Fatal(err)
	}
	client.logPayload(context.Background(), "response", "/api/buyPay.do", 200, encoded)
	entry, err := json.Marshal(logger.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(entry), "secret-one") || strings.Contains(string(entry), "secret-two") || !strings.Contains(string(entry), "keep+value") {
		t.Fatalf("incorrect nested URL redaction: %s", entry)
	}
}

func TestURLLoggingPreservesResponse(t *testing.T) {
	link := "https://pay.example/?pro_access_token=response-secret&order_id=order-123"
	body := `{"result_code":"fail","url":"` + link + `","device_name":"null"}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	client, err := NewClient(Config{BaseURL: srv.URL, ProgramID: "app", ShopID: "shop", Key: "key", HTTPClient: srv.Client(), LogRedactFields: []string{"pro_access_token"}})
	if err != nil {
		t.Fatal(err)
	}
	logger := &testLogger{}
	client = client.WithLogHook(logger.hook)
	result, err := client.QueryOrder(context.Background(), QueryOrderRequest{OrderID: "order-123"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResultCode != "fail" || result.URL != link || result.Raw.String("device_name") != "null" {
		t.Fatalf("logging changed response: %+v", result)
	}
	entries := logger.snapshot()
	payload := entries[1].Payload.(map[string]any)
	if payload["url"] == link || strings.Contains(payload["url"].(string), "response-secret") {
		t.Fatalf("configured URL query parameter not redacted: %v", payload)
	}
	// A hook is allowed to retain or modify its payload; it must not own Raw.
	payload["url"] = "changed by hook"
	if result.Raw.String("url") != link {
		t.Fatal("hook payload aliases response fields")
	}
}
