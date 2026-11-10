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
	"unicode/utf16"
)

// Sign computes the provider's uppercase MD5 signature over all provided
// fields except sign. Empty values and nulls are included, as in the provider's
// Java sample. Omit optional fields rather than setting them to empty values.
// Use json.Number for JSON numbers whose exact decimal representation matters.
func Sign(params Params, key string) (string, error) {
	if key == "" {
		return "", errors.New("congmingpay: signing key is required")
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("congmingpay: encode signing fields: %w", err)
	}
	var m map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&m); err != nil || m == nil {
		return "", errors.New("congmingpay: signing fields must be a JSON object")
	}
	delete(m, "sign")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Java String.compareTo compares UTF-16 code units.
	sort.Slice(keys, func(i, j int) bool { return javaLess(keys[i], keys[j]) })
	pairs := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		value, err := signValue(m[k])
		if err != nil {
			return "", err
		}
		pairs = append(pairs, k+"="+value)
	}
	pairs = append(pairs, "key="+key)
	sum := md5.Sum([]byte(strings.Join(pairs, "&")))
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

func javaLess(a, b string) bool {
	x, y := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return len(x) < len(y)
}

func signValue(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case json.Number:
		return x.String(), nil
	case nil:
		return "null", nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	default:
		// The Java sample converts JSON arrays/objects via their JSON toString.
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v); err != nil {
			return "", fmt.Errorf("congmingpay: encode nested signing value: %w", err)
		}
		return strings.TrimSuffix(b.String(), "\n"), nil
	}
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
