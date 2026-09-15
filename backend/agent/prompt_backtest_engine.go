package agent

// prompt_backtest_engine.go — 提示词模板主动回测引擎（阶段二）。
//
// 目标：批量重放历史交易日，对比多个提示词模板在同一素材下的选股质量：
//   任务 = 模板列表 × 采样交易日 × 重复次数 次 AI 调用
//   每次：拼装该交易日可见的市场素材（防未来数据 look-ahead）→ 模板作为系统提示词
//        （追加固定输出契约）→ AI 输出 JSON 选股 → 解析入库（prompt_backtest_picks）
//   全部调用完成后统一计算每条选股的后 N 交易日实际收益（vs 沪深300 超额），
//   再按模板聚合统计（胜率/收益/波动率/CV/回撤/Jaccard 稳定性），与阶段一的
//   被动回测指标口径一致（template_backtest.go）。
//
// 防未来数据：素材全部按日期查询历史快照（指数K线过滤 ≤ 当日、市场统计/龙虎榜/
// 板块资金流/涨停梯队按日期取当日数据）；仅入选股日后 N 日收益用于评估。
// 注意：ChatWithContext 内部注入的实时时间上下文与"今天是 D"存在冲突，
// 通过提问中的强锚定声明缓解（与每日复盘同一链路，实测可用）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const (
	// promptBacktestMaxDates 单任务采样交易日上限（控制 LLM 调用成本）
	promptBacktestMaxDates = 60
	// promptBacktestMaxTemplates 单任务模板数上限
	promptBacktestMaxTemplates = 5
	// promptBacktestCallTimeout 单次 AI 调用超时
	promptBacktestCallTimeout = 10 * time.Minute
	// promptBacktestSceneMarker 回测提问中的场景标记，isPromptBacktestCall 据此识别回测调用
	promptBacktestSceneMarker = "模拟回测场景"
	// promptBacktestContractMarker 回测输出契约的固定前缀，isPromptBacktestCall 据此识别回测 sysPrompt
	promptBacktestContractMarker = "【输出格式要求（必须严格遵守"
)

// promptBacktestOutputContract 追加在模板内容之后的固定输出契约（保证跨模板可比性）。
func promptBacktestOutputContract(periodDays, topN int) string {
	return fmt.Sprintf(`

`+promptBacktestContractMarker+`，优先级高于上文任何输出格式约定）】
你需要基于用户给出的当日真实素材，选出你认为未来 %d 个交易日最具上涨潜力的 A 股股票，不超过 %d 只。
不要调用任何工具，不要输出任何分析过程、解释或代码块标记，只输出一个 JSON 数组，格式如下：
[{"code":"600000","name":"浦发银行","rating":"强烈看好","reason":"一句话理由"}]
rating 取值：强烈看好/看好/中性。若素材不足无法选出，输出 []。`, periodDays, topN)
}

// promptBacktestPickJSON AI 输出的选股结构。
type promptBacktestPickJSON struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Rating string `json:"rating"`
	Reason string `json:"reason"`
}

// extractPicksJSON 从 AI 输出文本中提取选股 JSON 数组（兼容代码块包裹、前后杂文）。
func extractPicksJSON(content string) []promptBacktestPickJSON {
	s := strings.TrimSpace(content)
	// 去除 markdown 代码块
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx > 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	// 取第一个 [ 到最后一个 ]（数组）；若模型输出的是 {"stocks":[...]} 对象则取对象再找数组
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return nil
	}
	var picks []promptBacktestPickJSON
	if err := json.Unmarshal([]byte(s[start:end+1]), &picks); err != nil {
		return nil
	}
	return picks
}

// normalizePickCode 规范化股票代码为 6 位数字（去交易所前缀/后缀）。
func normalizePickCode(code string) string {
	code = strings.TrimSpace(code)
	// sh600000 / sz000001 / 600000.SH / 000001.SZ
	for _, sep := range []string{".", ":"} {
		if i := strings.IndexAny(code, sep); i > 0 {
			part1, part2 := code[:i], code[i+1:]
			if len(part1) == 6 && allDigits(part1) {
				code = part1
			} else if len(part2) == 6 && allDigits(part2) {
				code = part2
			}
		}
	}
	if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
		code = code[2:]
	}
	if len(code) != 6 || !allDigits(code) {
		return ""
	}
	return code
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ---- 交易日历（以沪深300 日K为日历源，天然跳过节假日） ----

// promptBacktestTradeDays 返回 [start, end] 内、且后续仍有 periodDays 个交易日
// （保证收益可计算）的交易日列表（升序），以沪深300 K线日期为准。
func promptBacktestTradeDays(start, end string, periodDays int) []string {
	res := data.FetchKLineWithFallback(backtestBenchmarkCode, "沪深300", "101", 300, "")
	if res == nil || res.Data == nil {
		return nil
	}
	bars := *res.Data
	var days []string
	for i := 0; i < len(bars); i++ {
		d := strings.TrimSpace(bars[i].Day)
		d = normalizeKLineDay(d)
		if d == "" {
			continue
		}
		if d < start || d > end {
			continue
		}
		// 需要后续 periodDays 根K线（含当日自身为第 0 根）
		if i+periodDays >= len(bars) {
			continue
		}
		days = append(days, d)
	}
	return days
}

// normalizeKLineDay 兼容 "2006-01-02" 与 "20060102" 两种日期格式，返回 yyyy-MM-dd。
func normalizeKLineDay(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 8 && allDigits(s) {
		return s[:4] + "-" + s[4:6] + "-" + s[6:]
	}
	if t, err := parseKLineDay(s); err == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

// ---- 素材拼装（全部按日期，防未来数据） ----

// buildPromptBacktestMaterial 拼装某交易日收盘后可见的市场素材 Markdown（各段容错，
// 无数据标注"（无数据）"防 AI 幻觉）。同一日期跨模板复用（调用方缓存）。
func buildPromptBacktestMaterial(date string) string {
	sb := &strings.Builder{}

	// 1. 指数行情：上证/沪深300/创业板 近 10 根日K（过滤 ≤ date）
	sb.WriteString("## 1. 指数近10日行情\n")
	for _, idx := range []struct{ code, name string }{
		{"000001.SH", "上证指数"},
		{"000300.SH", "沪深300"},
		{"399006.SZ", "创业板指"},
	} {
		sb.WriteString("- " + idx.name + "：")
		if res := data.FetchKLineWithFallback(idx.code, idx.name, "101", 40, ""); res != nil && res.Data != nil {
			bars := *res.Data
			cnt := 0
			for i := len(bars) - 1; i >= 0 && cnt < 10; i-- {
				d := normalizeKLineDay(bars[i].Day)
				if d == "" || d > date {
					continue
				}
				if cnt > 0 {
					sb.WriteString("，")
				}
				fmt.Fprintf(sb, "%s收%s(%s%%)", d[5:], bars[i].Close, bars[i].ChangePercent)
				cnt++
			}
			if cnt == 0 {
				sb.WriteString("（无数据）")
			}
		} else {
			sb.WriteString("（无数据）")
		}
		sb.WriteString("\n")
	}

	// 2. 市场统计（当日收盘快照）
	sb.WriteString("\n## 2. 市场统计\n")
	stats := data.NewMarketStatisticApi().GetByDate(date)
	if len(stats) == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		last := stats[len(stats)-1]
		fmt.Fprintf(sb, "- %s：上涨 %d 家 / 下跌 %d 家，涨停 %d 家 / 跌停 %d 家，市场情绪：%s\n",
			last.DataDate, last.UpCount, last.DownCount, last.LimitUp, last.LimitDown, last.SentimentDesc)
	}

	// 3. 涨停梯队（当日，JSON 原样）
	sb.WriteString("\n## 3. 涨停梯队（" + date + "）\n")
	res, actualDate := data.NewMarketNewsApi().GetUplimitHotSmart(date, 20)
	if res != nil {
		if bs, err := json.Marshal(res); err == nil && strings.TrimSpace(string(bs)) != "null" {
			sb.WriteString("数据日期：" + actualDate + "\n```json\n" + string(bs) + "\n```\n")
		} else {
			sb.WriteString("（无数据）\n")
		}
	} else {
		sb.WriteString("（无数据）\n")
	}

	// 4. 龙虎榜游资动向（当日）
	sb.WriteString("\n## 4. 龙虎榜游资/机构动向\n")
	summary := data.NewLhbSeatApi().GetLhbDailySummary(date)
	if summary == nil || (len(summary.HotMoneyActivities) == 0 && len(summary.InstitutionActions) == 0) {
		sb.WriteString("（无数据）\n")
	} else {
		fmt.Fprintf(sb, "当日上榜个股 %d 只\n", summary.StockCount)
		top := 10
		if len(summary.HotMoneyActivities) < top {
			top = len(summary.HotMoneyActivities)
		}
		for i := 0; i < top; i++ {
			hm := summary.HotMoneyActivities[i]
			fmt.Fprintf(sb, "- %s（%s）：合计买入 %.0f 万元，合计卖出 %.0f 万元", hm.HotMoneyName, hm.Tier, hm.TotalBuy/10000, hm.TotalSell/10000)
			if len(hm.Stocks) > 0 {
				sb.WriteString("，操作：")
				for j, st := range hm.Stocks {
					if j >= 3 {
						break
					}
					fmt.Fprintf(sb, "%s(买%.0f万/卖%.0f万) ", st.StockName, st.Buy/10000, st.Sell/10000)
				}
			}
			sb.WriteString("\n")
		}
	}

	// 5. 板块资金流（当日 top10 主力净流入）
	sb.WriteString("\n## 5. 板块主力资金净流入TOP10\n")
	flows := data.NewBKFundFlowApi().GetBKFundFlowTopListByDate(date, 10)
	if len(flows) == 0 {
		sb.WriteString("（无数据）\n")
	} else {
		for _, f := range flows {
			fmt.Fprintf(sb, "- %s：主力净流入 %.2f 亿元\n", f.Name, float64(f.NetInflow)/1e8)
		}
	}

	return sb.String()
}

// ---- AI 调用与解析 ----

// runPromptBacktestCall 执行一次 AI 选股调用，返回解析后的选股列表与原始输出。
// agentMode 显式锁定 React：回测问题携带长素材，空模式经 classifyComplexity
// 必然分到 PlanExecute（wordCount>80），规划+工具调用行为偏离输出契约且失败会
// 降级 React 造成两次行为差异；单轮 React 最贴合"读素材→输出 JSON"的回测语义。
func runPromptBacktestCall(ctx context.Context, sysPrompt, question string, aiConfigId int) (string, error) {
	ch := NewStockAiAgentApi().ChatWithContext(ctx, question, aiConfigId, nil, false, 0, false, string(React), sysPrompt)
	var content strings.Builder
	timeout := time.After(promptBacktestCallTimeout)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return content.String(), nil
			}
			if msg != nil && msg.Content != "" {
				content.WriteString(msg.Content)
			}
		case <-timeout:
			return content.String(), fmt.Errorf("AI 调用超时（%v）", promptBacktestCallTimeout)
		case <-ctx.Done():
			return content.String(), ctx.Err()
		}
	}
}

// ---- 任务编排 ----

// StartPromptBacktestTask 异步执行回测任务（goroutine），进度经 promptBacktestProgress 事件推送。
func StartPromptBacktestTask(ctx context.Context, taskId uint) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("提示词回测任务 %d panic：%v", taskId, r)
				// panic 兜底：标记失败，避免任务永远停留在 running
				var task models.PromptBacktestTask
				if err := db.Dao.First(&task, taskId).Error; err == nil {
					task.Status = "failed"
					task.ErrorMessage = fmt.Sprintf("内部错误：%v", r)
					task.ProgressMsg = task.ErrorMessage
					db.Dao.Save(&task)
					emitBacktestProgress(ctx, &task)
				}
			}
		}()
		if err := runPromptBacktestTask(ctx, taskId); err != nil {
			logger.SugaredLogger.Errorf("提示词回测任务 %d 失败：%v", taskId, err)
		}
	}()
}

func emitBacktestProgress(ctx context.Context, task *models.PromptBacktestTask) {
	safeEventsEmit(ctx, "promptBacktestProgress", map[string]any{
		"taskId":      task.ID,
		"status":      task.Status,
		"progress":    task.Progress,
		"progressMsg": task.ProgressMsg,
	})
}

func runPromptBacktestTask(ctx context.Context, taskId uint) error {
	start := time.Now()
	var task models.PromptBacktestTask
	if err := db.Dao.First(&task, taskId).Error; err != nil {
		return fmt.Errorf("任务不存在：%w", err)
	}
	if task.Status == "running" && time.Since(task.UpdatedAt) < 30*time.Minute {
		return fmt.Errorf("任务 %d 正在执行中", taskId)
	}

	// 解析模板
	tmplIds := parseTemplateIds(task.TemplateIds)
	if len(tmplIds) == 0 {
		return finishTaskFailed(&task, start, "任务未包含有效模板")
	}
	var templates []models.PromptTemplate
	if err := db.Dao.Model(&models.PromptTemplate{}).Where("id IN ?", tmplIds).Find(&templates).Error; err != nil || len(templates) == 0 {
		return finishTaskFailed(&task, start, "提示词模板查询失败或已删除")
	}

	task.Status = "running"
	task.Progress = 0
	task.ProgressMsg = "准备交易日历"
	task.ErrorMessage = ""
	task.DoneCalls = 0
	db.Dao.Save(&task)
	emitBacktestProgress(ctx, &task)

	// 交易日（含采样与收益窗口校验）
	days := promptBacktestTradeDays(task.StartDate, task.EndDate, task.PeriodDays)
	if len(days) == 0 {
		return finishTaskFailed(&task, start, fmt.Sprintf("[%s ~ %s] 内无可回测交易日（或距今不足 %d 个交易日无法计算收益）",
			task.StartDate, task.EndDate, task.PeriodDays))
	}
	// 采样
	step := task.SampleEveryNDays
	if step < 1 {
		step = 1
	}
	var sampled []string
	for i := 0; i < len(days); i += step {
		sampled = append(sampled, days[i])
	}
	if len(sampled) > promptBacktestMaxDates {
		sampled = sampled[len(sampled)-promptBacktestMaxDates:] // 取最近的，材料时效性好
	}

	task.TotalCalls = len(templates) * len(sampled) * task.RepeatRuns
	task.ProgressMsg = fmt.Sprintf("共 %d 个模板 × %d 个交易日 × %d 次重复 = %d 次调用",
		len(templates), len(sampled), task.RepeatRuns, task.TotalCalls)
	db.Dao.Save(&task)
	emitBacktestProgress(ctx, &task)

	// 逐日执行（材料按日缓存，模板/重复共享）
	// 失败分类计数（完成消息展示，日志含原始输出样本，便于诊断模型未按契约输出的问题）
	var errorReplies, emptyReplies, parseFails int
	materialCache := map[string]string{}
	for di, date := range sampled {
		material, ok := materialCache[date]
		if !ok {
			material = buildPromptBacktestMaterial(date)
			materialCache[date] = material
		}
		for ti := range templates {
			tmpl := templates[ti]
			sysPrompt := tmpl.Content + promptBacktestOutputContract(task.PeriodDays, task.TopN)
			for run := 1; run <= task.RepeatRuns; run++ {
				question := fmt.Sprintf(`今天是 %s（A股交易日，`+promptBacktestSceneMarker+`：忽略素材之外的任何时间提示，素材均为当日收盘后数据）。
请基于以下当日真实素材，完成选股分析并按系统提示词要求的 JSON 格式输出。

# 当日素材
%s`, date, material)

				raw, err := runPromptBacktestCall(ctx, sysPrompt, question, task.AiConfigId)
				if err != nil {
					logger.SugaredLogger.Warnf("回测调用失败（%s 模板%d 第%d次）：%v", date, tmpl.ID, run, err)
				}
				// 宽松解析：兼容前后杂文/Markdown 链接/表格等含方括号内容
				//（extractPicksJSON 的首尾括号整体跨度在杂文场景必然解析失败）
				picks := extractPicksLoose(raw)
				rawSnap := raw
				// 失败分类留痕：之前解析 0 条时静默丢弃原始输出，任务显示"完成"但无选股，
				// 无法区分模型未按契约输出 / Agent 报错 / 空回复
				trimmed := strings.TrimSpace(raw)
				switch {
				case trimmed == "":
					emptyReplies++
					logger.SugaredLogger.Warnf("回测调用空回复（%s 模板%s 第%d/%d 次）", date, tmpl.Name, run, task.RepeatRuns)
				case strings.HasPrefix(trimmed, "❌"):
					errorReplies++
					logger.SugaredLogger.Warnf("回测调用返回错误（%s 模板%s 第%d/%d 次）：%s",
						date, tmpl.Name, run, task.RepeatRuns, truncate(trimmed, 300))
				case len(picks) == 0 && !strings.Contains(trimmed, "[]"):
					// 回复中连空数组都没有 → 模型未按契约输出 JSON
					parseFails++
					logger.SugaredLogger.Warnf("回测输出未解析出选股（%s 模板%s 第%d/%d 次），原始输出前300字符：%s",
						date, tmpl.Name, run, task.RepeatRuns, truncate(trimmed, 300))
				}
				if len(rawSnap) > 2000 {
					rawSnap = rawSnap[:2000]
				}
				for _, p := range picks {
					code := normalizePickCode(p.Code)
					if code == "" {
						continue
					}
					pick := models.PromptBacktestPick{
						TaskId:     task.ID,
						TemplateId: tmpl.ID,
						RunIndex:   run,
						TradeDate:  date,
						StockCode:  code,
						StockName:  strings.TrimSpace(p.Name),
						Rating:     strings.TrimSpace(p.Rating),
						Reason:     strings.TrimSpace(p.Reason),
						RawOutput:  rawSnap,
					}
					db.Dao.Create(&pick)
				}
				task.DoneCalls++
				task.Progress = int(float64(task.DoneCalls) / float64(task.TotalCalls) * 90) // 后 10% 留给收益计算
				task.ProgressMsg = fmt.Sprintf("选股中：%s（%d/%d 天）模板 %s 第 %d/%d 次", date, di+1, len(sampled), tmpl.Name, run, task.RepeatRuns)
				db.Dao.Save(&task)
				emitBacktestProgress(ctx, &task)
				time.Sleep(500 * time.Millisecond) // 轻微限速，避免连续请求打满配额
			}
		}
	}

	// 收益计算
	task.ProgressMsg = "计算选股收益…"
	task.Progress = 90
	db.Dao.Save(&task)
	emitBacktestProgress(ctx, &task)
	computed, skipped, err := computePromptBacktestPicks(task.ID, task.PeriodDays)
	if err != nil {
		return finishTaskFailed(&task, start, fmt.Sprintf("收益计算失败：%v", err))
	}

	task.Status = "done"
	task.Progress = 100
	task.DurationMs = time.Since(start).Milliseconds()
	// 完成消息带失败分类统计：0 选股时可据此定位是模型未按契约输出还是调用报错
	fails := errorReplies + emptyReplies + parseFails
	if fails > 0 {
		task.ProgressMsg = fmt.Sprintf("完成：%d 次调用（错误回复 %d、空回复 %d、未解析出选股 %d），%d 条选股已计算收益（%d 条数据不足跳过）。未解析样本见后端日志",
			task.DoneCalls, errorReplies, emptyReplies, parseFails, computed, skipped)
	} else {
		task.ProgressMsg = fmt.Sprintf("完成：%d 次调用，%d 条选股已计算收益（%d 条数据不足跳过）", task.DoneCalls, computed, skipped)
	}
	db.Dao.Save(&task)
	emitBacktestProgress(ctx, &task)
	logger.SugaredLogger.Infof("提示词回测任务 %d 完成：%s", task.ID, task.ProgressMsg)
	return nil
}

// finishTaskFailed 标记任务失败并返回错误。
func finishTaskFailed(task *models.PromptBacktestTask, start time.Time, msg string) error {
	task.Status = "failed"
	task.ErrorMessage = msg
	task.ProgressMsg = msg
	task.DurationMs = time.Since(start).Milliseconds()
	db.Dao.Save(task)
	return fmt.Errorf("%s", msg)
}

// parseTemplateIds 解析逗号分隔的模板 ID 列表。
func parseTemplateIds(s string) []int {
	var ids []int
	for _, part := range strings.Split(s, ",") {
		if v, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && v > 0 {
			ids = append(ids, v)
		}
	}
	return ids
}

// computePromptBacktestPicks 计算任务内全部选股的后 N 交易日收益（vs 沪深300）。
// 个股 K 线按代码缓存，基准 K 线一次拉取。
func computePromptBacktestPicks(taskId uint, periodDays int) (computed, skipped int, err error) {
	var picks []models.PromptBacktestPick
	if err := db.Dao.Where("task_id = ?", taskId).Find(&picks).Error; err != nil {
		return 0, 0, err
	}
	if len(picks) == 0 {
		return 0, 0, nil
	}

	// 基准K线
	benchRes := data.FetchKLineWithFallback(backtestBenchmarkCode, "沪深300", "101", 300, "")
	benchBars := []data.KLineData{}
	if benchRes != nil && benchRes.Data != nil {
		benchBars = *benchRes.Data
	}

	type stockMeta struct {
		name string
		bars []data.KLineData
	}
	stockCache := map[string]*stockMeta{}

	for i := range picks {
		p := &picks[i]
		meta, ok := stockCache[p.StockCode]
		if !ok {
			res := data.FetchKLineWithFallback(p.StockCode, p.StockName, "101", 300, "")
			if res == nil || res.Data == nil || len(*res.Data) == 0 {
				stockCache[p.StockCode] = &stockMeta{name: p.StockName}
				continue
			}
			meta = &stockMeta{name: p.StockName, bars: *res.Data}
			stockCache[p.StockCode] = meta
		}
		if len(meta.bars) == 0 {
			skipped++
			continue
		}
		recDate, err := parseKLineDay(p.TradeDate)
		if err != nil {
			skipped++
			continue
		}
		bi, ei := findBacktestRange(meta.bars, recDate, periodDays)
		if bi < 0 || ei < 0 {
			skipped++
			continue
		}
		baseClose, e1 := parsePrice(meta.bars[bi].Close)
		endClose, e2 := parsePrice(meta.bars[ei].Close)
		if e1 != nil || e2 != nil || baseClose <= 0 {
			skipped++
			continue
		}
		p.RecommendPrice = round2(baseClose)
		p.EndPrice = round2(endClose)
		p.ReturnPct = round2((endClose - baseClose) / baseClose * 100)
		p.BenchmarkPct = 0
		if len(benchBars) > 0 {
			if bbi, bei := findBacktestRange(benchBars, recDate, periodDays); bbi >= 0 && bei >= 0 {
				if b, e := parsePrice(benchBars[bbi].Close); e == nil && b > 0 {
					if bv, e2 := parsePrice(benchBars[bei].Close); e2 == nil && bv > 0 {
						p.BenchmarkPct = round2((bv - b) / b * 100)
					}
				}
			}
		}
		p.ExcessPct = round2(p.ReturnPct - p.BenchmarkPct)
		db.Dao.Save(p)
		computed++
	}
	return computed, skipped, nil
}
