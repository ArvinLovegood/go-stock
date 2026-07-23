package data

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type calendarFetcher func() ([]any, error)

// decodeJSONArrayField decodes an array field from a third-party JSON response.
// It deliberately treats a missing or null field as an error so callers never
// need unsafe type assertions on partially successful business responses.
func decodeJSONArrayField(body []byte, field string) ([]any, error) {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	raw, ok := response[field]
	if !ok || len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		message := responseMessage(response)
		if message == "" {
			message = fmt.Sprintf("响应缺少有效字段 %q", field)
		}
		return nil, fmt.Errorf("第三方接口返回异常: %s", message)
	}

	var items []any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("字段 %q 不是数组: %w", field, err)
	}
	return items, nil
}

func responseMessage(response map[string]json.RawMessage) string {
	for _, field := range []string{"msg", "message", "error"} {
		raw, ok := response[field]
		if !ok {
			continue
		}
		var message string
		if err := json.Unmarshal(raw, &message); err == nil && message != "" {
			return message
		}
	}
	return ""
}

func resolveInvestCalendar(primary, fallback calendarFetcher) (map[string]any, error) {
	items, primaryErr := primary()
	if primaryErr == nil {
		return map[string]any{
			"source": "韭研公社投资日历",
			"items":  items,
		}, nil
	}

	items, fallbackErr := fallback()
	if fallbackErr == nil {
		return map[string]any{
			"source": "财联社投资日历",
			"items":  items,
		}, nil
	}

	return nil, fmt.Errorf("投资日历主接口失败: %v; 备用接口失败: %w", primaryErr, fallbackErr)
}
