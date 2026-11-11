package congmingpay

import "encoding/json"

// Response preserves all response fields, including newly added fields and their
// original JSON representation. A nil error does not imply result_code=success.
type Response map[string]json.RawMessage

// String returns a string field, or an empty string if it is absent or not a string.
func (r Response) String(key string) string {
	var s string
	_ = json.Unmarshal(r[key], &s)
	return s
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
