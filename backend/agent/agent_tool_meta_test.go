package agent

import (
	"strings"
	"testing"
)

func TestWrapToolResultMetadata(t *testing.T) {
	out := wrapToolResultMetadata("GetStockInfo", "price=100", "ok")
	if !strings.HasPrefix(out, "[as_of=") {
		t.Fatalf("expected metadata prefix: %q", out)
	}
	if !strings.Contains(out, "[tool=GetStockInfo]") {
		t.Fatalf("expected tool name in metadata: %q", out)
	}
	if !strings.Contains(out, "[as_of=unknown]") || !strings.Contains(out, "[observed_at=") {
		t.Fatalf("tool invocation time must not be presented as data time: %q", out)
	}
	if !strings.Contains(out, "price=100") {
		t.Fatalf("expected body preserved: %q", out)
	}

	already := wrapToolResultMetadata("X", out, "ok")
	if already != out {
		t.Fatalf("expected no double wrap: %q", already)
	}
}

func TestWrapToolResultMetadataExtractsEvidence(t *testing.T) {
	out := wrapToolResultMetadata("GetStockInfo", "来源：行情接口\n更新时间：2026-08-13 15:00:00\n价格=100", "ok")
	if !strings.Contains(out, "[as_of=2026-08-13 15:00:00]") {
		t.Fatalf("expected data time extracted: %q", out)
	}
	if !strings.Contains(out, "[source=行情接口]") {
		t.Fatalf("expected source extracted: %q", out)
	}
}

func TestDetectToolResultStatus(t *testing.T) {
	if detectToolResultStatus("无符合条件的数据") != "empty" {
		t.Fatal("expected empty")
	}
	if detectToolResultStatus("工具调用出错: timeout") != "error" {
		t.Fatal("expected error")
	}
	if detectToolResultStatus("price=12.3") != "ok" {
		t.Fatal("expected ok")
	}
}

func TestSplitToolMetadataPrefix(t *testing.T) {
	raw := "[as_of=2026-07-04 10:00:00] [tool=GetStockInfo] [status=ok]\n**更新时间**: 2026-07-04\nbody line"
	meta, body := splitToolMetadataPrefix(raw)
	if len(meta) != 2 {
		t.Fatalf("expected 2 meta lines, got %d: %v", len(meta), meta)
	}
	if !strings.Contains(body, "body line") {
		t.Fatalf("expected body preserved: %q", body)
	}

	compressed := smartContentCompress(raw, 80)
	if !strings.HasPrefix(compressed, "[as_of=") {
		t.Fatalf("compress should preserve metadata prefix: %q", compressed)
	}
}

func TestBuildAgentTimeContext(t *testing.T) {
	ctx := buildAgentTimeContext()
	if !strings.Contains(ctx, "【当前环境】") {
		t.Fatalf("missing context header: %q", ctx)
	}
	if !strings.Contains(ctx, "本地时间") {
		t.Fatalf("missing local time: %q", ctx)
	}
	if !strings.Contains(ctx, "[as_of=") {
		t.Fatalf("missing metadata guidance: %q", ctx)
	}
}
