package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// DailyReviewApi 每日自动复盘 API
type DailyReviewApi struct{}

func NewDailyReviewApi() *DailyReviewApi {
	return &DailyReviewApi{}
}

// dailyReviewDefaultSysPrompt 内置复盘专家系统提示词（sysPromptId==0 时使用）
const dailyReviewDefaultSysPrompt = `你是一位拥有20年A股实战经验的顶级盘后复盘教练，精通情绪周期、龙头战法与仓位管理。你的任务是基于给定的当日真实数据，输出一份专业、克制、可执行的每日复盘报告。

硬性规则：
1. 只能基于给定素材进行分析，素材中标注"（无数据）"的部分必须如实说明数据缺失，严禁编造任何数字、个股或板块名称。
2. 每个关键结论后用括号标注数据依据，如（上涨家数 3200，涨停 65 家）。
3. 复盘重点是"执行纪律检验"而非单纯涨跌描述，用户交易记录为空时要明确指出"今日无交易"。
4. 风险提示具体化，不说空话。

输出章节（Markdown 二级标题）：
## 一、市场概况与情绪定位
（涨跌家数、涨跌停、情绪描述，结合近5日趋势定位当前情绪周期阶段）
## 二、主线与板块
（基于素材推断当日主线，无把握时明确说明）
## 三、我的交易复盘
（对照当日交易记录逐笔点评：买卖时点/价格是否合理/心态纪律评分；无交易则给"空仓/观望纪律评分"）
## 四、龙虎榜与资金
（游资动向、机构动向解读，数据缺失时说明）
## 五、明日展望
（基于今日数据的推演要点与注意事项，不超过 5 条）`

// GenerateDailyReview 生成每日复盘报告（全流程编排，同步执行，调用方自行决定是否放协程）
func (a *DailyReviewApi) GenerateDailyReview(ctx context.Context, date string, aiConfigId, sysPromptId int, thinking bool, agentMode, triggerType string) (*models.DailyReview, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	logger.SugaredLogger.Infof("开始生成每日复盘报告：%s（trigger=%s, aiConfigId=%d, sysPromptId=%d, agentMode=%s）", date, triggerType, aiConfigId, sysPromptId, agentMode)

	// 并发防抖：当日已有 generating 记录且未超时（10 分钟，防止进程被杀导致永久卡 generating）
	var existing models.DailyReview
	db.Dao.Where("review_date = ?", date).First(&existing)
	if existing.ID != 0 && existing.Status == "generating" && time.Since(existing.UpdatedAt) < 10*time.Minute {
		return nil, fmt.Errorf("%s 的复盘报告正在生成中，请稍候", date)
	}

	// cron 触发时无市场统计数据（节假日/非交易日）直接跳过，不生成空报告
	if triggerType == "cron" && len(data.NewMarketStatisticApi().GetByDate(date)) == 0 {
		logger.SugaredLogger.Infof("%s 无市场统计数据（可能为非交易日），跳过复盘生成", date)
		return nil, nil
	}

	// 当日数据先刷新一次，确保拿到最新快照
	if date == time.Now().Format("2006-01-02") {
		_ = data.NewMarketStatisticApi().FetchAndSave()
	}

	// Upsert：一天一条，重新生成覆盖
	review := existing
	if review.ID == 0 {
		review = models.DailyReview{ReviewDate: date}
	}
	review.Status = "generating"
	review.TriggerType = triggerType
	review.AiConfigId = aiConfigId
	review.SysPromptId = sysPromptId
	review.ErrorMessage = ""
	if err := db.Dao.Save(&review).Error; err != nil {
		return nil, fmt.Errorf("保存复盘记录失败：%v", err)
	}

	start := time.Now()
	material := a.buildDailyReviewMaterial(date)
	prompt := fmt.Sprintf("请生成 %s 的每日复盘报告。\n\n# 当日素材\n%s", date, material)

	// sysPromptId>0 时使用提示词模板内容（ChatWithContext 内部按 ID 加载），否则注入内置复盘提示词
	var sysPromptOverride string
	if sysPromptId == 0 {
		sysPromptOverride = dailyReviewDefaultSysPrompt
	}
	content := &strings.Builder{}
	emitter := newProgressEmitter(ctx, "dailyReviewProgress", date)
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
		review.Status = "failed"
		review.ErrorMessage = "AI 返回内容为空"
		review.DurationMs = time.Since(start).Milliseconds()
		db.Dao.Save(&review)
		a.emitEvent(ctx, date, review)
		return nil, fmt.Errorf("复盘报告生成失败：AI 返回内容为空")
	}

	review.Status = "success"
	review.Content = result
	review.Summary = extractSummary(result, 200)
	review.GeneratedAt = &generatedAt
	review.DurationMs = time.Since(start).Milliseconds()
	if err := db.Dao.Save(&review).Error; err != nil {
		return nil, fmt.Errorf("保存复盘报告失败：%v", err)
	}

	logger.SugaredLogger.Infof("每日复盘报告生成完成：%s（耗时 %dms）", date, review.DurationMs)
	// 同步保存到 AI 分析报告，供研究中心查看
	go data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult("每日复盘", "每日复盘", result, "", prompt)
	a.emitEvent(ctx, date, review)
	return &review, nil
}

// emitEvent 生成结束后向前端推送事件（页面刷新 + 全局通知）
func (a *DailyReviewApi) emitEvent(ctx context.Context, date string, review models.DailyReview) {
	safeEventsEmit(ctx, "dailyReviewGenerated", map[string]any{
		"date":         date,
		"status":       review.Status,
		"summary":      review.Summary,
		"triggerType":  review.TriggerType,
		"errorMessage": review.ErrorMessage,
	})
}

// buildDailyReviewMaterial 拼装当日复盘素材 Markdown（各段容错，无数据时显式标注防 AI 幻觉）
func (a *DailyReviewApi) buildDailyReviewMaterial(date string) string {
	sb := &strings.Builder{}

	// 1. 市场统计：当日收盘快照 + 情绪演变 + 近5日趋势
	sb.WriteString("## 1. 市场统计数据\n")
	stats := data.NewMarketStatisticApi().GetByDate(date)
	if len(stats) == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		last := stats[len(stats)-1]
		fmt.Fprintf(sb, "- 收盘快照（%s %s）：上涨 %d 家 / 下跌 %d 家，涨停 %d 家 / 跌停 %d 家，市场情绪：%s（沪：%d涨/%d跌，深：%d涨/%d跌）\n",
			last.DataDate, last.DataTime, last.UpCount, last.DownCount, last.LimitUp, last.LimitDown,
			last.SentimentDesc, last.ShUpCount, last.ShDownCount, last.SzUpCount, last.SzDownCount)
		if len(stats) > 1 {
			sb.WriteString("- 当日情绪演变：")
			for _, s := range stats {
				fmt.Fprintf(sb, "%s %s(涨%d/跌%d,涨停%d) → ", s.DataTime, s.SentimentDesc, s.UpCount, s.DownCount, s.LimitUp)
			}
			sb.WriteString("\n")
		}
	}
	recent := data.NewMarketStatisticApi().GetRecentDaysData(5)
	if len(recent) > 0 {
		sb.WriteString("- 近5日趋势：")
		seen := map[string]bool{}
		for _, s := range recent {
			if seen[s.DataDate] {
				continue
			}
			if s.DataDate > date {
				continue
			}
			seen[s.DataDate] = true
			fmt.Fprintf(sb, "%s(%s,涨%d/跌%d,涨停%d) ", s.DataDate, s.SentimentDesc, s.UpCount, s.DownCount, s.LimitUp)
		}
		sb.WriteString("\n")
	}

	// 2. 当日交易记录
	sb.WriteString("\n## 2. 当日交易记录\n")
	trades, err := data.NewStockDataApi().GetTradingRecordList(data.TradingRecordListQuery{Page: 1, PageSize: 50, StartDate: date, EndDate: date})
	if err != nil || trades == nil || len(trades.List) == 0 {
		sb.WriteString("（无数据，今日无交易）\n")
	} else {
		fmt.Fprintf(sb, "共 %d 笔：\n", trades.Total)
		for _, t := range trades.List {
			mindset := ""
			if t.Mindset != "" {
				mindset = "，心态：" + t.Mindset
			}
			reason := ""
			if t.Reason != "" {
				reason = "，原因：" + t.Reason
			}
			fmt.Fprintf(sb, "- %s[%s] %s %.2f元 x %d股%s%s\n", t.StockName, t.StockCode, t.Direction, t.Price, t.Volume, reason, mindset)
		}
		if stat, e := data.NewStockDataApi().GetTradingRecordStatistics(); e == nil && stat != nil {
			fmt.Fprintf(sb, "- 当日合计：买入 %.2f 元，卖出 %.2f 元，今日总盈亏 %.2f 元（收益率 %.2f%%）\n",
				stat.TodayBuyAmount, stat.TodaySellAmount, stat.TodayProfit, stat.TodayProfitRate)
		}
	}

	// 3. 龙虎榜游资动向（收盘后约17点发布，可能无数据）
	sb.WriteString("\n## 3. 龙虎榜游资/机构动向\n")
	summary := data.NewLhbSeatApi().GetLhbDailySummary(date)
	if summary == nil || (len(summary.HotMoneyActivities) == 0 && len(summary.InstitutionActions) == 0) {
		sb.WriteString("（无数据，龙虎榜一般于收盘后约17点发布）\n")
	} else {
		fmt.Fprintf(sb, "当日上榜个股 %d 只\n", summary.StockCount)
		top := 10
		if len(summary.HotMoneyActivities) < top {
			top = len(summary.HotMoneyActivities)
		}
		for i := 0; i < top; i++ {
			hm := summary.HotMoneyActivities[i]
			fmt.Fprintf(sb, "- %s（%s，风格：%s）：合计买入 %.0f 万元，合计卖出 %.0f 万元", hm.HotMoneyName, hm.Tier, hm.Style, hm.TotalBuy/10000, hm.TotalSell/10000)
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
		if len(summary.InstitutionActions) > 0 {
			instTop := 5
			if len(summary.InstitutionActions) < instTop {
				instTop = len(summary.InstitutionActions)
			}
			sb.WriteString("机构动向（净额居前）：\n")
			for i := 0; i < instTop; i++ {
				ia := summary.InstitutionActions[i]
				fmt.Fprintf(sb, "- %s[%s] %.2f%%：买%d席/卖%d席，净额 %.0f 万元\n",
					ia.StockName, ia.StockCode, ia.ChangeRate, ia.BuyCount, ia.SellCount, ia.Net/10000)
			}
		}
	}

	// 4. 对照素材：昨日盘前策略预判 + 前一交易日复盘摘要
	prevDate := prevTradeDate(date)
	sb.WriteString("\n## 4. 昨日盘前策略预判（用于检验实际走势 vs 预判）\n")
	var prevStrategy models.MorningStrategy
	db.Dao.Where("strategy_date = ? AND status = ?", prevDate, "success").First(&prevStrategy)
	if prevStrategy.ID == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		fmt.Fprintf(sb, "昨日(%s)策略摘要：%s\n", prevDate, prevStrategy.Summary)
		if len(prevStrategy.Content) > 1500 {
			sb.WriteString(prevStrategy.Content[:1500] + "...(截断)\n")
		} else {
			sb.WriteString(prevStrategy.Content + "\n")
		}
	}
	sb.WriteString("\n## 5. 前一交易日复盘摘要（衔接参考）\n")
	var prevReview models.DailyReview
	db.Dao.Where("review_date < ? AND status = ?", date, "success").Order("review_date DESC").First(&prevReview)
	if prevReview.ID == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		fmt.Fprintf(sb, "%s 复盘摘要：%s\n", prevReview.ReviewDate, prevReview.Summary)
	}

	return sb.String()
}

// GetDailyReviewByDate 按日期查询复盘报告
func (a *DailyReviewApi) GetDailyReviewByDate(date string) *models.DailyReview {
	var review models.DailyReview
	err := db.Dao.Where("review_date = ?", date).First(&review).Error
	if err != nil {
		return nil
	}
	// 陈旧 generating 记录自动复位（进程被杀/异常退出导致，避免前端按钮永久锁定）
	if review.Status == "generating" && time.Since(review.UpdatedAt) > 10*time.Minute {
		review.Status = "failed"
		review.ErrorMessage = "生成中断（超过10分钟无进度，已自动复位，可重新生成）"
		db.Dao.Model(&models.DailyReview{}).Where("id = ?", review.ID).Updates(map[string]interface{}{
			"status": "failed", "error_message": review.ErrorMessage,
		})
	}
	return &review
}

// GetLatestDailyReview 查询最近一条复盘报告
func (a *DailyReviewApi) GetLatestDailyReview() *models.DailyReview {
	var review models.DailyReview
	err := db.Dao.Order("review_date DESC").First(&review).Error
	if err != nil {
		return nil
	}
	return &review
}

// GetDailyReviewList 分页查询历史复盘报告（列表不带全文）
func (a *DailyReviewApi) GetDailyReviewList(page, pageSize int) *models.DailyReviewPageData {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	var total int64
	db.Dao.Model(&models.DailyReview{}).Count(&total)
	var list []models.DailyReview
	db.Dao.Order("review_date DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	for i := range list {
		list[i].Content = ""
	}
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return &models.DailyReviewPageData{List: list, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}
}

// DeleteDailyReview 删除复盘报告
func (a *DailyReviewApi) DeleteDailyReview(id uint) error {
	return db.Dao.Delete(&models.DailyReview{}, id).Error
}

// ---------------- 通用辅助（复盘/盘前策略共用） ----------------

// safeEventsEmit 安全推送 Wails 事件：ctx 非法或未初始化时静默跳过（测试环境等场景）
func safeEventsEmit(ctx context.Context, event string, payload any) {
	if ctx == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Warnf("emit event %s panic recovered: %v", event, r)
			}
		}()
		runtime.EventsEmit(ctx, event, payload)
	}()
}

// progressEmitter 流式进度事件发射器（复盘/盘前策略共用）：
// AI 输出为 token 级分片，逐条直发会造成 IPC 洪泛与前端 Markdown 频繁重渲染，
// 这里按 300ms 节流合并增量后以 {date, chunk} 事件推送，生成结束后 Flush 清空缓冲。
type progressEmitter struct {
	ctx   context.Context
	event string
	date  string
	buf   strings.Builder
	last  time.Time
}

func newProgressEmitter(ctx context.Context, event, date string) *progressEmitter {
	return &progressEmitter{ctx: ctx, event: event, date: date, last: time.Now()}
}

// write 写入一段增量内容，距上次推送超过节流间隔时立即推送
func (p *progressEmitter) write(chunk string) {
	if chunk == "" {
		return
	}
	p.buf.WriteString(chunk)
	if time.Since(p.last) >= 300*time.Millisecond {
		p.flush()
	}
}

// flush 推送缓冲区中的增量内容（空缓冲为空操作）
func (p *progressEmitter) flush() {
	if p.buf.Len() == 0 {
		return
	}
	safeEventsEmit(p.ctx, p.event, map[string]any{
		"date":  p.date,
		"chunk": p.buf.String(),
	})
	p.buf.Reset()
	p.last = time.Now()
}

// extractSummary 从 Markdown 正文提取摘要：去掉 Markdown 符号后截取前 maxRunes 个字符
func extractSummary(content string, maxRunes int) string {
	if content == "" {
		return ""
	}
	// 优先取第一个非标题段落作摘要
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		content = line
		break
	}
	replacer := strings.NewReplacer("#", "", "*", "", ">", "", "`", "", "|", "", "-", "", "\n", " ")
	s := replacer.Replace(content)
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return s
}

// prevTradeDate 取 date 的前一交易日（跳过周末，不处理法定节假日）
func prevTradeDate(date string) string {
	t, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return ""
	}
	for {
		t = t.AddDate(0, 0, -1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			return t.Format("2006-01-02")
		}
	}
}

// FirstAiConfigId aiConfigId 为 0 时回退到第一个 AI 配置（保证默认任务开箱即用）
func FirstAiConfigId(aiConfigId int) int {
	if aiConfigId > 0 {
		return aiConfigId
	}
	config := data.GetSettingConfig()
	if len(config.AiConfigs) > 0 {
		return int(config.AiConfigs[0].ID)
	}
	return 0
}

// pushReportExternal 外部推送（飞书/钉钉，飞书卡片有长度限制截断到 3000 字）
func pushReportExternal(title, content string, pushFeishu, pushDingDing bool) {
	if pushFeishu {
		msg := content
		if utf8.RuneCountInString(msg) > 3000 {
			msg = string([]rune(msg)[:3000]) + "\n\n...(内容过长已截断，完整报告请查看软件)"
		}
		go data.NewFeishuAPI().SendToFeishu(title, msg)
	}
	if pushDingDing {
		msg := content
		if utf8.RuneCountInString(msg) > 3000 {
			msg = string([]rune(msg)[:3000])
		}
		go data.NewDingDingAPI().SendToDingDing(title, msg)
	}
}
