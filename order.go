package congmingpay

import "context"

type QueryOrderRequest struct {
	OrderID       string `json:"order_id,omitempty"`
	OutTradeNo    string `json:"out_trade_no,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	Ver           string `json:"ver,omitempty"`
}

func (c *Client) QueryOrder(ctx context.Context, r QueryOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/query.do", r, nil, nil, []string{"order_id", "out_trade_no", "transaction_id"})
}

type CancelOrderRequest struct {
	OrderID    string `json:"order_id,omitempty"`
	OutTradeNo string `json:"out_trade_no,omitempty"`
}

func (c *Client) CancelOrder(ctx context.Context, r CancelOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/cancelOrder.do", r, nil, nil, []string{"order_id", "out_trade_no"})
}

type UserCancelOrderRequest struct {
	OrderID      string `json:"order_id,omitempty"`
	OutTradeNo   string `json:"out_trade_no,omitempty"`
	ErrorMessage string `json:"error_msg,omitempty"`
}

func (c *Client) UserCancelOrder(ctx context.Context, r UserCancelOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/userCancelOrder.do", r, nil, []string{"error_msg"}, []string{"order_id", "out_trade_no"})
}
