package congmingpay

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSign(t *testing.T) {
	sum := md5.Sum([]byte("a=one&b=two&key=secret"))
	got, err := Sign(Params{"b": "two", "a": "one", "sign": "ignored"}, "secret")
	if err != nil || got != fmt.Sprintf("%X", sum) {
		t.Fatalf("signature %s: %v", got, err)
	}
}

func TestPHPCompatibleSignature(t *testing.T) {
	params := Params{"null": nil, "empty": "", "bool": false, "money": "12.30", "obj": json.RawMessage(`{"z":"中国/<>","a":1}`)}
	got, err := Sign(params, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	if got != "9CF2C8AF99BBA53E87C263C337A6EDB1" {
		t.Fatalf("PHP signature mismatch: %s", got)
	}
}
