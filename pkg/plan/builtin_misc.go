package plan

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"regexp"
	"strings"
)

func init() {
	registerMiscBuiltins()
}

func registerMiscBuiltins() {
	// Regex
	RegisterFunc(&FuncDef{
		Name: "REGEXP_REPLACE", Type: FuncScalar, MinArgs: 3, MaxArgs: 3,
		ScalarFn: fnRegexpReplace,
	})
	RegisterFunc(&FuncDef{
		Name: "REGEXP_EXTRACT", Type: FuncScalar, MinArgs: 2, MaxArgs: 3,
		ScalarFn: fnRegexpExtract,
	})
	RegisterFunc(&FuncDef{
		Name: "REGEXP_LIKE", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnRegexpLike,
	})
	// Encoding
	RegisterFunc(&FuncDef{
		Name: "BASE64", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnBase64,
	})
	RegisterFunc(&FuncDef{
		Name: "UNBASE64", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnUnbase64,
	})
	RegisterFunc(&FuncDef{
		Name: "HEX", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnHex,
	})
	RegisterFunc(&FuncDef{
		Name: "UNHEX", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnUnhex,
	})
	// Hash
	RegisterFunc(&FuncDef{
		Name: "MD5", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnMd5,
	})
	RegisterFunc(&FuncDef{
		Name: "SHA1", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnSha1,
	})
	RegisterFunc(&FuncDef{
		Name: "SHA2", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnSha2,
	})
	RegisterFunc(&FuncDef{
		Name: "CRC32", Type: FuncScalar, MinArgs: 1, MaxArgs: 1,
		ScalarFn: fnCrc32,
	})
	RegisterFunc(&FuncDef{
		Name: "HASH", Type: FuncScalar, MinArgs: 1, MaxArgs: -1,
		ScalarFn: fnHash,
	})
	// Encoding (charset)
	RegisterFunc(&FuncDef{
		Name: "ENCODE", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnEncode,
	})
	RegisterFunc(&FuncDef{
		Name: "DECODE", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnDecode,
	})
	// JSON
	RegisterFunc(&FuncDef{
		Name: "GET_JSON_OBJECT", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnGetJsonObject,
	})
	RegisterFunc(&FuncDef{
		Name: "JSON_TUPLE", Type: FuncScalar, MinArgs: 2, MaxArgs: -1,
		ScalarFn: fnJsonTuple,
	})
	// Misc
	RegisterFunc(&FuncDef{
		Name: "CURRENT_USER", Type: FuncScalar, MinArgs: 0, MaxArgs: 0,
		ScalarFn: fnCurrentUser,
	})
	RegisterFunc(&FuncDef{
		Name: "CURRENT_DATABASE", Type: FuncScalar, MinArgs: 0, MaxArgs: 0,
		ScalarFn: fnCurrentDatabase,
	})
	RegisterFunc(&FuncDef{
		Name: "VERSION", Type: FuncScalar, MinArgs: 0, MaxArgs: 0,
		ScalarFn: fnVersion,
	})
	// Mask
	RegisterFunc(&FuncDef{
		Name: "MASK", Type: FuncScalar, MinArgs: 1, MaxArgs: 4,
		ScalarFn: fnMask,
	})
	RegisterFunc(&FuncDef{
		Name: "MASK_FIRST_N", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMaskFirstN,
	})
	RegisterFunc(&FuncDef{
		Name: "MASK_LAST_N", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMaskLastN,
	})
	RegisterFunc(&FuncDef{
		Name: "MASK_SHOW_FIRST_N", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMaskShowFirstN,
	})
	RegisterFunc(&FuncDef{
		Name: "MASK_SHOW_LAST_N", Type: FuncScalar, MinArgs: 2, MaxArgs: 2,
		ScalarFn: fnMaskShowLastN,
	})
}

// --- Regex ---

func fnRegexpReplace(args []string) string {
	if len(args) < 3 {
		return ""
	}
	re, err := regexp.Compile(args[1])
	if err != nil {
		return args[0]
	}
	return re.ReplaceAllString(args[0], args[2])
}

func fnRegexpExtract(args []string) string {
	if len(args) < 2 {
		return ""
	}
	re, err := regexp.Compile(args[1])
	if err != nil {
		return ""
	}
	matches := re.FindStringSubmatch(args[0])
	if len(matches) == 0 {
		return ""
	}
	idx := 1
	if len(args) >= 3 {
		if i, err := fmt.Sscanf(args[2], "%d", &idx); err != nil || i == 0 {
			idx = 1
		}
	}
	if idx >= len(matches) {
		return ""
	}
	return matches[idx]
}

func fnRegexpLike(args []string) string {
	if len(args) < 2 {
		return "false"
	}
	re, err := regexp.Compile(args[1])
	if err != nil {
		return "false"
	}
	if re.MatchString(args[0]) {
		return "true"
	}
	return "false"
}

// --- Encoding ---

func fnBase64(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(args[0]))
}

func fnUnbase64(args []string) string {
	if len(args) < 1 {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return args[0]
	}
	return string(decoded)
}

func fnHex(args []string) string {
	if len(args) < 1 {
		return ""
	}
	return hex.EncodeToString([]byte(args[0]))
}

func fnUnhex(args []string) string {
	if len(args) < 1 {
		return ""
	}
	decoded, err := hex.DecodeString(args[0])
	if err != nil {
		return args[0]
	}
	return string(decoded)
}

// --- Hash ---

func fnMd5(args []string) string {
	if len(args) < 1 {
		return ""
	}
	h := md5.Sum([]byte(args[0]))
	return hex.EncodeToString(h[:])
}

func fnSha1(args []string) string {
	if len(args) < 1 {
		return ""
	}
	h := sha1.Sum([]byte(args[0]))
	return hex.EncodeToString(h[:])
}

func fnSha2(args []string) string {
	if len(args) < 2 {
		return ""
	}
	var h hash.Hash
	switch args[1] {
	case "224":
		h = sha256.New224()
	case "256", "0":
		h = sha256.New()
	case "384":
		h = sha512.New384()
	case "512":
		h = sha512.New()
	default:
		h = sha256.New()
	}
	h.Write([]byte(args[0]))
	return hex.EncodeToString(h.Sum(nil))
}

func fnCrc32(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	return fmt.Sprintf("%d", crc32.ChecksumIEEE([]byte(args[0])))
}

func fnHash(args []string) string {
	if len(args) < 1 {
		return "0"
	}
	// Simple hash: sum of character codes
	var h int64
	for _, s := range args {
		for _, c := range s {
			h = h*31 + int64(c)
		}
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%d", h)
}

// --- Encoding (charset) ---

func fnEncode(args []string) string {
	if len(args) < 2 {
		return args[0]
	}
	// Hive ENCODE(str, charset) - simple hex representation for non-UTF8 charsets
	charset := strings.ToUpper(args[1])
	switch charset {
	case "UTF-8", "UTF8":
		return args[0]
	default:
		// For other charsets, return hex representation
		return hex.EncodeToString([]byte(args[0]))
	}
}

func fnDecode(args []string) string {
	if len(args) < 2 {
		return args[0]
	}
	charset := strings.ToUpper(args[1])
	switch charset {
	case "UTF-8", "UTF8":
		return args[0]
	default:
		// Try to decode as hex
		decoded, err := hex.DecodeString(args[0])
		if err != nil {
			return args[0]
		}
		return string(decoded)
	}
}

// --- JSON ---

func fnGetJsonObject(args []string) string {
	if len(args) < 2 {
		return ""
	}
	jsonStr := args[0]
	path := args[1]
	// Simple implementation: extracts top-level key values
	// Path format: $.key or $.nested.key
	if !strings.HasPrefix(path, "$.") {
		return jsonStr
	}
	keyPath := strings.TrimPrefix(path, "$.")
	keys := strings.Split(keyPath, ".")
	return extractJSONPath(jsonStr, keys)
}

func fnJsonTuple(args []string) string {
	if len(args) < 2 {
		return ""
	}
	// Return first requested key's value (simplified: single value, not tuple)
	return fnGetJsonObject([]string{args[0], "$." + args[1]})
}

func extractJSONPath(jsonStr string, keys []string) string {
	if len(keys) == 0 {
		return jsonStr
	}
	current := strings.TrimSpace(jsonStr)
	for _, key := range keys {
		// Find "key": in JSON
		search := `"` + key + `":`
		idx := strings.Index(current, search)
		if idx < 0 {
			return ""
		}
		valStart := idx + len(search)
		remaining := current[valStart:]
		remaining = strings.TrimLeft(remaining, " ")
		if len(remaining) == 0 {
			return ""
		}
		if remaining[0] == '"' {
			// String value
			end := strings.Index(remaining[1:], `"`)
			if end < 0 {
				return ""
			}
			current = remaining[1 : end+1]
		} else if remaining[0] == '{' {
			// Nested object
			depth := 0
			for i, c := range remaining {
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
					if depth == 0 {
						current = remaining[:i+1]
						break
					}
				}
			}
		} else if remaining[0] == '[' {
			// Array
			depth := 0
			for i, c := range remaining {
				if c == '[' {
					depth++
				} else if c == ']' {
					depth--
					if depth == 0 {
						current = remaining[:i+1]
						break
					}
				}
			}
		} else {
			// Number or boolean
			end := strings.IndexAny(remaining, ",}\n")
			if end < 0 {
				current = strings.TrimRight(remaining, " ")
			} else {
				current = strings.TrimSpace(remaining[:end])
			}
		}
	}
	return current
}

// --- Misc ---

func fnCurrentUser([]string) string {
	return "gsql_user"
}

func fnCurrentDatabase([]string) string {
	return "default"
}

func fnVersion([]string) string {
	return "gsql 0.1.0"
}

// --- Mask ---

func fnMask(args []string) string {
	if len(args) < 1 {
		return ""
	}
	upper := "X"
	lower := "x"
	digit := "n"
	if len(args) >= 2 {
		upper = args[1]
	}
	if len(args) >= 3 {
		lower = args[2]
	}
	if len(args) >= 4 {
		digit = args[3]
	}
	return maskString(args[0], upper, lower, digit)
}

func fnMaskFirstN(args []string) string {
	if len(args) < 2 {
		return ""
	}
	n, err := parseInt(args[1])
	if err != nil || n <= 0 {
		return args[0]
	}
	s := args[0]
	if n > len(s) {
		n = len(s)
	}
	return maskString(s[:n], "X", "x", "n") + s[n:]
}

func fnMaskLastN(args []string) string {
	if len(args) < 2 {
		return ""
	}
	n, err := parseInt(args[1])
	if err != nil || n <= 0 {
		return args[0]
	}
	s := args[0]
	if n > len(s) {
		n = len(s)
	}
	return s[:len(s)-n] + maskString(s[len(s)-n:], "X", "x", "n")
}

func fnMaskShowFirstN(args []string) string {
	if len(args) < 2 {
		return ""
	}
	n, err := parseInt(args[1])
	if err != nil || n <= 0 {
		return args[0]
	}
	s := args[0]
	if n > len(s) {
		n = len(s)
	}
	return s[:n] + maskString(s[n:], "X", "x", "n")
}

func fnMaskShowLastN(args []string) string {
	if len(args) < 2 {
		return ""
	}
	n, err := parseInt(args[1])
	if err != nil || n <= 0 {
		return args[0]
	}
	s := args[0]
	if n > len(s) {
		n = len(s)
	}
	return maskString(s[:len(s)-n], "X", "x", "n") + s[len(s)-n:]
}

func maskString(s, upperChar, lowerChar, digitChar string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result.WriteString(upperChar)
		} else if c >= 'a' && c <= 'z' {
			result.WriteString(lowerChar)
		} else if c >= '0' && c <= '9' {
			result.WriteString(digitChar)
		} else {
			result.WriteRune(c)
		}
	}
	return result.String()
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
