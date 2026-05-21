package data

import (
	"bufio"
	"encoding/json"
	"go-stock/backend/db"
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
	Required             []string       `json:"required"`
	AdditionalProperties bool           `json:"additionalProperties"`
}

type ToolFunction struct {
	Name        string              `json:"name"`
	Strict      bool                `json:"strict"`
	Description string              `json:"description"`
	Parameters  *FunctionParameters `json:"parameters"`
}

// Internal analysis runs are tagged so their LLM output cannot be replayed by stock lookups.
const internalAIResponseChatIDPrefix = "auto:"

// appendToolMessages 统一向 messages 追加一次工具调用的 assistant/tool 两条消息
func appendToolMessages(
	messages *[]map[string]any,
	currentAIContent, reasoningContent, callID, funcName, funcArgs, toolContent string,
) {
	*messages = append(*messages, map[string]any{
		"role":              "assistant",
		"content":           currentAIContent,
		"reasoning_content": reasoningContent, "tool_calls": []map[string]any{
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
	})

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

func AskAi(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, think bool) {
	client := resty.New()
	client.SetBaseURL(strutil.Trim(o.BaseUrl))
	client.SetHeader("Authorization", "Bearer "+o.ApiKey)
	client.SetHeader("Content-Type", "application/json")
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	thinking := "disabled"
	if think {
		thinking = "enabled"
	}
	client.SetTimeout(time.Duration(o.TimeOut) * time.Second)
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client.SetProxy(o.HttpProxy)
	}
	//StripAssistantReasoningExceptLast(messages)
	bodyMap := map[string]interface{}{
		"model":       o.Model,
		"max_tokens":  o.MaxTokens,
		"temperature": o.Temperature,
		"stream":      true,
		"messages":    messages,
	}
	if think {
		bodyMap["thinking"] = map[string]any{
			"type": thinking,
		}
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetBody(bodyMap)
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post("/chat/completions")

	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Infof("Stream error : %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		//("Received data: %s", line)
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResponse); err == nil {
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
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
					if reasoningContent := choice.Delta.ReasoningContent; reasoningContent != "" {
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
				if err != nil {
					logger.SugaredLogger.Infof("Stream data error : %s", err.Error())
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  err.Error(),
					}
				} else {
					logger.SugaredLogger.Infof("Stream data error : %s", data)
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  data,
					}
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				logger.SugaredLogger.Infof("Stream data error : %s", line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
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
}

func AskAiWithTools(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool, thinkingMode bool) {
	//bytes, _ := json.Marshal(messages)
	//logger.SugaredLogger.Debugf("Stream request: \n%s\n", string(bytes))

	client := resty.New()
	client.SetBaseURL(strutil.Trim(o.BaseUrl))
	client.SetHeader("Authorization", "Bearer "+o.ApiKey)
	client.SetHeader("Content-Type", "application/json")
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	thinking := "disabled"
	if thinkingMode {
		thinking = "enabled"
	}
	client.SetTimeout(time.Duration(o.TimeOut) * time.Second)
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client.SetProxy(o.HttpProxy)
	}
	//StripAssistantReasoningExceptLast(messages)

	//bytes, _ := json.Marshal(messages)
	//logger.SugaredLogger.Debugf("Stream request messages:\n %s", string(bytes))

	bodyMap := map[string]interface{}{
		"model":       o.Model,
		"max_tokens":  o.MaxTokens,
		"temperature": o.Temperature,
		"stream":      true,
		"messages":    messages,
		"tools":       tools,
	}
	if thinkingMode {
		bodyMap["thinking"] = map[string]any{
			"type": thinking,
		}
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetBody(bodyMap)
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post("/chat/completions")

	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Infof("Stream error : %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	scanner := bufio.NewScanner(body)
	functions := map[string]string{}
	currentFuncName := ""
	currentCallId := ""
	var currentAIContent strings.Builder
	var reasoningContentText strings.Builder
	var contentText strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		//logger.SugaredLogger.Infof("Received data: %s", line)
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
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
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
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
					if reasoningContent := choice.Delta.ReasoningContent; reasoningContent != "" {
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
						for _, call := range choice.Delta.ToolCalls {
							if call.Type == "function" {
								functions[call.Function.Name] = ""
								currentFuncName = call.Function.Name
								currentCallId = call.Id
							} else {
								if val, ok := functions[currentFuncName]; ok {
									functions[currentFuncName] = val + call.Function.Arguments
								} else {
									functions[currentFuncName] = call.Function.Arguments
								}
							}
						}
					}

					if choice.FinishReason == "tool_calls" {
						//logger.SugaredLogger.Infof("functions: %+v", functions)
						for funcName, funcArguments := range functions {
							// 优先使用注册的 ToolHandler 处理
							if handler, ok := toolHandlers[funcName]; ok {
								if hErr := handler(o, funcArguments, &ToolContext{
									Question:             question,
									Messages:             &messages,
									CurrentAIContent:     &currentAIContent,
									ReasoningContentText: &reasoningContentText,
									CurrentCallID:        currentCallId,
									FuncName:             funcName,
									Ch:                   ch,
									StreamResponseID:     streamResponse.Id,
									Model:                streamResponse.Model,
								}); hErr != nil {
									logger.SugaredLogger.Infof("tool %s error : %s", funcName, hErr.Error())
									ch <- map[string]any{
										"code":     0,
										"question": question,
										"content":  hErr.Error(),
									}
								}
								// 已由 handler 完整处理，继续下一个 funcName
								continue
							}

							// 其余未拆分到独立 handler 的工具，走下面的分支逻辑

						}
						AskAiWithTools(o, err, messages, ch, question, tools, thinkingMode)
					}

					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				if err != nil {
					logger.SugaredLogger.Infof("Stream data error : %s", err.Error())
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  err.Error(),
					}
				} else {
					logger.SugaredLogger.Infof("Stream data error : %s", data)
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  data,
					}
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				logger.SugaredLogger.Infof("Stream data error : %s", line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
					}

					if msg == "Function call is not supported for this model." {
						var newMessages []map[string]any
						for _, message := range messages {
							if message["role"] == "tool" {
								continue
							}
							if _, ok := message["tool_calls"]; ok {
								continue
							}
							newMessages = append(newMessages, message)
						}
						AskAi(o, err, newMessages, ch, question, thinkingMode)
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
	db.Dao.Where("stock_code = ? AND (chat_id IS NULL OR chat_id = '' OR chat_id NOT LIKE ?)", stock, internalAIResponseChatIDPrefix+"%").Order("id desc").Limit(1).Find(&result)
	return &result
}
