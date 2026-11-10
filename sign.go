package congmingpay

import (
	"bytes"
	"crypto/md5" // MD5 is required by the provider's protocol, not chosen for security.
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Sign computes the provider's uppercase MD5 signature over all provided
// fields except sign. Top-level nulls are omitted, matching the PHP SDK.
// Use json.Number for JSON numbers whose exact decimal representation matters.
func Sign(params Params, key string) (string, error) {
	if key == "" {
		return "", errors.New("congmingpay: signing key is required")
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(params)
	if err != nil {
		return "", fmt.Errorf("congmingpay: encode signing fields: %w", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil || m == nil {
		return "", errors.New("congmingpay: signing fields must be a JSON object")
	}
	delete(m, "sign")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		raw := bytes.TrimSpace(m[k])
		if bytes.Equal(raw, []byte("null")) {
			continue
		}
		value := string(raw)
		if len(raw) > 0 && raw[0] == '"' {
			if err := json.Unmarshal(raw, &value); err != nil {
				return "", err
			}
		}
		pairs = append(pairs, k+"="+value)
	}
	pairs = append(pairs, "key="+key)
	sum := md5.Sum([]byte(strings.Join(pairs, "&")))
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

func equalSignature(expected, actual string) bool {
	a, err := hex.DecodeString(actual)
	if err != nil || len(a) != md5.Size {
		return false
	}
	b, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
