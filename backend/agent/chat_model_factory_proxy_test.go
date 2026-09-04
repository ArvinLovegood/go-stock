package agent

import (
	"net/http"
	"testing"
)

func TestNewChatModelTransportDisablesEnvironmentProxy(t *testing.T) {
	transport, err := newChatModelTransport(false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.Proxy != nil {
		t.Fatal("expected environment proxy to be disabled when app proxy is off")
	}
}

func TestNewChatModelTransportUsesConfiguredProxy(t *testing.T) {
	transport, err := newChatModelTransport(true, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.Proxy == nil {
		t.Fatal("expected configured proxy to be enabled")
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("resolve proxy: %v", err)
	}
	if proxyURL.String() != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected proxy URL: %s", proxyURL)
	}
}

func TestNewChatModelTransportRejectsInvalidProxy(t *testing.T) {
	transport, err := newChatModelTransport(true, "://invalid")
	if err == nil {
		t.Fatal("expected invalid proxy URL to return an error")
	}
	if transport.Proxy != nil {
		t.Fatal("invalid proxy URL must not install a proxy")
	}
}
