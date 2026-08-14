package data

import (
	"encoding/json"
	"errors"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
)

type Settings struct {
	gorm.Model
	TushareToken     string `json:"tushareToken"`
	LocalPushEnable  bool   `json:"localPushEnable"`
	DingPushEnable   bool   `json:"dingPushEnable"`
	DingRobot        string `json:"dingRobot"`
	FeishuPushEnable bool   `json:"feishuPushEnable"`
	FeishuRobot      string `json:"feishuRobot"`
	FeishuSecret     string `json:"feishuSecret" gorm:"column:feishu_secret"`
	// 飞书应用机器人（接收消息+AI回复，长连接模式，与 FeishuPush 自定义机器人推送独立）
	FeishuBotEnable        bool   `json:"feishuBotEnable"`
	FeishuAppID            string `json:"feishuAppId" gorm:"column:feishu_app_id"`
	FeishuAppSecret        string `json:"feishuAppSecret" gorm:"column:feishu_app_secret"`
	FeishuBotAiConfigId    int    `json:"feishuBotAiConfigId" gorm:"column:feishu_bot_ai_config_id"`
	FeishuBotSysPromptId   int    `json:"feishuBotSysPromptId" gorm:"column:feishu_bot_sys_prompt_id"`
	FeishuBotEnableTools   bool   `json:"feishuBotEnableTools"`
	FeishuBotThinking      bool   `json:"feishuBotThinking"`
	FeishuBotAgentMode     string `json:"feishuBotAgentMode" gorm:"column:feishu_bot_agent_mode"`
	UpdateBasicInfoOnStart bool   `json:"updateBasicInfoOnStart"`
	RefreshInterval        int64  `json:"refreshInterval"`
	OpenAiEnable           bool   `json:"openAiEnable"`
	Prompt                 string `json:"prompt"`
	CheckUpdate            bool   `json:"checkUpdate"`
	UpdateChannel          string `json:"updateChannel"`
	QuestionTemplate       string `json:"questionTemplate"`
	CrawlTimeOut           int64  `json:"crawlTimeOut"`
	KDays                  int64  `json:"kDays"`
	EnableDanmu            bool   `json:"enableDanmu"`
	BrowserPath            string `json:"browserPath"`
	EnableNews             bool   `json:"enableNews"`
	DarkTheme              bool   `json:"darkTheme"`
	BrowserPoolSize        int    `json:"browserPoolSize"`
	EnableFund             bool   `json:"enableFund"`
	EnablePushNews         bool   `json:"enablePushNews"`
	EnableOnlyPushRedNews  bool   `json:"enableOnlyPushRedNews"`
	SponsorCode            string `json:"sponsorCode"`
	HttpProxy              string `json:"httpProxy"`
	HttpProxyEnabled       bool   `json:"httpProxyEnabled"`
	EnableAgent            bool   `json:"enableAgent"`
	QgqpBId                string `json:"qgqpBId" gorm:"column:qgqp_b_id"`
	IwencaiApiKey          string `json:"iwencaiApiKey" gorm:"column:iwencai_api_key"`
	EmApiKey               string `json:"emApiKey" gorm:"column:em_api_key"`
	WindowWidth            int    `json:"windowWidth"`
	WindowHeight           int    `json:"windowHeight"`
	PromptPlazaApiBase     string `json:"promptPlazaApiBase" gorm:"column:prompt_plaza_api_base"`
	// LongTermMemoryAiConfigId 长期记忆向量检索绑定的 AIConfig ID。
	// 0=自动模式（优先 ModelType=embedding 的服务）；>0=用指定 AIConfig。
	// 用于让用户明确指定长期记忆用哪个向量服务，避免自动选错。
	LongTermMemoryAiConfigId int `json:"longTermMemoryAiConfigId" gorm:"column:long_term_memory_ai_config_id;default:0"`
}

func (receiver Settings) TableName() string {
	return "settings"
}

type AIConfig struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string `json:"name"`
	BaseUrl   string `json:"baseUrl"`
	ApiKey    string `json:"apiKey" `
	ModelName string `json:"modelName"`
	// ModelType 模型类型："chat"=文本对话模型（默认），"embedding"=向量模型。
	// 同一提供商的对话与向量接口地址可能不同，故拆分为独立 AIConfig 条目。
	// type=embedding 时，ModelName 即向量模型名（如 text-embedding-3-small / BAAI/bge-m3）。
	ModelType string `json:"modelType" gorm:"column:model_type;default:'chat'"`
	MaxTokens int    `json:"maxTokens"`
	// ContextWindow 模型上下文窗口大小（输入+输出总容量）。
	// 由 FetchAiModelInfo 从模型 API 的 max_context_length/context_length 自动获取，
	// 或从内置模型表推导。用于摘要中间件和消息压缩的 token 预算计算。
	// 为 0 时运行时按内置表/MaxTokens/默认值兜底（向后兼容旧配置）。
	ContextWindow    int     `json:"contextWindow" gorm:"column:context_window"`
	Temperature      float64 `json:"temperature"`
	TimeOut          int     `json:"timeOut"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	SessionId        string  `json:"sessionId" gorm:"index;size:64"`
	Thinking         bool    `json:"thinking"`
	// ExtraHeaders 自定义 HTTP 请求头（JSON 格式字符串，如 {"x-team-id":"...","x-agent-id":"..."}）。
	// 支持模板变量：{{sessionId}}（会话ID）、{{uuid}}（每次请求生成新UUID）。
	// 用于对接需携带额外 Header 的代理/网关（如腾讯云 TencentDB-Agent-Memory / CodeBuddy Proxy）。
	ExtraHeaders string `json:"extraHeaders" gorm:"type:text"`
	// EmbeddingModel 长期记忆向量检索使用的 embedding 模型名（OpenAI 兼容 /v1/embeddings 接口）。
	// 留空时默认 "text-embedding-3-small"；中文供应商可填其支持的模型名（如 Qwen 的 text-embedding-v3）。
	// 仅 backend/agent/long_term_memory.go 使用，与对话模型 ModelName 独立。
	EmbeddingModel string `json:"embeddingModel" gorm:"column:embedding_model"`
}

func (AIConfig) TableName() string {
	return "ai_config"
}

type SettingConfig struct {
	*Settings
	AiConfigs []*AIConfig `json:"aiConfigs"`
}

func (c *SettingConfig) GetAIConfigThinking(aiConfigId int) bool {
	if aiConfigId <= 0 && len(c.AiConfigs) > 0 {
		return c.AiConfigs[0].Thinking
	}
	for _, cfg := range c.AiConfigs {
		if int(cfg.ID) == aiConfigId {
			return cfg.Thinking
		}
	}
	return false
}

type SettingsApi struct {
	Config *SettingConfig
}

func NewSettingsApi() *SettingsApi {
	return &SettingsApi{
		Config: GetSettingConfig(),
	}
}

func (s *SettingsApi) Export() string {
	d, _ := json.MarshalIndent(s.Config, "", "    ")
	return string(d)
}

func UpdateConfig(s *SettingConfig) string {
	if s.Settings == nil {
		return "保存失败: 配置数据为空"
	}
	count := int64(0)
	db.Dao.Model(&Settings{}).Count(&count)
	if count > 0 {
		result := db.Dao.Model(&Settings{}).Where("id=?", s.ID).Updates(map[string]any{
			"local_push_enable":             s.LocalPushEnable,
			"ding_push_enable":              s.DingPushEnable,
			"ding_robot":                    s.DingRobot,
			"feishu_push_enable":            s.FeishuPushEnable,
			"feishu_robot":                  s.FeishuRobot,
			"feishu_secret":                 s.FeishuSecret,
			"feishu_bot_enable":             s.FeishuBotEnable,
			"feishu_app_id":                 s.FeishuAppID,
			"feishu_app_secret":             s.FeishuAppSecret,
			"feishu_bot_ai_config_id":       s.FeishuBotAiConfigId,
			"feishu_bot_sys_prompt_id":      s.FeishuBotSysPromptId,
			"feishu_bot_enable_tools":       s.FeishuBotEnableTools,
			"feishu_bot_thinking":           s.FeishuBotThinking,
			"feishu_bot_agent_mode":         s.FeishuBotAgentMode,
			"update_basic_info_on_start":    s.UpdateBasicInfoOnStart,
			"refresh_interval":              s.RefreshInterval,
			"open_ai_enable":                s.OpenAiEnable,
			"tushare_token":                 s.TushareToken,
			"prompt":                        s.Prompt,
			"check_update":                  s.CheckUpdate,
			"update_channel":                s.UpdateChannel,
			"question_template":             s.QuestionTemplate,
			"crawl_time_out":                s.CrawlTimeOut,
			"k_days":                        s.KDays,
			"enable_danmu":                  s.EnableDanmu,
			"browser_path":                  s.BrowserPath,
			"enable_news":                   s.EnableNews,
			"dark_theme":                    s.DarkTheme,
			"enable_fund":                   s.EnableFund,
			"enable_push_news":              s.EnablePushNews,
			"enable_only_push_red_news":     s.EnableOnlyPushRedNews,
			"sponsor_code":                  s.SponsorCode,
			"http_proxy":                    s.HttpProxy,
			"http_proxy_enabled":            s.HttpProxyEnabled,
			"enable_agent":                  s.EnableAgent,
			"qgqp_b_id":                     s.QgqpBId,
			"iwencai_api_key":               s.IwencaiApiKey,
			"em_api_key":                    s.EmApiKey,
			"window_width":                  s.WindowWidth,
			"window_height":                 s.WindowHeight,
			"prompt_plaza_api_base":         s.PromptPlazaApiBase,
			"long_term_memory_ai_config_id": s.LongTermMemoryAiConfigId,
		})
		if result.Error != nil {
			logger.SugaredLogger.Errorf("更新配置失败: %v", result.Error)
			return "保存失败: " + result.Error.Error()
		}

		err := updateAiConfigs(s.AiConfigs)
		if err != nil {
			logger.SugaredLogger.Errorf("更新AI模型服务配置失败: %v", err)
			return "更新AI模型服务配置失败: " + err.Error()
		}
	} else {
		result := db.Dao.Model(&Settings{}).Create(s.Settings)
		if result.Error != nil {
			logger.SugaredLogger.Error("创建配置失败:", result.Error)
			return "创建配置失败: " + result.Error.Error()
		}
		err := updateAiConfigs(s.AiConfigs)
		if err != nil {
			logger.SugaredLogger.Errorf("更新AI模型服务配置失败: %v", err)
			return "更新AI模型服务配置失败: " + err.Error()
		}
	}

	ConfigureFromSettings(s)

	return "保存成功！"
}

func updateAiConfigs(aiConfigs []*AIConfig) error {
	// nil 表示调用方不希望更新 AI 配置（保留现有配置）；
	// 空 slice（len==0）才表示清空所有 AI 配置
	if aiConfigs == nil {
		return nil
	}
	if len(aiConfigs) == 0 {
		err := db.Dao.Exec("DELETE FROM ai_config").Error
		if err != nil {
			return err
		}
		return db.Dao.Exec("DELETE FROM sqlite_sequence WHERE name='ai_config'").Error
	}
	// 仅收集大于 0 的 ID，用于识别已存在的配置；
	// ID<=0 视为“新配置”，强制走插入逻辑，避免多个 ID 为 0 的配置互相覆盖。
	var ids []uint
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if item.ID > 0 {
			ids = append(ids, item.ID)
		}
	})
	var existAiConfigs []*AIConfig
	err := db.Dao.Model(&AIConfig{}).Select("id").Where("id in (?) ", ids).Find(&existAiConfigs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	idMap := make(map[uint]bool)
	lo.ForEach(existAiConfigs, func(item *AIConfig, index int) {
		idMap[item.ID] = true
	})
	var addAiConfigs []*AIConfig
	var notDeleteIds []uint
	var e error
	lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
		if e != nil {
			return
		}
		// ID<=0 一律视为新配置，走插入逻辑；否则根据是否已存在决定更新或新增
		if item.ID <= 0 || !idMap[item.ID] {
			addAiConfigs = append(addAiConfigs, item)
		} else {
			notDeleteIds = append(notDeleteIds, item.ID)
			e = db.Dao.Model(&AIConfig{}).Where("id=?", item.ID).Updates(map[string]interface{}{
				"name":               item.Name,
				"base_url":           item.BaseUrl,
				"api_key":            item.ApiKey,
				"model_name":         item.ModelName,
				"max_tokens":         item.MaxTokens,
				"context_window":     item.ContextWindow,
				"temperature":        item.Temperature,
				"time_out":           item.TimeOut,
				"http_proxy":         item.HttpProxy,
				"http_proxy_enabled": item.HttpProxyEnabled,
				"session_id":         item.SessionId,
				"thinking":           item.Thinking,
				"extra_headers":      item.ExtraHeaders,
				"embedding_model":    item.EmbeddingModel,
				"model_type":         item.ModelType,
			}).Error
			if e != nil {
				return
			}
		}
	})
	if e != nil {
		return e
	}
	//删除旧的配置
	if len(notDeleteIds) > 0 {
		err = db.Dao.Exec("DELETE FROM ai_config WHERE id NOT IN ?", notDeleteIds).Error
		if err != nil {
			return err
		}
	}
	//logger.SugaredLogger.Infof("更新aiConfigs +%d", len(addAiConfigs))
	//批量新增的配置
	err = db.Dao.CreateInBatches(addAiConfigs, len(addAiConfigs)).Error
	return err
}

// UpdateAiConfigsOnly 仅更新 AI 模型服务配置，不影响其他设置项
// 供独立的 AI 模型服务管理页面调用，避免覆盖 settings 表中的其他字段
func UpdateAiConfigsOnly(aiConfigs []*AIConfig) string {
	if err := updateAiConfigs(aiConfigs); err != nil {
		logger.SugaredLogger.Errorf("更新AI配置失败: %v", err)
		return "保存失败: " + err.Error()
	}
	// 刷新内存中的配置缓存
	ConfigureFromSettings(GetSettingConfig())
	return "保存成功！"
}

func GetSettingConfig() *SettingConfig {
	settingConfig := &SettingConfig{}
	settings := &Settings{}
	aiConfigs := make([]*AIConfig, 0)
	// 处理数据库查询可能返回的空结果
	settingsResult := db.Dao.Model(&Settings{}).First(settings)
	// 新用户无设置记录时，默认启用暗黑主题
	if errors.Is(settingsResult.Error, gorm.ErrRecordNotFound) {
		settings.DarkTheme = true
	}
	// AI 配置始终查询，不依赖 OpenAiEnable 开关：
	// AI 配置管理页面、飞书机器人、AI 助手等独立功能可能在 OpenAiEnable=false 时也需要读取已保存的配置
	result := db.Dao.Model(&AIConfig{}).Find(&aiConfigs)
	if result.Error != nil {
		logger.SugaredLogger.Error("查询AI配置失败:", result.Error)
	} else if len(aiConfigs) > 0 {
		lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
			if item.TimeOut <= 0 {
				item.TimeOut = 60 * 5
			}
		})
	}
	if settings.OpenAiEnable {
		if settings.CrawlTimeOut <= 0 {
			settings.CrawlTimeOut = 60
		}
		if settings.KDays < 30 {
			settings.KDays = 60
		}
	}
	if settings.BrowserPath == "" {
		settings.BrowserPath, _ = CheckBrowser()
	}
	if settings.BrowserPoolSize <= 0 {
		settings.BrowserPoolSize = 1
	}
	settings.EnableAgent = false

	settingConfig.Settings = settings
	settingConfig.AiConfigs = aiConfigs

	return settingConfig
}
