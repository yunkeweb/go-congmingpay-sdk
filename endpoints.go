package congmingpay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

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

// PaymentFields are shared by the payment endpoints. Amounts are in yuan.
// ProfitShareInfo and Detail are JSON strings where required by the gateway.
type PaymentFields struct {
	Money           string `json:"money,omitempty"`
	OrderID         string `json:"order_id,omitempty"`
	OrderType       string `json:"order_type,omitempty"`
	Device          string `json:"device,omitempty"`
	DeviceID        string `json:"device_id,omitempty"`
	GoodsMessage    string `json:"goods_msg,omitempty"`
	NotifyURL       string `json:"notify_url,omitempty"`
	IsNotifyNew     string `json:"is_notify_new,omitempty"`
	ProfitShareType string `json:"profit_share_type,omitempty"`
	ProfitShareInfo string `json:"profit_share_info,omitempty"`
	GoodsTag        string `json:"goods_tag,omitempty"`
	Detail          string `json:"detail,omitempty"`
}
type BuyPayRequest struct {
	PaymentFields
	ReturnURL string `json:"return_url,omitempty"`
}
type JSNativePayRequest struct {
	PaymentFields
	PayType string `json:"pay_type,omitempty"`
	AppID   string `json:"appid,omitempty"`
	OpenID  string `json:"openid,omitempty"`
}
type MicroPayRequest struct {
	PaymentFields
	AuthCode string `json:"auth_code,omitempty"`
	PayType  string `json:"pay_type,omitempty"`
}
type PrePayRequest struct {
	Money           string `json:"money,omitempty"`
	OrderID         string `json:"order_id,omitempty"`
	Device          string `json:"device,omitempty"`
	NotifyURL       string `json:"notify_url,omitempty"`
	ProfitShareType string `json:"profit_share_type,omitempty"`
	ProfitShareInfo string `json:"profit_share_info,omitempty"`
	GoodsMessage    string `json:"goods_msg,omitempty"`
}
type MiniAppPayRequest struct {
	PaymentFields
	AppID     string `json:"appid,omitempty"`
	OpenID    string `json:"openid,omitempty"`
	DYScene   string `json:"dy_scene,omitempty"`
	DYAppType string `json:"dy_app_type,omitempty"`
}
type QueryOrderRequest struct {
	OrderID       string `json:"order_id,omitempty"`
	OutTradeNo    string `json:"out_trade_no,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	Ver           string `json:"ver,omitempty"`
}
type RefundRequest struct {
	RefundOrderID string `json:"refund_order_id,omitempty"`
	OrderID       string `json:"order_id,omitempty"`
	ShopOrderID   string `json:"shop_order_id,omitempty"`
	Money         string `json:"money,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
	ShortKey      string `json:"short_key,omitempty"`
	NotifyURL     string `json:"notify_url,omitempty"`
}
type QueryRefundRequest struct {
	RefundOrderID         string `json:"refund_order_id,omitempty"`
	PlatformRefundOrderID string `json:"plat_refund_order_id,omitempty"`
}
type CancelOrderRequest struct {
	OrderID    string `json:"order_id,omitempty"`
	OutTradeNo string `json:"out_trade_no,omitempty"`
}
type ProfitOrderRequest struct {
	OrderID    string          `json:"order_id,omitempty"`
	OutTradeNo string          `json:"out_trade_no,omitempty"`
	IsProfit   string          `json:"is_profit,omitempty"`
	ProfitRule json.RawMessage `json:"profit_rule,omitempty"`
	AuthMoney  string          `json:"auth_money,omitempty"`
}
type ProfitOrderBackRequest struct {
	OrderID             string      `json:"order_id,omitempty"`
	ProfitOrderID       string      `json:"profit_order_id,omitempty"`
	PSShopID            string      `json:"ps_shop_id,omitempty"`
	PSOrderBackMoney    json.Number `json:"ps_order_back_money,omitempty"`
	ProfitBackNotifyURL string      `json:"profit_back_notify_url,omitempty"`
}
type SearchMerchantWXAppRequest struct{}
type SetMerchantWXAppRequest struct {
	ConfigType string `json:"config_type,omitempty"`
	AppID      string `json:"appid,omitempty"`
	AuthList   string `json:"auth_list,omitempty"`
}
type OpenIDByAuthCodeRequest struct {
	AuthCode string `json:"auth_code,omitempty"`
	SubAppID string `json:"sub_appid,omitempty"`
}
type UserCancelOrderRequest struct {
	OrderID      string `json:"order_id,omitempty"`
	OutTradeNo   string `json:"out_trade_no,omitempty"`
	ErrorMessage string `json:"error_msg,omitempty"`
}
type UnionOpenIDRequest struct {
	Code       string `json:"code,omitempty"`
	PaymentApp string `json:"payment_app,omitempty"`
}

func (c *Client) call(ctx context.Context, path string, request any, defaults Params, required []string, alternatives ...[]string) (*Result, error) {
	b, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("congmingpay: encode parameters: %w", err)
	}
	params := Params{}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil {
		return nil, err
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(b, &rawFields); err != nil {
		return nil, err
	}
	for k, v := range rawFields {
		if len(v) > 0 && (v[0] == '{' || v[0] == '[') {
			params[k] = v
		}
	}
	for k, v := range defaults {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}
	for _, k := range required {
		if params[k] == nil || params[k] == "" {
			return nil, fmt.Errorf("congmingpay: %s is required", k)
		}
	}
	for _, group := range alternatives {
		found := false
		for _, k := range group {
			if params[k] != nil && params[k] != "" {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("congmingpay: one of %v is required", group)
		}
	}
	if path == "/api/jsNativePay.do" && !(params["order_type"] == "douyin" && params["pay_type"] == "app") && (params["openid"] == nil || params["openid"] == "") {
		return nil, fmt.Errorf("congmingpay: openid is required")
	}
	if path == "/api/profitorder.do" && params["is_profit"] == "1" && params["auth_money"] == nil && params["profit_rule"] == nil {
		return nil, fmt.Errorf("congmingpay: profit_rule is required when auth_money is absent")
	}
	if path == "/api/setMerchantWxAppMsg.do" {
		if params["config_type"] == "appid" && params["appid"] == nil {
			return nil, fmt.Errorf("congmingpay: appid is required")
		}
		if params["config_type"] == "path" && params["auth_list"] == nil {
			return nil, fmt.Errorf("congmingpay: auth_list is required")
		}
	}
	raw, err := c.Do(ctx, path, params)
	if err != nil {
		return nil, err
	}
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
	return result, nil
}

func (c *Client) BuyPay(ctx context.Context, r BuyPayRequest) (*Result, error) {
	return c.call(ctx, "/api/buyPay.do", r, Params{"ver": "3.0", "profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "notify_url"})
}
func (c *Client) JSNativePay(ctx context.Context, r JSNativePayRequest) (*Result, error) {
	return c.call(ctx, "/api/jsNativePay.do", r, Params{"profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "notify_url"})
}
func (c *Client) MicroPay(ctx context.Context, r MicroPayRequest) (*Result, error) {
	return c.call(ctx, "/api/microPay.do", r, Params{"profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "auth_code", "device"})
}
func (c *Client) PrePay(ctx context.Context, r PrePayRequest) (*Result, error) {
	return c.call(ctx, "/api/v3/vprePay.do", r, Params{"version": "3.0", "profit_share_type": "0"}, []string{"money", "order_id", "notify_url"})
}
func (c *Client) MiniAppPay(ctx context.Context, r MiniAppPayRequest) (*Result, error) {
	return c.call(ctx, "/api/miniAppPay.do", r, Params{"ver": "3.0", "profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "appid", "openid", "notify_url"})
}
func (c *Client) QueryOrder(ctx context.Context, r QueryOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/query.do", r, nil, nil, []string{"order_id", "out_trade_no", "transaction_id"})
}
func (c *Client) Refund(ctx context.Context, r RefundRequest) (*Result, error) {
	return c.call(ctx, "/api/refund.do", r, nil, nil, []string{"order_id", "shop_order_id"})
}
func (c *Client) QueryRefund(ctx context.Context, r QueryRefundRequest) (*Result, error) {
	return c.call(ctx, "/api/queryRefundOrder.do", r, nil, nil, []string{"refund_order_id", "plat_refund_order_id"})
}
func (c *Client) CancelOrder(ctx context.Context, r CancelOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/cancelOrder.do", r, nil, nil, []string{"order_id", "out_trade_no"})
}
func (c *Client) ProfitOrder(ctx context.Context, r ProfitOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/profitorder.do", r, nil, []string{"order_id", "out_trade_no", "is_profit"})
}
func (c *Client) ProfitOrderBack(ctx context.Context, r ProfitOrderBackRequest) (*Result, error) {
	return c.call(ctx, "/api/profitorderback.do", r, nil, []string{"order_id", "profit_order_id", "ps_shop_id", "ps_order_back_money", "profit_back_notify_url"})
}
func (c *Client) SearchMerchantWXApp(ctx context.Context, r SearchMerchantWXAppRequest) (*Result, error) {
	return c.call(ctx, "/api/searchMerchantWxAppMsg.do", r, nil, nil)
}
func (c *Client) SetMerchantWXApp(ctx context.Context, r SetMerchantWXAppRequest) (*Result, error) {
	return c.call(ctx, "/api/setMerchantWxAppMsg.do", r, nil, []string{"config_type"})
}
func (c *Client) OpenIDByAuthCode(ctx context.Context, r OpenIDByAuthCodeRequest) (*Result, error) {
	return c.call(ctx, "/api/getOpenidByAuthCode.do", r, nil, []string{"auth_code"})
}
func (c *Client) UserCancelOrder(ctx context.Context, r UserCancelOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/userCancelOrder.do", r, nil, []string{"error_msg"}, []string{"order_id", "out_trade_no"})
}
func (c *Client) UnionOpenID(ctx context.Context, r UnionOpenIDRequest) (*Result, error) {
	return c.call(ctx, "/api/getUnionOpenid.do", r, nil, []string{"code", "payment_app"})
}
