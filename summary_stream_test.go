package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryStreamFailureRecognizesFatalAndCodeZero(t *testing.T) {
	require.Equal(t, "panic", summaryStreamFailure(map[string]any{
		"fatal": true,
		"error": "panic",
	}))
	require.Equal(t, "upstream failed", summaryStreamFailure(map[string]any{
		"code":    0,
		"content": "upstream failed",
	}))
	require.Empty(t, summaryStreamFailure(map[string]any{
		"code":    1,
		"content": "report",
	}))
}
