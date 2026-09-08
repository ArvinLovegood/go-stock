package agent

import (
	"strings"
	"testing"

	"go-stock/backend/data"

	"github.com/cloudwego/eino/schema"
)

func TestSplitDataURL(t *testing.T) {
	mimeType, raw, err := splitDataURL("data:image/png;base64,aGVsbG8=")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("mimeType = %q, want image/png", mimeType)
	}
	if raw != "aGVsbG8=" {
		t.Fatalf("raw = %q, want aGVsbG8=", raw)
	}

	if _, _, err := splitDataURL("https://example.com/a.png"); err == nil {
		t.Fatal("http URL should be rejected")
	}
	if _, _, err := splitDataURL("data:xxx"); err == nil {
		t.Fatal("data URL without comma should be rejected")
	}
}

func TestBuildVisionImagePartsOpenAICompatible(t *testing.T) {
	cfg := &data.AIConfig{BaseUrl: "https://api.siliconflow.cn/v1", ModelName: "qwen-vl-plus"}
	httpURL := "https://img.example.com/a.png"
	parts, err := buildVisionImageParts([]string{httpURL}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0].Image == nil || parts[0].Image.URL == nil || *parts[0].Image.URL != httpURL {
		t.Fatalf("openai-compatible should pass through URL, got %+v", parts)
	}

	dataURL := "data:image/jpeg;base64,aGVsbG8="
	parts, err = buildVisionImageParts([]string{dataURL}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0].Image.URL == nil || *parts[0].Image.URL != dataURL {
		t.Fatalf("openai-compatible should pass through data URL, got %+v", parts)
	}
}

func TestBuildVisionImagePartsAnthropic(t *testing.T) {
	cfg := &data.AIConfig{BaseUrl: "https://api.anthropic.com", ModelName: "claude-sonnet-4-5"}

	// http URL → URL 字段直传
	parts, err := buildVisionImageParts([]string{"https://img.example.com/a.png"}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0].Image.URL == nil {
		t.Fatalf("anthropic http URL should use URL field, got %+v", parts)
	}

	// data URL → 拆解为 raw base64 + MIMEType
	parts, err = buildVisionImageParts([]string{"data:image/png;base64,aGVsbG8="}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	img := parts[0].Image
	if img.URL != nil {
		t.Fatalf("anthropic data URL should not use URL field")
	}
	if img.Base64Data == nil || *img.Base64Data != "aGVsbG8=" {
		t.Fatalf("base64 = %+v, want raw aGVsbG8=", img.Base64Data)
	}
	if img.MIMEType != "image/png" {
		t.Fatalf("mimeType = %q, want image/png", img.MIMEType)
	}
	if strings.HasPrefix(*img.Base64Data, "data:") {
		t.Fatalf("anthropic Base64Data must not carry data: prefix")
	}
}

func TestBuildVisionImagePartsGeminiOllamaDataURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *data.AIConfig
	}{
		{"gemini", &data.AIConfig{BaseUrl: "https://generativelanguage.googleapis.com", ModelName: "gemini-2.0-flash"}},
		{"ollama", &data.AIConfig{BaseUrl: "http://127.0.0.1:11434", ModelName: "llava:13b"}},
	} {
		parts, err := buildVisionImageParts([]string{"data:image/webp;base64,aGVsbG8="}, tc.cfg)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if len(parts) != 1 {
			t.Fatalf("%s: expected 1 part, got %d", tc.name, len(parts))
		}
		img := parts[0].Image
		if img.URL != nil {
			t.Fatalf("%s: must not use URL field", tc.name)
		}
		if img.Base64Data == nil || *img.Base64Data != "aGVsbG8=" {
			t.Fatalf("%s: base64 = %+v, want raw aGVsbG8=", tc.name, img.Base64Data)
		}
		if tc.name == "ollama" && img.MIMEType != "image/webp" {
			t.Fatalf("ollama requires MIMEType, got %q", img.MIMEType)
		}
	}
}

func TestBuildVisionImagePartsRejectsInvalid(t *testing.T) {
	cfg := &data.AIConfig{BaseUrl: "http://127.0.0.1:11434", ModelName: "llava"}
	if _, err := buildVisionImageParts([]string{"ftp://example.com/a.png"}, cfg); err == nil {
		t.Fatal("ftp URL should be rejected for gemini/ollama")
	}
}

func TestParseImagesJSON(t *testing.T) {
	if got := parseImagesJSON(""); got != nil {
		t.Fatalf("empty input should return nil, got %v", got)
	}
	if got := parseImagesJSON("not-json"); got != nil {
		t.Fatalf("invalid json should return nil, got %v", got)
	}
	got := parseImagesJSON(`["https://a.com/1.png","  ","data:image/png;base64,xx"]`)
	if len(got) != 2 {
		t.Fatalf("expected 2 valid images, got %v", got)
	}
}

// 兼容性回归：带 UserInputMultiContent 的消息不应被 validateAndFixMessages 当空消息删除
func TestValidateAndFixMessagesKeepsMultiContent(t *testing.T) {
	u := "https://img.example.com/a.png"
	msgs := []*schema.Message{
		{Role: schema.System, Content: "sys"},
		{Role: schema.User, Content: "看图", UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: "看图"},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{URL: &u},
			}},
		}},
	}
	fixed := validateAndFixMessages(msgs)
	if len(fixed) != 2 {
		t.Fatalf("multi-content user message should be kept, got %d messages", len(fixed))
	}
}
