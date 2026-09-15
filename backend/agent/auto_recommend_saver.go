package agent

// auto_recommend_saver.go — 推荐记录自动保存（默认开启）。
//
// 目标：每轮 AI 分析结束后，只要回复中包含具体股票推荐（固定 JSON 契约
// [{code,name,rating,reason}]），就自动保存到 ai_recommend_stocks，不依赖模型
// 是否"记得"调用推荐工具，保证推荐回测（recommend_backtest.go）与推荐列表有完整数据。
//
// 与工具保存的关系（防重复）：
//   - 主路径：模型按 staticRulesRecommendSave 指令调用 Create/BatchCreateAiRecommendStocks，
//     工具 InvokableRun 会置位本轮跟踪器（tools.MarkRecommendSaved）；
//   - 兜底路径：本轮未调用推荐工具时，此处解析回复中的 JSON 契约并批量入库。
//
// 跳过场景：提示词回测调用（isPromptBacktestCall）——回测选股走 prompt_backtest_picks
// 独立链路，自动保存会把模拟历史选股混入真实推荐记录，污染推荐回测。
//
// 全流程失败仅记日志，不影响主对话。

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go-stock/backend/agent/tools"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// autoSaveMaxPicks 单轮自动保存的推荐条数上限（防模型输出异常导致批量脏数据）。
const autoSaveMaxPicks = 20

// isPromptBacktestCall 判断本次 ChatWithContext 调用是否为提示词回测场景：
// 回测提问包含场景标记（promptBacktestSceneMarker），或 sysPrompt 为模板+输出契约
// （promptBacktestContractMarker）的 override。回测不注入推荐保存规则、也不做
// 回复自动保存，避免模拟历史选股污染真实推荐记录。
func isPromptBacktestCall(question, sysPrompt string) bool {
	return strings.Contains(question, promptBacktestSceneMarker) ||
		strings.Contains(sysPrompt, promptBacktestContractMarker)
}

// autoSaveRecommendRecords 在每轮 AI 分析收尾时调用：解析最终回复中的推荐 JSON
// 契约并保存。本轮已通过推荐工具保存过（RecommendSavedThisTurn）则跳过，避免重复。
func autoSaveRecommendRecords(ctx context.Context, question, response string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("autoSaveRecommendRecords panic: %v", r)
		}
	}()

	if strings.TrimSpace(response) == "" {
		return
	}
	// 本轮已调用推荐工具保存 → 模型走主路径，无需兜底
	if tools.RecommendSavedThisTurn(ctx) {
		return
	}
	// 回测场景：选股入库由回测引擎负责
	if isPromptBacktestCall(question, "") {
		return
	}
	meta, hasMeta := tools.AgentMetaFromCtx(ctx)
	if hasMeta && isPromptBacktestCall(question, meta.SystemPrompt) {
		return
	}

	picks := extractPicksLoose(response)
	if len(picks) == 0 {
		return
	}

	// 规范化 + 去重 + 截断
	now := time.Now()
	seen := make(map[string]bool, len(picks))
	var ordered []*promptBacktestPickJSON
	for _, p := range picks {
		code := normalizePickCode(p.Code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		pick := p // 拷贝出循环变量
		pick.Code = code
		ordered = append(ordered, &pick)
		if len(ordered) >= autoSaveMaxPicks {
			break
		}
	}
	if len(ordered) == 0 {
		return
	}

	// 批量拉取推荐时点实时价（失败则价格留空，不影响保存）
	priceMap := fetchPickRealtimePrices(ordered)

	recommends := make([]*models.AiRecommendStocks, 0, len(ordered))
	for _, p := range ordered {
		suffixedCode := aStockSuffixedCode(p.Code)
		rec := &models.AiRecommendStocks{
			DataTime:        &now,
			Rating:          strings.TrimSpace(p.Rating),
			StockCode:       suffixedCode,
			StockName:       strings.TrimSpace(p.Name),
			RecommendReason: strings.TrimSpace(p.Reason),
			Remarks:         "AI 分析自动保存",
		}
		if hasMeta {
			rec.ModelName = meta.ModelName
			rec.SystemPrompt = meta.SystemPrompt
			rec.UserPrompt = meta.UserPrompt
			rec.SysPromptId = meta.SysPromptId
			rec.SkillId = meta.SkillId
		}
		if info, ok := priceMap[tools.GetStockCode(suffixedCode)]; ok {
			rec.StockPrice = info.Price
			rec.StockPrePrice = info.PreClose
			rec.StockCurrentPrice = info.Price
			rec.StockCurrentPriceTime = info.Date + " " + info.Time
		}
		recommends = append(recommends, rec)
	}

	if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(recommends); err != nil {
		logger.SugaredLogger.Errorf("自动保存推荐记录失败: %v", err)
		return
	}
	logger.SugaredLogger.Infof("已自动保存 %d 条 AI 推荐记录（本轮未调用推荐工具，走 JSON 契约兜底）", len(recommends))
}

// fetchPickRealtimePrices 批量拉取选股的实时行情，key 为 tools.GetStockCode 归一化代码
// （如 sh601138）。拉取失败返回空 map，调用方按无价处理。
func fetchPickRealtimePrices(picks []*promptBacktestPickJSON) map[string]data.StockInfo {
	priceMap := make(map[string]data.StockInfo, len(picks))
	if len(picks) == 0 {
		return priceMap
	}
	codes := make([]string, 0, len(picks))
	for _, p := range picks {
		codes = append(codes, aStockSuffixedCode(p.Code))
	}
	infos, err := data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
	if err != nil || infos == nil {
		return priceMap
	}
	for _, info := range *infos {
		priceMap[tools.GetStockCode(info.Code)] = info
	}
	return priceMap
}

// aStockSuffixedCode 将 6 位 A 股代码转为带交易所后缀格式（601138 → 601138.SH），
// 与推荐工具的代码示例格式一致；推荐列表实时价匹配（ConvertTushareCodeToStockCode）
// 与回测 K 线拉取（FetchKLineWithFallback）均兼容该格式。交易所归属判断与
// tools.GetStockCode 保持一致。
func aStockSuffixedCode(code string) string {
	if len(code) == 0 {
		return code
	}
	switch code[0] {
	case '6':
		return code + ".SH"
	case '0', '3':
		return code + ".SZ"
	case '4', '8', '9':
		return code + ".BJ"
	}
	return code
}

// extractPicksLoose 从 AI 回复中解析推荐 JSON 契约（[{code,name,rating,reason}]）。
// 先复用 extractPicksJSON 的整体跨度解析（回测场景回复只有 JSON 时命中）；
// 失败时逐个扫描 "[{" 起点，用括号配对截取候选数组再解析——兼容回复正文
// 中含有其它方括号内容（如 Markdown 链接、表格）的场景。
func extractPicksLoose(content string) []promptBacktestPickJSON {
	if picks := extractPicksJSON(content); len(picks) > 0 {
		return picks
	}
	searchFrom := 0
	for {
		idx := strings.Index(content[searchFrom:], `[{`)
		if idx < 0 {
			return nil
		}
		start := searchFrom + idx
		if end := matchJSONArray(content, start); end > start {
			var picks []promptBacktestPickJSON
			if err := json.Unmarshal([]byte(content[start:end+1]), &picks); err == nil && len(picks) > 0 {
				return picks
			}
		}
		searchFrom = start + 2
	}
}

// matchJSONArray 从 start 处的 '[' 开始做括号配对，返回与之配对的 ']' 下标；
// 未闭合或 start 不是 '[' 返回 -1。字符串字面量内的括号不计入配对。
func matchJSONArray(s string, start int) int {
	if start < 0 || start >= len(s) || s[start] != '[' {
		return -1
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
