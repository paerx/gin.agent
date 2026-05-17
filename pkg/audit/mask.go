package audit

import (
	"fmt"
	"strings"
)

var defaultSensitiveKeys = map[string]struct{}{
	"password":      {},
	"secret":        {},
	"token":         {},
	"api_key":       {},
	"access_token":  {},
	"refresh_token": {},
	"private_key":   {},
	"phone":         {},
	"email":         {},
	"id_card":       {},
}

func MaskSensitiveMap(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	out := make(map[string]any, len(data))
	for key, value := range data {
		lower := strings.ToLower(key)
		if _, ok := defaultSensitiveKeys[lower]; ok {
			out[key] = "***"
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			out[key] = MaskSensitiveMap(typed)
		case []any:
			masked := make([]any, 0, len(typed))
			for _, item := range typed {
				if itemMap, ok := item.(map[string]any); ok {
					masked = append(masked, MaskSensitiveMap(itemMap))
				} else {
					masked = append(masked, item)
				}
			}
			out[key] = masked
		case string:
			out[key] = maskTextValue(lower, typed)
		default:
			out[key] = value
		}
	}
	return out
}

func maskTextValue(key, value string) string {
	switch key {
	case "wallet":
		if len(value) > 10 {
			return value[:6] + "..." + value[len(value)-4:]
		}
	case "phone":
		if len(value) >= 7 {
			return value[:3] + "****" + value[len(value)-4:]
		}
	case "email":
		if at := strings.IndexByte(value, '@'); at > 1 {
			return value[:1] + "***" + value[at:]
		}
	}
	return value
}

func MaskStringMap(data map[string]string) map[string]string {
	if data == nil {
		return nil
	}
	out := make(map[string]string, len(data))
	for k, v := range data {
		out[k] = fmt.Sprintf("%v", MaskSensitiveMap(map[string]any{k: v})[k])
	}
	return out
}
