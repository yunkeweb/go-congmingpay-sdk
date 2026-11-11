package congmingpay

import "context"

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

func (c *Client) BuyPay(ctx context.Context, r BuyPayRequest) (*Result, error) {
	return c.call(ctx, "/api/buyPay.do", r, Params{"ver": "3.0", "profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "notify_url"})
}

type JSNativePayRequest struct {
	PaymentFields
	PayType string `json:"pay_type,omitempty"`
	AppID   string `json:"appid,omitempty"`
	OpenID  string `json:"openid,omitempty"`
}

func (c *Client) JSNativePay(ctx context.Context, r JSNativePayRequest) (*Result, error) {
	return c.call(ctx, "/api/jsNativePay.do", r, Params{"profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "notify_url"})
}

type MicroPayRequest struct {
	PaymentFields
	AuthCode string `json:"auth_code,omitempty"`
	PayType  string `json:"pay_type,omitempty"`
}

func (c *Client) MicroPay(ctx context.Context, r MicroPayRequest) (*Result, error) {
	return c.call(ctx, "/api/microPay.do", r, Params{"profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "auth_code", "device"})
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

func (c *Client) PrePay(ctx context.Context, r PrePayRequest) (*Result, error) {
	return c.call(ctx, "/api/v3/vprePay.do", r, Params{"version": "3.0", "profit_share_type": "0"}, []string{"money", "order_id", "notify_url"})
}

type MiniAppPayRequest struct {
	PaymentFields
	AppID     string `json:"appid,omitempty"`
	OpenID    string `json:"openid,omitempty"`
	DYScene   string `json:"dy_scene,omitempty"`
	DYAppType string `json:"dy_app_type,omitempty"`
}

func (c *Client) MiniAppPay(ctx context.Context, r MiniAppPayRequest) (*Result, error) {
	return c.call(ctx, "/api/miniAppPay.do", r, Params{"ver": "3.0", "profit_share_type": "0", "is_notify_new": "0"}, []string{"money", "order_id", "order_type", "device", "appid", "openid", "notify_url"})
}
