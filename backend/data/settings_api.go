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
}

func (receiver Settings) TableName() string {
	return "settings"
}

type AIConfig struct {
	ID               uint `gorm:"primarykey"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Name             string  `json:"name"`
	BaseUrl          string  `json:"baseUrl"`
	ApiKey           string  `json:"apiKey" `
	ModelName        string  `json:"modelName"`
	MaxTokens        int     `json:"maxTokens"`
	Temperature      float64 `json:"temperature"`
	TimeOut          int     `json:"timeOut"`
	HttpProxy        string  `json:"httpProxy"`
	HttpProxyEnabled bool    `json:"httpProxyEnabled"`
	SessionId        string  `json:"sessionId" gorm:"index;size:64"`
	Thinking         bool    `json:"thinking"`
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
			"local_push_enable":          s.LocalPushEnable,
			"ding_push_enable":           s.DingPushEnable,
			"ding_robot":                 s.DingRobot,
			"feishu_push_enable":         s.FeishuPushEnable,
			"feishu_robot":               s.FeishuRobot,
			"feishu_secret":              s.FeishuSecret,
			"feishu_bot_enable":          s.FeishuBotEnable,
			"feishu_app_id":              s.FeishuAppID,
			"feishu_app_secret":          s.FeishuAppSecret,
			"feishu_bot_ai_config_id":    s.FeishuBotAiConfigId,
			"feishu_bot_sys_prompt_id":   s.FeishuBotSysPromptId,
			"feishu_bot_enable_tools":    s.FeishuBotEnableTools,
			"feishu_bot_thinking":        s.FeishuBotThinking,
			"feishu_bot_agent_mode":      s.FeishuBotAgentMode,
			"update_basic_info_on_start": s.UpdateBasicInfoOnStart,
			"refresh_interval":           s.RefreshInterval,
			"open_ai_enable":             s.OpenAiEnable,
			"tushare_token":              s.TushareToken,
			"prompt":                     s.Prompt,
			"check_update":               s.CheckUpdate,
			"update_channel":             s.UpdateChannel,
			"question_template":          s.QuestionTemplate,
			"crawl_time_out":             s.CrawlTimeOut,
			"k_days":                     s.KDays,
			"enable_danmu":               s.EnableDanmu,
			"browser_path":               s.BrowserPath,
			"enable_news":                s.EnableNews,
			"dark_theme":                 s.DarkTheme,
			"enable_fund":                s.EnableFund,
			"enable_push_news":           s.EnablePushNews,
			"enable_only_push_red_news":  s.EnableOnlyPushRedNews,
			"sponsor_code":               s.SponsorCode,
			"http_proxy":                 s.HttpProxy,
			"http_proxy_enabled":         s.HttpProxyEnabled,
			"enable_agent":               s.EnableAgent,
			"qgqp_b_id":                  s.QgqpBId,
			"iwencai_api_key":            s.IwencaiApiKey,
			"em_api_key":                 s.EmApiKey,
			"window_width":               s.WindowWidth,
			"window_height":              s.WindowHeight,
			"prompt_plaza_api_base":      s.PromptPlazaApiBase,
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
				"temperature":        item.Temperature,
				"time_out":           item.TimeOut,
				"http_proxy":         item.HttpProxy,
				"http_proxy_enabled": item.HttpProxyEnabled,
				"session_id":         item.SessionId,
				"thinking":           item.Thinking,
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

func GetSettingConfig() *SettingConfig {
	settingConfig := &SettingConfig{}
	settings := &Settings{}
	aiConfigs := make([]*AIConfig, 0)
	// 处理数据库查询可能返回的空结果
	result := db.Dao.Model(&Settings{}).First(settings)
	if settings.OpenAiEnable {
		// 处理AI配置查询可能出现的错误
		result = db.Dao.Model(&AIConfig{}).Find(&aiConfigs)
		if result.Error != nil {
			logger.SugaredLogger.Error("查询AI配置失败:", result.Error)
		} else if len(aiConfigs) > 0 {
			lo.ForEach(aiConfigs, func(item *AIConfig, index int) {
				if item.TimeOut <= 0 {
					item.TimeOut = 60 * 5
				}
			})
		}
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
