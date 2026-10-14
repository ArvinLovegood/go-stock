package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
)

type THSTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type AiResponse struct {
	Id          string `json:"id"`
	Object      string `json:"object"`
	Created     int    `json:"created"`
	Model       string `json:"model"`
	ServiceTier string `json:"service_tier"`
	Choices     []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			Role      string `json:"role"`
			ToolCalls []struct {
				Function struct {
					Arguments string `json:"arguments"`
					Name      string `json:"name"`
				} `json:"function"`
				Id    string `json:"id"`
				Index int    `json:"index"`
				Type  string `json:"type"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens          int `json:"prompt_tokens"`
		CompletionTokens      int `json:"completion_tokens"`
		TotalTokens           int `json:"total_tokens"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	} `json:"usage"`
	SystemFingerprint string `json:"system_fingerprint"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type FunctionParameters struct {
	Type                 string         `json:"type"`
	Properties           map[string]any `json:"properties"`
	Required             []string       `json:"required,omitempty"`
	AdditionalProperties bool           `json:"additionalProperties"`
}

type ToolFunction struct {
	Name        string              `json:"name"`
	Strict      bool                `json:"strict,omitempty"`
	Description string              `json:"description"`
	Parameters  *FunctionParameters `json:"parameters,omitempty"`
}

// appendToolMessages 统一向 messages 追加一次工具调用的 assistant/tool 两条消息
func appendToolMessages(
	messages *[]map[string]any,
	currentAIContent, reasoningContent, callID, funcName, funcArgs, toolContent string,
) {
	assistantMsg := map[string]any{
		"role":    "assistant",
		"content": currentAIContent,
		"tool_calls": []map[string]any{
			{
				"id": callID,
				//"tool_call_id": callID,
				"type": "function",
				"function": map[string]string{
					"name":      funcName,
					"arguments": funcArgs,
					//"parameters": funcArgs,
				},
			},
		},
	}
	if reasoningContent != "" {
		assistantMsg["reasoning_content"] = reasoningContent
	}
	*messages = append(*messages, assistantMsg)

	*messages = append(*messages, map[string]any{
		"role":         "tool",
		"content":      toolContent,
		"tool_call_id": callID,
	})
}

// thsResultToMarkdown 将通联/同花顺搜索结果统一转换为 markdown 表格
func thsResultToMarkdown(res map[string]any, title string) string {
	if convertor.ToString(res["code"]) != "100" {
		return "无符合条件的数据"
	}

	resData, ok := res["data"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}

	dataList, ok := result["dataList"].([]any)
	if !ok {
		return "无符合条件的数据"
	}
	columns, ok := result["columns"].([]any)
	if !ok {
		return "无符合条件的数据"
	}

	headers := map[string]string{}
	for _, v := range columns {
		d := v.(map[string]any)
		colTitle := convertor.ToString(d["title"])
		if dm := convertor.ToString(d["dateMsg"]); dm != "" {
			colTitle += "[" + dm + "]"
		}
		if u := convertor.ToString(d["unit"]); u != "" {
			colTitle += "(" + u + ")"
		}
		headers[d["key"].(string)] = colTitle
	}

	table := &[]map[string]any{}
	for _, v := range dataList {
		d := v.(map[string]any)
		row := map[string]any{}
		for key, colTitle := range headers {
			row[colTitle] = convertor.ToString(d[key])
		}
		*table = append(*table, row)
	}

	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	return "\r\n### " + title + "：\r\n" + markdownTable + "\r\n"
}

func stripReasoningContent(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		newMsg := make(map[string]interface{})
		for k, v := range msg {
			if k == "reasoning_content" {
				continue
			}
			newMsg[k] = v
		}
		result = append(result, newMsg)
	}
	return result
}

func hasReasoningContent(messages []map[string]interface{}) bool {
	for _, msg := range messages {
		if _, ok := msg["reasoning_content"]; ok {
			return true
		}
	}
	return false
}

func stripToolMessages(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, message := range messages {
		if message["role"] == "tool" {
			continue
		}
		if _, ok := message["tool_calls"]; ok {
			continue
		}
		result = append(result, message)
	}
	return result
}

func removeRequestField(messages []map[string]interface{}, field string) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		newMsg := make(map[string]interface{})
		for k, v := range msg {
			if k == field {
				continue
			}
			newMsg[k] = v
		}
		result = append(result, newMsg)
	}
	return result
}

func openAIHTTPErrorMessage(statusCode int, bodyBytes []byte) string {
	bodyText := strings.TrimSpace(string(bodyBytes))
	if bodyText == "" {
		return fmt.Sprintf("HTTP %d", statusCode)
	}

	res := &models.Resp{}
	if err := json.Unmarshal(bodyBytes, res); err == nil {
		if res.Error.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, res.Error.Message)
		}
		if res.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, res.Message)
		}
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, bodyText)
}

func isUnsupportedThinkingError(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "thinking") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "not_supported") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "unknown") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "extra")
}

func isUnsupportedReasoningContentError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "reasoning_content") ||
		strings.Contains(lower, "reasoning content")
}

func isUnsupportedToolError(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "tool") && !strings.Contains(lower, "function") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "not_supported") ||
		strings.Contains(lower, "invalid parameter") ||
		strings.Contains(lower, "unknown parameter") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "extra")
}

// isUnsupportedMaxTokensError 识别"该模型要求使用 max_completion_tokens"类错误。
// OpenAI 推理模型（gpt/o 系列）携带 max_tokens 参数会返回 400 并提示改用 max_completion_tokens。
func isUnsupportedMaxTokensError(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "max_tokens") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "not_supported") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "unrecognized")
}

// inlineStreamError 提取 SSE data 帧中的内嵌错误消息。
// 实测部分中转/网关（如 aixlau.me 的 gpt-6.1-sol）在上游过载或参数出错时，
// 不返回 HTTP 4xx/5xx，而是返回 HTTP 200 的 SSE 流，并在流中下发无 choices 字段的错误帧：
//
//	data: {"error":{"message":"Our servers are currently overloaded...","type":"upstream_error"}}
//
// 此类帧若不识别，解析器将静默跳过，前端长时间无任何输出。
// 返回空串表示该帧不是错误帧（正常 chunk 均携带 choices 字段，直接放行）。
func inlineStreamError(data string) string {
	var probe map[string]any
	if err := json.Unmarshal([]byte(data), &probe); err != nil {
		return ""
	}
	if _, hasChoices := probe["choices"]; hasChoices {
		return ""
	}
	switch e := probe["error"].(type) {
	case map[string]any:
		if m, _ := e["message"].(string); m != "" {
			return m
		}
	case string:
		if e != "" {
			return e
		}
	}
	if m, _ := probe["message"].(string); m != "" {
		return m
	}
	return ""
}

func isUnsupportedRequestFieldError(msg string, fields ...string) bool {
	lower := strings.ToLower(msg)
	for _, field := range fields {
		if strings.Contains(lower, strings.ToLower(field)) {
			return strings.Contains(lower, "unsupported") ||
				strings.Contains(lower, "not support") ||
				strings.Contains(lower, "invalid") ||
				strings.Contains(lower, "unknown") ||
				strings.Contains(lower, "unrecognized") ||
				strings.Contains(lower, "extra")
		}
	}
	return false
}

func openAIChatEndpoint(baseURL string) (string, string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return base, "/chat/completions"
	}

	if u, err := url.Parse(base); err == nil && u.Path != "" {
		path := strings.TrimRight(u.Path, "/")
		if strings.HasSuffix(path, "/chat/completions") {
			u.Path = strings.TrimSuffix(path, "/chat/completions")
			if u.Path == "" {
				u.Path = "/"
			}
			return strings.TrimRight(u.String(), "/"), "/chat/completions"
		}
	}

	if strings.HasSuffix(base, "/chat/completions") {
		return strings.TrimSuffix(base, "/chat/completions"), "/chat/completions"
	}
	return base, "/chat/completions"
}

func shouldHandleToolCalls(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "tool_calls", "tool_call", "function_call", "function_calls":
		return true
	default:
		return false
	}
}

func AskAi(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, think bool) {
	askAiInternal(o, err, messages, ch, question, think, 0, false)
}

// askAiInternal AskAi 的完整实现。
// sseRetry: 流内嵌错误帧（HTTP 200 + SSE {"error":...}）自动重试计数，仅重试一次，防止前端长时间无输出；
// useMaxCompletion: 参数名降级标记，OpenAI 推理模型要求 max_completion_tokens 时的重试形态。
func askAiInternal(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, think bool, sseRetry int, useMaxCompletion bool) {
	// 底层中断快速失败：会话已被取消（用户中断/新请求取代）时不再发起新的 LLM 请求，
	// 包括工具调用多轮之间的后续轮次
	if o.ctx != nil && o.ctx.Err() != nil {
		logger.SugaredLogger.Infof("Stream canceled before request: %s", o.ctx.Err().Error())
		return
	}
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	timeout := time.Duration(o.TimeOut) * time.Second

	var client *resty.Client
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client = createHTTPClientWithProxy(o.HttpProxy, o.TimeOut)
	} else {
		client = CreateHTTPClientWithTimeout(timeout)
	}

	baseURL, chatPath := openAIChatEndpoint(o.BaseUrl)
	client.SetBaseURL(baseURL)

	thinking := "disabled"
	if think {
		thinking = "enabled"
	}

	if !think {
		messages = stripReasoningContent(messages)
	}

	bodyMap := map[string]interface{}{
		"model":    o.Model,
		"stream":   true,
		"messages": messages,
	}
	if o.Temperature > 0 {
		bodyMap["temperature"] = o.Temperature
	}
	if o.MaxTokens > 0 {
		if useMaxCompletion {
			bodyMap["max_completion_tokens"] = o.MaxTokens
		} else {
			bodyMap["max_tokens"] = o.MaxTokens
		}
	}
	if think {
		bodyMap["thinking"] = map[string]any{
			"type": thinking,
		}
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetHeader("Authorization", "Bearer "+o.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(bodyMap)
	if extra := BuildExtraHeaders(o.ExtraHeaders, o.ChatSource); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post(chatPath)

	if err != nil {
		if o.ctx != nil && o.ctx.Err() != nil {
			// 会话已被取消（新请求或用户中断），不向前端输出错误，避免污染新会话的输出
			logger.SugaredLogger.Infof("Stream canceled: %s", err.Error())
			return
		}
		logger.SugaredLogger.Errorf("Stream error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	body := resp.RawBody()
	defer body.Close()

	if resp.StatusCode() != 200 {
		bodyBytes, _ := io.ReadAll(body)
		errMsg := openAIHTTPErrorMessage(resp.StatusCode(), bodyBytes)
		logger.SugaredLogger.Errorf("Stream HTTP error %d: %s", resp.StatusCode(), errMsg)
		if !useMaxCompletion && isUnsupportedMaxTokensError(errMsg) {
			logger.SugaredLogger.Warnf("max_tokens is not supported by model %s, retrying with max_completion_tokens", o.Model)
			askAiInternal(o, err, messages, ch, question, think, sseRetry, true)
			return
		}
		if think && isUnsupportedThinkingError(errMsg) {
			logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying without thinking", o.Model)
			askAiInternal(o, err, stripReasoningContent(messages), ch, question, false, sseRetry, useMaxCompletion)
			return
		}
		if hasReasoningContent(messages) && isUnsupportedReasoningContentError(errMsg) {
			logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying without reasoning_content", o.Model)
			askAiInternal(o, err, stripReasoningContent(messages), ch, question, think, sseRetry, useMaxCompletion)
			return
		}
		if resp.StatusCode() == 400 && think {
			logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without thinking", o.Model)
			askAiInternal(o, err, stripReasoningContent(messages), ch, question, false, sseRetry, useMaxCompletion)
			return
		}
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  errMsg,
		}
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	// nonData 累积非 SSE 行：部分网关对推理模型忽略 stream=true，直接返回完整
	// chat.completion JSON，需在流结束后兜底提取，避免前端长时间无响应。
	var nonData strings.Builder
	emitted := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}
			if data == "" {
				continue
			}

			// 中转/网关常见形态：HTTP 200 + SSE 流内嵌错误帧（如上游过载）。
			// 此类帧无 choices 字段，不识别会导致前端长时间无任何输出。
			if errMsg := inlineStreamError(data); errMsg != "" {
				logger.SugaredLogger.Warnf("Stream inline error frame: %s", errMsg)
				if !useMaxCompletion && isUnsupportedMaxTokensError(errMsg) {
					askAiInternal(o, err, messages, ch, question, think, sseRetry, true)
					return
				}
				if think && isUnsupportedThinkingError(errMsg) {
					askAiInternal(o, err, stripReasoningContent(messages), ch, question, false, sseRetry, useMaxCompletion)
					return
				}
				// 已有内容输出时不可重试（会造成前端内容重复）；未输出过且仅一次重试机会
				if !emitted && sseRetry == 0 {
					askAiInternal(o, err, messages, ch, question, think, sseRetry+1, useMaxCompletion)
					return
				}
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  errMsg,
				}
				return
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResponse); err == nil {
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
						emitted = true
						if content == "###" || content == "##" || content == "#" {
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  "\r\n" + content,
								"time":     time.Now().Format(time.DateTime),
							}
						} else {
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  content,
								"time":     time.Now().Format(time.DateTime),
							}
						}
					}
					reasoningContent := choice.Delta.ReasoningContent
					if reasoningContent == "" {
						// OpenAI 官方端点经 reasoning 字段下发推理增量（DeepSeek 系为 reasoning_content）
						reasoningContent = choice.Delta.Reasoning
					}
					if reasoningContent != "" {
						emitted = true
						ch <- map[string]any{
							"code":              1,
							"question":          question,
							"chatId":            streamResponse.Id,
							"model":             streamResponse.Model,
							"reasoning_content": reasoningContent,
							"time":              time.Now().Format(time.DateTime),
						}
					}
					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				logger.SugaredLogger.Warnf("Stream data parse error: %s", err.Error())
				emitted = true
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  err.Error(),
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				nonData.WriteString(line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
					}
					// 无错误消息的 JSON（如非流式完整响应）不作为错误推送，留待流结束兜底提取
					if msg == "" {
						continue
					}
					emitted = true
					if !useMaxCompletion && isUnsupportedMaxTokensError(msg) {
						logger.SugaredLogger.Warnf("max_tokens is not supported by model %s, retrying with max_completion_tokens", o.Model)
						askAiInternal(o, err, messages, ch, question, think, sseRetry, true)
						return
					}
					if think && isUnsupportedThinkingError(msg) {
						logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying without thinking", o.Model)
						askAiInternal(o, err, stripReasoningContent(messages), ch, question, false, sseRetry, useMaxCompletion)
						return
					}
					if hasReasoningContent(messages) && isUnsupportedReasoningContentError(msg) {
						logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying without reasoning_content", o.Model)
						askAiInternal(o, err, stripReasoningContent(messages), ch, question, think, sseRetry, useMaxCompletion)
						return
					}
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  msg,
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if o.ctx != nil && o.ctx.Err() != nil {
			// 会话已被取消，不向前端输出取消错误
			return
		}
		logger.SugaredLogger.Errorf("Stream scanner error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}
	// 非流式兜底：完整 chat.completion JSON 一次性返回（流式解析全程无输出）时提取内容
	if !emitted {
		if txt := strings.TrimSpace(nonData.String()); txt != "" {
			var full AiResponse
			if json.Unmarshal([]byte(txt), &full) == nil && len(full.Choices) > 0 && full.Choices[0].Message.Content != "" {
				ch <- map[string]any{
					"code":     1,
					"question": question,
					"chatId":   full.Id,
					"model":    full.Model,
					"content":  full.Choices[0].Message.Content,
					"time":     time.Now().Format(time.DateTime),
				}
				return
			}
			// 非流式返回的是错误 JSON（多行格式无法按行解析）时，提取错误信息推给前端
			if errMsg := inlineStreamError(txt); errMsg != "" {
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  errMsg,
				}
			}
		}
	}
}

func AskAiWithTools(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool, thinkingMode bool) {
	if len(tools) == 0 {
		AskAi(o, err, messages, ch, question, thinkingMode)
		return
	}
	AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, 0, 0, false)
}

func AskAiWithToolsDepth(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool, thinkingMode bool, depth int, sseRetry int, useMaxCompletion bool) {
	// 底层中断快速失败：会话已被取消时不再发起新的 LLM 请求（含工具调用多轮之间的后续轮次）
	if o.ctx != nil && o.ctx.Err() != nil {
		logger.SugaredLogger.Infof("Stream canceled before request: %s", o.ctx.Err().Error())
		return
	}
	const maxDepth = 200
	if depth > maxDepth {
		logger.SugaredLogger.Warnf("AskAiWithTools max depth exceeded: %d", depth)
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  "工具调用次数过多，已终止",
		}
		return
	}

	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	timeout := time.Duration(o.TimeOut) * time.Second

	var client *resty.Client
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client = createHTTPClientWithProxy(o.HttpProxy, o.TimeOut)
	} else {
		client = CreateHTTPClientWithTimeout(timeout)
	}

	baseURL, chatPath := openAIChatEndpoint(o.BaseUrl)
	client.SetBaseURL(baseURL)

	if !thinkingMode {
		messages = stripReasoningContent(messages)
	}

	bodyMap := map[string]interface{}{
		"model":    o.Model,
		"stream":   true,
		"messages": messages,
		"tools":    tools,
	}
	if o.Temperature > 0 {
		bodyMap["temperature"] = o.Temperature
	}
	if o.MaxTokens > 0 {
		if useMaxCompletion {
			bodyMap["max_completion_tokens"] = o.MaxTokens
		} else {
			bodyMap["max_tokens"] = o.MaxTokens
		}
	}
	if thinkingMode {
		bodyMap["thinking"] = map[string]any{
			"type": "enabled",
		}
	}

	reqBody, _ := json.Marshal(bodyMap)
	if len(reqBody) > 100000 {
		logger.SugaredLogger.Warnf("Request body too large: %d bytes, may cause API error", len(reqBody))
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetHeader("Authorization", "Bearer "+o.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(bodyMap)
	if extra := BuildExtraHeaders(o.ExtraHeaders, o.ChatSource); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post(chatPath)

	if err != nil {
		if o.ctx != nil && o.ctx.Err() != nil {
			// 会话已被取消（新请求或用户中断），不向前端输出错误，避免污染新会话的输出
			logger.SugaredLogger.Infof("Stream canceled: %s", err.Error())
			return
		}
		logger.SugaredLogger.Errorf("Stream error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	body := resp.RawBody()
	defer body.Close()

	if resp.StatusCode() != 200 {
		bodyBytes, _ := io.ReadAll(body)
		errMsg := openAIHTTPErrorMessage(resp.StatusCode(), bodyBytes)
		logger.SugaredLogger.Errorf("Stream HTTP error %d: %s", resp.StatusCode(), errMsg)

		if !useMaxCompletion && isUnsupportedMaxTokensError(errMsg) {
			logger.SugaredLogger.Warnf("max_tokens is not supported by model %s, retrying tools request with max_completion_tokens", o.Model)
			AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth, sseRetry, true)
			return
		}
		if thinkingMode && isUnsupportedThinkingError(errMsg) {
			logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying tools request without thinking", o.Model)
			AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth, sseRetry, useMaxCompletion)
			return
		}
		if hasReasoningContent(messages) && isUnsupportedReasoningContentError(errMsg) {
			logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying tools request without reasoning_content", o.Model)
			AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, thinkingMode, depth, sseRetry, useMaxCompletion)
			return
		}
		if isUnsupportedToolError(errMsg) {
			logger.SugaredLogger.Warnf("Tools are not supported by model %s, retrying without tools", o.Model)
			askAiInternal(o, err, stripToolMessages(messages), ch, question, thinkingMode, sseRetry, useMaxCompletion)
			return
		}

		if resp.StatusCode() == 400 {
			if thinkingMode {
				logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without thinking (depth=%d)", o.Model, depth)
				AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth, sseRetry, useMaxCompletion)
				return
			}
			if len(tools) > 0 {
				logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without tools (depth=%d)", o.Model, depth)
				askAiInternal(o, err, stripToolMessages(messages), ch, question, false, sseRetry, useMaxCompletion)
				return
			}
		}

		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  errMsg,
		}
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	// nonData 累积非 SSE 行：部分网关对推理模型忽略 stream=true，直接返回完整
	// chat.completion JSON，需在流结束后兜底提取，避免前端长时间无响应。
	var nonData strings.Builder
	emitted := false
	type pendingToolCall struct {
		ID        string
		Name      string
		Arguments strings.Builder
	}
	toolCalls := map[int]*pendingToolCall{}
	toolCallOrder := make([]int, 0)
	var currentAIContent strings.Builder
	var reasoningContentText strings.Builder
	var contentText strings.Builder
	handledToolCalls := false
	lastStreamResponseID := ""
	lastModel := ""

	handlePendingToolCalls := func(streamResponseID, model string) bool {
		if handledToolCalls || len(toolCallOrder) == 0 {
			return false
		}
		// 会话已被取消：不再执行工具调用与后续 LLM 轮次，直接结束本轮
		if o.ctx != nil && o.ctx.Err() != nil {
			logger.SugaredLogger.Infof("Stream canceled before tool calls: %s", o.ctx.Err().Error())
			return true
		}
		if streamResponseID == "" {
			streamResponseID = lastStreamResponseID
		}
		if model == "" {
			model = lastModel
		}
		handledToolCalls = true
		for _, idx := range toolCallOrder {
			call := toolCalls[idx]
			if call == nil || call.Name == "" {
				continue
			}
			if call.ID == "" {
				call.ID = fmt.Sprintf("call_%d", idx)
			}
			funcArguments := call.Arguments.String()
			if handler, ok := toolHandlers[call.Name]; ok {
				if hErr := handler(o, funcArguments, &ToolContext{
					Question:             question,
					Messages:             &messages,
					CurrentAIContent:     &currentAIContent,
					ReasoningContentText: &reasoningContentText,
					CurrentCallID:        call.ID,
					FuncName:             call.Name,
					Ch:                   ch,
					StreamResponseID:     streamResponseID,
					Model:                model,
					Source:               o.ChatSource,
					SystemPrompt:         extractSystemPrompt(&messages),
				}); hErr != nil {
					logger.SugaredLogger.Errorf("tool %s error: %s", call.Name, hErr.Error())
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  hErr.Error(),
					}
				}
				continue
			}
			logger.SugaredLogger.Warnf("tool handler not found: %s", call.Name)
			appendToolMessages(
				&messages,
				currentAIContent.String(),
				reasoningContentText.String(),
				call.ID,
				call.Name,
				funcArguments,
				fmt.Sprintf("工具 %s 不存在或未注册", call.Name),
			)
		}
		AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth+1, 0, useMaxCompletion)
		return true
	}

	for scanner.Scan() {
		line := scanner.Text()
		//logger.SugaredLogger.Infof("Received data: %s", line)
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				if handlePendingToolCalls("", "") {
					return
				}
				return
			}
			if data == "" {
				continue
			}

			// 中转/网关常见形态：HTTP 200 + SSE 流内嵌错误帧（如上游过载）。
			// 此类帧无 choices 字段，不识别会导致前端长时间无任何输出。
			if errMsg := inlineStreamError(data); errMsg != "" {
				logger.SugaredLogger.Warnf("Stream inline error frame: %s", errMsg)
				if !useMaxCompletion && isUnsupportedMaxTokensError(errMsg) {
					AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth, sseRetry, true)
					return
				}
				if thinkingMode && isUnsupportedThinkingError(errMsg) {
					AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth, sseRetry, useMaxCompletion)
					return
				}
				if isUnsupportedToolError(errMsg) {
					askAiInternal(o, err, stripToolMessages(messages), ch, question, thinkingMode, sseRetry, useMaxCompletion)
					return
				}
				// 已有内容输出时不可重试（会造成前端内容重复）；未输出过且仅一次重试机会
				if !emitted && sseRetry == 0 {
					AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth, sseRetry+1, useMaxCompletion)
					return
				}
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  errMsg,
				}
				return
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Reasoning        string `json:"reasoning"`
						Role             string `json:"role"`
						ToolCalls        []struct {
							Function struct {
								Arguments string `json:"arguments"`
								Name      string `json:"name"`
							} `json:"function"`
							Id    string `json:"id"`
							Index int    `json:"index"`
							Type  string `json:"type"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResponse); err == nil {
				if streamResponse.Id != "" {
					lastStreamResponseID = streamResponse.Id
				}
				if streamResponse.Model != "" {
					lastModel = streamResponse.Model
				}
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
						emitted = true
						contentText.WriteString(content)

						if content == "###" || content == "##" || content == "#" {
							currentAIContent.WriteString("\r\n" + content)
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  "\r\n" + content,
								"time":     time.Now().Format(time.DateTime),
							}
						} else {
							currentAIContent.WriteString(content)
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  content,
								"time":     time.Now().Format(time.DateTime),
							}
						}
					}
					reasoningContent := choice.Delta.ReasoningContent
					if reasoningContent == "" {
						// OpenAI 官方端点经 reasoning 字段下发推理增量（DeepSeek 系为 reasoning_content）
						reasoningContent = choice.Delta.Reasoning
					}
					if reasoningContent != "" {
						emitted = true
						reasoningContentText.WriteString(reasoningContent)
						ch <- map[string]any{
							"code":              1,
							"question":          question,
							"chatId":            streamResponse.Id,
							"model":             streamResponse.Model,
							"reasoning_content": reasoningContent,
							"time":              time.Now().Format(time.DateTime),
						}
					}
					if choice.Delta.ToolCalls != nil && len(choice.Delta.ToolCalls) > 0 {
						// 收到工具调用说明流是活的，标记已输出以避免错误重试导致工具重复执行
						emitted = true
						for _, call := range choice.Delta.ToolCalls {
							idx := call.Index
							tc, ok := toolCalls[idx]
							if !ok {
								tc = &pendingToolCall{}
								toolCalls[idx] = tc
								toolCallOrder = append(toolCallOrder, idx)
							}
							if call.Function.Name != "" {
								tc.Name = call.Function.Name
							}
							if call.Id != "" {
								tc.ID = call.Id
							}
							if call.Function.Arguments != "" {
								tc.Arguments.WriteString(call.Function.Arguments)
							}
						}
					}

					if shouldHandleToolCalls(choice.FinishReason) {
						handlePendingToolCalls(streamResponse.Id, streamResponse.Model)
						return
					}

					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				logger.SugaredLogger.Warnf("Stream data parse error: %s", err.Error())
				emitted = true
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  err.Error(),
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				nonData.WriteString(line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
					}
					// 无错误消息的 JSON（如非流式完整响应）不作为错误推送，留待流结束兜底提取
					if msg == "" {
						continue
					}
					emitted = true
					if !useMaxCompletion && isUnsupportedMaxTokensError(msg) {
						logger.SugaredLogger.Warnf("max_tokens is not supported by model %s, retrying tools request with max_completion_tokens", o.Model)
						AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth, sseRetry, true)
						return
					}
					if thinkingMode && isUnsupportedThinkingError(msg) {
						logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying tools request without thinking", o.Model)
						AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth, sseRetry, useMaxCompletion)
						return
					} else if hasReasoningContent(messages) && isUnsupportedReasoningContentError(msg) {
						logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying tools request without reasoning_content", o.Model)
						AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, thinkingMode, depth, sseRetry, useMaxCompletion)
						return
					} else if isUnsupportedToolError(msg) {
						logger.SugaredLogger.Warnf("Tools are not supported by model %s, retrying without tools", o.Model)
						askAiInternal(o, err, stripToolMessages(messages), ch, question, thinkingMode, sseRetry, useMaxCompletion)
						return
					} else {
						ch <- map[string]any{
							"code":     0,
							"question": question,
							"content":  msg,
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if o.ctx != nil && o.ctx.Err() != nil {
			// 会话已被取消，不向前端输出取消错误
			return
		}
		logger.SugaredLogger.Errorf("Stream scanner error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}
	// 非流式兜底：完整 chat.completion JSON 一次性返回（流式解析全程无输出）时提取内容
	if !emitted {
		if txt := strings.TrimSpace(nonData.String()); txt != "" {
			var full AiResponse
			if json.Unmarshal([]byte(txt), &full) == nil && len(full.Choices) > 0 && full.Choices[0].Message.Content != "" {
				content := full.Choices[0].Message.Content
				currentAIContent.WriteString(content)
				ch <- map[string]any{
					"code":     1,
					"question": question,
					"chatId":   full.Id,
					"model":    full.Model,
					"content":  content,
					"time":     time.Now().Format(time.DateTime),
				}
				return
			}
			// 非流式返回的是错误 JSON（多行格式无法按行解析）时，提取错误信息推给前端
			if errMsg := inlineStreamError(txt); errMsg != "" {
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  errMsg,
				}
			}
		}
	}
}

func (o *OpenAi) SaveAIResponseResult(stockCode, stockName, result, chatId, question string) {
	err := db.Dao.Create(&models.AIResponseResult{
		StockCode: stockCode,
		StockName: stockName,
		ModelName: o.Model,
		Content:   result,
		ChatId:    chatId,
		Question:  question,
	}).Error
	if err != nil {
		logger.SugaredLogger.Errorf("failed to save ai response result: %v", err)
	}
}

func (o *OpenAi) GetAIResponseResult(stock string) *models.AIResponseResult {
	var result models.AIResponseResult
	db.Dao.Where("stock_code = ?", stock).Order("id desc").Limit(1).Find(&result)
	return &result
}

func createHTTPClientWithProxy(proxyURL string, timeout int) *resty.Client {
	transport := &http.Transport{
		Proxy: http.ProxyURL(parseProxyURL(proxyURL)),
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	if timeout <= 0 {
		timeout = 300
	}
	httpClient.Timeout = time.Duration(timeout) * time.Second

	return resty.NewWithClient(httpClient).
		SetTimeout(time.Duration(timeout) * time.Second).
		SetRetryCount(0)
}
