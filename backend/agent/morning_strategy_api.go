package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// MorningStrategyApi 盘前策略 API
type MorningStrategyApi struct{}

func NewMorningStrategyApi() *MorningStrategyApi {
	return &MorningStrategyApi{}
}

// morningStrategyDefaultSysPrompt 内置盘前策略师系统提示词（sysPromptId==0 时使用）
const morningStrategyDefaultSysPrompt = `你是一位拥有20年A股实战经验的顶级盘前策略师，精通隔夜外围解读、情绪周期定位与情景推演。你的任务是基于给定的真实素材，输出一份专业、克制、可执行的今日盘前策略。

硬性规则：
1. 只能基于给定素材进行分析，素材中标注"（无数据）"的部分必须如实说明数据缺失，严禁编造任何数字、个股或板块名称。
2. 每个关键判断后用括号标注数据依据。
3. 关注标的池需给出：代码/名称/关注理由/触发条件（价格或形态），且必须来自素材（自选股清单、昨日复盘、龙虎榜）中出现的标的，不得凭空推荐。
4. 仓位与纪律建议要具体，风险提示具体化，不说空话。
5. 这是盘前策略，禁止给出"保证盈利"等收益承诺。

输出章节（Markdown 二级标题）：
## 一、隔夜与外围
（美股/欧洲/亚太主要指数涨跌解读）
## 二、昨日复盘要点回顾
（浓缩昨日复盘结论，无数据时说明）
## 三、今日推演
（乐观/中性/悲观三情景，各给触发条件与应对）
## 四、关注方向与标的池
（基于素材的结构化标的列表）
## 五、仓位与纪律建议
## 六、风险提示`

// GenerateMorningStrategy 生成盘前策略（全流程编排，同步执行，调用方自行决定是否放协程）
func (a *MorningStrategyApi) GenerateMorningStrategy(ctx context.Context, date string, aiConfigId, sysPromptId int, thinking bool, agentMode, triggerType string) (*models.MorningStrategy, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	logger.SugaredLogger.Infof("开始生成盘前策略：%s（trigger=%s, aiConfigId=%d, sysPromptId=%d, agentMode=%s）", date, triggerType, aiConfigId, sysPromptId, agentMode)

	// 并发防抖：当日已有 generating 记录且未超时（10 分钟，防止进程被杀导致永久卡 generating）
	var existing models.MorningStrategy
	db.Dao.Where("strategy_date = ?", date).First(&existing)
	if existing.ID != 0 && existing.Status == "generating" && time.Since(existing.UpdatedAt) < 10*time.Minute {
		return nil, fmt.Errorf("%s 的盘前策略正在生成中，请稍候", date)
	}

	// cron 触发时：昨日既无复盘也无市场统计（长假期间）直接跳过
	if triggerType == "cron" {
		prev := prevTradeDate(date)
		var prevReview models.DailyReview
		db.Dao.Where("review_date = ? AND status = ?", prev, "success").First(&prevReview)
		if prevReview.ID == 0 && len(data.NewMarketStatisticApi().GetByDate(prev)) == 0 {
			logger.SugaredLogger.Infof("%s 前一交易日无复盘与市场数据（可能为长假），跳过盘前策略生成", date)
			return nil, nil
		}
	}

	// Upsert：一天一条，重新生成覆盖
	strategy := existing
	if strategy.ID == 0 {
		strategy = models.MorningStrategy{StrategyDate: date}
	}
	strategy.Status = "generating"
	strategy.TriggerType = triggerType
	strategy.AiConfigId = aiConfigId
	strategy.SysPromptId = sysPromptId
	strategy.ErrorMessage = ""
	if err := db.Dao.Save(&strategy).Error; err != nil {
		return nil, fmt.Errorf("保存盘前策略记录失败：%v", err)
	}

	start := time.Now()
	material := a.buildMorningStrategyMaterial(date)
	prompt := fmt.Sprintf("请生成 %s 的盘前策略。\n\n# 盘前素材\n%s", date, material)

	// sysPromptId>0 时使用提示词模板内容（ChatWithContext 内部按 ID 加载），否则注入内置盘前策略提示词
	var sysPromptOverride string
	if sysPromptId == 0 {
		sysPromptOverride = morningStrategyDefaultSysPrompt
	}
	content := &strings.Builder{}
	emitter := newProgressEmitter(ctx, "morningStrategyProgress", date)
	ch := NewStockAiAgentApi().ChatWithContext(ctx, ChatRequest{
		Question:          prompt,
		AIConfigID:        aiConfigId,
		SysPromptID:       &sysPromptId,
		ThinkingMode:      thinking,
		AgentMode:         agentMode,
		SysPromptOverride: sysPromptOverride,
	})
	for msg := range ch {
		if msg == nil {
			continue
		}
		// 只消费最终报告正文（Content）：ReasoningContent 携带的是 [STEP] 步骤/工具调用预告与摘要等中间过程，
		// 不推送前端也不落库，用户只看最终结果
		if msg.Content != "" {
			content.WriteString(msg.Content)
			emitter.write(msg.Content)
		}
	}
	emitter.flush()

	result := strings.TrimSpace(content.String())
	generatedAt := time.Now()
	if result == "" {
		strategy.Status = "failed"
		strategy.ErrorMessage = "AI 返回内容为空"
		strategy.DurationMs = time.Since(start).Milliseconds()
		db.Dao.Save(&strategy)
		a.emitEvent(ctx, date, strategy)
		return nil, fmt.Errorf("盘前策略生成失败：AI 返回内容为空")
	}

	strategy.Status = "success"
	strategy.Content = result
	strategy.Summary = extractSummary(result, 200)
	strategy.GeneratedAt = &generatedAt
	strategy.DurationMs = time.Since(start).Milliseconds()
	if err := db.Dao.Save(&strategy).Error; err != nil {
		return nil, fmt.Errorf("保存盘前策略失败：%v", err)
	}

	logger.SugaredLogger.Infof("盘前策略生成完成：%s（耗时 %dms）", date, strategy.DurationMs)
	// 同步保存到 AI 分析报告，供研究中心查看
	go data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("盘前策略", "盘前策略", result, "", prompt)
	a.emitEvent(ctx, date, strategy)
	return &strategy, nil
}

// emitEvent 生成结束后向前端推送事件
func (a *MorningStrategyApi) emitEvent(ctx context.Context, date string, strategy models.MorningStrategy) {
	safeEventsEmit(ctx, "morningStrategyGenerated", map[string]any{
		"date":         date,
		"status":       strategy.Status,
		"summary":      strategy.Summary,
		"triggerType":  strategy.TriggerType,
		"errorMessage": strategy.ErrorMessage,
	})
}

// buildMorningStrategyMaterial 拼装盘前素材 Markdown（各段容错，无数据时显式标注防 AI 幻觉）
func (a *MorningStrategyApi) buildMorningStrategyMaterial(date string) string {
	sb := &strings.Builder{}
	prevDate := prevTradeDate(date)

	// 1. 昨日复盘全文（闭环核心）
	sb.WriteString("## 1. 昨日复盘报告\n")
	var prevReview models.DailyReview
	db.Dao.Where("review_date = ? AND status = ?", prevDate, "success").First(&prevReview)
	if prevReview.ID == 0 {
		// 回退：取最近一条成功复盘
		db.Dao.Where("review_date < ? AND status = ?", date, "success").Order("review_date DESC").First(&prevReview)
	}
	if prevReview.ID == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		content := prevReview.Content
		if len(content) > 4000 {
			content = content[:4000] + "...(截断)"
		}
		fmt.Fprintf(sb, "昨日(%s)复盘：\n%s\n", prevReview.ReviewDate, content)
	}

	// 2. 昨日收盘市场统计快照
	sb.WriteString("\n## 2. 昨日市场统计\n")
	stats := data.NewMarketStatisticApi().GetByDate(prevDate)
	if len(stats) == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		last := stats[len(stats)-1]
		fmt.Fprintf(sb, "- %s 收盘：上涨 %d 家 / 下跌 %d 家，涨停 %d 家 / 跌停 %d 家，市场情绪：%s\n",
			last.DataDate, last.UpCount, last.DownCount, last.LimitUp, last.LimitDown, last.SentimentDesc)
	}

	// 3. 隔夜外围（全球指数缓存）
	sb.WriteString("\n## 3. 隔夜外围市场\n")
	var indexes []models.GlobalStockIndex
	db.Dao.Find(&indexes)
	if len(indexes) == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		// 主要指数优先（美股三大 + 亚太主要 + 欧洲主要），其余忽略避免素材过长
		keywords := []string{"道琼斯", "纳斯达克", "标普", "恒生", "日经", "韩国", "富时", "德国", "法国", "上证", "深证", "创业板"}
		for _, kw := range keywords {
			for _, idx := range indexes {
				if strings.Contains(idx.Name, kw) {
					fmt.Fprintf(sb, "- %s（%s）：%.2f%% 报 %s\n", idx.Name, idx.Location, parseZdf(idx.Zdf), idx.Zxj)
					break
				}
			}
		}
	}

	// 4. 昨日龙虎榜游资动向摘要（此时数据已可得）
	sb.WriteString("\n## 4. 昨日龙虎榜游资/机构动向\n")
	summary := data.NewLhbSeatApi().GetLhbDailySummary(prevDate)
	if summary == nil || (len(summary.HotMoneyActivities) == 0 && len(summary.InstitutionActions) == 0) {
		sb.WriteString("（无数据）\n")
	} else {
		fmt.Fprintf(sb, "上榜个股 %d 只\n", summary.StockCount)
		top := 8
		if len(summary.HotMoneyActivities) < top {
			top = len(summary.HotMoneyActivities)
		}
		for i := 0; i < top; i++ {
			hm := summary.HotMoneyActivities[i]
			fmt.Fprintf(sb, "- %s（%s）：买入 %.0f 万元 / 卖出 %.0f 万元", hm.HotMoneyName, hm.Tier, hm.TotalBuy/10000, hm.TotalSell/10000)
			if len(hm.Stocks) > 0 {
				sb.WriteString("，操作：")
				for j, st := range hm.Stocks {
					if j >= 3 {
						break
					}
					fmt.Fprintf(sb, "%s(%.2f%%) ", st.StockName, st.ChangeRate)
				}
			}
			sb.WriteString("\n")
		}
	}

	// 5. 自选股清单
	sb.WriteString("\n## 5. 自选股清单\n")
	var stocks []data.FollowedStock
	db.Dao.Order("sort ASC").Find(&stocks)
	if len(stocks) == 0 {
		sb.WriteString("（无数据，自选股为空）\n")
	} else {
		limit := 20
		if len(stocks) < limit {
			limit = len(stocks)
		}
		for i := 0; i < limit; i++ {
			s := stocks[i]
			fmt.Fprintf(sb, "- %s[%s] 现价 %.2f，涨跌 %.2f%%", s.Name, s.StockCode, s.Price, s.ChangePercent)
			if s.StopLossPrice > 0 || s.TakeProfitPrice > 0 {
				fmt.Fprintf(sb, "，止损 %.2f / 止盈 %.2f", s.StopLossPrice, s.TakeProfitPrice)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// parseZdf 解析 GlobalStockIndex.Zdf（字符串涨跌幅，如 "-1.23"）
func parseZdf(zdf string) float64 {
	var v float64
	fmt.Sscanf(strings.TrimSpace(zdf), "%f", &v)
	return v
}

// GetMorningStrategyByDate 按日期查询盘前策略
func (a *MorningStrategyApi) GetMorningStrategyByDate(date string) *models.MorningStrategy {
	var strategy models.MorningStrategy
	err := db.Dao.Where("strategy_date = ?", date).First(&strategy).Error
	if err != nil {
		return nil
	}
	return &strategy
}

// GetLatestMorningStrategy 查询最近一条盘前策略
func (a *MorningStrategyApi) GetLatestMorningStrategy() *models.MorningStrategy {
	var strategy models.MorningStrategy
	err := db.Dao.Order("strategy_date DESC").First(&strategy).Error
	if err != nil {
		return nil
	}
	return &strategy
}

// GetMorningStrategyList 分页查询历史盘前策略（列表不带全文）
func (a *MorningStrategyApi) GetMorningStrategyList(page, pageSize int) *models.MorningStrategyPageData {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	var total int64
	db.Dao.Model(&models.MorningStrategy{}).Count(&total)
	var list []models.MorningStrategy
	db.Dao.Order("strategy_date DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	for i := range list {
		list[i].Content = ""
	}
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &models.MorningStrategyPageData{List: list, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}
}

// DeleteMorningStrategy 删除盘前策略
func (a *MorningStrategyApi) DeleteMorningStrategy(id uint) error {
	return db.Dao.Delete(&models.MorningStrategy{}, id).Error
}
