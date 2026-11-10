# go-congmingpay-sdk

聪明付基础支付 API 的 Go SDK，基于《基础支付接口文档及说明》版本 3120。仅依赖 Go 标准库，要求 Go 1.22 或更高版本。

## 安装

```sh
go get github.com/yunkeweb/go-congmingpay-sdk
```

## 使用

网关域名、应用 ID 和密钥由平台提供；商户须先完成授权。不要将密钥写入代码或提交到版本库。

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    congmingpay "github.com/yunkeweb/go-congmingpay-sdk"
)

func main() {
    client, err := congmingpay.NewClient(os.Getenv("CONGMINGPAY_BASE_URL"), os.Getenv("CONGMINGPAY_PROGRAM_ID"), os.Getenv("CONGMINGPAY_KEY"), nil)
    if err != nil { log.Fatal(err) }
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    resp, err := client.QueryOrder(ctx, congmingpay.Params{
        "shop_id": os.Getenv("CONGMINGPAY_SHOP_ID"),
        "order_id": "merchant-order-id",
        "ver": "3.0",
    })
    if err != nil { log.Fatal(err) }
    log.Printf("result=%s status=%s", resp.String("result_code"), resp.String("order_status"))
}
```

请求参数与文档字段名保持一致，`program_id` 和 `sign` 自动添加。金额使用字符串，例如 `"12.35"`；文档要求 JSON 数值时使用 `json.Number("12.35")`，避免浮点精度问题。可选参数不需要时应省略。请按各接口文档填写版本、回调地址及业务字段。`Response` 保留完整 JSON 字段，可用 `json.Unmarshal(resp["field"], &value)` 解码。

`error == nil` 仅表示 HTTP 和 JSON 处理成功，不表示支付成功。必须按对应接口检查 `result_code` 和订单、退款或分账状态；`processing` 表示处理中，不要将其视为失败或成功。请求超时也不表示交易未发生，先查询原订单，避免重复支付或退款。SDK 不自动重试。

支持方法：`BuyPay`、`JSNativePay`、`MicroPay`、`PrePay`、`MiniAppPay`、`QueryOrder`、`Refund`、`QueryRefund`、`CancelOrder`、`ProfitOrder`、`ProfitOrderBack`、`SearchMerchantWXApp`、`SetMerchantWXApp`、`OpenIDByAuthCode`、`UserCancelOrder`、`UnionOpenID`。新接口可使用 `Do`；只允许 `/api/` 下的相对路径，网关必须使用 HTTPS，且不会跟随重定向。

## 签名和回调安全

请求采用文档规定的排序参数和 MD5 签名。`Sign` 包含已提供的空值和 null，不包含 `sign`。嵌套数组/对象按紧凑 JSON 序列化；提供商未提供复杂对象的签名测试向量，使用分账明细等嵌套结构前须与平台联调确认。响应签名规则和无签名错误响应的处理在文档中不完整，SDK **不自动验证响应签名**；不可仅凭响应内容确认资金状态，须与平台确认验证规则并进行对账。

旧支付回调可使用 `VerifyPaymentNotification(r.URL.Query())`，退款回调使用 `VerifyRefundNotification(r.URL.Query())`。旧支付回调仅验证 `money`、`orderId`、`result_code`、`shopId`；退款回调仅验证 `money`、`order_id`、`result_code`、`shop_id`。其他字段均未经认证，包括旧支付回调中名为 `order_id` 的字段。验签后仍须核对自己的商户、订单号、金额和状态，并主动查询支付平台确认；实现幂等处理。旧支付回调处理完成后返回文档指定的 `success` 字符串。

`VerifyPaymentNotificationJSON` 按文档第 2-23 节的四字段示例验证新 JSON 回调，但文档没有明确新回调是否采用完全相同的签名规则，上线前必须与平台确认并用真实测试回调验证。分账回退回调验签规则未明确，SDK 不提供该回调的自动验签。回调 JSON 请求体应在读取前限制大小。MD5 是平台协议要求，不能为未签名字段提供完整性保护。

文档中退款回调返回格式与旧支付回调不同，接入时请与平台确认；SDK 不自动生成退款回调响应。交易及分账对账单为平台配置的文件下载流程，本文档未提供可调用的 API，因此 SDK 未封装下载功能。

## 测试

```sh
go test ./...
go test -race ./...
go vet ./...
```
