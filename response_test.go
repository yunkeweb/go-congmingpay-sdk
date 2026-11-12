package congmingpay

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResponseStringField(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want, gotType string
	}{
		{"empty", `""`, "", ""},
		{"text", `"支付成功"`, "支付成功", ""},
		{"literal null", `"null"`, "null", ""},
		{"null", ` null `, "", "null"},
		{"number", `2`, "", "number"},
		{"large number", `1e1000`, "", "number"},
		{"boolean", `true`, "", "boolean"},
		{"array", `[]`, "", "array"},
		{"object", `{"token":"secret"}`, "", "object"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Response{"field": json.RawMessage(tc.raw)}
			got, present, err := r.StringField("field")
			if !present || got != tc.want {
				t.Fatalf("got (%q, %v, %v)", got, present, err)
			}
			if tc.gotType == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var typeErr *FieldTypeError
				if !errors.As(err, &typeErr) || typeErr.Field != "field" || typeErr.Want != "string" || typeErr.Got != tc.gotType {
					t.Fatalf("unexpected type error: %v", err)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatal("type error includes field contents")
				}
			}
			if string(r["field"]) != tc.raw || r.String("field") != tc.want {
				t.Fatal("raw data or legacy accessor behavior changed")
			}
		})
	}
	for _, r := range []Response{nil, {}} {
		if value, present, err := r.StringField("missing"); value != "" || present || err != nil {
			t.Fatalf("missing field: (%q, %v, %v)", value, present, err)
		}
	}
	for _, raw := range []string{"", `"unterminated`, `"ok" true`, `{"token":}`} {
		_, present, err := (Response{"field": json.RawMessage(raw)}).StringField("field")
		var syntaxErr *json.SyntaxError
		if !present || !errors.As(err, &syntaxErr) {
			t.Fatalf("invalid raw %q: present=%v err=%v", raw, present, err)
		}
	}
}

func TestResponseDecode(t *testing.T) {
	var raw Response
	if err := json.Unmarshal([]byte(`{"result_code":"fail","device_name":"null","amount":12.30,"large":202611102033483123,"appid_list":["app"],"extra":{"state":"NEW"}}`), &raw); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(raw)
	var dst struct {
		ResultCode string          `json:"result_code"`
		DeviceName string          `json:"device_name"`
		Amount     json.Number     `json:"amount"`
		Large      json.Number     `json:"large"`
		AppIDs     []string        `json:"appid_list"`
		Extra      json.RawMessage `json:"extra"`
	}
	if err := raw.Decode(&dst); err != nil {
		t.Fatal(err)
	}
	if dst.ResultCode != "fail" || dst.DeviceName != "null" || dst.Amount != "12.30" || dst.Large != "202611102033483123" || !reflect.DeepEqual(dst.AppIDs, []string{"app"}) || string(dst.Extra) != `{"state":"NEW"}` {
		t.Fatalf("unexpected decoded fields: %+v", dst)
	}
	var typeMismatch struct {
		Amount string `json:"amount"`
	}
	var typeErr *json.UnmarshalTypeError
	if err := raw.Decode(&typeMismatch); !errors.As(err, &typeErr) {
		t.Fatalf("type error chain lost: %v", err)
	}
	var invalidPtr *struct{}
	for _, dst := range []any{nil, invalidPtr, struct{}{}} {
		var pointerErr *json.InvalidUnmarshalError
		if err := raw.Decode(dst); !errors.As(err, &pointerErr) {
			t.Fatalf("invalid destination error chain lost: %v", err)
		}
	}
	after, _ := json.Marshal(raw)
	if string(after) != string(before) {
		t.Fatal("Decode modified raw data")
	}
	if err := (Response(nil)).Decode(&dst); err == nil {
		t.Fatal("nil response accepted")
	}
	if err := (Response{"bad": json.RawMessage(`{invalid`)}).Decode(&dst); err == nil {
		t.Fatal("invalid raw JSON accepted")
	}
	var generic map[string]any
	if err := raw.Decode(&generic); err != nil {
		t.Fatal(err)
	}
	if _, ok := generic["amount"].(float64); !ok {
		t.Fatal("Decode changed standard interface number semantics")
	}
}
