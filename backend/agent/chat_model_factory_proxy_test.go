package agent

import (
	"net/http"
	"testing"
)

func TestNewChatModelTransportDisablesEnvironmentProxy(t *testing.T) {
	transport := newChatModelTransport("")
	if transport.Proxy != nil {
		t.Fatal("expected environment proxy to be disabled when app proxy is off")
	}
}

func TestNewChatModelTransportUsesConfiguredProxy(t *testing.T) {
	transport := newChatModelTransport("http://127.0.0.1:8080")
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
	transport := newChatModelTransport("://invalid")
	if transport.Proxy != nil {
		t.Fatal("invalid proxy URL must not install a proxy")
	}
}

func TestResolveAIProxyForConfigPerItemPriority(t *testing.T) {
	// 单项关闭：即使全局代理开启也必须直连（全局地址不参与解析）
	if got := resolveAIProxyAddr(false, ""); got != "" {
		t.Fatalf("expected direct connection when per-item proxy is off, got %q", got)
	}

	// 单项开启且填了地址：仅使用单项地址
	if got := resolveAIProxyAddr(true, "http://10.0.0.1:7890"); got != "http://10.0.0.1:7890" {
		t.Fatalf("expected per-item proxy address, got %q", got)
	}

	// 单项开启但未填地址：视为直连，不回退全局代理
	if got := resolveAIProxyAddr(true, "  "); got != "" {
		t.Fatalf("expected direct connection when per-item address is empty, got %q", got)
	}
}
