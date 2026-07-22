package data

import (
	"context"
	"fmt"
	"go-stock/backend/logger"

	"github.com/samber/lo"
)

// @Author spark
// @Date 2025/1/16 13:19
// @Desc
// -----------------------------------------------------------------------------------
type OpenAi struct {
	ctx              context.Context
	aiConfig         AIConfig
	BaseUrl          string  `json:"base_url"`
	ApiKey           string  `json:"api_key"`
	Model            string  `json:"model"`
	MaxTokens        int     `json:"max_tokens"`
	Temperature      float64 `json:"temperature"`
	Prompt           string  `json:"prompt"`
	TimeOut          int     `json:"time_out"`
	QuestionTemplate string  `json:"question_template"`
	CrawlTimeOut     int64   `json:"crawl_time_out"`
	KDays            int64   `json:"kDays"`
	BrowserPath      string  `json:"browser_path"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	ChatSource       string  `json:"-"`
}

func (o *OpenAi) Ctx() context.Context     { return o.ctx }
func (o *OpenAi) GetBaseURL() string       { return o.BaseUrl }
func (o *OpenAi) GetAPIKey() string        { return o.ApiKey }
func (o *OpenAi) GetModel() string         { return o.Model }
func (o *OpenAi) GetMaxTokens() int        { return o.MaxTokens }
func (o *OpenAi) GetTemperature() float64  { return o.Temperature }
func (o *OpenAi) GetTimeout() int          { return o.TimeOut }
func (o *OpenAi) IsHttpProxyEnabled() bool { return o.HttpProxyEnabled }
func (o *OpenAi) GetHttpProxy() string     { return o.HttpProxy }

func (o OpenAi) String() string {
	return fmt.Sprintf("OpenAi{BaseUrl: %s, Model: %s, MaxTokens: %d, Temperature: %.2f, Prompt: %s, TimeOut: %d, QuestionTemplate: %s, CrawlTimeOut: %d, KDays: %d, BrowserPath: %s, ApiKey: [MASKED]}",
		o.BaseUrl, o.Model, o.MaxTokens, o.Temperature, o.Prompt, o.TimeOut, o.QuestionTemplate, o.CrawlTimeOut, o.KDays, o.BrowserPath)
}

func NewDeepSeekOpenAi(ctx context.Context, aiConfigId int) *OpenAi {
	settingConfig := GetSettingConfig()
	aiConfig, find := lo.Find(settingConfig.AiConfigs, func(item *AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !find {
		aiConfig = &AIConfig{}
	}

	if settingConfig.OpenAiEnable {
		if aiConfig.TimeOut <= 0 {
			aiConfig.TimeOut = 60 * 5
		}
		if settingConfig.CrawlTimeOut <= 0 {
			settingConfig.CrawlTimeOut = 60
		}
		if settingConfig.KDays < 30 {
			settingConfig.KDays = 60
		}
	}
	o := &OpenAi{
		ctx:              ctx,
		aiConfig:         *aiConfig,
		BaseUrl:          aiConfig.BaseUrl,
		ApiKey:           aiConfig.ApiKey,
		Model:            aiConfig.ModelName,
		MaxTokens:        aiConfig.EffectiveMaxOutputTokens(),
		Temperature:      aiConfig.Temperature,
		TimeOut:          aiConfig.EffectiveTimeout(),
		HttpProxy:        aiConfig.HttpProxy,
		HttpProxyEnabled: aiConfig.HttpProxyEnabled,
		Prompt:           settingConfig.Prompt,
		QuestionTemplate: settingConfig.QuestionTemplate,
		CrawlTimeOut:     settingConfig.CrawlTimeOut,
		KDays:            settingConfig.KDays,
		BrowserPath:      settingConfig.BrowserPath,
	}
	logger.SugaredLogger.Infof(
		"AI policy model=%q family=%s input_tokens=%d output_tokens=%d reasoning=%q temperature_set=%v timeout=%ds",
		aiConfig.ModelName, aiConfig.Family(), aiConfig.EffectiveMaxInputTokens(), aiConfig.EffectiveMaxOutputTokens(),
		aiConfig.EffectiveReasoningEffort(), aiConfig.EffectiveTemperature() != nil, aiConfig.EffectiveTimeout(),
	)
	return o
}
