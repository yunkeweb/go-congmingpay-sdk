package congmingpay

import "context"

// The following methods correspond to sections 2-1 through 2-18 of the API
// document. Fields are intentionally supplied as Params so optional and newly
// introduced gateway fields can be passed without losing their exact values.
func (c *Client) BuyPay(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/buyPay.do", p)
}
func (c *Client) JSNativePay(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/jsNativePay.do", p)
}
func (c *Client) MicroPay(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/microPay.do", p)
}
func (c *Client) PrePay(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/v3/vprePay.do", p)
}
func (c *Client) MiniAppPay(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/miniAppPay.do", p)
}
func (c *Client) QueryOrder(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/query.do", p)
}
func (c *Client) Refund(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/refund.do", p)
}
func (c *Client) QueryRefund(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/queryRefundOrder.do", p)
}
func (c *Client) CancelOrder(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/cancelOrder.do", p)
}
func (c *Client) ProfitOrder(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/profitorder.do", p)
}
func (c *Client) ProfitOrderBack(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/profitorderback.do", p)
}
func (c *Client) SearchMerchantWXApp(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/searchMerchantWxAppMsg.do", p)
}
func (c *Client) SetMerchantWXApp(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/setMerchantWxAppMsg.do", p)
}
func (c *Client) OpenIDByAuthCode(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/getOpenidByAuthCode.do", p)
}
func (c *Client) UserCancelOrder(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/userCancelOrder.do", p)
}
func (c *Client) UnionOpenID(ctx context.Context, p Params) (Response, error) {
	return c.Do(ctx, "/api/getUnionOpenid.do", p)
}
