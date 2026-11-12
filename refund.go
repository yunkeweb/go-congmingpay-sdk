package congmingpay

import "context"

type RefundRequest struct {
	// RefundOrderID is the caller's refund number, also used by QueryRefund.
	RefundOrderID string `json:"refund_order_id,omitempty"`
	OrderID       string `json:"order_id,omitempty"`
	// ShopOrderID identifies the original payment by its merchant order number.
	ShopOrderID string `json:"shop_order_id,omitempty"`
	Money       string `json:"money,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	ShortKey    string `json:"short_key,omitempty"`
	NotifyURL   string `json:"notify_url,omitempty"`
}

func (c *Client) Refund(ctx context.Context, r RefundRequest) (*Result, error) {
	return c.call(ctx, "/api/refund.do", r, nil, nil, []string{"order_id", "shop_order_id"})
}

type QueryRefundRequest struct {
	// RefundOrderID is the caller's refund number supplied to Refund.
	RefundOrderID string `json:"refund_order_id,omitempty"`
	// PlatformRefundOrderID is the platform's refund number, not the caller's.
	PlatformRefundOrderID string `json:"plat_refund_order_id,omitempty"`
}

func (c *Client) QueryRefund(ctx context.Context, r QueryRefundRequest) (*Result, error) {
	return c.call(ctx, "/api/queryRefundOrder.do", r, nil, nil, []string{"refund_order_id", "plat_refund_order_id"})
}
