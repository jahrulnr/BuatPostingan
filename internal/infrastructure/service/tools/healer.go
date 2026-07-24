package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// healArgs coerces string args to number/bool per JSON Schema properties (weak models).
func healArgs(args map[string]any, parameters map[string]any) map[string]any {
	props, _ := parameters["properties"].(map[string]any)
	if props == nil {
		return args
	}
	out := make(map[string]any, len(args))
	for key, value := range args {
		schema, _ := props[key].(map[string]any)
		out[key] = healValue(value, schema)
	}
	return out
}

func healValue(value any, schema map[string]any) any {
	s, ok := value.(string)
	if !ok || schema == nil {
		return value
	}
	types := schemaTypes(schema["type"])
	trimmed := strings.TrimSpace(s)

	if hasType(types, "integer") && isIntString(trimmed) {
		n, err := strconv.Atoi(trimmed)
		if err == nil {
			return n
		}
	}
	if hasType(types, "number") {
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return f
		}
	}
	if hasType(types, "boolean") {
		switch strings.ToLower(trimmed) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	if hasType(types, "null") && strings.ToLower(trimmed) == "null" {
		return nil
	}
	return value
}

func schemaTypes(t any) []string {
	switch v := t.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func hasType(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func isIntString(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '+' || s[0] == '-' {
		i = 1
	}
	if i >= len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10)
		}
		return fmt.Sprint(x)
	case json.Number:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func asInt(v any, def int) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return int(n)
		}
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil {
			return n
		}
	}
	return def
}

func asBool(v any, def bool) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
