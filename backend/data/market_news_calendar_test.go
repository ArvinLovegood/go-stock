package data

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeJSONArrayFieldRejectsBusinessErrorWithoutPanicking(t *testing.T) {
	body := []byte(`{"errCode":"9","msg":"token无效，请重试","serverTime":1784769598}`)

	items, err := decodeJSONArrayField(body, "data")

	require.ErrorContains(t, err, "token无效，请重试")
	require.Empty(t, items)
}

func TestDecodeJSONArrayFieldReturnsItems(t *testing.T) {
	body := []byte(`{"code":200,"data":[{"title":"政策发布"}]}`)

	items, err := decodeJSONArrayField(body, "data")

	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestDecodeJSONArrayFieldRejectsMalformedAndWrongShapeResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `not-json`},
		{name: "object instead of array", body: `{"data":{"title":"政策发布"}}`},
		{name: "missing data", body: `{"code":200}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := decodeJSONArrayField([]byte(tt.body), "data")

			require.Error(t, err)
			require.Empty(t, items)
		})
	}
}

func TestResolveInvestCalendarFallsBackToCLS(t *testing.T) {
	result, err := resolveInvestCalendar(
		func() ([]any, error) { return nil, errors.New("primary failed") },
		func() ([]any, error) { return []any{map[string]any{"calendar_day": "2026-07-23"}}, nil },
	)

	require.NoError(t, err)
	require.Equal(t, "财联社投资日历", result["source"])
	require.Len(t, result["items"], 1)
}

func TestResolveInvestCalendarReturnsCombinedFailure(t *testing.T) {
	result, err := resolveInvestCalendar(
		func() ([]any, error) { return nil, errors.New("primary failed") },
		func() ([]any, error) { return nil, errors.New("fallback failed") },
	)

	require.ErrorContains(t, err, "primary failed")
	require.ErrorContains(t, err, "fallback failed")
	require.Nil(t, result)
}
