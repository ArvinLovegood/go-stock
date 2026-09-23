package data

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-stock/backend/logger"

	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
)

// @Author spark
// @Date 2025/1/3 13:53
// @Desc
//-----------------------------------------------------------------------------------

// 钉钉机器人错误码
const (
	// errcodeAtAllForbidden 群开启了「仅群主可@所有人」，带 isAtAll 的消息会被整条拒绝
	errcodeAtAllForbidden = 450103
	// errcodeOK 发送成功
	errcodeOK = 0
)

type DingDingAPI struct {
	client *resty.Client
}

func NewDingDingAPI() *DingDingAPI {
	return &DingDingAPI{
		client: SharedHTTPClient,
	}
}

// SendDingDingMessage 发送钉钉消息，message 支持两种格式：
// 1. 完整的钉钉消息 JSON 字符串（msgtype/markdown/at）
// 2. 纯 markdown 文本，自动包装成 markdown 消息
func (DingDingAPI) SendDingDingMessage(message string) string {
	if GetSettingConfig().DingPushEnable == false {
		//logger.SugaredLogger.Info("钉钉推送未开启")
		return "钉钉推送未开启"
	}
	msg, err := parseDingMessage(message)
	if err != nil {
		logger.SugaredLogger.Errorf("钉钉消息格式错误: %s", err.Error())
		return "钉钉消息格式错误"
	}
	return postDingMessage(msg, getApiURL())
}

// SendDingDingMessageByRobot 使用指定的机器人地址发送消息
// 供设置页「发送测试通知」使用，直接使用页面当前填写的地址，不读取数据库配置
func (DingDingAPI) SendDingDingMessageByRobot(message, robot string) string {
	robot = strings.TrimSpace(robot)
	if robot == "" {
		return "钉钉推送未配置机器人地址"
	}
	msg, err := parseDingMessage(message)
	if err != nil {
		logger.SugaredLogger.Errorf("钉钉消息格式错误: %s", err.Error())
		return "钉钉消息格式错误"
	}
	return postDingMessage(msg, robot)
}

// SendToDingDing 以 markdown 格式推送钉钉消息
func (DingDingAPI) SendToDingDing(title, message string) string {
	message = strutil.ReplaceWithMap(message, map[string]string{
		"\\n":   "\n",
		"\\r":   "\r",
		"\\t":   "\t",
		"\\\\n": "\n",
		"\\\\r": "\r",
		"\\\\t": "\t",
	})
	return postDingMessage(&Message{
		Msgtype: "markdown",
		Markdown: Markdown{
			Title: "go-stock " + title,
			Text:  message,
		},
		At: At{
			IsAtAll: true,
		},
	}, getApiURL())
}

func getApiURL() string {
	return GetSettingConfig().DingRobot
}

// parseDingMessage 解析钉钉消息：合法 JSON 直接使用，否则按纯 markdown 文本包装
func parseDingMessage(message string) (*Message, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return nil, fmt.Errorf("消息内容为空")
	}
	if strings.HasPrefix(trimmed, "{") {
		var msg Message
		if err := json.Unmarshal([]byte(trimmed), &msg); err == nil && msg.Msgtype != "" {
			if msg.Msgtype == "markdown" && msg.Markdown.Text == "" {
				return nil, fmt.Errorf("markdown.text 为空")
			}
			return &msg, nil
		}
	}
	return &Message{
		Msgtype: "markdown",
		Markdown: Markdown{
			Title: "go-stock " + dingTitle(trimmed),
			Text:  trimmed,
		},
		At: At{
			IsAtAll: true,
		},
	}, nil
}

// dingTitle 从 markdown 文本首行提取通知标题
func dingTitle(text string) string {
	line := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	line = strings.TrimSpace(strings.TrimLeft(line, "#*>- "))
	if line == "" {
		return "消息通知"
	}
	if r := []rune(line); len(r) > 32 {
		line = string(r[:32])
	}
	return line
}

// postDingMessage 发送消息到指定机器人地址并校验钉钉返回的 errcode；
// 若因群限制「仅群主可@所有人」被拒，则自动降级为不 @ 任何人重发一次
func postDingMessage(msg *Message, robot string) string {
	result, err := doPostDingMessage(msg, robot)
	if err != nil {
		logger.SugaredLogger.Errorf("发送钉钉消息失败: %s", err.Error())
		return "发送钉钉消息失败：" + err.Error()
	}
	if result.Errcode == errcodeOK {
		return "发送钉钉消息成功"
	}
	if result.Errcode == errcodeAtAllForbidden && msg.At.IsAtAll {
		logger.SugaredLogger.Warnf("钉钉群限制@所有人，改为不@任何人重试: %s", result.Errmsg)
		retry := *msg
		retry.At = At{}
		retryResult, retryErr := doPostDingMessage(&retry, robot)
		if retryErr == nil && retryResult.Errcode == errcodeOK {
			return "发送钉钉消息成功"
		}
		if retryErr != nil {
			logger.SugaredLogger.Errorf("发送钉钉消息失败: %s", retryErr.Error())
			return "发送钉钉消息失败：" + retryErr.Error()
		}
		result = retryResult
	}
	logger.SugaredLogger.Errorf("发送钉钉消息失败: %s", result.String())
	return "发送钉钉消息失败：" + result.Errmsg
}

func doPostDingMessage(msg *Message, robot string) (dingDingResp, error) {
	resp, err := SharedHTTPClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(msg).
		Post(robot)
	if err != nil {
		return dingDingResp{}, err
	}
	logger.SugaredLogger.Infof("send dingding message: %s", resp.String())
	return parseDingResp(resp.String()), nil
}

func parseDingResp(body string) dingDingResp {
	var result dingDingResp
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &result); err != nil {
		logger.SugaredLogger.Errorf("解析钉钉响应失败: %s", body)
		return dingDingResp{Errcode: -1, Errmsg: "响应解析失败"}
	}
	return result
}

type dingDingResp struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

func (r dingDingResp) String() string {
	return fmt.Sprintf("errcode %d, errmsg %s", r.Errcode, r.Errmsg)
}

type Message struct {
	Msgtype  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
	At       At       `json:"at"`
}

type Markdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type At struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll"`
}