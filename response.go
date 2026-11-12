package congmingpay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Response preserves all response fields, including newly added fields and their
// original field-level JSON representation. It does not preserve the complete
// HTTP body layout or duplicate keys. A nil error does not imply result_code=success.
type Response map[string]json.RawMessage

// String returns a string field, or an empty string if it is absent or not a string.
func (r Response) String(key string) string {
	var s string
	_ = json.Unmarshal(r[key], &s)
	return s
}

// Decode decodes the response into dst using json.Unmarshal semantics. A nil
// response is an error. dst must be a non-nil pointer and may be partially filled
// on error. Interface values use float64 for numbers; use json.Number or
// json.RawMessage fields when exact numeric values are required.
// Decode neither changes r nor validates required fields or business statuses.
func (r Response) Decode(dst any) error {
	if r == nil {
		return errors.New("congmingpay: decode nil response")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("congmingpay: encode response fields: %w", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("congmingpay: decode response fields: %w", err)
	}
	return nil
}

// StringField distinguishes missing fields, empty strings and type errors.
// Null and non-string values return a FieldTypeError with present=true.
// The string "null" is returned unchanged. Invalid JSON returns a wrapped error.
func (r Response) StringField(key string) (value string, present bool, err error) {
	raw, present := r[key]
	if !present {
		return "", false, nil
	}
	var checked json.RawMessage
	if err := json.Unmarshal(raw, &checked); err != nil {
		return "", true, fmt.Errorf("congmingpay: decode field %q: %w", key, err)
	}
	checked = bytes.TrimSpace(checked)
	var got string
	switch checked[0] {
	case '"':
		if err := json.Unmarshal(checked, &value); err != nil {
			return "", true, fmt.Errorf("congmingpay: decode field %q: %w", key, err)
		}
		return value, true, nil
	case 'n':
		got = "null"
	case 't', 'f':
		got = "boolean"
	case '[':
		got = "array"
	case '{':
		got = "object"
	default:
		got = "number"
	}
	return "", true, &FieldTypeError{Field: key, Want: "string", Got: got}
}

// FieldTypeError describes a field type mismatch without including its value.
type FieldTypeError struct {
	Field string
	Want  string
	Got   string
}

func (e *FieldTypeError) Error() string {
	return fmt.Sprintf("congmingpay: field %q: want %s, got %s", e.Field, e.Want, e.Got)
}

// Result exposes common response fields and preserves the full response in Raw.
// An HTTP success does not imply that a payment or refund succeeded.
type Result struct {
	ResultCode            string   `json:"result_code"`
	ErrorMessage          string   `json:"error_msg"`
	URL                   string   `json:"url"`
	CodeURL               string   `json:"code_url"`
	OrderID               string   `json:"order_id"`
	OutTradeNo            string   `json:"out_trade_no"`
	OrderStatus           string   `json:"order_status"`
	PayStatus             string   `json:"pay_status"`
	RefundStatus          string   `json:"refund_status"`
	RefundOrderID         string   `json:"refund_order_id"`
	PlatformRefundOrderID string   `json:"plat_refund_order_id"`
	ProfitOrderID         string   `json:"profit_order_id"`
	OpenID                string   `json:"openid"`
	SubOpenID             string   `json:"sub_openid"`
	TransactionID         string   `json:"transaction_id"`
	ChannelTradeNo        string   `json:"channel_trade_no"`
	OrderType             string   `json:"order_type"`
	ShopID                string   `json:"shop_id"`
	Payable               string   `json:"pay_ble"`
	PaidOut               string   `json:"paid_out"`
	RefundMoney           string   `json:"refund_money"`
	ProfitStatus          string   `json:"profit_status"`
	ProfitOrderBackID     string   `json:"profit_order_back_id"`
	Sign                  string   `json:"sign"`
	Raw                   Response `json:"-"`
}

func parseResult(raw Response) *Result {
	result := &Result{Raw: raw}
	result.ResultCode = raw.String("result_code")
	result.ErrorMessage = raw.String("error_msg")
	result.URL = raw.String("url")
	result.CodeURL = raw.String("code_url")
	result.OrderID = raw.String("order_id")
	result.OutTradeNo = raw.String("out_trade_no")
	result.OrderStatus = raw.String("order_status")
	result.PayStatus = raw.String("pay_status")
	result.RefundStatus = raw.String("refund_status")
	result.RefundOrderID = raw.String("refund_order_id")
	result.PlatformRefundOrderID = raw.String("plat_refund_order_id")
	result.ProfitOrderID = raw.String("profit_order_id")
	result.OpenID = raw.String("openid")
	result.SubOpenID = raw.String("sub_openid")
	result.TransactionID = raw.String("transaction_id")
	result.ChannelTradeNo = raw.String("channel_trade_no")
	result.OrderType = raw.String("order_type")
	result.ShopID = raw.String("shop_id")
	result.Payable = raw.String("pay_ble")
	result.PaidOut = raw.String("paid_out")
	result.RefundMoney = raw.String("refund_money")
	result.ProfitStatus = raw.String("profit_status")
	result.ProfitOrderBackID = raw.String("profit_order_back_id")
	result.Sign = raw.String("sign")
	return result
}
