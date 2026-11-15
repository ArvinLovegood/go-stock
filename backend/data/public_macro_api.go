package data

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	worldBankURL = "https://api.worldbank.org/v2"
	treasuryURL  = "https://api.fiscaldata.treasury.gov/services/api/fiscal_service/v1/accounting/od/rates_of_exchange"
)

type PublicMacroAPI struct {
	client       *resty.Client
	worldBankURL string
	treasuryURL  string
}

func NewPublicMacroAPI() *PublicMacroAPI {
	return &PublicMacroAPI{
		client:       CreateGlobalProxiedHTTPClient(20 * time.Second),
		worldBankURL: worldBankURL,
		treasuryURL:  treasuryURL,
	}
}

type worldBankIndicator struct {
	code string
	name string
}

var worldBankIndicators = map[string]worldBankIndicator{
	"gdp_growth":   {"NY.GDP.MKTP.KD.ZG", "GDP实际增长率"},
	"inflation":    {"FP.CPI.TOTL.ZG", "消费者物价通胀率"},
	"unemployment": {"SL.UEM.TOTL.ZS", "失业率"},
}

var treasuryCurrencies = map[string]string{
	"CNY": "China-Renminbi",
	"HKD": "Hong Kong-Dollar",
	"JPY": "Japan-Yen",
	"EUR": "Euro Zone-Euro",
	"GBP": "United Kingdom-Pound",
}

func publicMacroLimit(limit int) (int, error) {
	if limit == 0 {
		return 5, nil
	}
	if limit < 1 || limit > 30 {
		return 0, fmt.Errorf("记录数必须在 1 到 30 之间")
	}
	return limit, nil
}

func (api *PublicMacroAPI) get(rawURL string, query map[string]string) ([]byte, error) {
	resp, err := api.client.R().SetQueryParams(query).Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("请求公开数据失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("公开数据服务返回 HTTP %d", resp.StatusCode())
	}
	return resp.Body(), nil
}

// WorldBankIndicator 返回世界银行年度指标。国家使用 ISO 3166-1 alpha-3 代码。
func (api *PublicMacroAPI) WorldBankIndicator(country, indicator string, limit int) (string, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if len(country) != 3 {
		return "", fmt.Errorf("国家代码必须为三个英文字母，例如 CHN")
	}
	for _, c := range country {
		if c < 'A' || c > 'Z' {
			return "", fmt.Errorf("国家代码必须为三个英文字母，例如 CHN")
		}
	}
	ind, ok := worldBankIndicators[indicator]
	if !ok {
		return "", fmt.Errorf("不支持的世界银行指标 %q；支持 gdp_growth、inflation、unemployment", indicator)
	}
	limit, err := publicMacroLimit(limit)
	if err != nil {
		return "", err
	}
	body, err := api.get(api.worldBankURL+"/country/"+country+"/indicator/"+url.PathEscape(ind.code),
		map[string]string{"format": "json", "per_page": strconv.Itoa(limit)})
	if err != nil {
		return "", err
	}
	var envelope []json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope) != 2 {
		return "", fmt.Errorf("世界银行响应格式无效")
	}
	var meta struct {
		LastUpdated string `json:"lastupdated"`
	}
	var rows []struct {
		Date    string   `json:"date"`
		Value   *float64 `json:"value"`
		Country struct {
			Name string `json:"value"`
		} `json:"country"`
	}
	if err := json.Unmarshal(envelope[0], &meta); err != nil {
		return "", fmt.Errorf("解析世界银行元数据失败: %w", err)
	}
	if err := json.Unmarshal(envelope[1], &rows); err != nil {
		return "", fmt.Errorf("解析世界银行指标失败: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("世界银行未返回 %s 的%s数据", country, ind.name)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "## 世界银行：%s（%s，年度，%%）\n\n数据集更新日期：%s；数值可能修订，不代表历史时点可得值。\n\n| 年份 | 数值(%%) |\n|---|---:|\n", ind.name, country, meta.LastUpdated)
	for _, row := range rows {
		value := "暂无数据"
		if row.Value != nil {
			value = strconv.FormatFloat(*row.Value, 'f', 4, 64)
		}
		fmt.Fprintf(&out, "| %s | %s |\n", row.Date, value)
	}
	fmt.Fprintf(&out, "\n来源：https://data.worldbank.org/indicator/%s?locations=%s\n", ind.code, country)
	return out.String(), nil
}

// TreasuryExchangeRates 返回美国财政部报告汇率，不是实时市场汇率。
func (api *PublicMacroAPI) TreasuryExchangeRates(currency string, limit int) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	description, ok := treasuryCurrencies[currency]
	if !ok {
		return "", fmt.Errorf("不支持的币种 %q；支持 CNY、HKD、JPY、EUR、GBP", currency)
	}
	limit, err := publicMacroLimit(limit)
	if err != nil {
		return "", err
	}
	body, err := api.get(api.treasuryURL, map[string]string{
		"fields":     "record_date,country_currency_desc,exchange_rate",
		"filter":     "country_currency_desc:eq:" + description,
		"sort":       "-record_date",
		"page[size]": strconv.Itoa(limit),
	})
	if err != nil {
		return "", err
	}
	var result struct {
		Data []struct {
			Date string `json:"record_date"`
			Rate string `json:"exchange_rate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析美国财政部汇率失败: %w", err)
	}
	if len(result.Data) == 0 {
		return "", fmt.Errorf("美国财政部未返回 %s 汇率数据", currency)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "## 美国财政部报告汇率：USD/%s\n\n季度报告汇率，1 美元折算为对应币种的单位数；不是实时交易汇率。\n\n| 记录日期 | 每美元折算 %s |\n|---|---:|\n", currency, currency)
	for _, row := range result.Data {
		if _, err := strconv.ParseFloat(row.Rate, 64); err != nil {
			return "", fmt.Errorf("美国财政部返回无效汇率 %q", row.Rate)
		}
		fmt.Fprintf(&out, "| %s | %s |\n", row.Date, row.Rate)
	}
	out.WriteString("\n来源：https://fiscaldata.treasury.gov/datasets/treasury-reporting-rates-exchange/\n")
	return out.String(), nil
}
