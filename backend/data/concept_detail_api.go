package data

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/duke-git/lancet/v2/convertor"
)

// @Author spark
// @Date 2026/8/6
// @Desc 同花顺概念详情页数据抓取
// -----------------------------------------------------------------------------------

// ConceptDetail 获取同花顺概念详情页数据
// conceptCode 为概念代码（URL 中的 code，如 309269）
// 数据来源：https://q.10jqka.com.cn/gn/detail/code/{conceptCode}/ （GBK 编码的 HTML 页面）
func (m MarketNewsApi) ConceptDetail(conceptCode string) *models.ConceptDetailInfo {
	info := &models.ConceptDetailInfo{ConceptCode: conceptCode}
	if conceptCode == "" {
		return info
	}
	url := fmt.Sprintf("https://q.10jqka.com.cn/gn/detail/code/%s/", conceptCode)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "q.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptDetail request err: %s", err.Error())
		return info
	}

	// 页面是 GBK 编码，转为 UTF8
	htmlContent := GB18030ToUTF8(resp.Body())
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptDetail goquery err: %s", err.Error())
		return info
	}

	// 1. 提取板块代码（plateCode）— 优先从隐藏域 <input id="clid" value="886112">
	if clid := doc.Find("input#clid").AttrOr("value", ""); clid != "" {
		info.PlateCode = strings.TrimSpace(clid)
	} else {
		// 兜底：从 <h3>概念名<span>886112</span></h3> 中提取
		doc.Find(".board-hq h3 span").First().Each(func(_ int, s *goquery.Selection) {
			info.PlateCode = strings.TrimSpace(s.Text())
		})
	}

	// 2. 提取概念名称 — 从 <h3>MLCC概念<span>886112</span></h3>
	doc.Find(".board-hq h3").First().Each(func(_ int, s *goquery.Selection) {
		// 克隆并移除 span 子节点，取剩余文本作为名称
		clone := s.Clone()
		clone.Find("span").Remove()
		info.Name = strings.TrimSpace(clone.Text())
	})
	if info.Name == "" {
		// 兜底：从标题中提取
		if title := doc.Find("title").Text(); title != "" {
			info.Name = strings.TrimSpace(strings.Split(title, "_")[0])
		}
	}

	// 3. 提取板块定义 — <div class="board-txt board-aside"><h4>定义</h4><p>...</p></div>
	doc.Find(".board-txt.board-aside p").First().Each(func(_ int, s *goquery.Selection) {
		info.Definition = strings.TrimSpace(s.Text())
	})

	// 4. 提取板块行情数据 — <div class="board-infos"> 下的 <dl><dt>标签</dt><dd>值</dd></dl>
	marketMap := map[string]string{}
	doc.Find(".board-infos dl").Each(func(_ int, s *goquery.Selection) {
		dt := strings.TrimSpace(s.Find("dt").Text())
		dd := strings.TrimSpace(s.Find("dd").Text())
		// 涨跌家数 dd 含两个 span（涨家数 跌家数），合并为 "涨/跌"
		if dt == "涨跌家数" {
			parts := []string{}
			s.Find("dd span").Each(func(_ int, sp *goquery.Selection) {
				t := strings.TrimSpace(sp.Text())
				if t != "" {
					parts = append(parts, t)
				}
			})
			if len(parts) >= 2 {
				dd = parts[0] + "/" + parts[1]
			}
		}
		if dt != "" {
			marketMap[dt] = dd
		}
	})
	info.Market = models.ConceptMarket{
		Open:          cleanMarketValue(marketMap["今开"]),
		PreClose:      cleanMarketValue(marketMap["昨收"]),
		Low:           cleanMarketValue(marketMap["最低"]),
		High:          cleanMarketValue(marketMap["最高"]),
		Volume:        cleanMarketValue(marketMap["成交量(万手)"]),
		ChangePercent: cleanMarketValue(marketMap["板块涨幅"]),
		ChangeRank:    cleanMarketValue(marketMap["涨幅排名"]),
		UpDownCount:   cleanMarketValue(marketMap["涨跌家数"]),
		NetInflow:     cleanMarketValue(marketMap["资金净流入(亿)"]),
		DealAmount:    cleanMarketValue(marketMap["成交额(亿)"]),
	}

	// 5. 解析成分股表格 — <table class="m-table"><thead>...<tbody>
	doc.Find("table.m-table tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 14 {
			return
		}
		// td[1]=代码（在 <a> 内）, td[2]=名称（在 <a> 内）, td[3..13]=数据列
		code := strings.TrimSpace(row.Find("td").Eq(1).Find("a").Text())
		if code == "" {
			code = strings.TrimSpace(row.Find("td").Eq(1).Text())
		}
		name := strings.TrimSpace(row.Find("td").Eq(2).Find("a").Text())
		if name == "" {
			name = strings.TrimSpace(row.Find("td").Eq(2).Text())
		}
		if code == "" {
			return
		}
		stock := models.ConceptStock{
			Code:          code,
			Name:          name,
			Price:         strings.TrimSpace(cells.Eq(3).Text()),
			ChangePercent: strings.TrimSpace(cells.Eq(4).Text()),
			Change:        strings.TrimSpace(cells.Eq(5).Text()),
			Speed:         strings.TrimSpace(cells.Eq(6).Text()),
			Turnover:      strings.TrimSpace(cells.Eq(7).Text()),
			VolumeRatio:   strings.TrimSpace(cells.Eq(8).Text()),
			Amplitude:     strings.TrimSpace(cells.Eq(9).Text()),
			DealAmount:    strings.TrimSpace(cells.Eq(10).Text()),
			FlowShares:    strings.TrimSpace(cells.Eq(11).Text()),
			FlowMarketCap: strings.TrimSpace(cells.Eq(12).Text()),
			PERatio:       strings.TrimSpace(cells.Eq(13).Text()),
		}
		info.Stocks = append(info.Stocks, stock)
	})

	return info
}

// ConceptStocks 获取同花顺概念成分股列表（支持分页）
// conceptCode 为概念代码（如 307550），page 为页码（从 1 开始）
// 数据来源：https://q.10jqka.com.cn/gn/detail/field/199112/order/desc/page/{page}/ajax/1/code/{conceptCode}
// 注意：AJAX 接口可能被 chameleon 反爬拦截，此时第 1 页回退到 ConceptDetail 的 HTML 解析结果
func (m MarketNewsApi) ConceptStocks(conceptCode string, page int) []models.ConceptStock {
	if conceptCode == "" || page < 1 {
		return nil
	}
	url := fmt.Sprintf("https://q.10jqka.com.cn/gn/detail/field/199112/order/desc/page/%d/ajax/1/code/%s", page, conceptCode)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "q.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptStocks request err: %s", err.Error())
		return nil
	}
	htmlContent := GB18030ToUTF8(resp.Body())
	// 检测 chameleon 反爬重定向（响应体很小且包含 window.location）
	if len(htmlContent) < 1000 && strings.Contains(htmlContent, "chameleon") {
		// AJAX 被反爬拦截，第 1 页回退到 ConceptDetail
		if page == 1 {
			if info := m.ConceptDetail(conceptCode); info != nil && len(info.Stocks) > 0 {
				return info.Stocks
			}
		}
		return nil
	}
	return parseConceptStocksHTML(htmlContent)
}

// parseConceptStocksHTML 解析成分股 HTML 表格片段（AJAX 返回）或完整页面
func parseConceptStocksHTML(htmlContent string) []models.ConceptStock {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("parseConceptStocksHTML goquery err: %s", err.Error())
		return nil
	}
	var stocks []models.ConceptStock
	doc.Find("table.m-table tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 14 {
			return
		}
		code := strings.TrimSpace(row.Find("td").Eq(1).Find("a").Text())
		if code == "" {
			code = strings.TrimSpace(row.Find("td").Eq(1).Text())
		}
		name := strings.TrimSpace(row.Find("td").Eq(2).Find("a").Text())
		if name == "" {
			name = strings.TrimSpace(row.Find("td").Eq(2).Text())
		}
		if code == "" {
			return
		}
		stock := models.ConceptStock{
			Code:          code,
			Name:          name,
			Price:         strings.TrimSpace(cells.Eq(3).Text()),
			ChangePercent: strings.TrimSpace(cells.Eq(4).Text()),
			Change:        strings.TrimSpace(cells.Eq(5).Text()),
			Speed:         strings.TrimSpace(cells.Eq(6).Text()),
			Turnover:      strings.TrimSpace(cells.Eq(7).Text()),
			VolumeRatio:   strings.TrimSpace(cells.Eq(8).Text()),
			Amplitude:     strings.TrimSpace(cells.Eq(9).Text()),
			DealAmount:    strings.TrimSpace(cells.Eq(10).Text()),
			FlowShares:    strings.TrimSpace(cells.Eq(11).Text()),
			FlowMarketCap: strings.TrimSpace(cells.Eq(12).Text()),
			PERatio:       strings.TrimSpace(cells.Eq(13).Text()),
		}
		stocks = append(stocks, stock)
	})
	return stocks
}

// cleanMarketValue 去除行情值中的多余空白和非数字字符（保留 数字/./-/%//）
func cleanMarketValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// 折叠内部空白
	re := regexp.MustCompile(`\s+`)
	v = re.ReplaceAllString(v, "")
	return v
}

// ConceptKLine 获取同花顺概念板块 K 线数据
// plateCode 为板块代码（如 886112），由 ConceptDetail 返回的 PlateCode 字段提供
// 数据来源：http://d.10jqka.com.cn/v6/line/bk_{plateCode}/01/all.js （JSONP）
//
// 响应格式：
//
//	quotebridge_v6_line_bk_886112_01_all({
//	  "total":"3","start":"20260803","name":"MLCC概念",
//	  "priceFactor":1000,
//	  "price":"995010,11251,38229,23240,1003432,18806,68914,64676,1055680,0,103066,87273",
//	  "volumn":"697108560,1372429600,1659754900",
//	  "dates":"0803,0804,0805",
//	  "issuePrice":"1020.658"
//	})
//
// price 数组每 4 个值为一组对应一天，含义：
//
//	[v1=open, v2=?, v3=?, v4=change=close-open]
//	因此 close = open + change
func (m MarketNewsApi) ConceptKLine(plateCode string) *models.ConceptKLineData {
	result := &models.ConceptKLineData{}
	if plateCode == "" {
		return result
	}
	url := fmt.Sprintf("http://d.10jqka.com.cn/v6/line/bk_%s/01/all.js", plateCode)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "d.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptKLine request err: %s", err.Error())
		return result
	}
	body := string(resp.Body())
	// 提取 JSONP 中的 JSON 部分
	jsonStr := extractJSONPPayload(body)
	if jsonStr == "" {
		logger.SugaredLogger.Errorf("ConceptKLine parse JSONP err, body: %s", truncate(body, 200))
		return result
	}

	// 解析为通用 map
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		logger.SugaredLogger.Errorf("ConceptKLine unmarshal err: %s, body: %s", err.Error(), truncate(body, 200))
		return result
	}

	result.Name = convertor.ToString(raw["name"])
	result.Total, _ = strconv.Atoi(convertor.ToString(raw["total"]))
	result.Start = convertor.ToString(raw["start"])
	result.Factor, _ = convertor.ToFloat(raw["priceFactor"])
	result.IssuePrice, _ = convertor.ToFloat(raw["issuePrice"])

	priceStr := convertor.ToString(raw["price"])
	volStr := convertor.ToString(raw["volumn"])
	dateStr := convertor.ToString(raw["dates"])
	dates := splitNonEmpty(dateStr, ",")
	vols := splitNonEmpty(volStr, ",")
	prices := splitNonEmpty(priceStr, ",")

	if len(dates) == 0 {
		return result
	}
	perDay := 4
	if len(prices)/perDay < len(dates) {
		perDay = len(prices) / len(dates)
		if perDay < 1 {
			perDay = 1
		}
	}
	factor := result.Factor
	if factor == 0 {
		factor = 1
	}
	for i, d := range dates {
		// 日期补全为 YYYYMMDD（dates 仅含 MMDD，年份取自 start 前 4 位）
		fullDate := d
		if len(d) == 4 && len(result.Start) >= 4 {
			fullDate = result.Start[:4] + d
		}
		item := models.ConceptKLineItem{Date: fullDate}
		base := i * perDay
		if base+0 < len(prices) {
			open, _ := convertor.ToFloat(prices[base+0])
			item.Open = open / factor
		}
		if perDay >= 4 && base+3 < len(prices) {
			change, _ := convertor.ToFloat(prices[base+3])
			item.Close = item.Open + change/factor
			item.High = item.Close // 暂以 close 代替 high（接口未明确给出 high）
			item.Low = item.Open   // 暂以 open 代替 low
		} else {
			item.Close = item.Open
		}
		if i < len(vols) {
			v, _ := convertor.ToFloat(vols[i])
			item.Volume = v
		}
		result.KLines = append(result.KLines, item)
	}
	return result
}

// ConceptRealHead 获取同花顺概念板块实时行情数据
// plateCode 为板块代码（如 886112），数据来源：http://d.10jqka.com.cn/v2/realhead/bk_{plateCode}/last.js
// 响应 JSONP，外层 key 为 items，值是字段代码到数值的映射（与个股 realhead 字段一致）
func (m MarketNewsApi) ConceptRealHead(plateCode string) *models.ConceptMarket {
	result := &models.ConceptMarket{}
	if plateCode == "" {
		return result
	}
	url := fmt.Sprintf("http://d.10jqka.com.cn/v2/realhead/bk_%s/last.js", plateCode)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "d.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ConceptRealHead request err: %s", err.Error())
		return result
	}
	body := GB18030ToUTF8(resp.Body())
	jsonStr := extractJSONPPayload(body)
	if jsonStr == "" {
		logger.SugaredLogger.Errorf("ConceptRealHead parse JSONP err, body: %s", truncate(body, 200))
		return result
	}
	raw := map[string]any{}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		logger.SugaredLogger.Errorf("ConceptRealHead unmarshal err: %s, body: %s", err.Error(), truncate(body, 200))
		return result
	}
	items, _ := raw["items"].(map[string]any)
	if items == nil {
		return result
	}
	get := func(key string) string {
		return convertor.ToString(items[key])
	}
	// 资金净流入 = (大单-小单-中单) 净额之和；接口单位为元，转成亿与 HTML 路径一致
	bigIn, _ := convertor.ToFloat(get("223"))
	bigOut, _ := convertor.ToFloat(get("224"))
	smallIn, _ := convertor.ToFloat(get("237"))
	smallOut, _ := convertor.ToFloat(get("238"))
	midIn, _ := convertor.ToFloat(get("259"))
	midOut, _ := convertor.ToFloat(get("260"))
	netInflow := (bigIn - bigOut) + (smallIn - smallOut) + (midIn - midOut)

	volume, _ := convertor.ToFloat(get("13"))     // 手
	dealAmount, _ := convertor.ToFloat(get("19")) // 元

	result.Open = get("7")
	result.PreClose = get("6")
	result.High = get("8")
	result.Low = get("9")
	result.Volume = fmt.Sprintf("%.2f", volume/1e4) // 万手
	result.ChangePercent = get("199112")
	result.UpDownCount = get("37")
	result.NetInflow = fmt.Sprintf("%.2f", netInflow/1e8)   // 亿
	result.DealAmount = fmt.Sprintf("%.2f", dealAmount/1e8) // 亿
	return result
}

// extractJSONPPayload 从 JSONP 响应中提取括号内的 JSON 字符串
// 例如：quotebridge_v6_line_bk_886112_01_all({...}) → 返回 {...}
func extractJSONPPayload(body string) string {
	start := strings.Index(body, "(")
	end := strings.LastIndex(body, ")")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return body[start+1 : end]
}

func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ConceptPlate 概念板块字典项
type ConceptPlate struct {
	Code string `json:"code"` // 概念代码，如 307550
	Name string `json:"name"` // 概念名称，如 冰雪产业
}

// GetAllConceptPlates 获取同花顺所有概念板块字典（代码+名称）
// 数据来源：https://q.10jqka.com.cn/gn/ （GBK 编码的 HTML 页面，按字母分组的链接列表）
func (m MarketNewsApi) GetAllConceptPlates() []ConceptPlate {
	url := "https://q.10jqka.com.cn/gn/"
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "q.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllConceptPlates request err: %s", err.Error())
		return nil
	}
	htmlContent := GB18030ToUTF8(resp.Body())
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllConceptPlates goquery err: %s", err.Error())
		return nil
	}
	var plates []ConceptPlate
	doc.Find(`a[href*="/gn/detail/code/"]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		idx := strings.Index(href, "/gn/detail/code/")
		if idx < 0 {
			return
		}
		rest := href[idx+len("/gn/detail/code/"):]
		code := rest
		if slash := strings.Index(rest, "/"); slash >= 0 {
			code = rest[:slash]
		}
		code = strings.TrimSpace(code)
		name := strings.TrimSpace(s.Text())
		if code == "" || name == "" {
			return
		}
		plates = append(plates, ConceptPlate{Code: code, Name: name})
	})
	return plates
}

// FindConceptCodeByName 通过名称在概念板块字典中查找概念代码
// name 为题材名称（如"航空发动机"），返回概念代码（如"301470"），未找到返回空字符串
func (m MarketNewsApi) FindConceptCodeByName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	plates := m.GetAllConceptPlates()
	if len(plates) == 0 {
		return ""
	}
	// 1. 精确匹配
	for _, plate := range plates {
		if plate.Name == name {
			return plate.Code
		}
	}
	// 2. 包含匹配（双向）
	for _, plate := range plates {
		if strings.Contains(plate.Name, name) || strings.Contains(name, plate.Name) {
			return plate.Code
		}
	}
	return ""
}

// IndustryPlate 行业板块字典项
type IndustryPlate struct {
	Code string `json:"code"` // 行业代码，如 881121
	Name string `json:"name"` // 行业名称，如 半导体
}

// GetAllIndustryPlates 获取同花顺所有行业板块字典（代码+名称）
// 数据来源：https://q.10jqka.com.cn/thshy/ （GBK 编码的 HTML 页面，含按字母分组的链接列表）
// 行业代码为 881xxx，与概念板块（885xxx/886xxx）区分
func (m MarketNewsApi) GetAllIndustryPlates() []IndustryPlate {
	url := "https://q.10jqka.com.cn/thshy/"
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "q.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllIndustryPlates request err: %s", err.Error())
		return nil
	}
	htmlContent := GB18030ToUTF8(resp.Body())
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("GetAllIndustryPlates goquery err: %s", err.Error())
		return nil
	}
	var plates []IndustryPlate
	seen := map[string]bool{}
	doc.Find(`a[href*="/thshy/detail/code/"]`).Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		idx := strings.Index(href, "/thshy/detail/code/")
		if idx < 0 {
			return
		}
		rest := href[idx+len("/thshy/detail/code/"):]
		code := rest
		if slash := strings.Index(rest, "/"); slash >= 0 {
			code = rest[:slash]
		}
		code = strings.TrimSpace(code)
		name := strings.TrimSpace(s.Text())
		if code == "" || name == "" || seen[code] {
			return
		}
		seen[code] = true
		plates = append(plates, IndustryPlate{Code: code, Name: name})
	})
	return plates
}

// IndustryDetail 获取同花顺行业板块详情页数据
// industryCode 为行业代码（URL 中的 code，如 881121）
// 数据来源：https://q.10jqka.com.cn/thshy/detail/code/{industryCode}/ （GBK 编码的 HTML 页面）
// 行业详情页结构与概念详情页基本一致，差异：行业页无板块定义文本
func (m MarketNewsApi) IndustryDetail(industryCode string) *models.ConceptDetailInfo {
	info := &models.ConceptDetailInfo{ConceptCode: industryCode}
	if industryCode == "" {
		return info
	}
	url := fmt.Sprintf("https://q.10jqka.com.cn/thshy/detail/code/%s/", industryCode)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("Host", "q.10jqka.com.cn").
		SetHeader("Referer", "https://q.10jqka.com.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("IndustryDetail request err: %s", err.Error())
		return info
	}

	htmlContent := GB18030ToUTF8(resp.Body())
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.SugaredLogger.Errorf("IndustryDetail goquery err: %s", err.Error())
		return info
	}

	// 1. 提取板块代码（plateCode）— 优先从隐藏域 <input id="clid" value="881121">
	if clid := doc.Find("input#clid").AttrOr("value", ""); clid != "" {
		info.PlateCode = strings.TrimSpace(clid)
	} else {
		doc.Find(".board-hq h3 span").First().Each(func(_ int, s *goquery.Selection) {
			info.PlateCode = strings.TrimSpace(s.Text())
		})
	}

	// 2. 提取行业名称 — 从 <h3>半导体<span>881121</span></h3>
	doc.Find(".board-hq h3").First().Each(func(_ int, s *goquery.Selection) {
		clone := s.Clone()
		clone.Find("span").Remove()
		info.Name = strings.TrimSpace(clone.Text())
	})
	if info.Name == "" {
		if title := doc.Find("title").Text(); title != "" {
			info.Name = strings.TrimSpace(strings.Split(title, "_")[0])
		}
	}

	// 3. 行业板块无定义文本（.board-txt.board-aside p 为空），Definition 保持空

	// 4. 提取板块行情数据 — <div class="board-infos"> 下的 <dl><dt>标签</dt><dd>值</dd></dl>
	marketMap := map[string]string{}
	doc.Find(".board-infos dl").Each(func(_ int, s *goquery.Selection) {
		dt := strings.TrimSpace(s.Find("dt").Text())
		dd := strings.TrimSpace(s.Find("dd").Text())
		if dt == "涨跌家数" {
			parts := []string{}
			s.Find("dd span").Each(func(_ int, sp *goquery.Selection) {
				t := strings.TrimSpace(sp.Text())
				if t != "" {
					parts = append(parts, t)
				}
			})
			if len(parts) >= 2 {
				dd = parts[0] + "/" + parts[1]
			}
		}
		if dt != "" {
			marketMap[dt] = dd
		}
	})
	info.Market = models.ConceptMarket{
		Open:          cleanMarketValue(marketMap["今开"]),
		PreClose:      cleanMarketValue(marketMap["昨收"]),
		Low:           cleanMarketValue(marketMap["最低"]),
		High:          cleanMarketValue(marketMap["最高"]),
		Volume:        cleanMarketValue(marketMap["成交量(万手)"]),
		ChangePercent: cleanMarketValue(marketMap["板块涨幅"]),
		ChangeRank:    cleanMarketValue(marketMap["涨幅排名"]),
		UpDownCount:   cleanMarketValue(marketMap["涨跌家数"]),
		NetInflow:     cleanMarketValue(marketMap["资金净流入(亿)"]),
		DealAmount:    cleanMarketValue(marketMap["成交额(亿)"]),
	}

	// 5. 解析成分股表格 — <table class="m-table"><thead>...<tbody>
	doc.Find("table.m-table tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 14 {
			return
		}
		code := strings.TrimSpace(row.Find("td").Eq(1).Find("a").Text())
		if code == "" {
			code = strings.TrimSpace(row.Find("td").Eq(1).Text())
		}
		name := strings.TrimSpace(row.Find("td").Eq(2).Find("a").Text())
		if name == "" {
			name = strings.TrimSpace(row.Find("td").Eq(2).Text())
		}
		if code == "" {
			return
		}
		stock := models.ConceptStock{
			Code:          code,
			Name:          name,
			Price:         strings.TrimSpace(cells.Eq(3).Text()),
			ChangePercent: strings.TrimSpace(cells.Eq(4).Text()),
			Change:        strings.TrimSpace(cells.Eq(5).Text()),
			Speed:         strings.TrimSpace(cells.Eq(6).Text()),
			Turnover:      strings.TrimSpace(cells.Eq(7).Text()),
			VolumeRatio:   strings.TrimSpace(cells.Eq(8).Text()),
			Amplitude:     strings.TrimSpace(cells.Eq(9).Text()),
			DealAmount:    strings.TrimSpace(cells.Eq(10).Text()),
			FlowShares:    strings.TrimSpace(cells.Eq(11).Text()),
			FlowMarketCap: strings.TrimSpace(cells.Eq(12).Text()),
			PERatio:       strings.TrimSpace(cells.Eq(13).Text()),
		}
		info.Stocks = append(info.Stocks, stock)
	})

	return info
}

// IndustryKLine 获取同花顺行业板块 K 线数据
// plateCode 为板块代码（如 881121），由 IndustryDetail 返回的 PlateCode 字段提供
// 行业板块与概念板块共用同一 K 线接口（bk_{plateCode} 前缀），直接复用 ConceptKLine
func (m MarketNewsApi) IndustryKLine(plateCode string) *models.ConceptKLineData {
	return m.ConceptKLine(plateCode)
}

// IndustryRealHead 获取同花顺行业板块实时行情数据
// plateCode 为板块代码（如 881121），行业板块与概念板块共用同一 realhead 接口（bk_{plateCode} 前缀）
// 直接复用 ConceptRealHead
func (m MarketNewsApi) IndustryRealHead(plateCode string) *models.ConceptMarket {
	return m.ConceptRealHead(plateCode)
}
