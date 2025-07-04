package data

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/random"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"strings"
	"sync"
	"time"
)

// @Author spark
// @Date 2025/1/16 13:19
// @Desc
// -----------------------------------------------------------------------------------
type OpenAi struct {
	ctx              context.Context
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
}

func NewDeepSeekOpenAi(ctx context.Context) *OpenAi {
	config := GetConfig()
	if config.OpenAiEnable {
		if config.OpenAiApiTimeOut <= 0 {
			config.OpenAiApiTimeOut = 60 * 5
		}
		if config.CrawlTimeOut <= 0 {
			config.CrawlTimeOut = 60
		}
		if config.KDays < 30 {
			config.KDays = 120
		}
	}
	return &OpenAi{
		ctx:              ctx,
		BaseUrl:          config.OpenAiBaseUrl,
		ApiKey:           config.OpenAiApiKey,
		Model:            config.OpenAiModelName,
		MaxTokens:        config.OpenAiMaxTokens,
		Temperature:      config.OpenAiTemperature,
		Prompt:           config.Prompt,
		TimeOut:          config.OpenAiApiTimeOut,
		QuestionTemplate: config.QuestionTemplate,
		CrawlTimeOut:     config.CrawlTimeOut,
		KDays:            config.KDays,
		BrowserPath:      config.BrowserPath,
	}
}

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
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}
type ToolFunction struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Parameters  FunctionParameters `json:"parameters"`
}

func (o OpenAi) NewSummaryStockNewsStreamWithTools(userQuestion string, sysPromptId *int, tools []Tool) <-chan map[string]any {
	ch := make(chan map[string]any, 512)
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewSummaryStockNewsStream panic", err)
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic :%s", err)
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic  config:%v", o)
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		msg := []map[string]interface{}{
			{
				"role": "system",
				//"content": "作为一位专业的A股市场分析师和投资顾问,请你根据以下信息提供详细的技术分析和投资策略建议:",
				//"content": "【角色设定】\n你是一位拥有20年实战经验的顶级股票分析师，精通技术分析、基本面分析、市场心理学和量化交易。擅长发现成长股、捕捉行业轮动机会，在牛熊市中都能保持稳定收益。你的风格是价值投资与技术择时相结合，注重风险控制。\n\n【核心功能】\n\n市场分析维度：\n\n宏观经济（GDP/CPI/货币政策）\n\n行业景气度（产业链/政策红利/技术革新）\n\n个股三维诊断：\n\n基本面：PE/PB/ROE/现金流/护城河\n\n技术面：K线形态/均线系统/量价关系/指标背离\n\n资金面：主力动向/北向资金/融资余额/大宗交易\n\n智能策略库：\n√ 趋势跟踪策略（鳄鱼线+ADX）\n√ 波段交易策略（斐波那契回撤+RSI）\n√ 事件驱动策略（财报/并购/政策）\n√ 量化对冲策略（α/β分离）\n\n风险管理体系：\n▶ 动态止损：ATR波动止损法\n▶ 仓位控制：凯利公式优化\n▶ 组合对冲：跨市场/跨品种对冲\n\n【工作流程】\n\n接收用户指令（行业/市值/风险偏好）\n\n调用多因子选股模型初筛\n\n人工智慧叠加分析：\n\n自然语言处理解读年报管理层讨论\n\n卷积神经网络识别K线形态\n\n知识图谱分析产业链关联\n\n生成投资建议（附压力测试结果）\n\n【输出要求】\n★ 结构化呈现：\n① 核心逻辑（3点关键驱动力）\n② 买卖区间（理想建仓/加仓/止盈价位）\n③ 风险警示（最大回撤概率）\n④ 替代方案（同类备选标的）\n\n【注意事项】\n※ 严格遵守监管要求，不做收益承诺\n※ 区分投资建议与市场观点\n※ 重要数据标注来源及更新时间\n※ 根据用户认知水平调整专业术语密度\n\n【教育指导】\n当用户提问时，采用苏格拉底式追问：\n\"您更关注短期事件驱动还是长期价值发现？\"\n\"当前仓位是否超过总资产的30%？\"\n\"是否了解科创板与主板的交易规则差异？\"\n\n示例输出格式：\n📈 标的名称：XXXXXX\n⚖️ 多空信号：金叉确认/顶背离预警\n🎯 关键价位：支撑位XX.XX/压力位XX.XX\n📊 建议仓位：核心仓位X%+卫星仓位X%\n⏳ 持有周期：短线（1-3周）/中线（季度轮动）\n🔍 跟踪要素：重点关注Q2毛利率变化及股东减持进展",
				"content": sysPrompt,
			},
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})
		wg := &sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			var market strings.Builder
			market.WriteString(getZSInfo("创业板指数", "sz399006", 30) + "\n")
			market.WriteString(getZSInfo("上证综合指数", "sh000001", 30) + "\n")
			market.WriteString(getZSInfo("沪深300指数", "sh000300", 30) + "\n")
			//logger.SugaredLogger.Infof("NewChatStream getZSInfo=\n%s", market.String())
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "当前市场指数行情",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "当前市场指数行情情况如下：\n" + market.String(),
			})
		}()

		go func() {
			defer wg.Done()
			md := strings.Builder{}
			res := NewMarketNewsApi().ClsCalendar()
			for _, a := range res {
				bytes, err := json.Marshal(a)
				if err != nil {
					continue
				}
				//logger.SugaredLogger.Debugf("value: %+v", string(bytes))
				date := gjson.Get(string(bytes), "calendar_day")
				md.WriteString("\n### 事件/会议日期：" + date.String())
				list := gjson.Get(string(bytes), "items")
				//logger.SugaredLogger.Debugf("value: %+v,list: %+v", date.String(), list)
				list.ForEach(func(key, value gjson.Result) bool {
					logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
					md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
					return true
				})
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "近期重大事件/会议",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "近期重大事件/会议如下：\n" + md.String(),
			})

		}()

		wg.Wait()

		news := NewMarketNewsApi().GetNewsList("财联社电报", random.RandInt(50, 150))
		messageText := strings.Builder{}
		for _, telegraph := range *news {
			messageText.WriteString("## " + telegraph.Time + ":" + "\n")
			messageText.WriteString("### " + telegraph.Content + "\n")
		}
		//logger.SugaredLogger.Infof("市场资讯 messageText=\n%s", messageText.String())

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "市场资讯",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": messageText.String(),
		})
		if userQuestion == "" {
			userQuestion = "请根据当前时间，总结和分析股票市场新闻中的投资机会"
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": userQuestion,
		})
		AskAiWithTools(o, errors.New(""), msg, ch, userQuestion, tools)
	}()
	return ch
}

func (o OpenAi) NewSummaryStockNewsStream(userQuestion string, sysPromptId *int) <-chan map[string]any {
	ch := make(chan map[string]any, 512)
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewSummaryStockNewsStream panic", err)
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic :%s", err)
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic  config:%v", o)
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		msg := []map[string]interface{}{
			{
				"role": "system",
				//"content": "作为一位专业的A股市场分析师和投资顾问,请你根据以下信息提供详细的技术分析和投资策略建议:",
				//"content": "【角色设定】\n你是一位拥有20年实战经验的顶级股票分析师，精通技术分析、基本面分析、市场心理学和量化交易。擅长发现成长股、捕捉行业轮动机会，在牛熊市中都能保持稳定收益。你的风格是价值投资与技术择时相结合，注重风险控制。\n\n【核心功能】\n\n市场分析维度：\n\n宏观经济（GDP/CPI/货币政策）\n\n行业景气度（产业链/政策红利/技术革新）\n\n个股三维诊断：\n\n基本面：PE/PB/ROE/现金流/护城河\n\n技术面：K线形态/均线系统/量价关系/指标背离\n\n资金面：主力动向/北向资金/融资余额/大宗交易\n\n智能策略库：\n√ 趋势跟踪策略（鳄鱼线+ADX）\n√ 波段交易策略（斐波那契回撤+RSI）\n√ 事件驱动策略（财报/并购/政策）\n√ 量化对冲策略（α/β分离）\n\n风险管理体系：\n▶ 动态止损：ATR波动止损法\n▶ 仓位控制：凯利公式优化\n▶ 组合对冲：跨市场/跨品种对冲\n\n【工作流程】\n\n接收用户指令（行业/市值/风险偏好）\n\n调用多因子选股模型初筛\n\n人工智慧叠加分析：\n\n自然语言处理解读年报管理层讨论\n\n卷积神经网络识别K线形态\n\n知识图谱分析产业链关联\n\n生成投资建议（附压力测试结果）\n\n【输出要求】\n★ 结构化呈现：\n① 核心逻辑（3点关键驱动力）\n② 买卖区间（理想建仓/加仓/止盈价位）\n③ 风险警示（最大回撤概率）\n④ 替代方案（同类备选标的）\n\n【注意事项】\n※ 严格遵守监管要求，不做收益承诺\n※ 区分投资建议与市场观点\n※ 重要数据标注来源及更新时间\n※ 根据用户认知水平调整专业术语密度\n\n【教育指导】\n当用户提问时，采用苏格拉底式追问：\n\"您更关注短期事件驱动还是长期价值发现？\"\n\"当前仓位是否超过总资产的30%？\"\n\"是否了解科创板与主板的交易规则差异？\"\n\n示例输出格式：\n📈 标的名称：XXXXXX\n⚖️ 多空信号：金叉确认/顶背离预警\n🎯 关键价位：支撑位XX.XX/压力位XX.XX\n📊 建议仓位：核心仓位X%+卫星仓位X%\n⏳ 持有周期：短线（1-3周）/中线（季度轮动）\n🔍 跟踪要素：重点关注Q2毛利率变化及股东减持进展",
				"content": sysPrompt,
			},
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var market strings.Builder
			market.WriteString(getZSInfo("创业板指数", "sz399006", 30) + "\n")
			market.WriteString(getZSInfo("上证综合指数", "sh000001", 30) + "\n")
			market.WriteString(getZSInfo("沪深300指数", "sh000300", 30) + "\n")
			//logger.SugaredLogger.Infof("NewChatStream getZSInfo=\n%s", market.String())
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "当前市场指数行情",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "当前市场指数行情情况如下：\n" + market.String(),
			})
		}()
		wg.Wait()

		news := NewMarketNewsApi().GetNewsList("", 100)
		messageText := strings.Builder{}
		for _, telegraph := range *news {
			messageText.WriteString("## " + telegraph.Time + ":" + "\n")
			messageText.WriteString("### " + telegraph.Content + "\n")
		}
		//logger.SugaredLogger.Infof("市场资讯 messageText=\n%s", messageText.String())

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "市场资讯",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": messageText.String(),
		})
		if userQuestion == "" {
			userQuestion = "请根据当前时间，总结和分析股票市场新闻中的投资机会"
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": userQuestion,
		})
		AskAi(o, errors.New(""), msg, ch, userQuestion)
	}()
	return ch
}

func (o OpenAi) NewChatStream(stock, stockCode, userQuestion string, sysPromptId *int, tools []Tool) <-chan map[string]any {
	ch := make(chan map[string]any, 512)

	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewChatStream panic", err)
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic :%s", err)
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic  stock:%s stockCode:%s", stock, stockCode)
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic  config:%v", o)
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		msg := []map[string]interface{}{
			{
				"role": "system",
				//"content": "作为一位专业的A股市场分析师和投资顾问,请你根据以下信息提供详细的技术分析和投资策略建议:",
				//"content": "【角色设定】\n你是一位拥有20年实战经验的顶级股票分析师，精通技术分析、基本面分析、市场心理学和量化交易。擅长发现成长股、捕捉行业轮动机会，在牛熊市中都能保持稳定收益。你的风格是价值投资与技术择时相结合，注重风险控制。\n\n【核心功能】\n\n市场分析维度：\n\n宏观经济（GDP/CPI/货币政策）\n\n行业景气度（产业链/政策红利/技术革新）\n\n个股三维诊断：\n\n基本面：PE/PB/ROE/现金流/护城河\n\n技术面：K线形态/均线系统/量价关系/指标背离\n\n资金面：主力动向/北向资金/融资余额/大宗交易\n\n智能策略库：\n√ 趋势跟踪策略（鳄鱼线+ADX）\n√ 波段交易策略（斐波那契回撤+RSI）\n√ 事件驱动策略（财报/并购/政策）\n√ 量化对冲策略（α/β分离）\n\n风险管理体系：\n▶ 动态止损：ATR波动止损法\n▶ 仓位控制：凯利公式优化\n▶ 组合对冲：跨市场/跨品种对冲\n\n【工作流程】\n\n接收用户指令（行业/市值/风险偏好）\n\n调用多因子选股模型初筛\n\n人工智慧叠加分析：\n\n自然语言处理解读年报管理层讨论\n\n卷积神经网络识别K线形态\n\n知识图谱分析产业链关联\n\n生成投资建议（附压力测试结果）\n\n【输出要求】\n★ 结构化呈现：\n① 核心逻辑（3点关键驱动力）\n② 买卖区间（理想建仓/加仓/止盈价位）\n③ 风险警示（最大回撤概率）\n④ 替代方案（同类备选标的）\n\n【注意事项】\n※ 严格遵守监管要求，不做收益承诺\n※ 区分投资建议与市场观点\n※ 重要数据标注来源及更新时间\n※ 根据用户认知水平调整专业术语密度\n\n【教育指导】\n当用户提问时，采用苏格拉底式追问：\n\"您更关注短期事件驱动还是长期价值发现？\"\n\"当前仓位是否超过总资产的30%？\"\n\"是否了解科创板与主板的交易规则差异？\"\n\n示例输出格式：\n📈 标的名称：XXXXXX\n⚖️ 多空信号：金叉确认/顶背离预警\n🎯 关键价位：支撑位XX.XX/压力位XX.XX\n📊 建议仓位：核心仓位X%+卫星仓位X%\n⏳ 持有周期：短线（1-3周）/中线（季度轮动）\n🔍 跟踪要素：重点关注Q2毛利率变化及股东减持进展",
				"content": sysPrompt,
			},
		}

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})

		replaceTemplates := map[string]string{
			"{{stockName}}": RemoveAllBlankChar(stock),
			"{{stockCode}}": RemoveAllBlankChar(stockCode),
			"{stockName}":   RemoveAllBlankChar(stock),
			"{stockCode}":   RemoveAllBlankChar(stockCode),
			"stockName":     RemoveAllBlankChar(stock),
			"stockCode":     RemoveAllBlankChar(stockCode),
		}
		followedStock := NewStockDataApi().GetFollowedStockByStockCode(stockCode)
		stockData, err := NewStockDataApi().GetStockCodeRealTimeData(stockCode)
		if err == nil && len(*stockData) > 0 {
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": fmt.Sprintf("当前%s[%s]价格是多少？", stock, stockCode),
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": fmt.Sprintf("截止到%s,当前%s[%s]价格是%s", (*stockData)[0].Date+" "+(*stockData)[0].Time, stock, stockCode, (*stockData)[0].Price),
			})
		}
		if followedStock.CostPrice > 0 {
			replaceTemplates["{{costPrice}}"] = convertor.ToString(followedStock.CostPrice)
			replaceTemplates["{costPrice}"] = convertor.ToString(followedStock.CostPrice)
			replaceTemplates["costPrice"] = convertor.ToString(followedStock.CostPrice)
		}

		question := ""
		if userQuestion == "" {
			question = strutil.ReplaceWithMap(o.QuestionTemplate, replaceTemplates)
		} else {
			question = userQuestion
			question = strutil.ReplaceWithMap(userQuestion, replaceTemplates)
		}

		logger.SugaredLogger.Infof("NewChatStream stock:%s stockCode:%s", stock, stockCode)
		logger.SugaredLogger.Infof("Prompt：%s", sysPrompt)
		logger.SugaredLogger.Infof("final question:%s", question)

		wg := &sync.WaitGroup{}
		wg.Add(7)

		go func() {
			defer wg.Done()
			var market strings.Builder
			market.WriteString(getZSInfo("创业板指数", "sz399006", 30) + "\n")
			market.WriteString(getZSInfo("上证综合指数", "sh000001", 30) + "\n")
			market.WriteString(getZSInfo("沪深300指数", "sh000300", 30) + "\n")
			//logger.SugaredLogger.Infof("NewChatStream getZSInfo=\n%s", market.String())
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "市场指数",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "市场指数情况如下：\n" + market.String(),
			})
		}()

		go func() {
			defer wg.Done()
			//endDate := time.Now().Format("20060102")
			//startDate := time.Now().Add(-time.Hour * time.Duration(24*o.KDays)).Format("20060102")
			//code := stockCode
			//if strutil.HasPrefixAny(stockCode, []string{"hk"}) {
			//	code = ConvertStockCodeToTushareCode(stockCode)
			//	K := NewTushareApi(GetConfig()).GetDaily(code, startDate, endDate, o.CrawlTimeOut)
			//	msg = append(msg, map[string]interface{}{
			//		"role":    "user",
			//		"content": stock + "日K数据",
			//	})
			//	msg = append(msg, map[string]interface{}{
			//		"role":    "assistant",
			//		"content": stock + "日K数据如下：\n" + K,
			//	})
			//}

			logger.SugaredLogger.Infof("NewChatStream getKLineData stock:%s stockCode:%s", stock, stockCode)
			if strutil.HasPrefixAny(stockCode, []string{"sz", "sh", "hk", "us", "gb_"}) {
				K := &[]KLineData{}
				logger.SugaredLogger.Infof("NewChatStream getKLineData stock:%s stockCode:%s", stock, stockCode)
				if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
					K = NewStockDataApi().GetKLineData(stockCode, "240", o.KDays)
				}
				if strutil.HasPrefixAny(stockCode, []string{"hk", "us", "gb_"}) {
					K = NewStockDataApi().GetHK_KLineData(stockCode, "day", o.KDays)
				}
				Kmap := &[]map[string]any{}
				for _, kline := range *K {
					mapk := make(map[string]any, 6)
					mapk["日期"] = kline.Day
					mapk["开盘价"] = kline.Open
					mapk["最高价"] = kline.High
					mapk["最低价"] = kline.Low
					mapk["收盘价"] = kline.Close
					Volume, _ := convertor.ToFloat(kline.Volume)
					mapk["成交量(万手)"] = Volume / 10000.00 / 100.00
					*Kmap = append(*Kmap, mapk)
				}
				jsonData, _ := json.Marshal(Kmap)
				markdownTable, _ := JSONToMarkdownTable(jsonData)
				msg = append(msg, map[string]interface{}{
					"role":    "user",
					"content": stock + "日K数据",
				})
				msg = append(msg, map[string]interface{}{
					"role":    "assistant",
					"content": "## " + stock + "日K数据如下：\n" + markdownTable,
				})
				logger.SugaredLogger.Infof("getKLineData=\n%s", markdownTable)
			}

		}()

		go func() {
			defer wg.Done()
			messages := SearchStockPriceInfo(stock, stockCode, o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取股票价格失败")
				//ch <- "***❗获取股票价格失败,分析结果可能不准确***<hr>"
				ch <- map[string]any{
					"code":         1,
					"question":     question,
					"extraContent": "***❗获取股票价格失败,分析结果可能不准确***<hr>",
				}
				go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票价格失败,分析结果可能不准确")
				return
			}
			price := ""
			for _, message := range *messages {
				price += message + ";"
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "股价数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "\n## " + stock + "股价数据：\n" + price,
			})
			logger.SugaredLogger.Infof("SearchStockPriceInfo stock:%s stockCode:%s", stock, stockCode)
			logger.SugaredLogger.Infof("SearchStockPriceInfo assistant:%s", "\n## "+stock+"股价数据：\n"+price)
		}()

		go func() {
			defer wg.Done()

			if checkIsIndexBasic(stock) {
				return
			}
			messages := GetFinancialReportsByXUEQIU(stockCode, o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取股票财报失败")
				// "***❗获取股票财报失败,分析结果可能不准确***<hr>"
				ch <- map[string]any{
					"code":         1,
					"question":     question,
					"extraContent": "***❗获取股票财报失败,分析结果可能不准确***<hr>",
				}
				go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票财报失败,分析结果可能不准确")
				return
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "财报数据",
			})
			for _, message := range *messages {
				msg = append(msg, map[string]interface{}{
					"role":    "assistant",
					"content": stock + message,
				})
			}
		}()

		go func() {
			defer wg.Done()
			messages := GetTelegraphList(o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取市场资讯失败")
				//ch <- "***❗获取市场资讯失败,分析结果可能不准确***<hr>"
				//go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取市场资讯失败,分析结果可能不准确")
				return
			}
			var messageText strings.Builder
			for _, message := range *messages {
				messageText.WriteString(message + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "市场资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": messageText.String(),
			})

			messages = GetTopNewsList(o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取新闻资讯失败")
				//ch <- "***❗获取新闻资讯失败,分析结果可能不准确***<hr>"
				//go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取新闻资讯失败,分析结果可能不准确")
				return
			}
			var newsText strings.Builder
			for _, message := range *messages {
				newsText.WriteString(message + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "新闻资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": newsText.String(),
			})
		}()

		//go func() {
		//	defer wg.Done()
		//	messages := SearchStockInfo(stock, "depth", o.CrawlTimeOut)
		//	if messages == nil || len(*messages) == 0 {
		//		logger.SugaredLogger.Error("获取股票资讯失败")
		//		//ch <- "***❗获取股票资讯失败,分析结果可能不准确***<hr>"
		//		//go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票资讯失败,分析结果可能不准确")
		//		return
		//	}
		//	for _, message := range *messages {
		//		msg = append(msg, map[string]interface{}{
		//			"role":    "user",
		//			"content": message,
		//		})
		//	}
		//}()
		go func() {
			defer wg.Done()
			messages := SearchStockInfo(stock, "telegram", o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取股票电报资讯失败")
				//ch <- "***❗获取股票电报资讯失败,分析结果可能不准确***<hr>"
				//go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票电报资讯失败,分析结果可能不准确")
				return
			}
			var newsText strings.Builder
			for _, message := range *messages {
				newsText.WriteString(message + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "相关新闻资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": newsText.String(),
			})
		}()

		go func() {
			defer wg.Done()

			if checkIsIndexBasic(stock) {
				return
			}

			messages := SearchGuShiTongStockInfo(stockCode, o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				logger.SugaredLogger.Error("获取股势通资讯失败")
				//ch <- "***❗获取股势通资讯失败,分析结果可能不准确***<hr>"
				//go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股势通资讯失败,分析结果可能不准确")
				return
			}
			var newsText strings.Builder
			for _, message := range *messages {
				newsText.WriteString(message + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "相关新闻资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": newsText.String(),
			})
		}()

		wg.Wait()
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": question,
		})

		//reqJson, _ := json.Marshal(msg)
		//logger.SugaredLogger.Errorf("Stream request: \n%s\n", reqJson)
		if tools != nil && len(tools) > 0 {
			AskAiWithTools(o, err, msg, ch, question, tools)
		} else {
			AskAi(o, err, msg, ch, question)
		}
	}()
	return ch
}

func AskAi(o OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string) {
	client := resty.New()
	client.SetBaseURL(strutil.Trim(o.BaseUrl))
	client.SetHeader("Authorization", "Bearer "+o.ApiKey)
	client.SetHeader("Content-Type", "application/json")
	//client.SetRetryCount(3)
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	client.SetTimeout(time.Duration(o.TimeOut) * time.Second)
	resp, err := client.R().
		SetDoNotParseResponse(true).
		SetBody(map[string]interface{}{
			"model":       o.Model,
			"max_tokens":  o.MaxTokens,
			"temperature": o.Temperature,
			"stream":      true,
			"messages":    messages,
		}).
		Post("/chat/completions")

	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Infof("Stream error : %s", err.Error())
		//ch <- err.Error()
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}
	//location, _ := time.LoadLocation("Asia/Shanghai")

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		logger.SugaredLogger.Infof("Received data: %s", line)
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
						//ch <- content
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

						//logger.SugaredLogger.Infof("Content data: %s", content)
					}
					if reasoningContent := choice.Delta.ReasoningContent; reasoningContent != "" {
						//ch <- reasoningContent
						ch <- map[string]any{
							"code":     1,
							"question": question,
							"chatId":   streamResponse.Id,
							"model":    streamResponse.Model,
							"content":  reasoningContent,
							"time":     time.Now().Format(time.DateTime),
						}

						//logger.SugaredLogger.Infof("ReasoningContent data: %s", reasoningContent)
					}
					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				if err != nil {
					logger.SugaredLogger.Infof("Stream data error : %s", err.Error())
					//ch <- err.Error()
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  err.Error(),
					}
				} else {
					logger.SugaredLogger.Infof("Stream data error : %s", data)
					//ch <- data
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
					//ch <- line
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
func AskAiWithTools(o OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool) {
	client := resty.New()
	client.SetBaseURL(strutil.Trim(o.BaseUrl))
	client.SetHeader("Authorization", "Bearer "+o.ApiKey)
	client.SetHeader("Content-Type", "application/json")
	//client.SetRetryCount(3)
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	client.SetTimeout(time.Duration(o.TimeOut) * time.Second)
	resp, err := client.R().
		SetDoNotParseResponse(true).
		SetBody(map[string]interface{}{
			"model":       o.Model,
			"max_tokens":  o.MaxTokens,
			"temperature": o.Temperature,
			"stream":      true,
			"messages":    messages,
			"tools":       tools,
		}).
		Post("/chat/completions")

	body := resp.RawBody()
	defer body.Close()
	if err != nil {
		logger.SugaredLogger.Infof("Stream error : %s", err.Error())
		//ch <- err.Error()
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}
	//location, _ := time.LoadLocation("Asia/Shanghai")

	scanner := bufio.NewScanner(body)
	functions := map[string]string{}
	currentFuncName := ""
	currentCallId := ""
	var currentAIContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		logger.SugaredLogger.Infof("Received data: %s", line)
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
						//ch <- content
						//logger.SugaredLogger.Infof("Content data: %s", content)

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
						//ch <- reasoningContent
						ch <- map[string]any{
							"code":     1,
							"question": question,
							"chatId":   streamResponse.Id,
							"model":    streamResponse.Model,
							"content":  reasoningContent,
							"time":     time.Now().Format(time.DateTime),
						}

						//logger.SugaredLogger.Infof("ReasoningContent data: %s", reasoningContent)
						currentAIContent.WriteString(reasoningContent)

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
						logger.SugaredLogger.Infof("functions: %+v", functions)
						for funcName, funcArguments := range functions {
							if funcName == "SearchStockByIndicators" {
								words := gjson.Get(funcArguments, "words").String()

								ch <- map[string]any{
									"code":     1,
									"question": question,
									"chatId":   streamResponse.Id,
									"model":    streamResponse.Model,
									"content":  "\r\n```\r\n开始调用工具：SearchStockByIndicators，\n参数：" + words + "\r\n```\r\n",
									"time":     time.Now().Format(time.DateTime),
								}

								content := "无符合条件的数据"
								res := NewSearchStockApi(words).SearchStock(random.RandInt(5, 10))
								if convertor.ToString(res["code"]) == "100" {
									resData := res["data"].(map[string]any)
									result := resData["result"].(map[string]any)
									dataList := result["dataList"].([]any)
									columns := result["columns"].([]any)
									headers := map[string]string{}
									for _, v := range columns {
										//logger.SugaredLogger.Infof("v:%+v", v)
										d := v.(map[string]any)
										//logger.SugaredLogger.Infof("key:%s title:%s dateMsg:%s unit:%s", d["key"], d["title"], d["dateMsg"], d["unit"])
										title := convertor.ToString(d["title"])
										if convertor.ToString(d["dateMsg"]) != "" {
											title = title + "[" + convertor.ToString(d["dateMsg"]) + "]"
										}
										if convertor.ToString(d["unit"]) != "" {
											title = title + "(" + convertor.ToString(d["unit"]) + ")"
										}
										headers[d["key"].(string)] = title
									}
									table := &[]map[string]any{}
									for _, v := range dataList {
										d := v.(map[string]any)
										tmp := map[string]any{}
										for key, title := range headers {
											tmp[title] = convertor.ToString(d[key])
										}
										*table = append(*table, tmp)
									}
									jsonData, _ := json.Marshal(*table)
									markdownTable, _ := JSONToMarkdownTable(jsonData)
									//logger.SugaredLogger.Infof("markdownTable=\n%s", markdownTable)
									content = "\r\n### 工具筛选出的股票数据：\r\n" + markdownTable + "\r\n"
								}
								logger.SugaredLogger.Infof("SearchStockByIndicators:words:%s  --> \n%s", words, content)

								messages = append(messages, map[string]interface{}{
									"role":    "assistant",
									"content": currentAIContent.String(),
									"tool_calls": []map[string]any{
										{
											"id":           currentCallId,
											"tool_call_id": currentCallId,
											"type":         "function",
											"function": map[string]string{
												"name":       funcName,
												"arguments":  funcArguments,
												"parameters": funcArguments,
											},
										},
									},
								})
								messages = append(messages, map[string]interface{}{
									"role":         "tool",
									"content":      content,
									"tool_call_id": currentCallId,
								})

								//ch <- map[string]any{
								//	"code":     1,
								//	"question": question,
								//	"chatId":   streamResponse.Id,
								//	"model":    streamResponse.Model,
								//	"content":  "\r\n```\r\n调用工具：SearchStockByIndicators，\n结果：" + content + "\r\n```\r\n",
								//	"time":     time.Now().Format(time.DateTime),
								//}

							}

							if funcName == "GetStockKLine" {
								stockCode := gjson.Get(funcArguments, "stockCode").String()
								days := gjson.Get(funcArguments, "days").String()
								ch <- map[string]any{
									"code":     1,
									"question": question,
									"chatId":   streamResponse.Id,
									"model":    streamResponse.Model,
									"content":  "\r\n```\r\n开始调用工具：GetStockKLine，\n参数：" + stockCode + "," + days + "\r\n```\r\n",
									"time":     time.Now().Format(time.DateTime),
								}
								toIntDay, err := convertor.ToInt(days)
								if err != nil {
									toIntDay = 90
								}

								if strutil.HasPrefixAny(stockCode, []string{"sz", "sh", "hk", "us", "gb_"}) {
									K := &[]KLineData{}
									if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
										K = NewStockDataApi().GetKLineData(stockCode, "240", o.KDays)
									}
									if strutil.HasPrefixAny(stockCode, []string{"hk", "us", "gb_"}) {
										K = NewStockDataApi().GetHK_KLineData(stockCode, "day", o.KDays)
									}
									Kmap := &[]map[string]any{}
									for _, kline := range *K {
										mapk := make(map[string]any, 6)
										mapk["日期"] = kline.Day
										mapk["开盘价"] = kline.Open
										mapk["最高价"] = kline.High
										mapk["最低价"] = kline.Low
										mapk["收盘价"] = kline.Close
										Volume, _ := convertor.ToFloat(kline.Volume)
										mapk["成交量(万手)"] = Volume / 10000.00 / 100.00
										*Kmap = append(*Kmap, mapk)
									}
									jsonData, _ := json.Marshal(Kmap)
									markdownTable, _ := JSONToMarkdownTable(jsonData)
									logger.SugaredLogger.Infof("getKLineData=\n%s", markdownTable)

									messages = append(messages, map[string]interface{}{
										"role":    "assistant",
										"content": currentAIContent.String(),
										"tool_calls": []map[string]any{
											{
												"id":           currentCallId,
												"tool_call_id": currentCallId,
												"type":         "function",
												"function": map[string]string{
													"name":       funcName,
													"arguments":  funcArguments,
													"parameters": funcArguments,
												},
											},
										},
									})
									res := "\r\n ### " + stockCode + convertor.ToString(toIntDay) + "日K线数据：\r\n" + markdownTable + "\r\n"
									messages = append(messages, map[string]interface{}{
										"role":         "tool",
										"content":      res,
										"tool_call_id": currentCallId,
									})
									logger.SugaredLogger.Infof("GetStockKLine:stockCode:%s days:%s --> \n%s", stockCode, days, res)

									//ch <- map[string]any{
									//	"code":     1,
									//	"question": question,
									//	"chatId":   streamResponse.Id,
									//	"model":    streamResponse.Model,
									//	"content":  "\r\n```\r\n调用工具：GetStockKLine，\n结果：" + res + "\r\n```\r\n",
									//	"time":     time.Now().Format(time.DateTime),
									//}
								} else {
									messages = append(messages, map[string]interface{}{
										"role":    "assistant",
										"content": currentAIContent.String(),
										"tool_calls": []map[string]any{
											{
												"id":           currentCallId,
												"tool_call_id": currentCallId,
												"type":         "function",
												"function": map[string]string{
													"name":       funcName,
													"arguments":  funcArguments,
													"parameters": funcArguments,
												},
											},
										},
									})
									messages = append(messages, map[string]interface{}{
										"role":         "tool",
										"content":      "无数据，可能股票代码错误。（A股：sh,sz开头;港股hk开头,美股：us开头）",
										"tool_call_id": currentCallId,
									})
								}
							}

						}
						AskAiWithTools(o, err, messages, ch, question, tools)
					}

					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				if err != nil {
					logger.SugaredLogger.Infof("Stream data error : %s", err.Error())
					//ch <- err.Error()
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  err.Error(),
					}
				} else {
					logger.SugaredLogger.Infof("Stream data error : %s", data)
					//ch <- data
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
					//ch <- line
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
						AskAi(o, err, newMessages, ch, question)
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
func checkIsIndexBasic(stock string) bool {
	count := int64(0)
	db.Dao.Model(&IndexBasic{}).Where("name =  ?", stock).Count(&count)
	return count > 0
}

func SearchGuShiTongStockInfo(stock string, crawlTimeOut int64) *[]string {
	crawlerAPI := CrawlerApi{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()

	crawlerAPI = crawlerAPI.NewCrawler(ctx, CrawlerBaseInfo{
		Name:    "百度股市通",
		BaseUrl: "https://gushitong.baidu.com",
		Headers: map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	})
	url := "https://gushitong.baidu.com/stock/ab-" + RemoveAllNonDigitChar(stock)

	if strutil.HasPrefixAny(stock, []string{"HK", "hk"}) {
		url = "https://gushitong.baidu.com/stock/hk-" + RemoveAllNonDigitChar(stock)
	}
	if strutil.HasPrefixAny(stock, []string{"SZ", "SH", "sh", "sz"}) {
		url = "https://gushitong.baidu.com/stock/ab-" + RemoveAllNonDigitChar(stock)
	}
	if strutil.HasPrefixAny(stock, []string{"us", "US", "gb_", "gb"}) {
		url = "https://gushitong.baidu.com/stock/us-" + strings.Replace(stock, "gb_", "", 1)
	}

	//logger.SugaredLogger.Infof("SearchGuShiTongStockInfo搜索股票-%s: %s", stock, url)
	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitVisible("div.cos-tab"),
		chromedp.Click("div.cos-tab:nth-child(5)", chromedp.ByQuery),
		chromedp.ScrollIntoView("div.body-box"),
		chromedp.WaitVisible("div.body-col"),
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight);`, nil),
		chromedp.Sleep(1 * time.Second),
	}
	htmlContent, success := crawlerAPI.GetHtmlWithActions(&actions, true)
	var messages []string
	if success {
		document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return &[]string{}
		}
		document.Find("div.finance-hover,div.list-date").Each(func(i int, selection *goquery.Selection) {
			text := strutil.RemoveWhiteSpace(selection.Text(), false)
			messages = append(messages, ReplaceSensitiveWords(text))
			//logger.SugaredLogger.Infof("SearchGuShiTongStockInfo搜索到消息-%s: %s", "", text)
		})
		//logger.SugaredLogger.Infof("messages:%d", len(messages))
	}
	return &messages
}
func GetFinancialReportsByXUEQIU(stockCode string, crawlTimeOut int64) *[]string {
	if strutil.HasPrefixAny(stockCode, []string{"HK", "hk"}) {
		stockCode = strings.ReplaceAll(stockCode, "hk", "")
		stockCode = strings.ReplaceAll(stockCode, "HK", "")
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "gb_"}) {
		stockCode = strings.ReplaceAll(stockCode, "us", "")
		stockCode = strings.ReplaceAll(stockCode, "gb_", "")
	}
	url := fmt.Sprintf("https://xueqiu.com/snowman/S/%s/detail#/ZYCWZB", stockCode)
	waitVisible := "div.tab-table-responsive table"
	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://xueqiu.com",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	var markdown strings.Builder
	markdown.WriteString("\n## 财务数据：\n")
	html, ok := crawlerAPI.GetHtml(url, waitVisible, true)
	if !ok {
		return &[]string{""}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	GetTableMarkdown(document, waitVisible, &markdown)
	return &[]string{markdown.String()}
}
func GetFinancialReports(stockCode string, crawlTimeOut int64) *[]string {
	url := "https://emweb.securities.eastmoney.com/pc_hsf10/pages/index.html?type=web&code=" + stockCode + "#/cwfx"
	waitVisible := "div.report_table table"
	if strutil.HasPrefixAny(stockCode, []string{"HK", "hk"}) {
		stockCode = strings.ReplaceAll(stockCode, "hk", "")
		stockCode = strings.ReplaceAll(stockCode, "HK", "")
		url = "https://emweb.securities.eastmoney.com/PC_HKF10/pages/home/index.html?code=" + stockCode + "&type=web&color=w#/NewFinancialAnalysis"
		waitVisible = "div table.commonTable"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "gb_"}) {
		stockCode = strings.ReplaceAll(stockCode, "us", "")
		stockCode = strings.ReplaceAll(stockCode, "gb_", "")
		url = "https://emweb.securities.eastmoney.com/pc_usf10/pages/index.html?type=web&code=" + stockCode + "#/cwfx"
		waitVisible = "div.zyzb_table_detail table"

	}

	//logger.SugaredLogger.Infof("GetFinancialReports搜索股票-%s: %s", stockCode, url)

	crawlerAPI := CrawlerApi{}
	crawlerBaseInfo := CrawlerBaseInfo{
		Name:        "TestCrawler",
		Description: "Test Crawler Description",
		BaseUrl:     "https://emweb.securities.eastmoney.com",
		Headers:     map[string]string{"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36 Edg/133.0.0.0"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(crawlTimeOut)*time.Second)
	defer cancel()
	crawlerAPI = crawlerAPI.NewCrawler(ctx, crawlerBaseInfo)

	var markdown strings.Builder
	markdown.WriteString("\n## 财务数据：\n")
	html, ok := crawlerAPI.GetHtml(url, waitVisible, true)
	if !ok {
		return &[]string{""}
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	}
	GetTableMarkdown(document, waitVisible, &markdown)
	return &[]string{markdown.String()}
}

func GetTelegraphList(crawlTimeOut int64) *[]string {
	url := "https://www.cls.cn/telegraph"
	response, err := resty.New().SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(fmt.Sprintf(url))
	if err != nil {
		return &[]string{}
	}
	//logger.SugaredLogger.Info(string(response.Body()))
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body())))
	if err != nil {
		return &[]string{}
	}
	var telegraph []string
	document.Find("div.telegraph-content-box").Each(func(i int, selection *goquery.Selection) {
		//logger.SugaredLogger.Info(selection.Text())
		telegraph = append(telegraph, ReplaceSensitiveWords(selection.Text()))
	})
	return &telegraph
}

func GetTopNewsList(crawlTimeOut int64) *[]string {
	url := "https://www.cls.cn"
	response, err := resty.New().SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(fmt.Sprintf(url))
	if err != nil {
		return &[]string{}
	}
	//logger.SugaredLogger.Info(string(response.Body()))
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body())))
	if err != nil {
		return &[]string{}
	}
	var telegraph []string
	document.Find("div.home-article-title a,div.home-article-rec a").Each(func(i int, selection *goquery.Selection) {
		//logger.SugaredLogger.Info(selection.Text())
		telegraph = append(telegraph, ReplaceSensitiveWords(selection.Text()))
	})
	return &telegraph
}

func (o OpenAi) SaveAIResponseResult(stockCode, stockName, result, chatId, question string) {
	db.Dao.Create(&models.AIResponseResult{
		StockCode: stockCode,
		StockName: stockName,
		ModelName: o.Model,
		Content:   result,
		ChatId:    chatId,
		Question:  question,
	})
}

func (o OpenAi) GetAIResponseResult(stock string) *models.AIResponseResult {
	var result models.AIResponseResult
	db.Dao.Where("stock_code = ?", stock).Order("id desc").Limit(1).Find(&result)
	return &result
}
