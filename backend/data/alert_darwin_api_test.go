//go:build darwin
// +build darwin

package data

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// @Author 2lovecode
// @Date 2025/02/06 17:50
// @Desc
// -----------------------------------------------------------------------------------

func TestNewAlertWindowsApiDarwin(t *testing.T) {
	alert := NewAlertWindowsApi("go-stock", "Hello, World!", "This is a notification.", "../../build/appicon.png")

	require.Equal(t, "go-stock", alert.AppID)
	require.Equal(t, "Hello, World!", alert.Title)
	require.Equal(t, "This is a notification.", alert.Content)
}
