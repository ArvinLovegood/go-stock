package data

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"

	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/07/05
// @Desc 飞书自定义机器人 webhook 推送
// 文档：https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot
//-----------------------------------------------------------------------------------

type FeishuAPI struct {
	client *resty.Client
}

func NewFeishuAPI() *FeishuAPI {
	return &FeishuAPI{
		client: SharedHTTPClient,
	}
}

// SendFeishuMessage 直接 POST 原始 message 体到飞书 webhook（对齐 SendDingDingMessage，供前端测试/原始发送）
func (FeishuAPI) SendFeishuMessage(message string) string {
	cfg := GetSettingConfig()
	if cfg == nil || !cfg.FeishuPushEnable {
		return "飞书推送未开启"
	}
	if strings.TrimSpace(cfg.FeishuRobot) == "" {
		return "飞书推送未配置机器人地址"
	}
	resp, err := SharedHTTPClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(message).
		Post(cfg.FeishuRobot)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "发送飞书消息失败"
	}
	logger.SugaredLogger.Infof("send feishu message: %s", resp.String())
	return parseFeishuResponse(resp.String())
}

// SendToFeishu 构造 interactive 卡片消息（header 标题 + markdown 内容 + @所有人）发送到飞书机器人
func (f FeishuAPI) SendToFeishu(title, message string) string {
	cfg := GetSettingConfig()
	if cfg == nil || !cfg.FeishuPushEnable {
		return "飞书推送未开启"
	}
	if strings.TrimSpace(cfg.FeishuRobot) == "" {
		return "飞书推送未配置机器人地址"
	}

	message = strutil.ReplaceWithMap(message, map[string]string{
		"\\n":   "\n",
		"\\r":   "\r",
		"\\t":   "\t",
		"\\\\n": "\n",
		"\\\\r": "\r",
		"\\\\t": "\t",
	})

	// 飞书卡片 JSON 2.0 协议：
	// 必须显式声明 schema="2.0"，内容放在 body.elements 中，用 {"tag":"markdown","content":"..."} 渲染 markdown
	// 2.0 的 @所有人语法为 <at id=all></at>（与 1.0 的 <at user_id="all">所有人</at> 不同）
	// 文档：https://open.feishu.cn/document/feishu-cards/card-json-v2-components/content-components/rich-text
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

	// 可选签名校验：FeishuSecret 非空时启用
	if secret := strings.TrimSpace(cfg.FeishuSecret); secret != "" {
		ts := time.Now().Unix()
		body.Timestamp = fmt.Sprintf("%d", ts)
		body.Sign = genFeishuSign(secret, ts)
	}

	resp, err := SharedHTTPClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(&body).
		Post(cfg.FeishuRobot)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "发送飞书消息失败"
	}
	logger.SugaredLogger.Infof("send feishu message: %s", resp.String())
	return parseFeishuResponse(resp.String())
}

// genFeishuSign 飞书自定义机器人签名计算
// 规则：以 timestamp + "\n" + secret 作为签名串，用 HmacSHA256 计算空串的签名，再 Base64 编码
func genFeishuSign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, _ = h.Write([]byte{})
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// parseFeishuResponse 解析飞书返回体，code==0 为成功
func parseFeishuResponse(body string) string {
	code := int(gjson.Get(body, "code").Int())
	if code == 0 {
		return "发送飞书消息成功"
	}
	msg := gjson.Get(body, "msg").String()
	if msg == "" {
		msg = body
	}
	return fmt.Sprintf("发送飞书消息失败: code=%d msg=%s", code, msg)
}

// FeishuCardMessage 飞书自定义机器人消息体（支持 timestamp/sign 签名字段）
type FeishuCardMessage struct {
	MsgType   string     `json:"msg_type"`
	Card      FeishuCard `json:"card"`
	Timestamp string     `json:"timestamp,omitempty"`
	Sign      string     `json:"sign,omitempty"`
}

// FeishuCard 飞书卡片 JSON 2.0 结构
// 文档：https://open.feishu.cn/document/feishu-cards/card-json-v2-structure
type FeishuCard struct {
	Schema string         `json:"schema"` // 必须显式声明 "2.0"
	Header *FeishuHeader  `json:"header,omitempty"`
	Body   FeishuCardBody `json:"body"`
}

// FeishuCardBody 卡片正文容器
type FeishuCardBody struct {
	Elements []FeishuElement `json:"elements"`
}

type FeishuHeader struct {
	Title FeishuHeaderText `json:"title"`
}

type FeishuHeaderText struct {
	Tag     string `json:"tag"` // plain_text 或 lark_md
	Content string `json:"content"`
}

// FeishuElement 2.0 markdown 元素
type FeishuElement struct {
	Tag     string `json:"tag"`     // "markdown"
	Content string `json:"content"` // markdown 内容
}

type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
