package data

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"go-stock/backend/db"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/07/05
// @Desc 飞书自定义机器人单测
//-----------------------------------------------------------------------------------

// TestFeishuSign 验证签名算法与独立 hmac 计算结果一致
func TestFeishuSign(t *testing.T) {
	secret := ""
	timestamp := int64(1599360473)

	got := genFeishuSign(secret, timestamp)

	// 独立计算期望值：hmac-sha256(timestamp+"\n"+secret, "") → base64
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	h.Write([]byte{})
	want := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if got != want {
		t.Fatalf("genFeishuSign mismatch: got=%s want=%s", got, want)
	}
	if got == "" {
		t.Fatal("genFeishuSign returned empty sign")
	}

	// 不同 secret 应产生不同签名
	other := genFeishuSign("other-secret", timestamp)
	if other == got {
		t.Fatal("different secret should produce different sign")
	}
	// 不同 timestamp 应产生不同签名
	otherTs := genFeishuSign(secret, timestamp+1)
	if otherTs == got {
		t.Fatal("different timestamp should produce different sign")
	}
	// 同输入应幂等
	if genFeishuSign(secret, timestamp) != got {
		t.Fatal("same input should produce same sign")
	}
}

// TestFeishuMessageBuild 验证 interactive 卡片消息体结构（不实际发送）
func TestFeishuMessageBuild(t *testing.T) {
	title := "测试标题"
	message := "## 内容\n这是一条测试消息"

	card := FeishuCard{
		Schema: "2.0",
		Header: &FeishuHeader{
			Title: FeishuHeaderText{
				Tag:     "plain_text",
				Content: "go-stock " + title,
			},
		},
		Body: FeishuCardBody{
			Elements: []FeishuElement{
				{
					Tag:     "markdown",
					Content: "<at id=all></at>\n" + message,
				},
			},
		},
	}
	body := FeishuCardMessage{
		MsgType: "interactive",
		Card:    card,
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	jsonStr := string(data)

	if gjson.Get(jsonStr, "msg_type").String() != "interactive" {
		t.Fatalf("msg_type mismatch: %s", jsonStr)
	}
	// 2.0 协议必须显式声明 schema="2.0"
	if gjson.Get(jsonStr, "card.schema").String() != "2.0" {
		t.Fatalf("schema should be 2.0: %s", jsonStr)
	}
	if gjson.Get(jsonStr, "card.header.title.tag").String() != "plain_text" {
		t.Fatalf("header title tag mismatch: %s", jsonStr)
	}
	if gjson.Get(jsonStr, "card.header.title.content").String() != "go-stock "+title {
		t.Fatalf("header title content mismatch: %s", jsonStr)
	}
	// 2.0 协议元素在 body.elements 中
	elems := gjson.Get(jsonStr, "card.body.elements").Array()
	if len(elems) != 1 {
		t.Fatalf("elements length mismatch: %d", len(elems))
	}
	if elems[0].Get("tag").String() != "markdown" {
		t.Fatalf("element tag should be markdown: %s", elems[0].Raw)
	}
	if !strings.Contains(elems[0].Get("content").String(), message) {
		t.Fatalf("element content mismatch: %s", elems[0].Raw)
	}
	// 2.0 @所有人语法为 <at id=all></at>
	if !strings.Contains(elems[0].Get("content").String(), `<at id=all></at>`) {
		t.Fatalf("element content should contain <at id=all></at>: %s", elems[0].Raw)
	}
	// 未设置签名时不应有 timestamp/sign 字段
	if gjson.Get(jsonStr, "timestamp").Exists() {
		t.Fatalf("timestamp should not exist without secret: %s", jsonStr)
	}
	if gjson.Get(jsonStr, "sign").Exists() {
		t.Fatalf("sign should not exist without secret: %s", jsonStr)
	}

	// 启用签名后应包含 timestamp/sign
	bodyWithSign := body
	bodyWithSign.Timestamp = "1599360473"
	bodyWithSign.Sign = genFeishuSign("demo-secret", 1599360473)
	data2, _ := json.Marshal(bodyWithSign)
	jsonStr2 := string(data2)
	if !gjson.Get(jsonStr2, "timestamp").Exists() {
		t.Fatal("timestamp should exist with secret")
	}
	if !gjson.Get(jsonStr2, "sign").Exists() {
		t.Fatal("sign should exist with secret")
	}
}

// TestParseFeishuResponse 验证返回体解析
func TestParseFeishuResponse(t *testing.T) {
	if got := parseFeishuResponse(`{"code":0,"msg":"success"}`); got != "发送飞书消息成功" {
		t.Fatalf("success case mismatch: %s", got)
	}
	if got := parseFeishuResponse(`{"code":19021,"msg":"sign match fail or timestamp is not within one hour from current time"}`); !strings.Contains(got, "19021") || !strings.Contains(got, "sign match fail") {
		t.Fatalf("fail case mismatch: %s", got)
	}
	if got := parseFeishuResponse(`{"code":9499,"msg":"Bad Request"}`); !strings.Contains(got, "9499") {
		t.Fatalf("bad request case mismatch: %s", got)
	}
	// 兼容旧字段：code 缺失时 gjson 返回 0，按成功处理（飞书成功响应 code 本就为 0）
	if got := parseFeishuResponse(`{"StatusCode":0,"StatusMessage":"success"}`); got != "发送飞书消息成功" {
		t.Fatalf("legacy StatusCode=0 should be treated as success: %s", got)
	}
	// 非法 JSON / 空响应：code 缺失按 0 处理，视为成功（容错）
	if got := parseFeishuResponse(``); got != "发送飞书消息成功" {
		t.Fatalf("empty body should default to success: %s", got)
	}
}

// TestSendToFeishu 集成测试：从 DB 读配置发送一条测试 markdown（未配置则跳过）
func TestSendToFeishu(t *testing.T) {
	db.Init("../../data/stock.db")
	cfg := GetSettingConfig()
	if cfg == nil || strings.TrimSpace(cfg.FeishuRobot) == "" {
		t.Skip("飞书机器人未配置，跳过集成测试")
	}
	text := "# 飞书机器人集成测试\n\n这是一条来自 go-stock 的测试消息。\n\n- 项目：go-stock\n- 时间：自动生成\n\n**投资有风险，入市需谨慎**"
	result := NewFeishuAPI().SendToFeishu("测试", text)
	t.Logf("send result: %s", result)
}
