package data

import (
	"strings"
	"testing"
)

func TestIwencaiExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "JSON string literal (401 quota exhaustion)",
			body: []byte(`"您今天的次数已用完，建议升级权益"`),
			want: "您今天的次数已用完，建议升级权益",
		},
		{
			name: "JSON object with status_msg",
			body: []byte(`{"status_code":1,"status_msg":"参数错误"}`),
			want: "参数错误",
		},
		{
			name: "JSON object with msg field",
			body: []byte(`{"msg":"invalid api key"}`),
			want: "invalid api key",
		},
		{
			name: "plain text body",
			body: []byte("Service Unavailable"),
			want: "Service Unavailable",
		},
		{
			name: "empty body",
			body: []byte{},
			want: "",
		},
		{
			name: "JSON object without status fields falls back to raw",
			body: []byte(`{"foo":"bar"}`),
			want: `{"foo":"bar"}`,
		},
		{
			name: "whitespace-only body",
			body: []byte("   "),
			want: "",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := iwencaiExtractErrorMessage(tc.body)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIwencaiHTTPError(t *testing.T) {
	t.Run("with detail from JSON string body", func(t *testing.T) {
		err := iwencaiHTTPError("同花顺问财API", 401, []byte(`"您今天的次数已用完"`))
		msg := err.Error()
		for _, want := range []string{"401", "您今天的次数已用完", "同花顺问财API", "详情"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("expected %q in error, got: %s", want, msg)
			}
		}
	})

	t.Run("empty body omits detail suffix", func(t *testing.T) {
		err := iwencaiHTTPError("问财网关搜索", 502, nil)
		msg := err.Error()
		if !strings.Contains(msg, "502") {
			t.Fatalf("missing status code: %s", msg)
		}
		if strings.Contains(msg, "详情") {
			t.Fatalf("should not append detail for empty body: %s", msg)
		}
	})

	t.Run("long message is truncated", func(t *testing.T) {
		long := strings.Repeat("详情", 200)
		err := iwencaiHTTPError("prefix", 500, []byte(`"`+long+`"`))
		if !strings.HasSuffix(err.Error(), "...") {
			t.Fatalf("expected truncated suffix, got: %s", err.Error())
		}
	})
}
