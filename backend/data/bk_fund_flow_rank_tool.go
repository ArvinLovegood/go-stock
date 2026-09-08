package data

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/models"
)

// bk_fund_flow_rank_tool.go — 板块/概念资金流向与成分股 AI 工具输出层。
//
// 提供两个 AI 工具的 Markdown 渲染：
//  1. GetBkFundFlowRank        板块/概念资金流向 TOP 榜（主力净流入/流出）
//  2. GetBkConstituentStocks   板块/概念成分股 TOP N（按涨跌幅/量比/换手率/市值/主力净流入等排序）

// ---------- 板块/概念资金流向 TOP 榜 ----------

// normalizeFundFlowBoardType 归一化板块类型参数（板块/行业 → industry，概念 → concept，both → both）
func normalizeFundFlowBoardType(boardType string) string {
	switch strings.ToLower(strings.TrimSpace(boardType)) {
	case "concept", "概念", "概念板块", "gn":
		return "concept"
	case "both", "全部", "板块+概念":
		return "both"
	default: // industry/板块/行业/空
		return "industry"
	}
}

// normalizeFundFlowDirection 归一化方向参数（inflow/outflow/both）
func normalizeFundFlowDirection(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "inflow", "in", "流入", "净流入", "流入榜":
		return "inflow"
	case "outflow", "out", "流出", "净流出", "流出榜":
		return "outflow"
	default: // both/双向/空
		return "both"
	}
}

// fmtYiWan 金额自适应单位：|v|≥1亿 显示 "X.XX亿"，否则 "X.XX万"（遵循项目金额显示约定）
func fmtYiWan(v float64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	if abs >= 1e8 {
		return fmt.Sprintf("%.2f亿", v/1e8)
	}
	if abs >= 1e4 {
		return fmt.Sprintf("%.2f万", v/1e4)
	}
	return fmt.Sprintf("%.0f", v)
}

// renderFundFlowRankTable 渲染资金流向榜单为 Markdown 表格行（snapTime 为数据快照时间）
func renderFundFlowRankTable(items interface{}, label, snapTime string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 主力净%s TOP榜（快照时间 %s）\n\n", label, snapTime))
	sb.WriteString("| 排名 | 代码 | 名称 | 主力净流入 |\n")
	sb.WriteString("| --- | --- | --- | --- |\n")
	switch list := items.(type) {
	case []models.BKFundFlow:
		for i, item := range list {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Code, item.Name, fmtYiWan(float64(item.NetInflow))))
		}
	case []models.ConceptFundFlow:
		for i, item := range list {
			sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n", i+1, item.Code, item.Name, fmtYiWan(float64(item.NetInflow))))
		}
	}
	return sb.String()
}

// GetBkFundFlowRankToMarkdown 获取板块/概念资金流向 TOP 榜（AI 工具 GetBkFundFlowRank 输出层）
// boardType: industry(行业板块)/concept(概念板块)/both(两者都查)；direction: inflow/outflow/both；date 为空取最新快照（非交易日自动回退最近交易日）
func GetBkFundFlowRankToMarkdown(boardType, date, direction string, topN int) string {
	boardType = normalizeFundFlowBoardType(boardType)
	direction = normalizeFundFlowDirection(direction)
	if topN <= 0 {
		topN = 20
	}
	if topN > 100 {
		topN = 100
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	dirs := []string{"inflow"}
	if direction == "outflow" {
		dirs = []string{"outflow"}
	} else if direction == "both" {
		dirs = []string{"inflow", "outflow"}
	}

	var sb strings.Builder
	typeName := "板块资金流向"
	if boardType == "concept" {
		typeName = "概念资金流向"
	}
	sb.WriteString(fmt.Sprintf("## %s 主力资金排名（查询日期 %s）\n\n", typeName, date))

	dirsLabel := map[string]string{"inflow": "流入", "outflow": "流出"}
	empty := true

	fetchBk := func(dir string) []models.BKFundFlow {
		if date == time.Now().Format("2006-01-02") {
			return NewBKFundFlowApi().GetBKFundFlowRankList(topN, dir)
		}
		return NewBKFundFlowApi().GetBKFundFlowRankListByDate(date, topN, dir)
	}
	fetchConcept := func(dir string) []models.ConceptFundFlow {
		if date == time.Now().Format("2006-01-02") {
			return NewConceptFundFlowApi().GetConceptFundFlowRankList(topN, dir)
		}
		return NewConceptFundFlowApi().GetConceptFundFlowRankListByDate(date, topN, dir)
	}

	if boardType == "industry" || boardType == "both" {
		for _, dir := range dirs {
			list := fetchBk(dir)
			if len(list) == 0 {
				continue
			}
			empty = false
			sb.WriteString(renderFundFlowRankTable(list, dirsLabel[dir], list[0].SnapTime))
			sb.WriteString("\n")
		}
	}
	if boardType == "concept" || boardType == "both" {
		for _, dir := range dirs {
			list := fetchConcept(dir)
			if len(list) == 0 {
				continue
			}
			empty = false
			sb.WriteString(renderFundFlowRankTable(list, dirsLabel[dir], list[0].SnapTime))
			sb.WriteString("\n")
		}
	}

	if empty {
		sb.WriteString(fmt.Sprintf("%s 无数据（可能为非交易日或当日数据尚未采集，可换一个交易日重试）\n", date))
	} else {
		sb.WriteString("> 提示：可调用 GetBkConstituentStocks 查看某板块/概念的成分股明细\n")
	}
	return sb.String()
}

// ---------- 板块/概念成分股 ----------

// bkConstituentSortKeys 成分股排序字段别名归一化
func normalizeBKConstituentSortKey(sortBy string) (field string, label string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "changepercent", "涨跌幅", "涨幅", "跌幅", "涨跌":
		return "changePercent", "涨跌幅"
	case "volumeratio", "量比":
		return "volumeRatio", "量比"
	case "turnoverrate", "换手率":
		return "turnoverRate", "换手率"
	case "marketcap", "totalmarketcap", "市值", "总市值":
		return "totalMarketCap", "总市值"
	case "flowmarketcap", "流通市值":
		return "flowMarketCap", "流通市值"
	case "mainnetinflowpct", "主力净流入占比", "净流入占比":
		return "mainNetInflowPct", "主力净流入占比"
	case "mainnetinflow", "主力净流入", "净流入":
		return "mainNetInflow", "主力净流入"
	case "dealamount", "成交额":
		return "dealAmount", "成交额"
	default:
		return "mainNetInflow", "主力净流入"
	}
}

// resolveBKCode 将板块/概念代码或名称解析为 BK 代码（先行业后概念，精确优先，其次包含匹配）
func resolveBKCode(bkCodeOrName string) (code string, name string, boardType string) {
	input := strings.TrimSpace(bkCodeOrName)
	if input == "" {
		return "", "", ""
	}
	upper := strings.ToUpper(input)
	// BKxxxx 代码直接使用
	if strings.HasPrefix(upper, "BK") && len(upper) >= 4 {
		return upper, "", ""
	}
	// 名称解析：先行业板块后概念板块，精确匹配优先于包含匹配（列表各只拉取一次）
	type boardSource struct {
		boardType string
		codes     []map[string]string
	}
	sources := []boardSource{
		{"industry", NewBKFundFlowApi().GetAllBKCodes()},
		{"concept", NewConceptFundFlowApi().GetAllConceptCodes()},
	}
	for _, src := range sources {
		for _, m := range src.codes {
			if m["name"] == input {
				return m["code"], m["name"], src.boardType
			}
		}
	}
	for _, src := range sources {
		for _, m := range src.codes {
			if strings.Contains(m["name"], input) {
				return m["code"], m["name"], src.boardType
			}
		}
	}
	return "", "", ""
}

// sortBKConstituentStocks 按指定字段与方向排序成分股（field 须为归一化后的字段名）
func sortBKConstituentStocks(stocks []models.BKConstituentStock, field, order string) {
	asc := strings.EqualFold(strings.TrimSpace(order), "asc") || strings.Contains(order, "升序")
	value := func(s models.BKConstituentStock) float64 {
		switch field {
		case "changePercent":
			return s.ChangePercent
		case "volumeRatio":
			return s.VolumeRatio
		case "turnoverRate":
			return s.TurnoverRate
		case "totalMarketCap":
			return s.TotalMarketCap
		case "flowMarketCap":
			return s.FlowMarketCap
		case "mainNetInflowPct":
			return s.MainNetInflowPct
		case "dealAmount":
			return s.DealAmount
		default:
			return s.MainNetInflow
		}
	}
	sort.SliceStable(stocks, func(i, j int) bool {
		if asc {
			return value(stocks[i]) < value(stocks[j])
		}
		return value(stocks[i]) > value(stocks[j])
	})
}

// GetBkConstituentStocksToMarkdown 获取板块/概念成分股 TOP N（AI 工具 GetBkConstituentStocks 输出层）
// bkCodeOrName 支持东财板块代码（BK0475）或名称（银行/机器人概念）；sortBy 见 normalizeBKConstituentSortKey；order: desc/asc
func GetBkConstituentStocksToMarkdown(bkCodeOrName, sortBy, order string, topN int) string {
	if topN <= 0 {
		topN = 20
	}
	if topN > 50 {
		topN = 50
	}
	field, fieldLabel := normalizeBKConstituentSortKey(sortBy)
	orderLabel := "降序"
	if strings.EqualFold(strings.TrimSpace(order), "asc") || strings.Contains(order, "升序") {
		orderLabel = "升序"
	}

	code, name, boardType := resolveBKCode(bkCodeOrName)
	if code == "" {
		return fmt.Sprintf("未能识别板块/概念「%s」。请使用东财板块代码（如 BK0475）或准确名称（如 银行、机器人概念）；可先调用 GetBkFundFlowRank 获取板块/概念代码列表", bkCodeOrName)
	}

	stocks := NewBKConstituentsApi().GetBKConstituentStocks(code)
	if len(stocks) == 0 {
		return fmt.Sprintf("板块/概念「%s（%s）」暂无成分股数据", name, code)
	}

	sortBKConstituentStocks(stocks, field, order)
	if len(stocks) > topN {
		stocks = stocks[:topN]
	}

	typeName := "板块"
	if boardType == "concept" {
		typeName = "概念"
	}
	displayName := name
	if displayName == "" {
		displayName = code
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s（%s）成分股 TOP%d（按%s%s）\n\n", displayName, code, len(stocks), fieldLabel, orderLabel))
	sb.WriteString(fmt.Sprintf("> 类型：%s｜排序：%s %s\n\n", typeName, fieldLabel, orderLabel))
	sb.WriteString("| 排名 | 代码 | 名称 | 最新价 | 涨跌幅% | 换手率% | 量比 | 流通市值 | 总市值 | 主力净流入 | 主力净流入占比% |\n")
	sb.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for i, s := range stocks {
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %.2f | %.2f | %.2f | %.2f | %s | %s | %s | %.2f |\n",
			i+1, s.Code, s.Name, s.Price, s.ChangePercent, s.TurnoverRate, s.VolumeRatio,
			fmtYiWan(s.FlowMarketCap), fmtYiWan(s.TotalMarketCap), fmtYiWan(s.MainNetInflow), s.MainNetInflowPct))
	}
	return sb.String()
}
