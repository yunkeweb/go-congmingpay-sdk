# go-congmingpay-sdk

聪明付基础支付 API 的 Go SDK，基于飞书文档 revision 3121 和 PHP SDK 的实现。仅依赖 Go 标准库，需要 Go 1.22 或更高版本。

## 安装

```sh
go get github.com/yunkeweb/go-congmingpay-sdk
```

## 客户端

每个 Client 对应一组商户配置。可以同时创建多个 Client；不要把真实密钥提交到代码仓库。

```go
client, err := congmingpay.NewClient(congmingpay.Config{
    BaseURL:   os.Getenv("CONGMINGPAY_BASE_URL"),
    ProgramID: os.Getenv("CONGMINGPAY_PROGRAM_ID"),
    ShopID:    os.Getenv("CONGMINGPAY_SHOP_ID"),
    Key:       os.Getenv("CONGMINGPAY_KEY"),
})
if err != nil {
    return err
}

result, err := client.QueryOrder(ctx, congmingpay.QueryOrderRequest{
    OrderID: "merchant-order-id",
})
if err != nil {
    return err
}
log.Printf("result=%s status=%s", result.ResultCode, result.OrderStatus)
```

可通过 `Config.HTTPClient` 提供自定义 HTTP 客户端；默认超时 30 秒。SDK 要求 HTTPS，不跟随重定向，不自动重试。`program_id`、`shop_id` 和 `sign` 由客户端注入，不能在通用请求中覆盖。分账接收方字段不受此限制。Client 可并发使用，HTTP 客户端和日志钩子也必须支持并发。

接口方法包括 `BuyPay`、`JSNativePay`、`MicroPay`、`PrePay`、`MiniAppPay`、`QueryOrder`、`Refund`、`QueryRefund`、`CancelOrder`、`ProfitOrder`、`ProfitOrderBack`、`SearchMerchantWXApp`、`SetMerchantWXApp`、`OpenIDByAuthCode`、`UserCancelOrder` 和 `UnionOpenID`。方法的请求结构体以方法名加 `Request` 命名。

`BuyPayRequest`、`JSNativePayRequest`、`MicroPayRequest` 和 `MiniAppPayRequest` 通过嵌入的 `PaymentFields` 设置共同字段。金额通常使用字符串，单位和精度遵循对应接口；分账回退的 JSON 数值使用 `json.Number` 保留原始十进制表示。指定版本的接口自动填充 `3.0`；查询接口的可选 `Ver` 仅在显式指定时发送。支付接口的分账及回调格式默认值与 PHP SDK 一致，可通过请求字段覆盖。

每个类型化方法返回 `Result`。常用字段可直接读取，`Result.Raw` 保留完整 JSON 响应字段；使用 `json.Unmarshal(result.Raw["字段名"], &value)` 可解析其他字段。`error == nil` 只表示 HTTP 和 JSON 处理成功，不能说明交易成功；必须检查业务状态。请求超时不代表交易未发生，重试支付、退款前应先查询，避免重复操作。

未封装接口可用 `Do(ctx, "/api/custom.do", congmingpay.Params{...})`，返回原始 `Response`。路径必须是 `/api/` 下的相对路径。参数中顶层 `nil` 不发送；涉及嵌套对象的签名需要与平台联调确认。若字段在文档中是 JSON 字符串，应传入字符串，而不是数组或对象。

## 回调和签名

请求签名参考 PHP SDK：排除 `sign` 和顶层 null，按字段名排序后拼接并使用大写 MD5。对于嵌套对象，签名与发送的 JSON 字节保持一致；跨语言的对象字段顺序、数字格式等仍需通过平台样例核对。

`ParsePaymentNotification` 和 `ParseRefundNotification` 接收 `url.Values`；`ParsePaymentNotificationJSON` 接收 JSON 原文。它们返回的 `Callback` 中 `Money`、`OrderID`、`ResultCode`、`ShopID` 是参与验签的字段，`Raw` 中其他字段并未经过认证。支付回调优先验证 `orderId`，不存在时按 PHP 实现验证 `order_id`；退款回调使用 `shop_id`。实际新回调格式须与平台确认并联调。解析 JSON 前调用方应限制请求体大小。

验签后仍要核对商户、订单、金额和状态，并进行幂等处理及必要的订单查询。响应签名和分账回退回调的签名规则尚不明确，SDK 不声称已验证。退款回调响应格式也须与平台确认。

## 日志

默认使用 `fmt.Printf` 输出请求、响应和错误。`client.WithLogHook(hook)` 返回一个使用自定义钩子的客户端副本；传入 `nil` 关闭日志。钩子同步调用，必须支持并发且不应阻塞。日志按字段名遮蔽签名、密钥、令牌、授权码、OpenID/UnionID、手机号及银行卡等敏感值，普通业务字段（如订单号、状态和金额）会保留。日志不包含回调验签。

## 测试与发布

```sh
go test ./...
go test -race ./...
go vet ./...
```

GitHub Actions 执行格式、测试和构建检查，并支持通过版本标签创建 Release。已发布的标签不应修改。当前模块路径支持 v0 和 v1；发布 v2 需要调整模块路径。本次配置和类型化 API 是不兼容变更，升级时需调整调用代码。
