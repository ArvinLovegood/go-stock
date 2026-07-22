//go:build darwin
// +build darwin

package data

import (
	"testing"
)

// @Author 2lovecode
// @Date 2025/02/06 17:50
// @Desc
// -----------------------------------------------------------------------------------

func TestAlert(t *testing.T) {
	notification := NewAlertWindowsApi("go-stock", "Hello, World!", "This is a native notification.", "../../build/appicon.png")
	if notification.AppID != "go-stock" || notification.Title != "Hello, World!" {
		t.Fatalf("unexpected notification: %+v", notification)
	}
}
