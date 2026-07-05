package data

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	iwencaiPrimaryColumnLimit    = 10
	iwencaiWideColumnThreshold   = 12
	iwencaiSearchSummaryMaxRunes = 280
	iwencaiLongTextMaxRunes      = 320
)

var (
	iwencaiColumnBracketDateRe = regexp.MustCompile(`^(.+)\[(\d{8})\]$`)
	iwencaiPercentColumnRe     = regexp.MustCompile(`涨跌幅|涨幅|跌幅|收益率|比例|占比|百分|变动率`)
	iwencaiLongTextColumnRe    = regexp.MustCompile(`内容|摘要|说明|正文|问答|解读|点评|原因类别`)
	iwencaiDateValueColumnRe   = regexp.MustCompile(`日期|时间`)
	iwencaiLargeNumberColumnRe = regexp.MustCompile(`市值|股数|a股|股本|份额|数量|成本|成交量|封单`)
)

type iwencaiResultKind int

const (
	iwencaiKindGeneric iwencaiResultKind = iota
	iwencaiKindStock
	iwencaiKindIndex
	iwencaiKindFund
	iwencaiKindMacro
)

var iwencaiMacroHiddenColumns = map[string]struct{}{
	"macro_id":   {},
	"macro_name": {},
}

var iwencaiColumnPriority = []string{
	"股票代码", "股票简称", "基金代码", "基金简称", "指数代码", "指数简称", "代码", "名称", "简称",
	"最新价", "最新涨跌幅", "涨跌幅", "涨跌额", "现价", "开盘价", "收盘价", "最高价", "最低价",
	"指标", "指标值", "单位", "时间", "国家", "周期",
	"解禁日期", "解禁数量", "解禁市值", "解禁股数", "公告日期", "事件类型", "事件日期", "变动日期",
	"几天几板", "连续涨停天数", "涨停原因", "最终涨停时间", "首次涨停时间",
	"指标名称", "日期", "报告期",
	"调研日期", "机构家数", "方式", "地点", "接待人员",
}

type iwencaiDateSeriesGroup struct {
	prefix string
	cols   []iwencaiDateColumn
}

type iwencaiDateColumn struct {
	name   string
	prefix string
	date   string
}

func detectIwencaiResultKind(datas []map[string]any) iwencaiResultKind {
	if len(datas) == 0 {
		return iwencaiKindGeneric
	}
	row := datas[0]
	if _, ok := row["macro_id"]; ok {
		return iwencaiKindMacro
	}
	if _, ok := row["macro_name"]; ok {
		return iwencaiKindMacro
	}
	hasStock := false
	hasIndex := false
	hasFund := false
	for _, r := range datas {
		if v, ok := r["股票代码"]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			hasStock = true
		}
		if v, ok := r["指数代码"]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			hasIndex = true
		}
		if v, ok := r["基金代码"]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
			hasFund = true
		}
	}
	switch {
	case hasStock:
		return iwencaiKindStock
	case hasIndex:
		return iwencaiKindIndex
	case hasFund:
		return iwencaiKindFund
	default:
		return iwencaiKindGeneric
	}
}

func extractCommonBracketDate(cols []string) string {
	var dates []string
	for _, col := range cols {
		m := iwencaiColumnBracketDateRe.FindStringSubmatch(col)
		if len(m) != 3 {
			continue
		}
		dates = append(dates, m[2])
	}
	if len(dates) == 0 {
		return ""
	}
	first := dates[0]
	for _, d := range dates[1:] {
		if d != first {
			return ""
		}
	}
	return first
}

func formatIwencaiColumnHeader(col, commonDate string) string {
	m := iwencaiColumnBracketDateRe.FindStringSubmatch(col)
	if len(m) != 3 {
		return col
	}
	if commonDate != "" && m[2] == commonDate {
		return m[1]
	}
	return fmt.Sprintf("%s（%s）", m[1], formatIwencaiDate(m[2]))
}

func isIwencaiLongTextColumn(col string) bool {
	base := col
	if m := iwencaiColumnBracketDateRe.FindStringSubmatch(col); len(m) == 3 {
		base = m[1]
	}
	return iwencaiLongTextColumnRe.MatchString(base)
}

func splitLongTextColumns(cols []string) (normal, longText []string) {
	for _, col := range cols {
		if isIwencaiLongTextColumn(col) {
			longText = append(longText, col)
			continue
		}
		normal = append(normal, col)
	}
	return normal, longText
}

func isIwencaiAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func formatIwencaiDateValue(v any) string {
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if len(s) == 8 && isIwencaiAllDigits(s) {
		return formatIwencaiDate(s)
	}
	return s
}

func formatIwencaiLargeNumber(f float64) string {
	abs := math.Abs(f)
	switch {
	case abs >= 1e8:
		return fmt.Sprintf("%.2f亿", f/1e8)
	case abs >= 1e4:
		return fmt.Sprintf("%.2f万", f/1e4)
	default:
		if math.Mod(f, 1) == 0 {
			return fmt.Sprintf("%.0f", f)
		}
		return fmt.Sprintf("%.2f", f)
	}
}

func formatIwencaiArrayValue(v any) string {
	switch arr := v.(type) {
	case []any:
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	case []string:
		parts := make([]string, 0, len(arr))
		for _, s := range arr {
			s = strings.TrimSpace(s)
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "、")
	default:
		return formatIwencaiCell(v)
	}
}

func formatIwencaiBoolean(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

func collectIwencaiColumnKeys(datas []map[string]any) []string {
	if len(datas) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for _, row := range datas {
		for k := range row {
			seen[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(seen))
	for k := range seen {
		cols = append(cols, k)
	}
	sortIwencaiColumns(cols)
	return cols
}

func sortIwencaiColumns(cols []string) {
	sort.Slice(cols, func(i, j int) bool {
		ti, di := iwencaiColumnRank(cols[i])
		tj, dj := iwencaiColumnRank(cols[j])
		if ti != tj {
			return ti < tj
		}
		if di != dj {
			return di < dj
		}
		return cols[i] < cols[j]
	})
}

func iwencaiColumnRank(col string) (tier int, dateKey string) {
	for i, p := range iwencaiColumnPriority {
		if col == p || strings.HasPrefix(col, p+"[") {
			return 0, fmt.Sprintf("%03d", i)
		}
	}
	if m := iwencaiColumnBracketDateRe.FindStringSubmatch(col); len(m) == 3 {
		return 1, m[2]
	}
	return 2, col
}

func splitIwencaiColumns(cols []string) (primary, extra []string) {
	if len(cols) <= iwencaiWideColumnThreshold {
		return cols, nil
	}
	limit := iwencaiPrimaryColumnLimit
	if limit > len(cols) {
		limit = len(cols)
	}
	return cols[:limit], cols[limit:]
}

func partitionDateColumns(cols []string) (static []string, groups []iwencaiDateSeriesGroup) {
	groupMap := map[string][]iwencaiDateColumn{}
	for _, col := range cols {
		m := iwencaiColumnBracketDateRe.FindStringSubmatch(col)
		if len(m) != 3 {
			static = append(static, col)
			continue
		}
		groupMap[m[1]] = append(groupMap[m[1]], iwencaiDateColumn{name: col, prefix: m[1], date: m[2]})
	}
	for prefix, items := range groupMap {
		sort.Slice(items, func(i, j int) bool { return items[i].date < items[j].date })
		if len(items) == 1 {
			static = append(static, items[0].name)
			continue
		}
		groups = append(groups, iwencaiDateSeriesGroup{prefix: prefix, cols: items})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].prefix < groups[j].prefix })
	sortIwencaiColumns(static)
	return static, groups
}

func formatIwencaiCell(v any) string {
	if v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func formatIwencaiCellByColumn(col string, v any) string {
	if v == nil {
		return ""
	}
	if b, ok := v.(bool); ok {
		return formatIwencaiBoolean(b)
	}
	base := col
	if m := iwencaiColumnBracketDateRe.FindStringSubmatch(col); len(m) == 3 {
		base = m[1]
	}
	if iwencaiDateValueColumnRe.MatchString(base) {
		return formatIwencaiDateValue(v)
	}
	if iwencaiPercentColumnRe.MatchString(col) || iwencaiPercentColumnRe.MatchString(base) {
		if f, ok := iwencaiToFloat(v); ok {
			return fmt.Sprintf("%.2f%%", f)
		}
	}
	if f, ok := iwencaiToFloat(v); ok {
		if iwencaiLargeNumberColumnRe.MatchString(base) && math.Abs(f) >= 1e4 {
			return formatIwencaiLargeNumber(f)
		}
		if math.Mod(f, 1) == 0 {
			return fmt.Sprintf("%.0f", f)
		}
		abs := math.Abs(f)
		switch {
		case abs >= 1000:
			return fmt.Sprintf("%.2f", f)
		case abs >= 1:
			return fmt.Sprintf("%.4f", f)
		default:
			return fmt.Sprintf("%.6f", f)
		}
	}
	switch v.(type) {
	case []any, []string:
		return formatIwencaiArrayValue(v)
	}
	return formatIwencaiCell(v)
}

func iwencaiToFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(n, ",", ""))
		if s == "" {
			return 0, false
		}
		if strings.HasSuffix(s, "%") {
			s = strings.TrimSuffix(s, "%")
		}
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func rowIdentityLabel(row map[string]any) string {
	name := ""
	code := ""
	for _, k := range []string{"股票简称", "基金简称", "指数简称", "简称", "名称"} {
		if v, ok := row[k]; ok && v != nil {
			name = strings.TrimSpace(fmt.Sprintf("%v", v))
			if name != "" {
				break
			}
		}
	}
	for _, k := range []string{"股票代码", "基金代码", "指数代码", "代码"} {
		if v, ok := row[k]; ok && v != nil {
			code = strings.TrimSpace(fmt.Sprintf("%v", v))
			if code != "" {
				break
			}
		}
	}
	switch {
	case name != "" && code != "":
		return fmt.Sprintf("%s (%s)", name, code)
	case name != "":
		return name
	case code != "":
		return code
	default:
		return "标的"
	}
}

func buildEntityTitle(row map[string]any) string {
	label := rowIdentityLabel(row)
	if label == "标的" {
		return ""
	}
	return label
}

func renderIwencaiKVTable(cols []string, row map[string]any, commonDate string) string {
	normalCols, longTextCols := splitLongTextColumns(cols)
	if len(normalCols) == 0 && len(longTextCols) == 0 {
		return ""
	}
	var sb strings.Builder
	if len(normalCols) > 0 {
		sb.WriteString("| 字段 | 值 |\n| --- | --- |\n")
		for _, col := range normalCols {
			val := ""
			if v, ok := row[col]; ok && v != nil {
				val = formatIwencaiCellByColumn(col, v)
			}
			if val == "" {
				continue
			}
			sb.WriteString("| ")
			sb.WriteString(formatIwencaiCell(formatIwencaiColumnHeader(col, commonDate)))
			sb.WriteString(" | ")
			sb.WriteString(val)
			sb.WriteString(" |\n")
		}
	}
	sb.WriteString(renderIwencaiLongTextSections(row, longTextCols, commonDate))
	return sb.String()
}

func renderIwencaiLongTextSections(row map[string]any, cols []string, commonDate string) string {
	if len(cols) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, col := range cols {
		v, ok := row[col]
		if !ok || v == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprintf("%v", v))
		if text == "" {
			continue
		}
		sb.WriteString("\n**")
		sb.WriteString(formatIwencaiCell(formatIwencaiColumnHeader(col, commonDate)))
		sb.WriteString("**\n\n")
		sb.WriteString("> ")
		sb.WriteString(truncateIwencaiText(strings.ReplaceAll(text, "\n", " "), iwencaiLongTextMaxRunes))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderIwencaiDateSeriesTables(row map[string]any, groups []iwencaiDateSeriesGroup) string {
	if len(groups) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, g := range groups {
		sb.WriteString(fmt.Sprintf("\n**%s（按日期）**\n\n", g.prefix))
		sb.WriteString("| 日期 | 数值 |\n| --- | --- |\n")
		for _, col := range g.cols {
			val := ""
			if v, ok := row[col.name]; ok && v != nil {
				val = formatIwencaiCellByColumn(col.name, v)
			}
			if val == "" {
				continue
			}
			sb.WriteString("| ")
			sb.WriteString(formatIwencaiDate(col.date))
			sb.WriteString(" | ")
			sb.WriteString(val)
			sb.WriteString(" |\n")
		}
	}
	return sb.String()
}

func formatIwencaiDate(raw string) string {
	if len(raw) != 8 {
		return raw
	}
	return raw[0:4] + "-" + raw[4:6] + "-" + raw[6:8]
}

func renderIwencaiMarkdownTable(headers []string, datas []map[string]any, commonDate string) string {
	if len(headers) == 0 || len(datas) == 0 {
		return ""
	}
	displayHeaders := make([]string, len(headers))
	for i, h := range headers {
		displayHeaders[i] = formatIwencaiColumnHeader(h, commonDate)
	}
	var sb strings.Builder
	sb.WriteString("| ")
	for _, h := range displayHeaders {
		sb.WriteString(formatIwencaiCell(h))
		sb.WriteString(" | ")
	}
	sb.WriteString("\n| ")
	for range headers {
		sb.WriteString("--- | ")
	}
	sb.WriteString("\n")
	for _, row := range datas {
		sb.WriteString("| ")
		for _, h := range headers {
			val := ""
			if v, ok := row[h]; ok {
				val = formatIwencaiCellByColumn(h, v)
			}
			sb.WriteString(val)
			sb.WriteString(" | ")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderIwencaiExtraFields(datas []map[string]any, extraCols []string, commonDate string) string {
	if len(extraCols) == 0 {
		return ""
	}
	normalCols, longTextCols := splitLongTextColumns(extraCols)
	var sb strings.Builder
	sb.WriteString("\n**补充字段**\n\n")
	for i, row := range datas {
		if len(datas) > 1 {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, rowIdentityLabel(row)))
		}
		for _, col := range normalCols {
			val := ""
			if v, ok := row[col]; ok && v != nil {
				val = formatIwencaiCellByColumn(col, v)
			}
			if val == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s：%s\n", formatIwencaiColumnHeader(col, commonDate), val))
		}
		sb.WriteString(renderIwencaiLongTextSections(row, longTextCols, commonDate))
		if i < len(datas)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func renderIwencaiMacroMarkdown(row map[string]any) string {
	title := ""
	for _, k := range []string{"指标", "macro_name", "指标名称"} {
		if v, ok := row[k]; ok && v != nil {
			title = strings.TrimSpace(fmt.Sprintf("%v", v))
			if title != "" {
				break
			}
		}
	}
	priority := []string{"指标值", "单位", "时间", "国家", "周期", "地区级别", "指标"}
	var sb strings.Builder
	if title != "" {
		sb.WriteString("#### ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString("| 字段 | 值 |\n| --- | --- |\n")
	seen := map[string]struct{}{}
	for _, col := range priority {
		v, ok := row[col]
		if !ok || v == nil {
			continue
		}
		val := formatIwencaiCellByColumn(col, v)
		if val == "" {
			continue
		}
		sb.WriteString("| ")
		sb.WriteString(formatIwencaiCell(col))
		sb.WriteString(" | ")
		sb.WriteString(val)
		sb.WriteString(" |\n")
		seen[col] = struct{}{}
	}
	for _, col := range collectIwencaiColumnKeys([]map[string]any{row}) {
		if _, ok := seen[col]; ok {
			continue
		}
		if _, hide := iwencaiMacroHiddenColumns[col]; hide {
			continue
		}
		v, ok := row[col]
		if !ok || v == nil {
			continue
		}
		val := formatIwencaiCellByColumn(col, v)
		if val == "" {
			continue
		}
		sb.WriteString("| ")
		sb.WriteString(formatIwencaiCell(col))
		sb.WriteString(" | ")
		sb.WriteString(val)
		sb.WriteString(" |\n")
	}
	return sb.String()
}

func renderIwencaiSingleEntityMarkdown(row map[string]any) string {
	cols := collectIwencaiColumnKeys([]map[string]any{row})
	commonDate := extractCommonBracketDate(cols)
	static, series := partitionDateColumns(cols)
	var sb strings.Builder
	if title := buildEntityTitle(row); title != "" {
		sb.WriteString("#### ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString(renderIwencaiKVTable(static, row, commonDate))
	sb.WriteString(renderIwencaiDateSeriesTables(row, series))
	return sb.String()
}

func renderIwencaiMultiRowMarkdown(datas []map[string]any) string {
	allCols := collectIwencaiColumnKeys(datas)
	commonDate := extractCommonBracketDate(allCols)
	primary, extra := splitIwencaiColumns(allCols)
	var sb strings.Builder
	sb.WriteString(renderIwencaiMarkdownTable(primary, datas, commonDate))
	if len(extra) > 0 {
		sb.WriteString(renderIwencaiExtraFields(datas, extra, commonDate))
	}
	if len(allCols) > len(primary) {
		sb.WriteString(fmt.Sprintf("\n> 共 %d 个字段，主表展示 %d 个，其余见补充字段。\n", len(allCols), len(primary)))
	}
	return sb.String()
}

func renderIwencaiDatasMarkdown(datas []map[string]any) string {
	if len(datas) == 0 {
		return ""
	}
	kind := detectIwencaiResultKind(datas)
	if kind == iwencaiKindMacro {
		return renderIwencaiMacroMarkdown(datas[0])
	}
	if len(datas) == 1 {
		return renderIwencaiSingleEntityMarkdown(datas[0])
	}
	return renderIwencaiMultiRowMarkdown(datas)
}

func iwencaiResultCountLabel(result *IwencaiResponse, kind iwencaiResultKind) string {
	if result == nil {
		return "0 条记录"
	}
	count := len(result.Datas)
	if result.CodeCount > 0 {
		count = result.CodeCount
	}
	unit := "条记录"
	switch kind {
	case iwencaiKindStock, iwencaiKindFund:
		unit = "只标的"
	case iwencaiKindIndex:
		unit = "个指数"
	case iwencaiKindMacro:
		unit = "条记录"
	}
	return fmt.Sprintf("%d %s", count, unit)
}

func formatIwencaiChunksInfo(chunks any) string {
	if chunks == nil {
		return ""
	}
	parts := iwencaiChunksToStrings(chunks)
	if len(parts) == 0 {
		return ""
	}
	return "匹配条件：" + strings.Join(parts, "；")
}

func iwencaiChunksToStrings(chunks any) []string {
	switch v := chunks.(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				parts = append(parts, s)
			}
		}
		return parts
	case []string:
		parts := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				parts = append(parts, s)
			}
		}
		return parts
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		if strings.HasPrefix(s, "[") {
			var arr []string
			if err := json.Unmarshal([]byte(s), &arr); err == nil {
				return iwencaiChunksToStrings(arr)
			}
			var arrAny []any
			if err := json.Unmarshal([]byte(s), &arrAny); err == nil {
				return iwencaiChunksToStrings(arrAny)
			}
		}
		return []string{s}
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", chunks))
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func buildIwencaiQuerySummary(result *IwencaiResponse, page, limit, colCount int, kind iwencaiResultKind) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共查到 %s，当前第 %d 页（每页 %d 条", iwencaiResultCountLabel(result, kind), page, limit))
	if colCount > 0 {
		sb.WriteString(fmt.Sprintf("，%d 个字段", colCount))
	}
	sb.WriteString("）")
	if hint := formatIwencaiChunksInfo(result.ChunksInfo); hint != "" {
		sb.WriteByte('\n')
		sb.WriteString(hint)
	}
	return sb.String()
}

func renderIwencaiQueryMarkdown(query string, result *IwencaiResponse, page, limit int) string {
	kind := detectIwencaiResultKind(result.Datas)
	allCols := collectIwencaiColumnKeys(result.Datas)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 同花顺问财查询结果（%s）\n\n", query))
	sb.WriteString(buildIwencaiQuerySummary(result, page, limit, len(allCols), kind))
	sb.WriteString("\n\n")
	sb.WriteString(renderIwencaiDatasMarkdown(result.Datas))
	sb.WriteString("\n> 数据来源于同花顺问财")
	return sb.String()
}

func truncateIwencaiText(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "..."
}

func renderIwencaiSearchMarkdown(query, label string, items []IwencaiSearchItem) string {
	if len(items) == 0 {
		return fmt.Sprintf("未搜索到「%s」的相关%s。", query, label)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 同花顺问财%s搜索结果（%s）\n\n", label, query))
	sb.WriteString(fmt.Sprintf("共找到 %d 条结果\n\n", len(items)))

	if len(items) >= 4 {
		sb.WriteString("| # | 标题 | 发布时间 |\n| --- | --- | --- |\n")
		for i, item := range items {
			title := formatIwencaiCell(item.Title)
			date := formatIwencaiCell(item.PublishDate)
			sb.WriteString(fmt.Sprintf("| %d | %s | %s |\n", i+1, title, date))
		}
		sb.WriteString("\n**详情**\n\n")
	}

	for i, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "（无标题）"
		}
		sb.WriteString(fmt.Sprintf("%d. **%s**", i+1, title))
		if item.PublishDate != "" {
			sb.WriteString(fmt.Sprintf("（%s）", formatIwencaiCell(item.PublishDate)))
		}
		sb.WriteByte('\n')
		if item.Summary != "" {
			sb.WriteString("   - 摘要：")
			sb.WriteString(truncateIwencaiText(item.Summary, iwencaiSearchSummaryMaxRunes))
			sb.WriteByte('\n')
		}
		if item.URL != "" {
			sb.WriteString("   - 链接：")
			sb.WriteString(item.URL)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("> 数据来源于同花顺问财")
	return sb.String()
}
