package data

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	return NewFeishuAPI().SendFeishuMessageByRobot(message, cfg.FeishuRobot, cfg.FeishuSecret)
}

// SendFeishuMessageByRobot 使用指定的机器人地址与签名密钥发送原始 message 体
// 供设置页「发送测试通知」使用，直接使用页面当前填写的地址，不读取数据库配置
func (FeishuAPI) SendFeishuMessageByRobot(message, robot, secret string) string {
	robot = strings.TrimSpace(robot)
	if robot == "" {
		return "飞书推送未配置机器人地址"
	}
	body := strings.TrimSpace(message)
	if !gjson.Valid(body) {
		logger.SugaredLogger.Errorf("飞书消息格式错误: %s", body)
		return "飞书消息格式错误"
	}
	// secret 非空时注入签名（timestamp + sign），与 SendToFeishu 保持一致
	if secret = strings.TrimSpace(secret); secret != "" {
		var payload map[string]any
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			logger.SugaredLogger.Errorf("飞书消息格式错误: %s", err.Error())
			return "飞书消息格式错误"
		}
		ts := time.Now().Unix()
		payload["timestamp"] = fmt.Sprintf("%d", ts)
		payload["sign"] = genFeishuSign(secret, ts)
		signed, err := json.Marshal(payload)
		if err != nil {
			logger.SugaredLogger.Errorf("飞书消息签名失败: %s", err.Error())
			return "飞书消息格式错误"
		}
		body = string(signed)
	}
	return postFeishuMessage(body, robot)
}

// feishuAtAllMark 飞书卡片 2.0 的 @所有人 标记
const feishuAtAllMark = "<at id=all></at>"

// postFeishuMessage 发送飞书消息体（JSON 字符串）；
// 消息默认带 @所有人，若发送失败（常见于机器人或群未开放 @所有人 权限）则去掉 @所有人 重试一次
func postFeishuMessage(body, robot string) string {
	resp, err := doPostFeishuMessage(body, robot)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return "发送飞书消息失败：" + err.Error()
	}
	result := resp.String()
	logger.SugaredLogger.Infof("send feishu message: %s", result)
	if feishuCode(result) == 0 || !strings.Contains(body, feishuAtAllMark) {
		return parseFeishuResponse(result)
	}
	logger.SugaredLogger.Warnf("飞书消息发送失败，去掉@所有人重试: %s", result)
	retryResp, retryErr := doPostFeishuMessage(strings.ReplaceAll(body, feishuAtAllMark, ""), robot)
	if retryErr != nil {
		logger.SugaredLogger.Error(retryErr.Error())
		return "发送飞书消息失败：" + retryErr.Error()
	}
	retryResult := retryResp.String()
	logger.SugaredLogger.Infof("send feishu message(no at all): %s", retryResult)
	if feishuCode(retryResult) == 0 {
		return "发送飞书消息成功"
	}
	return parseFeishuResponse(retryResult)
}

func doPostFeishuMessage(body, robot string) (*resty.Response, error) {
	return SharedHTTPClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(robot)
}

func feishuCode(body string) int {
	return int(gjson.Get(body, "code").Int())
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

	payload, err := json.Marshal(&body)
	if err != nil {
		logger.SugaredLogger.Errorf("飞书消息格式错误: %s", err.Error())
		return "飞书消息格式错误"
	}
	return postFeishuMessage(string(payload), cfg.FeishuRobot)
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
