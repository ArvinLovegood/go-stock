package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"

	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2026/8/31 20:00
// @Desc 龙虎榜席位明细（游资/机构买卖数据）
// 数据来源：东方财富数据中心 datacenter-web.eastmoney.com
// 买入席位报表 RPT_BILLBOARD_DAILYDETAILSBUY（旧报表 RPT_BILLBOARD_TRADEDETAILSBUY 已下线）
// 卖出席位报表 RPT_BILLBOARD_DAILYDETAILSSELL
// -----------------------------------------------------------------------------------

type LhbSeatApi struct {
}

func NewLhbSeatApi() *LhbSeatApi {
	return &LhbSeatApi{}
}

const lhbSeatDataURL = "https://datacenter-web.eastmoney.com/api/data/v1/get"

// normalizeLhbCode 归一化股票代码为纯数字（沪深的东财数据中心席位报表用 SECURITY_CODE 纯代码过滤）
func normalizeLhbCode(stockCode string) string {
	code := strings.TrimSpace(stockCode)
	// 剥离常见前后缀：sh600519 / 600519.SH / SZ000001 / 000001.SZ
	code = strings.TrimPrefix(strings.TrimPrefix(code, "sh"), "sz")
	code = strings.TrimPrefix(strings.TrimPrefix(code, "SH"), "SZ")
	code = strings.TrimSuffix(strings.TrimSuffix(code, ".SH"), ".SZ")
	code = strings.TrimSuffix(strings.TrimSuffix(code, ".sh"), ".sz")
	return code
}

// GetLhbSeatDetail 查询个股某交易日龙虎榜买5卖5席位明细。
// date 为空取当天；返回买卖席位列表（含机构/游资/北向识别）。
func (receiver LhbSeatApi) GetLhbSeatDetail(stockCode, date string) *models.LhbSeatDetailData {
	stockCode = normalizeLhbCode(stockCode)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	result := &models.LhbSeatDetailData{
		StockCode: stockCode,
		TradeDate: date,
	}
	var firstRow, sellFirstRow gjson.Result
	result.BuySeats, firstRow = fetchLhbSeatList(stockCode, date, "RPT_BILLBOARD_DAILYDETAILSBUY", "BUY")
	result.SellSeats, sellFirstRow = fetchLhbSeatList(stockCode, date, "RPT_BILLBOARD_DAILYDETAILSSELL", "SELL")
	if !firstRow.Exists() {
		firstRow = sellFirstRow
	}
	// 上榜原因/收盘价等公共字段从首条记录取
	if firstRow.Exists() {
		result.Explanation = firstRow.Get("EXPLANATION").String()
		result.ChangeRate = firstRow.Get("CHANGE_RATE").Float()
		result.ClosePrice = firstRow.Get("CLOSE_PRICE").Float()
		result.AccumAmount = firstRow.Get("ACCUM_AMOUNT").Float()
	}
	return result
}

// fetchLhbSeatList 拉取指定方向的席位明细并做类型识别；返回席位列表与首条原始记录
func fetchLhbSeatList(stockCode, date, reportName, sortColumn string) ([]models.LhbSeatItem, gjson.Result) {
	items := []models.LhbSeatItem{}
	var firstRow gjson.Result
	params := map[string]string{
		"sortColumns": sortColumn,
		"sortTypes":   "-1",
		"pageSize":    "50",
		"pageNumber":  "1",
		"reportName":  reportName,
		"columns":     "ALL",
		"source":      "WEB",
		"client":      "WEB",
		"filter":      fmt.Sprintf(`(SECURITY_CODE="%s")(TRADE_DATE='%s')`, stockCode, date),
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/stock/tradedetail.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetQueryParams(params).
		Get(lhbSeatDataURL)
	if err != nil {
		logger.SugaredLogger.Errorf("获取龙虎榜席位明细失败(%s %s %s): %v", stockCode, date, reportName, err)
		return items, firstRow
	}
	arr := gjson.Get(string(resp.Body()), "result.data").Array()
	// 同一席位可能因多个上榜原因(EXPLANATION)重复出现（如"日涨幅偏离7%"+
	// "三日涨幅偏离20%"各返回一次），按席位名去重，保留成交额最大的一行，
	// 否则前端展示与游资动向聚合金额会翻倍
	pos := map[string]int{}
	for _, row := range arr {
		if !firstRow.Exists() {
			firstRow = row
		}
		seat := models.LhbSeatItem{
			OperateDeptName: row.Get("OPERATEDEPT_NAME").String(),
			Buy:             row.Get("BUY").Float(),
			Sell:            row.Get("SELL").Float(),
			Net:             row.Get("NET").Float(),
			BuyRatio:        row.Get("TOTAL_BUYRIO").Float(),
			SellRatio:       row.Get("TOTAL_SELLRIO").Float(),
		}
		if entry := lookupHotMoneySeat(seat.OperateDeptName); entry != nil {
			seat.SeatType = entry.category
			seat.HotMoneyName = entry.name
			seat.Tier = entry.tier
			seat.Style = entry.style
			seat.RiskLevel = entry.risk
		} else {
			seat.SeatType, seat.HotMoneyName = classifyLhbSeat(seat.OperateDeptName)
		}
		if old, ok := pos[seat.OperateDeptName]; ok {
			// 重复席位：保留买卖合计更大的一行（多榜单金额偶有差异）
			if seat.Buy+seat.Sell > items[old].Buy+items[old].Sell {
				items[old] = seat
			}
			continue
		}
		pos[seat.OperateDeptName] = len(items)
		items = append(items, seat)
	}
	return items, firstRow
}

// classifyLhbSeat 识别席位类型：机构专用/北向通道/游资/散户集中营/量化/普通营业部。
// 返回 (席位类型, 标签)；游资的 tier/style/riskLevel 通过 lookupHotMoneySeat 获取。
func classifyLhbSeat(name string) (seatType, hotMoneyName string) {
	if strings.Contains(name, "机构专用") {
		return "机构", ""
	}
	if strings.Contains(name, "沪股通专用") || strings.Contains(name, "深股通专用") {
		return "北向资金", ""
	}
	if entry := lookupHotMoneySeat(name); entry != nil {
		return entry.category, entry.name
	}
	return "营业部", ""
}

// ---------- 游资席位名录（外置 data/hot_money_seats.json，标准格式） ----------

// HotMoneySeatBranch 游资单个席位；primary=true 为该游资的核心席位
type HotMoneySeatBranch struct {
	Branch  string `json:"branch"`
	Primary bool   `json:"primary"`
}

// HotMoneySeat 游资条目：花名 + 画像（梯队/风格/风险） + 名下席位列表
type HotMoneySeat struct {
	Name     string               `json:"name"`
	RealName string               `json:"real_name"`
	Aliases  []string             `json:"aliases"`
	Tier     string               `json:"tier"`
	Style    string               `json:"style"`
	Risk     string               `json:"risk_level"`
	Seats    []HotMoneySeatBranch `json:"seats"`
}

// QuantSeat 量化/机构通道席位
type QuantSeat struct {
	Branch string `json:"branch"`
	Type   string `json:"type"`
}

// RetailClusterSeats 散户集中营席位（拉萨天团）
// 注意：Wails bindings 生成 TS 时对匿名嵌套 struct 会产出非法类型（[]data. 空名），
// 此处全部使用具名结构体
type RetailClusterSeats struct {
	Name  string   `json:"name"`
	Note  string   `json:"note"`
	Seats []string `json:"seats"`
}

// QuantChannelSeats 量化/机构通道席位
type QuantChannelSeats struct {
	Name  string      `json:"name"`
	Note  string      `json:"note"`
	Seats []QuantSeat `json:"seats"`
}

// HotMoneySpecialSeats 特殊席位：散户集中营（拉萨天团）与量化通道
type HotMoneySpecialSeats struct {
	RetailCluster RetailClusterSeats `json:"retail_cluster"`
	QuantSeats    QuantChannelSeats  `json:"quant_seats"`
}

// HotMoneySeatFileMeta 名录元信息
type HotMoneySeatFileMeta struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// HotMoneySeatFile 游资名录外置文件（可编辑/可远程更新），结构见 data/hot_money_seats.json。
// remoteUrl 为本项目扩展字段：配置后启动时异步拉取远程名录刷新本地。
type HotMoneySeatFile struct {
	Meta         HotMoneySeatFileMeta `json:"meta"`
	RemoteURL    string               `json:"remoteUrl"`
	HotMoneyList []HotMoneySeat       `json:"hot_money_list"`
	SpecialSeats HotMoneySpecialSeats `json:"special_seats"`
}

const hotMoneySeatsFile = "data/hot_money_seats.json"

// defaultHotMoneySeatsRemoteURL 默认远程名录源（上游仓库 dev 分支）
const defaultHotMoneySeatsRemoteURL = "https://gh-proxy.com/https://github.com/ArvinLovegood/go-stock/blob/dev/data/hot_money_seats.json"

// normalizeRemoteSeatURL 归一化远程名录 URL：GitHub blob 链接返回 HTML 页面而非 JSON，
// 需转为 raw 地址（github.com/OWNER/REPO/blob/REF/PATH → raw.githubusercontent.com/OWNER/REPO/REF/PATH）。
// gh-proxy 等反代直接透传完整 URL，转换后再拼接即可正常返回原始 JSON。
func normalizeRemoteSeatURL(url string) string {
	u := strings.TrimSpace(url)
	if u == "" {
		return u
	}
	const marker = "github.com/"
	idx := strings.Index(u, marker)
	if idx < 0 {
		return u
	}
	rest := u[idx+len(marker):]
	// rest 形如 OWNER/REPO/blob/REF/PATH
	parts := strings.SplitN(rest, "/", 5)
	if len(parts) == 5 && (parts[2] == "blob" || parts[2] == "raw") {
		return u[:idx] + "raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/" + parts[3] + "/" + parts[4]
	}
	return u
}

// hotMoneyIndexEntry 归一化后的索引条目（用于席位全称模糊匹配）
type hotMoneyIndexEntry struct {
	branch   string // 归一化席位关键词
	category string // 游资 / 散户 / 量化
	name     string // 游资花名或席位标签
	tier     string
	style    string
	risk     string
}

// builtinHotMoneySeatsJSON 内置游资名录（完整标准格式，编译进二进制）。
// 外置 data/hot_money_seats.json 不存在时用它兜底并自动生成文件；
// 后续名录更新只需同步修改此文件。游资标签为社区推断性共识（非官方认定），
// 席位可能随券商更名/迁址变化，未命中仅表示"未收录"，不代表非游资。
//
//go:embed hot_money_seats_builtin.json
var builtinHotMoneySeatsJSON []byte

// builtinHotMoneySeatsSeed 解析内置名录
func builtinHotMoneySeatsSeed() HotMoneySeatFile {
	var f HotMoneySeatFile
	if err := json.Unmarshal(builtinHotMoneySeatsJSON, &f); err != nil {
		logger.SugaredLogger.Errorf("内置游资名录解析失败: %v", err)
		return HotMoneySeatFile{}
	}
	if f.RemoteURL == "" {
		f.RemoteURL = defaultHotMoneySeatsRemoteURL
	}
	return f
}

var (
	hotMoneySeatsOnce sync.Once
	hotMoneySeatsMu   sync.RWMutex
	hotMoneySeatIndex []hotMoneyIndexEntry
)

// normalizeLhbBranch 席位名称归一化：剥离公司组织形式后缀，
// 使 "国泰君安证券上海江苏路证券营业部" 能匹配龙虎榜全称 "国泰君安证券股份有限公司上海江苏路证券营业部"。
var lhbBranchNormalizer = strings.NewReplacer("股份有限公司", "", "有限责任公司", "")

func normalizeLhbBranch(s string) string {
	return lhbBranchNormalizer.Replace(strings.TrimSpace(s))
}

// buildHotMoneySeatIndex 由名录构建匹配索引：游资（跳过历史人物如徐翔）→ 散户集中营 → 量化
func buildHotMoneySeatIndex(f *HotMoneySeatFile) []hotMoneyIndexEntry {
	var idx []hotMoneyIndexEntry
	for _, hm := range f.HotMoneyList {
		// 历史人物（已退出）不参与现代数据匹配，避免席位被误标
		if strings.Contains(hm.Tier, "历史") || strings.Contains(hm.Style, "已退出") {
			continue
		}
		for _, s := range hm.Seats {
			idx = append(idx, hotMoneyIndexEntry{
				branch: normalizeLhbBranch(s.Branch), category: "游资",
				name: hm.Name, tier: hm.Tier, style: hm.Style, risk: hm.Risk,
			})
		}
	}
	for _, b := range f.SpecialSeats.RetailCluster.Seats {
		idx = append(idx, hotMoneyIndexEntry{
			branch: normalizeLhbBranch(b), category: "散户",
			name: f.SpecialSeats.RetailCluster.Name,
		})
	}
	for _, q := range f.SpecialSeats.QuantSeats.Seats {
		idx = append(idx, hotMoneyIndexEntry{
			branch: normalizeLhbBranch(q.Branch), category: "量化",
			name: q.Type,
		})
	}
	return idx
}

// loadHotMoneySeatIndex 懒加载游资名录索引：优先读外置 JSON（data/hot_money_seats.json），
// 文件不存在时用内置种子生成一份，之后用户可直接编辑该文件（进程重启生效）。
func loadHotMoneySeatIndex() []hotMoneyIndexEntry {
	hotMoneySeatsOnce.Do(func() {
		f, ok := readHotMoneySeatFile()
		if !ok {
			return
		}
		hotMoneySeatsMu.Lock()
		hotMoneySeatIndex = buildHotMoneySeatIndex(&f)
		hotMoneySeatsMu.Unlock()
		// 配置了远程名录源则异步刷新一次（失败静默回退本地）
		if f.RemoteURL != "" {
			go RefreshHotMoneySeats(f.RemoteURL)
		}
	})
	hotMoneySeatsMu.RLock()
	defer hotMoneySeatsMu.RUnlock()
	return hotMoneySeatIndex
}

// readHotMoneySeatFile 读取外置名录文件；文件不存在时用内置种子生成一份并返回。
// ok=false 表示读取/解析均失败（调用方回退空索引，匹配退化为基础分类）。
func readHotMoneySeatFile() (HotMoneySeatFile, bool) {
	raw, err := os.ReadFile(hotMoneySeatsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.SugaredLogger.Warnf("读取游资名录失败: %v", err)
			return HotMoneySeatFile{}, false
		}
		// 文件不存在：直接落盘内置名录原始内容（保留 meta 扩展字段），方便用户后续自行维护
		_ = os.MkdirAll(filepath.Dir(hotMoneySeatsFile), 0755)
		if werr := os.WriteFile(hotMoneySeatsFile, builtinHotMoneySeatsJSON, 0644); werr != nil {
			logger.SugaredLogger.Warnf("写入游资名录种子文件失败: %v", werr)
		}
		return builtinHotMoneySeatsSeed(), true
	}
	var f HotMoneySeatFile
	if err := json.Unmarshal(raw, &f); err != nil {
		logger.SugaredLogger.Warnf("游资名录 JSON 解析失败: %v", err)
		return HotMoneySeatFile{}, false
	}
	return f, true
}

// GetHotMoneySeats 读取游资名录（供前端维护页面展示；文件不存在时返回内置种子）
func GetHotMoneySeats() *HotMoneySeatFile {
	f, _ := readHotMoneySeatFile()
	return &f
}

// SaveHotMoneySeats 保存游资名录（前端维护页面提交）：校验后落盘并热更新内存索引，即时生效。
func SaveHotMoneySeats(f *HotMoneySeatFile) error {
	if f == nil {
		return fmt.Errorf("名录数据不能为空")
	}
	if len(f.HotMoneyList) == 0 {
		return fmt.Errorf("游资列表为空，至少保留一条记录")
	}
	for i, hm := range f.HotMoneyList {
		if strings.TrimSpace(hm.Name) == "" {
			return fmt.Errorf("第%d个游资花名为空", i+1)
		}
		if len(hm.Seats) == 0 {
			return fmt.Errorf("游资[%s]未配置席位", hm.Name)
		}
		for j, s := range hm.Seats {
			if strings.TrimSpace(s.Branch) == "" {
				return fmt.Errorf("游资[%s]第%d个席位名称为空", hm.Name, j+1)
			}
		}
	}
	if err := writeHotMoneySeatFile(f); err != nil {
		return err
	}
	hotMoneySeatsMu.Lock()
	hotMoneySeatIndex = buildHotMoneySeatIndex(f)
	hotMoneySeatsMu.Unlock()
	logger.SugaredLogger.Infof("游资名录已保存: 游资数=%d", len(f.HotMoneyList))
	return nil
}

// ResetHotMoneySeats 恢复内置种子名录（覆盖外置文件并热更新内存索引）
func ResetHotMoneySeats() error {
	// 直接写内置原始内容（保留 meta 扩展字段）
	_ = os.MkdirAll(filepath.Dir(hotMoneySeatsFile), 0755)
	if err := os.WriteFile(hotMoneySeatsFile, builtinHotMoneySeatsJSON, 0644); err != nil {
		return fmt.Errorf("写游资名录文件失败: %w", err)
	}
	seed := builtinHotMoneySeatsSeed()
	hotMoneySeatsMu.Lock()
	hotMoneySeatIndex = buildHotMoneySeatIndex(&seed)
	hotMoneySeatsMu.Unlock()
	logger.SugaredLogger.Info("游资名录已重置为内置数据")
	return nil
}

// writeHotMoneySeatFile 名录落盘
func writeHotMoneySeatFile(f *HotMoneySeatFile) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化游资名录失败: %w", err)
	}
	_ = os.MkdirAll(filepath.Dir(hotMoneySeatsFile), 0755)
	if err := os.WriteFile(hotMoneySeatsFile, b, 0644); err != nil {
		return fmt.Errorf("写游资名录文件失败: %w", err)
	}
	return nil
}

// RefreshHotMoneySeats 从远程 URL 拉取游资名录 JSON（标准格式）并落盘（覆盖本地文件、更新内存索引）。
// 用于接入用户自托管或社区维护的名录源；GitHub blob 链接会自动转为 raw 地址。
func RefreshHotMoneySeats(url string) error {
	rawURL := normalizeRemoteSeatURL(url)
	if rawURL == "" {
		return fmt.Errorf("远程名录 URL 为空")
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		Get(rawURL)
	if err != nil {
		return fmt.Errorf("拉取游资名录失败: %w", err)
	}
	var f HotMoneySeatFile
	if err := json.Unmarshal(resp.Body(), &f); err != nil {
		return fmt.Errorf("游资名录 JSON 解析失败（URL 需指向 JSON 原始文件而非网页）: %w", err)
	}
	if len(f.HotMoneyList) == 0 {
		return fmt.Errorf("远程游资名录为空，已忽略")
	}
	// 保留 remoteUrl 配置（远程文件本身不含该字段，落盘后仍可持续自动更新）
	f.RemoteURL = strings.TrimSpace(url)
	if err := writeHotMoneySeatFile(&f); err != nil {
		return err
	}
	hotMoneySeatsMu.Lock()
	hotMoneySeatIndex = buildHotMoneySeatIndex(&f)
	hotMoneySeatsMu.Unlock()
	logger.SugaredLogger.Infof("游资名录已从远程刷新: version=%s 游资数=%d", f.Meta.Version, len(f.HotMoneyList))
	return nil
}

// lookupHotMoneySeat 按归一化子串匹配席位，返回命中的索引条目；未命中返回 nil
func lookupHotMoneySeat(deptName string) *hotMoneyIndexEntry {
	norm := normalizeLhbBranch(deptName)
	idx := loadHotMoneySeatIndex()
	for i := range idx {
		if idx[i].branch != "" && strings.Contains(norm, idx[i].branch) {
			return &idx[i]
		}
	}
	return nil
}

// matchHotMoneySeat 兼容保留：返回命中的游资花名（仅游资类目），未收录返回空串
func matchHotMoneySeat(deptName string) string {
	if entry := lookupHotMoneySeat(deptName); entry != nil && entry.category == "游资" {
		return entry.name
	}
	return ""
}

// GetLhbSeatDetailToMarkdown 渲染席位明细为 Markdown（AI 工具输出用）
func (receiver LhbSeatApi) GetLhbSeatDetailToMarkdown(stockCode, date string) string {
	detail := receiver.GetLhbSeatDetail(stockCode, date)
	if detail == nil || (len(detail.BuySeats) == 0 && len(detail.SellSeats) == 0) {
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		return fmt.Sprintf("## %s %s 龙虎榜席位明细\n\n当日未上榜或无席位明细数据", stockCode, date)
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s %s 龙虎榜席位明细\n\n", detail.StockCode, detail.TradeDate))
	if detail.Explanation != "" {
		sb.WriteString(fmt.Sprintf("- 上榜原因：%s\n", detail.Explanation))
	}
	if detail.ClosePrice > 0 {
		sb.WriteString(fmt.Sprintf("- 收盘价：%.2f（涨跌幅 %.2f%%）\n", detail.ClosePrice, detail.ChangeRate))
	}
	if detail.AccumAmount > 0 {
		sb.WriteString(fmt.Sprintf("- 市场总成交额：%.2f 亿\n", detail.AccumAmount/1e8))
	}
	seatLabel := func(s models.LhbSeatItem) string {
		if s.HotMoneyName == "" {
			return "-"
		}
		if s.Tier != "" {
			return fmt.Sprintf("%s（%s）", s.HotMoneyName, s.Tier)
		}
		return s.HotMoneyName
	}
	sb.WriteString(fmt.Sprintf("\n### 买入席位 TOP%d\n\n", len(detail.BuySeats)))
	sb.WriteString("| 席位 | 类型 | 游资标签 | 买入金额(万) | 买入占总成交比 |\n")
	sb.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, seat := range detail.BuySeats {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.2f | %.2f%% |\n",
			seat.OperateDeptName, seat.SeatType, seatLabel(seat), seat.Buy/1e4, seat.BuyRatio*100))
	}
	sb.WriteString(fmt.Sprintf("\n### 卖出席位 TOP%d\n\n", len(detail.SellSeats)))
	sb.WriteString("| 席位 | 类型 | 游资标签 | 卖出金额(万) | 卖出占总成交比 |\n")
	sb.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, seat := range detail.SellSeats {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.2f | %.2f%% |\n",
			seat.OperateDeptName, seat.SeatType, seatLabel(seat), seat.Sell/1e4, seat.SellRatio*100))
	}
	// 游资画像：去重汇总买卖两侧命中的游资及其风格/风险
	profiles := map[string]*models.LhbSeatItem{}
	for i := range detail.BuySeats {
		if detail.BuySeats[i].SeatType == "游资" {
			profiles[detail.BuySeats[i].HotMoneyName] = &detail.BuySeats[i]
		}
	}
	for i := range detail.SellSeats {
		if detail.SellSeats[i].SeatType == "游资" {
			if _, ok := profiles[detail.SellSeats[i].HotMoneyName]; !ok {
				profiles[detail.SellSeats[i].HotMoneyName] = &detail.SellSeats[i]
			}
		}
	}
	if len(profiles) > 0 {
		sb.WriteString("\n### 游资画像\n\n")
		for _, p := range profiles {
			sb.WriteString(fmt.Sprintf("- **%s**（%s｜风险：%s）：%s\n",
				p.HotMoneyName, p.Tier, p.RiskLevel, p.Style))
		}
	}
	return sb.String()
}

// ---------- 当日游资/机构动向汇总 ----------

// lhbDailySummaryCache 当日汇总缓存（龙虎榜收盘后数据不变，10 分钟缓存避免重复全量抓取）
var (
	lhbDailySummaryMu    sync.Mutex
	lhbDailySummaryCache = map[string]*models.LhbDailySummary{}
)

// lhbBillboardStock 当日上榜个股（从龙虎榜榜单接口取基础信息）
type lhbBillboardStock struct {
	StockCode  string
	StockName  string
	ChangeRate float64
	ClosePrice float64
}

// fetchLhbBillboardStocks 拉取某交易日全部上榜个股（复用 RPT_DAILYBILLBOARD_DETAILSNEW 榜单报表）
func fetchLhbBillboardStocks(date string) []lhbBillboardStock {
	params := map[string]string{
		"sortColumns": "TURNOVERRATE,TRADE_DATE,SECURITY_CODE",
		"sortTypes":   "-1,-1,1",
		"pageSize":    "500",
		"pageNumber":  "1",
		"reportName":  "RPT_DAILYBILLBOARD_DETAILSNEW",
		"columns":     "SECURITY_CODE,SECURITY_NAME_ABBR,TRADE_DATE,CHANGE_RATE,CLOSE_PRICE",
		"source":      "WEB",
		"client":      "WEB",
		"filter":      fmt.Sprintf("(TRADE_DATE<='%s')(TRADE_DATE>='%s')", date, date),
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/stock/tradedetail.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetQueryParams(params).
		Get(lhbSeatDataURL)
	if err != nil {
		logger.SugaredLogger.Errorf("获取龙虎榜榜单失败(%s): %v", date, err)
		return nil
	}
	// 同一股票可能因多个上榜原因(EXPLANATION)重复出现，按代码去重
	var stocks []lhbBillboardStock
	seen := map[string]bool{}
	for _, row := range gjson.Get(string(resp.Body()), "result.data").Array() {
		s := lhbBillboardStock{
			StockCode:  row.Get("SECURITY_CODE").String(),
			StockName:  row.Get("SECURITY_NAME_ABBR").String(),
			ChangeRate: row.Get("CHANGE_RATE").Float(),
			ClosePrice: row.Get("CLOSE_PRICE").Float(),
		}
		if seen[s.StockCode] {
			continue
		}
		seen[s.StockCode] = true
		stocks = append(stocks, s)
	}
	return stocks
}

// GetLhbDailySummary 汇总某交易日龙虎榜游资/机构动向：
// 拉取当日上榜个股列表，并发抓取每只个股买5卖5席位明细，按游资/机构聚合。
func (receiver LhbSeatApi) GetLhbDailySummary(date string) *models.LhbDailySummary {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	lhbDailySummaryMu.Lock()
	if c, ok := lhbDailySummaryCache[date]; ok {
		lhbDailySummaryMu.Unlock()
		return c
	}
	lhbDailySummaryMu.Unlock()

	summary := &models.LhbDailySummary{Date: date}
	stocks := fetchLhbBillboardStocks(date)
	if len(stocks) == 0 {
		return summary
	}
	summary.StockCount = len(stocks)

	type stockSeats struct {
		stock lhbBillboardStock
		buys  []models.LhbSeatItem
		sells []models.LhbSeatItem
	}
	results := make([]stockSeats, len(stocks))
	sem := make(chan struct{}, 8) // 并发限制，避免触发东财限流
	var wg sync.WaitGroup
	for i, s := range stocks {
		wg.Add(1)
		go func(i int, s lhbBillboardStock) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			buys, _ := fetchLhbSeatList(s.StockCode, date, "RPT_BILLBOARD_DAILYDETAILSBUY", "BUY")
			sells, _ := fetchLhbSeatList(s.StockCode, date, "RPT_BILLBOARD_DAILYDETAILSSELL", "SELL")
			results[i] = stockSeats{stock: s, buys: buys, sells: sells}
		}(i, s)
	}
	wg.Wait()

	// 聚合游资动向
	hmMap := map[string]*models.LhbHotMoneyActivity{}
	for _, r := range results {
		hmStock := func(seat models.LhbSeatItem, isBuy bool) {
			act, ok := hmMap[seat.HotMoneyName]
			if !ok {
				act = &models.LhbHotMoneyActivity{
					HotMoneyName: seat.HotMoneyName,
					Tier:         seat.Tier,
					Style:        seat.Style,
					RiskLevel:    seat.RiskLevel,
				}
				hmMap[seat.HotMoneyName] = act
			}
			// 同一游资同一股票合并（可能买卖双向）
			var sa *models.LhbHotMoneyStockAction
			for j := range act.Stocks {
				if act.Stocks[j].StockCode == r.stock.StockCode {
					sa = &act.Stocks[j]
					break
				}
			}
			if sa == nil {
				act.Stocks = append(act.Stocks, models.LhbHotMoneyStockAction{
					StockCode:  r.stock.StockCode,
					StockName:  r.stock.StockName,
					ChangeRate: r.stock.ChangeRate,
					ClosePrice: r.stock.ClosePrice,
				})
				sa = &act.Stocks[len(act.Stocks)-1]
			}
			if isBuy {
				sa.Buy += seat.Buy
				act.TotalBuy += seat.Buy
			} else {
				sa.Sell += seat.Sell
				act.TotalSell += seat.Sell
			}
			sa.Net = sa.Buy - sa.Sell
		}
		for _, seat := range r.buys {
			if seat.SeatType == "游资" {
				hmStock(seat, true)
			}
		}
		for _, seat := range r.sells {
			if seat.SeatType == "游资" {
				hmStock(seat, false)
			}
		}
	}
	for _, act := range hmMap {
		summary.HotMoneyActivities = append(summary.HotMoneyActivities, *act)
	}
	// 按当日合计买入+卖出（成交活跃度）降序
	sort.Slice(summary.HotMoneyActivities, func(i, j int) bool {
		return summary.HotMoneyActivities[i].TotalBuy+summary.HotMoneyActivities[i].TotalSell >
			summary.HotMoneyActivities[j].TotalBuy+summary.HotMoneyActivities[j].TotalSell
	})

	// 聚合机构席位动向（按个股）
	instMap := map[string]*models.LhbInstitutionStockAction{}
	for _, r := range results {
		var ia *models.LhbInstitutionStockAction
		get := func() *models.LhbInstitutionStockAction {
			if ia == nil {
				ia = &models.LhbInstitutionStockAction{
					StockCode:  r.stock.StockCode,
					StockName:  r.stock.StockName,
					ChangeRate: r.stock.ChangeRate,
				}
				instMap[r.stock.StockCode] = ia
			}
			return ia
		}
		for _, seat := range r.buys {
			if seat.SeatType == "机构" {
				a := get()
				a.BuyCount++
				a.Buy += seat.Buy
			}
		}
		for _, seat := range r.sells {
			if seat.SeatType == "机构" {
				a := get()
				a.SellCount++
				a.Sell += seat.Sell
			}
		}
	}
	for _, ia := range instMap {
		ia.Net = ia.Buy - ia.Sell
		summary.InstitutionActions = append(summary.InstitutionActions, *ia)
	}
	sort.Slice(summary.InstitutionActions, func(i, j int) bool {
		return summary.InstitutionActions[i].Net > summary.InstitutionActions[j].Net
	})

	// 缓存 10 分钟（当日龙虎榜收盘后数据不再变化）
	lhbDailySummaryMu.Lock()
	lhbDailySummaryCache[date] = summary
	lhbDailySummaryMu.Unlock()
	go func() {
		time.Sleep(10 * time.Minute)
		lhbDailySummaryMu.Lock()
		delete(lhbDailySummaryCache, date)
		lhbDailySummaryMu.Unlock()
	}()
	return summary
}
