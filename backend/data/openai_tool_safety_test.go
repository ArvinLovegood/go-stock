package data

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecuteToolHandlerSafelyConvertsPanicToError(t *testing.T) {
	handler := func(_ *OpenAi, _ string, _ *ToolContext) error {
		panic("calendar response shape changed")
	}

	err := executeToolHandlerSafely("GetInvestCalendar", handler, nil, `{}`, nil)

	require.ErrorContains(t, err, "GetInvestCalendar")
	require.ErrorContains(t, err, "calendar response shape changed")
}

func TestExecuteToolHandlerSafelyReturnsHandlerError(t *testing.T) {
	expected := errors.New("handler failed")
	handler := func(_ *OpenAi, _ string, _ *ToolContext) error { return expected }

	err := executeToolHandlerSafely("Example", handler, nil, `{}`, nil)

	require.ErrorIs(t, err, expected)
}
