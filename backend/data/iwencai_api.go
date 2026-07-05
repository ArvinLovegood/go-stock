package data

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"strings"

	"github.com/go-resty/resty/v2"
)

const iwencaiAPIURL = "https://openapi.iwencai.com/v1/query2data"
const iwencaiSearchURL = "https://openapi.iwencai.com/v1/comprehensive/search"
const iwencaiGatewaySearchURL = "https://www.iwencai.com/gateway/mobilesearch/comprehensive/search"

const iwencaiGatewaySearchDefaultSize = 20

func generateTraceID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func iwencaiCommonHeaders(apiKey, skillID, skillVersion string) map[string]string {
	return map[string]string{
		"Authorization":         "Bearer " + apiKey,
		"Content-Type":          "application/json",
		"X-Claw-Call-Type":      "normal",
		"X-Claw-Skill-Id":       skillID,
		"X-Claw-Skill-Version":  skillVersion,
		"X-Claw-Plugin-Id":      "none",
		"X-Claw-Plugin-Version": "none",
		"X-Claw-Trace-Id":       generateTraceID(),
	}
}

// iwencaiExtractErrorMessage extracts a human-readable message from a non-200
// response body. The iwencai openapi returns errors in two shapes:
//   - a JSON string literal (e.g. 401 quota exhaustion): `"您今天的次数已用完..."`
//   - a JSON object with status_msg / msg field: `{"status_msg":"..."}`
//
// Plain text bodies are returned as-is (truncated by the caller).
func iwencaiExtractErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	// JSON string literal (covers 401 quota-exhaustion case where the body is
	// `"..."` rather than the expected `{...}` envelope).
	var s string
	if err := json.Unmarshal(body, &s); err == nil {
		return strings.TrimSpace(s)
	}
	// JSON object with status_msg / msg field.
	var obj struct {
		StatusMsg string `json:"status_msg"`
		Msg       string `json:"msg"`
	}
	if err := json.Unmarshal(body, &obj); err == nil {
		if msg := strings.TrimSpace(obj.StatusMsg); msg != "" {
			return msg
		}
		if msg := strings.TrimSpace(obj.Msg); msg != "" {
			return msg
		}
	}
	// Plain text fallback.
	return strings.TrimSpace(string(body))
}

// iwencaiHTTPError builds an error for a non-200 response, appending the
// response body's message (when available) so callers can tell quota
// exhaustion apart from invalid keys, network issues, etc.
func iwencaiHTTPError(prefix string, statusCode int, body []byte) error {
	msg := iwencaiExtractErrorMessage(body)
	if msg == "" {
		return fmt.Errorf("%s返回HTTP错误: %d", prefix, statusCode)
	}
	if r := []rune(msg); len(r) > 200 {
		msg = string(r[:200]) + "..."
	}
	return fmt.Errorf("%s返回HTTP错误: %d，详情: %s", prefix, statusCode, msg)
}

type IwencaiAPI struct {
	client *resty.Client
	config *SettingConfig
}

func NewIwencaiAPI() *IwencaiAPI {
	return &IwencaiAPI{
		client: SharedHTTPClient,
		config: GetSettingConfig(),
	}
}

type IwencaiRequest struct {
	Query       string `json:"query"`
	Page        string `json:"page"`
	Limit       string `json:"limit"`
	IsCache     string `json:"is_cache"`
	ExpandIndex string `json:"expand_index"`
}

type IwencaiResponse struct {
	StatusCode int              `json:"status_code"`
	StatusMsg  string           `json:"status_msg"`
	Datas      []map[string]any `json:"datas"`
	CodeCount  int              `json:"code_count"`
	ChunksInfo any              `json:"chunks_info"`
}

func (api *IwencaiAPI) Query(query string, page, limit int) (*IwencaiResponse, error) {
	apiKey := api.config.Settings.IwencaiApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("同花顺问财API密钥未配置，请在设置中填写IwencaiApiKey")
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	reqBody := IwencaiRequest{
		Query:       query,
		Page:        fmt.Sprintf("%d", page),
		Limit:       fmt.Sprintf("%d", limit),
		IsCache:     "1",
		ExpandIndex: "true",
	}

	var result IwencaiResponse
	resp, err := api.client.R().
		SetHeaders(iwencaiCommonHeaders(apiKey, "query2data", "1.0.0")).
		SetBody(reqBody).
		SetResult(&result).
		Post(iwencaiAPIURL)

	if err != nil {
		return nil, fmt.Errorf("调用同花顺问财API失败: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, iwencaiHTTPError("同花顺问财API", resp.StatusCode(), resp.Body())
	}

	if result.StatusCode != 0 {
		return nil, fmt.Errorf("同花顺问财API返回错误: %s", result.StatusMsg)
	}

	return &result, nil
}

func (api *IwencaiAPI) QueryToMarkdown(query string, page, limit int) string {
	result, err := api.Query(query, page, limit)
	if err != nil {
		logger.SugaredLogger.Errorf("问财查询失败: %v", err)
		return fmt.Sprintf("查询失败: %v", err)
	}

	if len(result.Datas) == 0 {
		return fmt.Sprintf("未查询到「%s」的相关数据。可到同花顺问财查询：https://www.iwencai.com/unifiedwap/chat", query)
	}

	return renderIwencaiQueryMarkdown(query, result, page, limit)
}

type IwencaiSearchRequest struct {
	Channels []string `json:"channels"`
	AppID    string   `json:"app_id"`
	Query    string   `json:"query"`
}

type IwencaiSearchResponse struct {
	Data []IwencaiSearchItem `json:"data"`
}

type IwencaiSearchItem struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	URL         string `json:"url"`
	PublishDate string `json:"publish_date"`
}

// iwencaiOpenAPISearchResponse mirrors the OpenAPI search envelope, which
// includes status_code/status_msg. The exported IwencaiSearchResponse is the
// cleaned type shared with the gateway path and omits these fields.
type iwencaiOpenAPISearchResponse struct {
	StatusCode int                 `json:"status_code"`
	StatusMsg  string              `json:"status_msg"`
	Data       []IwencaiSearchItem `json:"data"`
}

type iwencaiGatewaySearchRequest struct {
	Offset   int      `json:"offset"`
	Size     int      `json:"size"`
	AppID    string   `json:"app_id"`
	Query    string   `json:"query"`
	Channels []string `json:"channels"`
	Platform string   `json:"platform"`
	Slots    []any    `json:"slots"`
}

type iwencaiGatewaySearchItem struct {
	Channel     string `json:"channel"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	URL         string `json:"url"`
	PublishDate string `json:"publish_date"`
}

type iwencaiGatewaySearchResponse struct {
	StatusCode int                        `json:"status_code"`
	StatusMsg  string                     `json:"status_msg"`
	Total      int                        `json:"total"`
	Took       int                        `json:"took"`
	Data       []iwencaiGatewaySearchItem `json:"data"`
}

func iwencaiGatewayChannel(channel string) string {
	if channel == "investor" {
		return "interact"
	}
	return channel
}

func gatewaySearchItemsToResponse(items []iwencaiGatewaySearchItem) *IwencaiSearchResponse {
	result := &IwencaiSearchResponse{Data: make([]IwencaiSearchItem, 0, len(items))}
	for _, item := range items {
		result.Data = append(result.Data, IwencaiSearchItem{
			Title:       item.Title,
			Summary:     item.Summary,
			URL:         item.URL,
			PublishDate: item.PublishDate,
		})
	}
	return result
}

func (api *IwencaiAPI) searchComprehensiveGateway(channel string, query string, size int) (*IwencaiSearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}
	if size <= 0 {
		size = iwencaiGatewaySearchDefaultSize
	}

	reqBody := iwencaiGatewaySearchRequest{
		Offset:   0,
		Size:     size,
		AppID:    "wencai_pc",
		Query:    query,
		Channels: []string{iwencaiGatewayChannel(channel)},
		Platform: "pc",
		Slots:    []any{},
	}

	var gwResult iwencaiGatewaySearchResponse
	resp, err := api.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		SetResult(&gwResult).
		Post(iwencaiGatewaySearchURL)

	if err != nil {
		return nil, fmt.Errorf("调用问财网关搜索失败: %v", err)
	}
	if resp.StatusCode() != 200 {
		return nil, iwencaiHTTPError("问财网关搜索", resp.StatusCode(), resp.Body())
	}
	if gwResult.StatusCode != 0 {
		return nil, fmt.Errorf("问财网关搜索返回错误: %s", gwResult.StatusMsg)
	}

	return gatewaySearchItemsToResponse(gwResult.Data), nil
}

func (api *IwencaiAPI) searchComprehensiveOpenAPI(channel string, query string) (*IwencaiSearchResponse, error) {
	apiKey := api.config.Settings.IwencaiApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("同花顺问财API密钥未配置，请在设置中填写IwencaiApiKey")
	}

	if query == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	reqBody := IwencaiSearchRequest{
		Channels: []string{channel},
		AppID:    "AIME_SKILL",
		Query:    query,
	}

	var raw iwencaiOpenAPISearchResponse
	resp, err := api.client.R().
		SetHeaders(iwencaiCommonHeaders(apiKey, "news-search", "1.0.0")).
		SetBody(reqBody).
		SetResult(&raw).
		Post(iwencaiSearchURL)

	if err != nil {
		return nil, fmt.Errorf("调用同花顺问财搜索API失败: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, iwencaiHTTPError("同花顺问财搜索API", resp.StatusCode(), resp.Body())
	}

	if raw.StatusCode != 0 {
		return nil, fmt.Errorf("同花顺问财搜索API返回错误: %s", raw.StatusMsg)
	}

	return &IwencaiSearchResponse{Data: raw.Data}, nil
}

func (api *IwencaiAPI) searchComprehensive(channel string, query string) (*IwencaiSearchResponse, error) {
	result, err := api.searchComprehensiveGateway(channel, query, iwencaiGatewaySearchDefaultSize)
	if err == nil {
		return result, nil
	}
	logger.SugaredLogger.Warnf("问财网关搜索失败，尝试OpenAPI: %v", err)
	return api.searchComprehensiveOpenAPI(channel, query)
}

func (api *IwencaiAPI) SearchReport(query string) (*IwencaiSearchResponse, error) {
	return api.searchComprehensive("report", query)
}

func (api *IwencaiAPI) SearchNews(query string) (*IwencaiSearchResponse, error) {
	return api.searchComprehensive("news", query)
}

func (api *IwencaiAPI) SearchInvestor(query string) (*IwencaiSearchResponse, error) {
	return api.searchComprehensive("investor", query)
}

func (api *IwencaiAPI) SearchAnnouncement(query string) (*IwencaiSearchResponse, error) {
	return api.searchComprehensive("announcement", query)
}

func searchResultToMarkdown(query string, result *IwencaiSearchResponse, label string) string {
	return renderIwencaiSearchMarkdown(query, label, result.Data)
}

func (api *IwencaiAPI) SearchReportToMarkdown(query string) string {
	result, err := api.SearchReport(query)
	if err != nil {
		logger.SugaredLogger.Errorf("研报搜索失败: %v", err)
		return fmt.Sprintf("搜索失败: %v", err)
	}
	return searchResultToMarkdown(query, result, "研报")
}

func (api *IwencaiAPI) SearchNewsToMarkdown(query string) string {
	result, err := api.SearchNews(query)
	if err != nil {
		logger.SugaredLogger.Errorf("新闻搜索失败: %v", err)
		return fmt.Sprintf("搜索失败: %v", err)
	}
	return searchResultToMarkdown(query, result, "新闻")
}

func (api *IwencaiAPI) SearchInvestorToMarkdown(query string) string {
	result, err := api.SearchInvestor(query)
	if err != nil {
		logger.SugaredLogger.Errorf("投资者关系活动搜索失败: %v", err)
		return fmt.Sprintf("搜索失败: %v", err)
	}
	return searchResultToMarkdown(query, result, "投资者关系活动")
}

func (api *IwencaiAPI) SearchAnnouncementToMarkdown(query string) string {
	result, err := api.SearchAnnouncement(query)
	if err != nil {
		logger.SugaredLogger.Errorf("公告搜索失败: %v", err)
		return fmt.Sprintf("搜索失败: %v", err)
	}
	return searchResultToMarkdown(query, result, "公告")
}
