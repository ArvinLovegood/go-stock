package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
)

const emF10BaseURL = "https://datacenter.eastmoney.com/securities/api/data/v1/get"

func (receiver StockDataApi) f10Request(url string, result any) error {
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Host", "datacenter.eastmoney.com").
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("Origin", "https://emweb.securities.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	if err := json.Unmarshal(resp.Body(), result); err != nil {
		return fmt.Errorf("parse failed: %v", err)
	}
	return nil
}

func normalizeF10Code(stockCode string) string {
	if strutil.ContainsAny(stockCode, []string{"."}) {
		return stockCode
	}
	converted := ConvertStockCodeToTushareCode(stockCode)
	if strutil.ContainsAny(converted, []string{"."}) {
		return converted
	}
	code := RemoveAllNonDigitChar(stockCode)
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		return code + ".SH"
	}
	if strings.HasPrefix(code, "0") || strings.HasPrefix(code, "3") {
		return code + ".SZ"
	}
	if strings.HasPrefix(code, "4") || strings.HasPrefix(code, "8") {
		return code + ".BJ"
	}
	if strings.HasPrefix(code, "5") {
		return code + ".SH"
	}
	return code + ".SZ"
}

type F10GenericResp struct {
	Version string     `json:"version"`
	Result  *F10Result `json:"result"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Code    int        `json:"code"`
}

type F10Result struct {
	Count int              `json:"count"`
	Data  []map[string]any `json:"data"`
}

var f10FieldCN = map[string]string{
	"SECUCODE":                   "证券代码",
	"SECURITY_CODE":              "股票代码",
	"SECURITY_NAME_ABBR":         "股票简称",
	"ORG_CODE":                   "机构代码",
	"REPORT_DATE":                "报告日期",
	"REPORT_TYPE":                "报告类型",
	"FORMERNAME":                 "曾用名",
	"MAKET_CODE":                 "市场代码",
	"SECURITY_TYPE_CODE":         "证券类型代码",
	"SECURITY_INNER_CODE":        "证券内部代码",
	"SECURITY_TYPE":              "证券类型",
	"SECURITY_TYPE_WEB":          "证券类型",
	"EPSJB":                      "基本每股收益",
	"EPSKCJB":                    "扣非每股收益",
	"EPSXS":                      "稀释每股收益",
	"EPSJB_PL":                   "摊薄每股收益",
	"BPS":                        "每股净资产",
	"BPS_PL":                     "摊薄每股净资产",
	"MGZBGJ":                     "每股资本公积",
	"MGWFPLR":                    "每股未分配利润",
	"MGJYXJJE":                   "每股经营现金流",
	"PER_CAPITAL_RESERVE":        "每股资本公积",
	"PER_UNASSIGN_PROFIT":        "每股未分配利润",
	"PER_NETCASH":                "每股经营现金流",
	"TOTAL_OPERATEINCOME":        "营业总收入",
	"TOTAL_OPERATEINCOME_LAST":   "营业总收入(上年)",
	"PARENT_NETPROFIT":           "归属净利润",
	"PARENT_NETPROFIT_LAST":      "归属净利润(上年)",
	"KCFJCXSYJLR":                "扣非净利润",
	"KCFJCXSYJLR_LAST":           "扣非净利润(上年)",
	"ROEJQ":                      "ROE(加权)",
	"ROEJQ_LAST":                 "ROE(加权)(上年)",
	"XSMLL":                      "销售毛利率",
	"XSMLL_LAST":                 "销售毛利率(上年)",
	"ZCFZL":                      "资产负债率",
	"ZCFZL_LAST":                 "资产负债率(上年)",
	"YYZSRGDHBZC":                "营收环比增长",
	"YYZSRGDHBZC_LAST":           "营收环比增长(上年)",
	"NETPROFITRPHBZC":            "净利润环比增长",
	"NETPROFITRPHBZC_LAST":       "净利润环比增长(上年)",
	"KFJLRGDHBZC":                "扣非净利环比增长",
	"KFJLRGDHBZC_LAST":           "扣非净利环比增长(上年)",
	"TOTALOPERATEREVETZ":         "营收同比增长",
	"TOTALOPERATEREVETZ_LAST":    "营收同比增长(上年)",
	"PARENTNETPROFITTZ":          "净利同比增长",
	"PARENTNETPROFITTZ_LAST":     "净利同比增长(上年)",
	"KCFJCXSYJLRTZ":              "扣非净利同比增长",
	"KCFJCXSYJLRTZ_LAST":         "扣非净利同比增长(上年)",
	"TOTAL_SHARE":                "总股本",
	"FREE_SHARE":                 "流通股",
	"TOTALOPERATEREVE":           "营业总收入",
	"GROSS_PROFIT":               "毛利润",
	"PARENTNETPROFIT":            "归属净利润",
	"DEDU_PARENT_PROFIT":         "扣非净利润",
	"DPNP_YOY_RATIO":             "扣非净利同比增长",
	"ROE_DILUTED":                "ROE(摊薄)",
	"JROA":                       "总资产净利率",
	"NET_PROFIT_RATIO":           "净利率",
	"GROSS_PROFIT_RATIO":         "毛利率",
	"SEASON_LABEL":               "报告期",
	"PUBLISH_DATE":               "发布日期",
	"ORG_NAME_ABBR":              "机构简称",
	"YEAR1":                      "预测年份1",
	"YEAR_MARK1":                 "标识1",
	"EPS1":                       "每股收益预测1",
	"PE1":                        "预测市盈率1",
	"YEAR2":                      "预测年份2",
	"YEAR_MARK2":                 "标识2",
	"EPS2":                       "每股收益预测2",
	"PE2":                        "预测市盈率2",
	"YEAR3":                      "预测年份3",
	"YEAR_MARK3":                 "标识3",
	"EPS3":                       "每股收益预测3",
	"PE3":                        "预测市盈率3",
	"YEAR4":                      "预测年份4",
	"YEAR_MARK4":                 "标识4",
	"EPS4":                       "每股收益预测4",
	"PE4":                        "预测市盈率4",
	"YEAR":                       "年份",
	"YEAR_MARK":                  "标识",
	"EPS":                        "每股收益",
	"EPS_RATIO":                  "EPS增长率",
	"PE":                         "市盈率",
	"ROE":                        "ROE",
	"RANK":                       "排名",
	"PARENT_NETPROFIT_RATIO":     "净利润增长率",
	"TOTAL_OPERATE_INCOME":       "营业总收入",
	"TOTAL_OPERATE_INCOME_RATIO": "营收增长率",
	"OPERATE_PROFIT":             "营业利润",
	"STATISTICS_CYCLE":           "统计周期",
	"INDEX_TYPE":                 "指标类型",
	"PERCENTILE_THIRTY":          "30%分位",
	"PERCENTILE_FIFTY":           "50%分位(中位数)",
	"PERCENTILE_SEVENTY":         "70%分位",
	"MARGIN_BALANCE":             "融资融券余额",
	"MARGIN_BALANCE_RATIO":       "两融余额占比",
	"FIN_BALANCE":                "融资余额",
	"FIN_BALANCE_RATIO":          "融资余额占比",
	"FIN_BUY_AMT":                "融资买入额",
	"FIN_REPAY_AMT":              "融资偿还额",
	"FIN_NETBUY_AMT":             "融资净买入",
	"FIN_TVAL_RATIO":             "融资净买占比",
	"LOAN_BALANCE":               "融券余额",
	"LOAN_BALANCE_RATIO":         "融券余额占比",
	"LOAN_SELL_VOL":              "融券卖出量",
	"LOAN_REPAY_VOL":             "融券偿还量",
	"LOAN_BALANCE_VOL":           "融券余量",
	"TRADE_DATE":                 "交易日期",
	"TRADE_YEAR":                 "交易年份",
	"CHANGE_RATE":                "涨跌幅",
	"CLOSE_PRICE":                "收盘价",
	"DEAL_PRICE":                 "成交价",
	"PREMIUM_RATIO":              "溢价率",
	"DEAL_VOLUME":                "成交量",
	"DEAL_AMT":                   "成交额",
	"BUYER_NAME":                 "买方营业部",
	"SELLER_NAME":                "卖方营业部",
	"BUYER_CODE":                 "买方代码",
	"SELLER_CODE":                "卖方代码",
	"DAILY_RANK":                 "当日排名",
	"TURNOVER_RATE":              "换手率",
	"CHANGE_RATE_1DAYS":          "1日涨跌幅",
	"CHANGE_RATE_5DAYS":          "5日涨跌幅",
	"CHANGE_RATE_10DAYS":         "10日涨跌幅",
	"CHANGE_RATE_20DAYS":         "20日涨跌幅",
	"PREMIUM_TURNOVER":           "溢价成交",
	"DISCOUNT_TURNOVER":          "折价成交",
	"UNLIMITED_A_SHARES":         "无限售A股",
	"TRADE_MARKET_OLD":           "交易市场",
	"INDICATORTYPE":              "指标类型",
	"INDICATOR_VALUE":            "指标值",
	"EXPLANATION":                "说明",
	"TOTAL_BUY":                  "买入额",
	"TOTAL_SELL":                 "卖出额",
	"TOTAL_BUYRIOTOP":            "买入占比",
	"TOTAL_SELLRIOTOP":           "卖出占比",
	"OPERATEDEPT_NAME":           "营业部名称",
	"BUY_AMT_REAL":               "买入额",
	"SELL_AMT_REAL":              "卖出额",
	"BUY_RATIO":                  "买入占比",
	"SELL_RATIO":                 "卖出占比",
	"DISCOUNT_RATIO":             "折价率",
	"ACCUM_AMOUNT":               "累计成交额",
	"ACCUM_VOLUME":               "累计成交量",
}

var f10HiddenFields = map[string]bool{
	"SECUCODE":            true,
	"ORG_CODE":            true,
	"SECURITY_INNER_CODE": true,
	"SECURITY_TYPE_CODE":  true,
	"SECURITY_TYPE_WEB":   true,
	"BUYER_CODE":          true,
	"SELLER_CODE":         true,
	"MAKET_CODE":          true,
	"TRADE_UNIT":          true,
	"TRADE_MARKET_OLD":    true,
	"INDICATORTYPE":       true,
	"INDEX_TYPE":          true,
	"STATISTICS_CYCLE":    true,
	"TRADE_ID":            true,
	"OPERATEDEPT_CODE":    true,
	"TRADE_DIRECTION":     true,
	"STATISTICS_DAYS":     true,
	"CHANGE_TYPE":         true,
	"SECURITY_TYPE":       true,
	"PREMIUM_TURNOVER":    true,
	"DISCOUNT_TURNOVER":   true,
	"UNLIMITED_A_SHARES":  true,
	"DATETYPE":            true,
}

var f10PercentFields = map[string]bool{
	"TURNOVER_RATE": true, "CHANGE_RATE": true,
	"CHANGE_RATE_1DAYS": true, "CHANGE_RATE_5DAYS": true,
	"CHANGE_RATE_10DAYS": true, "CHANGE_RATE_20DAYS": true,
	"PREMIUM_RATIO": true, "DISCOUNT_RATIO": true,
	"ROEJQ": true, "ROEJQ_LAST": true,
	"ROE_DILUTED": true, "JROA": true,
	"XSMLL": true, "XSMLL_LAST": true,
	"NET_PROFIT_RATIO": true, "GROSS_PROFIT_RATIO": true,
	"ZCFZL": true, "ZCFZL_LAST": true,
	"FIN_BALANCE_RATIO": true, "MARGIN_BALANCE_RATIO": true,
	"LOAN_BALANCE_RATIO": true, "FIN_TVAL_RATIO": true,
	"LOAN_SHARE_RATIO": true, "BUY_RATIO": true,
	"SELL_RATIO": true, "BUY_RATIO_TOTAL": true,
	"SELL_RATIO_TOTAL": true, "TOTAL_BUYRIOTOP": true,
	"TOTAL_SELLRIOTOP": true, "EPS_RATIO": true,
	"PARENT_NETPROFIT_RATIO": true, "TOTAL_OPERATE_INCOME_RATIO": true,
	"DPNP_YOY_RATIO": true, "LOAN_TVAL_RATIO": true,
	"FIN_AMOUNT_RATIO": true, "FREE_SHARES_RATIO": true,
	"TOTAL_SHARES_RATIO": true, "FIN_DEGREE": true,
	"LOAN_DEGREE": true, "FINLOAN_DIFF_RATIO": true,
	"YYZSRGDHBZC": true, "YYZSRGDHBZC_LAST": true,
	"NETPROFITRPHBZC": true, "NETPROFITRPHBZC_LAST": true,
	"KFJLRGDHBZC": true, "KFJLRGDHBZC_LAST": true,
	"TOTALOPERATEREVETZ": true, "TOTALOPERATEREVETZ_LAST": true,
	"PARENTNETPROFITTZ": true, "PARENTNETPROFITTZ_LAST": true,
	"KCFJCXSYJLRTZ": true, "KCFJCXSYJLRTZ_LAST": true,
}

var f10MoneyFields = map[string]bool{
	"TOTAL_OPERATEINCOME": true, "TOTAL_OPERATEINCOME_LAST": true,
	"PARENT_NETPROFIT": true, "PARENT_NETPROFIT_LAST": true,
	"KCFJCXSYJLR": true, "KCFJCXSYJLR_LAST": true,
	"TOTALOPERATEREVE": true, "GROSS_PROFIT": true,
	"PARENTNETPROFIT": true, "DEDU_PARENT_PROFIT": true,
	"TOTAL_OPERATE_INCOME": true, "OPERATE_PROFIT": true,
	"DEAL_AMT": true, "TOTAL_BUY": true, "TOTAL_SELL": true,
	"BUY_AMT_REAL": true, "SELL_AMT_REAL": true,
	"ACCUM_AMOUNT": true, "MARGIN_BALANCE": true,
	"FIN_BALANCE": true, "LOAN_BALANCE": true,
	"FIN_BUY_AMT": true, "FIN_REPAY_AMT": true, "FIN_NETBUY_AMT": true,
	"NET_BUY": true,
}

var f10VolumeFields = map[string]bool{
	"TOTAL_SHARE": true, "FREE_SHARE": true,
	"DEAL_VOLUME": true, "UNLIMITED_A_SHARES": true,
	"ACCUM_VOLUME": true, "LOAN_SELL_VOL": true,
	"LOAN_REPAY_VOL": true, "LOAN_BALANCE_VOL": true,
}

var f10PriceFields = map[string]bool{
	"DEAL_PRICE": true, "CLOSE_PRICE": true, "PRE_CLOSE_PRICE": true,
	"CLOSE_FORWARD_ADJPRICE": true, "CLOSE_ADJPRICE": true,
}

var f10DateFields = map[string]bool{
	"REPORT_DATE": true, "TRADE_DATE": true, "PUBLISH_DATE": true,
}

var f10IntegerFields = map[string]bool{
	"YEAR1": true, "YEAR2": true, "YEAR3": true, "YEAR4": true,
	"YEAR": true, "RANK": true, "DAILY_RANK": true,
}

var f10LatestFinanceColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "REPORT_DATE", "REPORT_TYPE",
	"EPSJB", "EPSKCJB", "EPSXS", "EPSJB_PL",
	"BPS", "BPS_PL", "MGZBGJ", "MGWFPLR", "MGJYXJJE",
	"TOTAL_OPERATEINCOME", "TOTAL_OPERATEINCOME_LAST",
	"PARENT_NETPROFIT", "PARENT_NETPROFIT_LAST",
	"KCFJCXSYJLR", "KCFJCXSYJLR_LAST",
	"ROEJQ", "ROEJQ_LAST",
	"XSMLL", "XSMLL_LAST",
	"ZCFZL", "ZCFZL_LAST",
	"TOTALOPERATEREVETZ", "TOTALOPERATEREVETZ_LAST",
	"PARENTNETPROFITTZ", "PARENTNETPROFITTZ_LAST",
	"KCFJCXSYJLRTZ", "KCFJCXSYJLRTZ_LAST",
	"YYZSRGDHBZC", "YYZSRGDHBZC_LAST",
	"NETPROFITRPHBZC", "NETPROFITRPHBZC_LAST",
	"KFJLRGDHBZC", "KFJLRGDHBZC_LAST",
	"TOTAL_SHARE", "FREE_SHARE",
	"FORMERNAME",
}

var f10QtrFinanceColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "REPORT_DATE",
	"EPSJB", "BPS", "PER_CAPITAL_RESERVE", "PER_UNASSIGN_PROFIT", "PER_NETCASH",
	"TOTALOPERATEREVE", "GROSS_PROFIT", "PARENTNETPROFIT", "DEDU_PARENT_PROFIT",
	"TOTALOPERATEREVETZ", "PARENTNETPROFITTZ", "DPNP_YOY_RATIO",
	"YYZSRGDHBZC", "NETPROFITRPHBZC", "KFJLRGDHBZC",
	"ROE_DILUTED", "JROA", "NET_PROFIT_RATIO", "GROSS_PROFIT_RATIO",
}

var f10OrgPredictColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "PUBLISH_DATE", "ORG_NAME_ABBR",
	"YEAR1", "YEAR_MARK1", "EPS1", "PE1",
	"YEAR2", "YEAR_MARK2", "EPS2", "PE2",
	"YEAR3", "YEAR_MARK3", "EPS3", "PE3",
	"YEAR4", "YEAR_MARK4", "EPS4", "PE4",
}

var f10PredictSummaryColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR",
	"YEAR", "YEAR_MARK", "EPS", "EPS_RATIO", "PE", "RANK",
}

var f10MarginColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE",
	"FIN_BUY_AMT", "FIN_REPAY_AMT", "FIN_BALANCE",
	"LOAN_SELL_VOL", "LOAN_REPAY_VOL", "LOAN_BALANCE",
}

var f10BlockTradeColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE",
	"DEAL_PRICE", "PREMIUM_RATIO", "DEAL_VOLUME", "DEAL_AMT",
	"BUYER_NAME", "SELLER_NAME", "DAILY_RANK",
	"CLOSE_PRICE", "TURNOVER_RATE", "CHANGE_RATE",
	"CHANGE_RATE_1DAYS", "CHANGE_RATE_5DAYS",
}

var f10BillboardColOrder = []string{
	"SECURITY_CODE", "TRADE_DATE", "EXPLANATION",
	"TOTAL_BUY", "TOTAL_SELL", "TOTAL_BUYRIOTOP", "TOTAL_SELLRIOTOP",
}

var f10OperDeptColOrder = []string{
	"TRADE_DATE", "EXPLANATION", "OPERATEDEPT_NAME",
	"BUY_AMT_REAL", "BUY_RATIO", "SELL_AMT_REAL", "SELL_RATIO",
}

var f10ValuationColOrder = []string{
	"PERCENTILE_THIRTY", "PERCENTILE_FIFTY", "PERCENTILE_SEVENTY",
}

var f10HolderTrendColOrder = []string{
	"SECURITY_CODE", "SECURITY_NAME_ABBR", "TRADE_DATE", "INDICATOR_VALUE",
}

func f10FieldNameCN(name string) string {
	if cn, ok := f10FieldCN[name]; ok {
		return cn
	}
	return name
}

func f10FormatValue(key string, v any) string {
	if v == nil {
		return "-"
	}
	switch val := v.(type) {
	case float64:
		return f10FormatNumber(key, val)
	case string:
		return f10FormatString(key, val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func f10FormatMoney(val float64) string {
	abs := math.Abs(val)
	if abs >= 1e8 {
		return fmt.Sprintf("%.2f亿", val/1e8)
	}
	if abs >= 1e4 {
		return fmt.Sprintf("%.2f万", val/1e4)
	}
	return fmt.Sprintf("%.2f", val)
}

func f10FormatVolume(val float64) string {
	abs := math.Abs(val)
	if abs >= 1e8 {
		return fmt.Sprintf("%.2f亿股", val/1e8)
	}
	if abs >= 1e4 {
		return fmt.Sprintf("%.2f万股", val/1e4)
	}
	if val == float64(int64(val)) {
		return strconv.FormatInt(int64(val), 10) + "股"
	}
	return fmt.Sprintf("%.0f股", val)
}

func f10FormatNumber(key string, val float64) string {
	if f10IntegerFields[key] {
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return fmt.Sprintf("%.0f", val)
	}
	if f10PercentFields[key] {
		return fmt.Sprintf("%.2f%%", val)
	}
	if f10PriceFields[key] {
		return fmt.Sprintf("%.2f", val)
	}
	if f10MoneyFields[key] {
		return f10FormatMoney(val)
	}
	if f10VolumeFields[key] {
		return f10FormatVolume(val)
	}
	switch key {
	case "EPSJB", "EPSKCJB", "EPSXS", "EPSJB_PL", "BPS", "BPS_PL",
		"MGZBGJ", "MGWFPLR", "MGJYXJJE",
		"PER_CAPITAL_RESERVE", "PER_UNASSIGN_PROFIT", "PER_NETCASH",
		"EPS1", "EPS2", "EPS3", "EPS4", "EPS",
		"PERCENTILE_THIRTY", "PERCENTILE_FIFTY", "PERCENTILE_SEVENTY":
		return fmt.Sprintf("%.2f", val)
	case "PE1", "PE2", "PE3", "PE4", "PE", "ROE":
		return fmt.Sprintf("%.2f", val)
	default:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return fmt.Sprintf("%.2f", val)
	}
}

func f10FormatString(key, val string) string {
	if f10DateFields[key] {
		return strings.Split(val, " ")[0]
	}
	return val
}

func f10SortedCols(data []map[string]any, ordered []string) []string {
	existing := make(map[string]bool)
	for _, row := range data {
		for k := range row {
			existing[k] = true
		}
	}
	orderedSet := make(map[string]bool)
	result := make([]string, 0, len(existing))
	for _, col := range ordered {
		if existing[col] && !f10HiddenFields[col] {
			result = append(result, col)
			orderedSet[col] = true
		}
	}
	for k := range existing {
		if !orderedSet[k] && !f10HiddenFields[k] {
			result = append(result, k)
		}
	}
	return result
}

func f10GenericToMarkdown(title string, resp *F10GenericResp) string {
	return f10GenericToMarkdownOrdered(title, resp, nil)
}

func f10GenericToMarkdownOrdered(title string, resp *F10GenericResp, colOrder []string) string {
	if resp == nil || resp.Result == nil || len(resp.Result.Data) == 0 {
		return fmt.Sprintf("## %s\n\n暂无数据", title)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))

	data := resp.Result.Data
	if len(data) == 0 {
		sb.WriteString("暂无数据\n")
		return sb.String()
	}

	var cols []string
	if len(colOrder) > 0 {
		cols = f10SortedCols(data, colOrder)
	} else {
		colSet := make(map[string]bool)
		for _, row := range data {
			for k := range row {
				if !colSet[k] && !f10HiddenFields[k] {
					colSet[k] = true
					cols = append(cols, k)
				}
			}
		}
	}

	if len(data) == 1 {
		sb.WriteString("| 指标 | 数值 |\n| --- | --- |\n")
		row := data[0]
		for _, c := range cols {
			v := row[c]
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", f10FieldNameCN(c), f10FormatValue(c, v)))
		}
	} else {
		sb.WriteString("| ")
		for _, c := range cols {
			sb.WriteString(f10FieldNameCN(c) + " | ")
		}
		sb.WriteString("\n| ")
		for range cols {
			sb.WriteString("--- | ")
		}
		sb.WriteString("\n")
		for _, row := range data {
			sb.WriteString("| ")
			for _, c := range cols {
				v := row[c]
				sb.WriteString(f10FormatValue(c, v) + " | ")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (receiver StockDataApi) GetStockLatestFinance(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_PCF10_FINANCEMAINFINADATA&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CREPORT_DATE%2CREPORT_TYPE%2CEPSJB%2CEPSKCJB%2CEPSXS%2CBPS%2CMGZBGJ%2CMGWFPLR%2CMGJYXJJE%2CTOTAL_OPERATEINCOME%2CTOTAL_OPERATEINCOME_LAST%2CPARENT_NETPROFIT%2CPARENT_NETPROFIT_LAST%2CKCFJCXSYJLR%2CKCFJCXSYJLR_LAST%2CROEJQ%2CROEJQ_LAST%2CXSMLL%2CXSMLL_LAST%2CZCFZL%2CZCFZL_LAST%2CYYZSRGDHBZC_LAST%2CYYZSRGDHBZC%2CNETPROFITRPHBZC%2CNETPROFITRPHBZC_LAST%2CKFJLRGDHBZC%2CKFJLRGDHBZC_LAST%2CTOTALOPERATEREVETZ%2CTOTALOPERATEREVETZ_LAST%2CPARENTNETPROFITTZ%2CPARENTNETPROFITTZ_LAST%2CKCFJCXSYJLRTZ%2CKCFJCXSYJLRTZ_LAST%2CTOTAL_SHARE%2CFREE_SHARE%2CEPSJB_PL%2CBPS_PL%2CFORMERNAME&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&sortTypes=-1&sortColumns=REPORT_DATE&pageNumber=1&pageSize=1&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockQtrMainFinance(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_F10_QTR_MAINFINADATA&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CORG_CODE%2CREPORT_DATE%2CEPSJB%2CBPS%2CPER_CAPITAL_RESERVE%2CPER_UNASSIGN_PROFIT%2CPER_NETCASH%2CTOTALOPERATEREVE%2CGROSS_PROFIT%2CPARENTNETPROFIT%2CDEDU_PARENT_PROFIT%2CTOTALOPERATEREVETZ%2CPARENTNETPROFITTZ%2CDPNP_YOY_RATIO%2CYYZSRGDHBZC%2CNETPROFITRPHBZC%2CKFJLRGDHBZC%2CROE_DILUTED%2CJROA%2CNET_PROFIT_RATIO%2CGROSS_PROFIT_RATIO&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=9&sortTypes=-1&sortColumns=REPORT_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockOrgPredict(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_HSF10_RES_ORGPREDICT&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CPUBLISH_DATE%2CORG_CODE%2CORG_NAME_ABBR%2CYEAR1%2CYEAR_MARK1%2CEPS1%2CPE1%2CYEAR2%2CYEAR_MARK2%2CEPS2%2CPE2%2CYEAR3%2CYEAR_MARK3%2CEPS3%2CPE3%2CYEAR4%2CYEAR_MARK4%2CEPS4%2CPE4&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=200&sortTypes=&sortColumns=&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockPredictSummary(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_HSF10_RESPREDICT_STATISTICS&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CYEAR%2CYEAR_MARK%2CEPS%2CEPS_RATIO%2CPE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=200&sortTypes=1&sortColumns=RANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockValuationPercentile(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_STOCKVALUATIONTANTILE&columns=SECUCODE%2CSTATISTICS_CYCLE%2CINDEX_TYPE%2CPERCENTILE_THIRTY%2CPERCENTILE_FIFTY%2CPERCENTILE_SEVENTY&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(INDEX_TYPE%3D%221%22)(STATISTICS_CYCLE%3D%223%22)&pageNumber=1&pageSize=&sortTypes=&sortColumns=&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockMarginTrading(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_MARGIN_STATISTICS_STOCKS&columns=SECUCODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CTRADE_DATE%2CFIN_BUY_AMT%2CFIN_REPAY_AMT%2CFIN_BALANCE%2CLOAN_SELL_VOL%2CLOAN_REPAY_VOL%2CLOAN_BALANCE&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=10&sortTypes=-1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockBlockTrade(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_DATA_BLOCKTRADE&columns=SECUCODE%2CSECURITY_INNER_CODE%2CSECURITY_CODE%2CSECURITY_NAME_ABBR%2CSECURITY_TYPE%2CSECURITY_TYPE_WEB%2CTRADE_DATE%2CDEAL_PRICE%2CPREMIUM_RATIO%2CDEAL_VOLUME%2CDEAL_AMT%2CBUYER_NAME%2CSELLER_NAME%2CDAILY_RANK%2CCLOSE_PRICE%2CTRADE_UNIT%2CTURNOVER_RATE%2CCHANGE_RATE%2CCHANGE_RATE_1DAYS%2CCHANGE_RATE_5DAYS%2CBUYER_CODE%2CSELLER_CODE%2CPREMIUM_TURNOVER%2CDISCOUNT_TURNOVER%2CUNLIMITED_A_SHARES%2CTRADE_MARKET_OLD&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=10&sortTypes=-1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockHolderTrend(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_CUSTOM_DMSK_TREND&columns=ALL&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(INDICATORTYPE%3D1)(DATETYPE%3D3)&pageNumber=1&pageSize=&sortTypes=1&sortColumns=TRADE_DATE&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockBillboard(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_BILLBOARD_DAILYDETAILS&columns=SECURITY_CODE%2CSECUCODE%2CTRADE_DATE%2CEXPLANATION%2CTOTAL_BUY%2CTOTAL_SELL%2CTOTAL_BUYRIOTOP%2CTOTAL_SELLRIOTOP&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)&pageNumber=1&pageSize=5&sortTypes=-1%2C-1&sortColumns=TRADE_DATE%2CEXPLANATION&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockOperationDeptTrade(stockCode string) (*F10GenericResp, error) {
	stockCode = normalizeF10Code(stockCode)
	url := emF10BaseURL + "?reportName=RPT_OPERATEDEPT_TRADE&columns=TRADE_DATE%2CEXPLANATION%2COPERATEDEPT_NAME%2CBUY_AMT_REAL%2CBUY_RATIO%2CSELL_AMT_REAL%2CSELL_RATIO&quoteColumns=&filter=(SECUCODE%3D%22" + stockCode + "%22)(TRADE_DIRECTION%3D%220%22)&pageNumber=1&pageSize=15&sortTypes=-1%2C-1%2C1&sortColumns=TRADE_DATE%2CEXPLANATION%2CRANK&source=HSF10&client=PC&v=" + convertor.ToString(time.Now().Unix())
	var data F10GenericResp
	err := receiver.f10Request(url, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func (receiver StockDataApi) GetStockLatestFinanceToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockLatestFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取最新财务数据失败: %v", err)
		return fmt.Sprintf("获取最新财务数据失败: %v", err)
	}
	name := ""
	if len(resp.Result.Data) > 0 {
		if n, ok := resp.Result.Data[0]["SECURITY_NAME_ABBR"].(string); ok {
			name = n
		}
	}
	return f10GenericToMarkdownOrdered(name+" 最新财务主要数据", resp, f10LatestFinanceColOrder)
}

// HKF10MainIndicatorResp 东方财富港股F10主要指标接口响应。
// 数据结构为二维数组（rows × cols），每行第一项为日期/分组名，其余为指标值。
// 接口示例：http://emweb.securities.eastmoney.com/PC_HKF10/NewFinancialAnalysis/GetZYZB?code=00700
// 返回 data 含两个键：zyzb_an（年度，仅年报）和 zyzb_abgq（报告期，全部季度）。
// 每个二维数组首行为列头（含 "每股指标"/"成长能力指标"/"盈利能力指标"/"盈利质量指标"/"财务风险指标" 等分组列），
// 数据行的对应列重复出现同一日期，渲染时需折叠为单一日期列。
type HKF10MainIndicatorResp struct {
	Status int               `json:"status"`
	Msg    string            `json:"msg"`
	Data   *HKF10MainIndData `json:"data"`
}

type HKF10MainIndData struct {
	Abgq [][]string `json:"zyzb_abgq"` // 报告期（全部季度，最新在前）
	An   [][]string `json:"zyzb_an"`   // 年度（仅年报，最新在前）
}

// normalizeHKF10Code 港股代码归一化为 EastMoney HKF10 接口所需的纯数字 code（如 00700）。
// 支持 00700.HK / hk00700 / HK00700 / 00700 等格式。
func normalizeHKF10Code(stockCode string) string {
	code := strings.TrimSpace(stockCode)
	upper := strings.ToUpper(code)
	// 去 .HK 后缀（不区分大小写）
	if strings.HasSuffix(upper, ".HK") {
		code = code[:len(code)-3]
		upper = strings.ToUpper(code)
	}
	// 去 HK 前缀（不区分大小写）
	if strings.HasPrefix(upper, "HK") {
		code = code[2:]
	}
	// 仅保留数字，再左侧补零到 5 位
	code = RemoveAllNonDigitChar(code)
	if len(code) < 5 {
		code = strings.Repeat("0", 5-len(code)) + code
	}
	return code
}

// GetHKStockLatestFinance 获取港股最新财务主要指标（东方财富 HKF10 接口）。
// 返回报告期数据（zyzb_abgq），最新季度排在最前；若需要年度数据请使用 GetHKStockAnnualFinance。
// 仅适用于港股（.HK 后缀，如 00700.HK 腾讯控股）。A股请使用 GetStockLatestFinance。
func (receiver StockDataApi) GetHKStockLatestFinance(stockCode string) (*HKF10MainIndicatorResp, error) {
	code := normalizeHKF10Code(stockCode)
	url := "http://emweb.securities.eastmoney.com/PC_HKF10/NewFinancialAnalysis/GetZYZB?code=" + code
	resp, err := receiver.client.SetTimeout(time.Duration(receiver.config.CrawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://emweb.securities.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").
		Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	var data HKF10MainIndicatorResp
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return nil, fmt.Errorf("parse failed: %v", err)
	}
	if data.Status != 1 {
		return &data, fmt.Errorf("获取港股财务数据失败: %s", data.Msg)
	}
	return &data, nil
}

// GetHKStockLatestFinanceToMarkdown 港股最新财务主要指标渲染为 Markdown 表格。
// 输出最新一期与去年同期两列，便于直观对比同比变化。
func (receiver StockDataApi) GetHKStockLatestFinanceToMarkdown(stockCode string) string {
	code := normalizeHKF10Code(stockCode)
	resp, err := receiver.GetHKStockLatestFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取港股最新财务数据失败(code=%s): %v", code, err)
		return fmt.Sprintf("获取港股最新财务数据失败: %v", err)
	}
	return hkF10MainIndicatorToMarkdown(stockCode+" 港股最新财务主要指标", resp, true)
}

// GetHKStockAnnualFinanceToMarkdown 港股年度财务主要指标渲染为 Markdown 表格。
// 输出最近 5 个年度数据，便于查看长期趋势。
func (receiver StockDataApi) GetHKStockAnnualFinanceToMarkdown(stockCode string) string {
	code := normalizeHKF10Code(stockCode)
	resp, err := receiver.GetHKStockLatestFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取港股年度财务数据失败(code=%s): %v", code, err)
		return fmt.Sprintf("获取港股年度财务数据失败: %v", err)
	}
	return hkF10MainIndicatorToMarkdown(stockCode+" 港股年度财务主要指标", resp, false)
}

// hkF10IndicatorRow HK F10 指标表的一行（指标名 + 各期数值）。
type hkF10IndicatorRow struct {
	Name string
	Vals []string
}

// hkF10MainIndicatorToMarkdown 将 HK F10 主要指标二维数组渲染为 Markdown 表格。
// useLatest=true 时使用 zyzb_abgq 最新 2 期（最新 + 去年同期），便于同比对比；
// useLatest=false 时使用 zyzb_an 最近 5 个年度，便于长期趋势分析。
//
// 数据结构说明：HK F10 接口返回的二维数组首行为列头，含 5 个分组标记列：
//
//	位置 0  "每股指标"      -> 后续 6 列：基本/稀释/TTM EPS、每股净资产/经营现金流/营业收入
//	位置 7  "成长能力指标"   -> 后续 8 列：营收/毛利/归母净利、3 项同比、3 项滚动环比
//	位置 17 "盈利能力指标"   -> 后续 6 列：平均/年化 ROE、总资产净利率、毛利率、净利率、投资回报率
//	位置 24 "盈利质量指标"   -> 后续 2 列：所得税/利润总额、经营现金流/营业收入
//	位置 27 "财务风险指标"   -> 后续 3 列：资产负债率、流动负债/总负债、流动比率
//
// 数据行中这 5 个位置均为同一日期（如 "25-12-31"），渲染时折叠为单一日期列。
func hkF10MainIndicatorToMarkdown(title string, resp *HKF10MainIndicatorResp, useLatest bool) string {
	if resp == nil || resp.Data == nil {
		return fmt.Sprintf("## %s\n\n暂无数据", title)
	}
	var rows [][]string
	if useLatest {
		rows = resp.Data.Abgq
	} else {
		rows = resp.Data.An
	}
	if len(rows) < 2 {
		return fmt.Sprintf("## %s\n\n暂无数据", title)
	}

	// 选择要展示的列：最新一期；如为报告期模式则再选去年同期对比
	headerRow := rows[0]
	dataRows := rows[1:]
	// 数据按日期降序排列（最新在前），选取最新 N 期
	maxCols := 5
	if useLatest {
		maxCols = 2
	}
	if len(dataRows) > maxCols {
		dataRows = dataRows[:maxCols]
	}

	// 5 个分组标记列的索引
	groupMarkIdx := map[int]bool{0: true, 7: true, 17: true, 24: true, 27: true}
	// 构建指标行：跳过分组标记列，将其作为分组标题行插入
	var indicatorRows []hkF10IndicatorRow
	currentGroup := ""
	for colIdx := 0; colIdx < len(headerRow); colIdx++ {
		hdr := headerRow[colIdx]
		if groupMarkIdx[colIdx] {
			currentGroup = hdr
			// 第一列 "每股指标" 分组名特殊处理：保持为 "基本指标"
			if currentGroup == "每股指标" {
				currentGroup = "基本指标"
			}
			continue
		}
		row := hkF10IndicatorRow{
			Name: fmt.Sprintf("%s · %s", currentGroup, hdr),
		}
		for _, dr := range dataRows {
			if colIdx < len(dr) {
				row.Vals = append(row.Vals, dr[colIdx])
			} else {
				row.Vals = append(row.Vals, "--")
			}
		}
		indicatorRows = append(indicatorRows, row)
	}

	// 收集日期作为表头（取自每行的第 0 列，即 "每股指标" 分组下对应的日期）
	var dateHeaders []string
	for _, dr := range dataRows {
		if len(dr) > 0 {
			dateHeaders = append(dateHeaders, dr[0])
		} else {
			dateHeaders = append(dateHeaders, "--")
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))
	sb.WriteString("| 指标 |")
	for _, d := range dateHeaders {
		sb.WriteString(" " + d + " |")
	}
	sb.WriteString("\n| --- |")
	for range dateHeaders {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	for _, r := range indicatorRows {
		sb.WriteString(fmt.Sprintf("| %s |", r.Name))
		for _, v := range r.Vals {
			sb.WriteString(" " + v + " |")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (receiver StockDataApi) GetStockQtrMainFinanceToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockQtrMainFinance(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取季度财务指标失败: %v", err)
		return fmt.Sprintf("获取季度财务指标失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("季度主要财务指标", resp, f10QtrFinanceColOrder)
}

func (receiver StockDataApi) GetStockOrgPredictToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockOrgPredict(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取机构预测失败: %v", err)
		return fmt.Sprintf("获取机构预测失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("机构预测明细", resp, f10OrgPredictColOrder)
}

func (receiver StockDataApi) GetStockPredictSummaryToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockPredictSummary(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取机构预测汇总失败: %v", err)
		return fmt.Sprintf("获取机构预测汇总失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("机构预测汇总", resp, f10PredictSummaryColOrder)
}

func (receiver StockDataApi) GetStockValuationPercentileToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockValuationPercentile(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取估值百分位失败: %v", err)
		return fmt.Sprintf("获取估值百分位失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("估值百分位", resp, f10ValuationColOrder)
}

func (receiver StockDataApi) GetStockMarginTradingToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockMarginTrading(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取融资融券数据失败: %v", err)
		return fmt.Sprintf("获取融资融券数据失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("融资融券数据", resp, f10MarginColOrder)
}

func (receiver StockDataApi) GetStockBlockTradeToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockBlockTrade(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取大宗交易数据失败: %v", err)
		return fmt.Sprintf("获取大宗交易数据失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("大宗交易数据", resp, f10BlockTradeColOrder)
}

func (receiver StockDataApi) GetStockHolderTrendToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockHolderTrend(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取户均持股趋势失败: %v", err)
		return fmt.Sprintf("获取户均持股趋势失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("户均持股趋势", resp, f10HolderTrendColOrder)
}

func (receiver StockDataApi) GetStockBillboardToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockBillboard(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取龙虎榜数据失败: %v", err)
		return fmt.Sprintf("获取龙虎榜数据失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("龙虎榜数据", resp, f10BillboardColOrder)
}

func (receiver StockDataApi) GetStockOperationDeptTradeToMarkdown(stockCode string) string {
	resp, err := receiver.GetStockOperationDeptTrade(stockCode)
	if err != nil {
		logger.SugaredLogger.Errorf("获取营业部买卖明细失败: %v", err)
		return fmt.Sprintf("获取营业部买卖明细失败: %v", err)
	}
	return f10GenericToMarkdownOrdered("营业部买卖明细", resp, f10OperDeptColOrder)
}
