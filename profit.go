package congmingpay

import (
	"context"
	"encoding/json"
)

type ProfitOrderRequest struct {
	OrderID    string          `json:"order_id,omitempty"`
	OutTradeNo string          `json:"out_trade_no,omitempty"`
	IsProfit   string          `json:"is_profit,omitempty"`
	ProfitRule json.RawMessage `json:"profit_rule,omitempty"`
	AuthMoney  string          `json:"auth_money,omitempty"`
}

func (c *Client) ProfitOrder(ctx context.Context, r ProfitOrderRequest) (*Result, error) {
	return c.call(ctx, "/api/profitorder.do", r, nil, []string{"order_id", "out_trade_no", "is_profit"})
}

type ProfitOrderBackRequest struct {
	OrderID             string      `json:"order_id,omitempty"`
	ProfitOrderID       string      `json:"profit_order_id,omitempty"`
	PSShopID            string      `json:"ps_shop_id,omitempty"`
	PSOrderBackMoney    json.Number `json:"ps_order_back_money,omitempty"`
	ProfitBackNotifyURL string      `json:"profit_back_notify_url,omitempty"`
}

func (c *Client) ProfitOrderBack(ctx context.Context, r ProfitOrderBackRequest) (*Result, error) {
	return c.call(ctx, "/api/profitorderback.do", r, nil, []string{"order_id", "profit_order_id", "ps_shop_id", "ps_order_back_money", "profit_back_notify_url"})
}
