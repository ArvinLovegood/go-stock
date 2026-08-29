package data

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
)

// @Author spark
// @Date 2025/6/28 21:02
// @Desc
// -----------------------------------------------------------------------------------
type SearchStockApi struct {
	words string
}

func NewSearchStockApi(words string) *SearchStockApi {
	return &SearchStockApi{words: words}
}

// generateRequestId 生成东财要求的 requestId：32位随机字母数字 + 13位毫秒时间戳
func generateRequestId() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 32)
	for i := range b {
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars)))); err == nil {
			b[i] = chars[n.Int64()]
		} else {
			b[i] = chars[i%len(chars)]
		}
	}
	return string(b) + fmt.Sprintf("%d", time.Now().UnixMilli())
}

// searchCode 东财智能选股通用请求（2026-08 契约：keyWordNew/customDataNew/biz/微秒时间戳/32+13位requestId）
// 请求体必须用 json.Marshal 构造，避免关键词含引号/反斜杠时破坏 JSON 结构
func (s SearchStockApi) searchCode(url, biz string, pageSize int) map[string]any {
	qgqpBId := NewSettingsApi().Config.QgqpBId
	if qgqpBId == "" {
		return map[string]any{
			"code":    -1,
			"message": "请先获取东财用户标识（qgqp_b_id）：打开浏览器,访问东财网站，按F12打开开发人员工具-》网络面板，随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。保存到设置中的东财唯一标识输入框",
		}
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	// customDataNew 是"内嵌 JSON 的字符串"：[{"type":"text","value":"关键词","extra":""}]
	customData, _ := json.Marshal([]map[string]string{{"type": "text", "value": s.words, "extra": ""}})
	reqBody, err := json.Marshal(map[string]any{
		"needAmbiguousSuggest":   true,
		"pageSize":               pageSize,
		"pageNo":                 1,
		"fingerprint":            qgqpBId,
		"matchWord":              "",
		"shareToGuba":            false,
		"timestamp":              fmt.Sprintf("%d", time.Now().UnixMicro()),
		"requestId":              generateRequestId(),
		"removedConditionIdList": []any{},
		"ownSelectAll":           false,
		"needCorrect":            true,
		"client":                 "WEB",
		"product":                "",
		"needShowStockNum":       false,
		"biz":                    biz,
		"gids":                   []any{},
		"dxInfoNew":              []any{},
		"keyWordNew":             s.words,
		"customDataNew":          string(customData),
	})
	if err != nil {
		return map[string]any{
			"code":    -1,
			"message": "构造请求失败: " + err.Error(),
		}
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0").
		SetHeader("Content-Type", "application/json").
		SetHeader("Cookie", "qgqp_b_id="+qgqpBId).
		SetBody(reqBody).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("searchCode-err:%+v", err)
		return map[string]any{
			"code":    -1,
			"message": err.Error(),
		}
	}
	respMap := map[string]any{}
	if err := json.Unmarshal(resp.Body(), &respMap); err != nil {
		logger.SugaredLogger.Errorf("searchCode-unmarshal-err:%+v body:%s", err, string(resp.Body()[:min(500, len(resp.Body()))]))
		return map[string]any{
			"code":    -1,
			"message": "解析东财响应失败: " + string(resp.Body()[:min(500, len(resp.Body()))]),
		}
	}
	// 东财服务端故障时返回 code=503 "抱歉，查询数据出现问题，请稍后重试"（官方网页同样失败），
	// 透传原始 msg 让前端如实展示，并补充提示来源
	if code, _ := respMap["code"].(string); code == "503" {
		msg, _ := respMap["msg"].(string)
		if msg == "" {
			msg = "查询数据出现问题"
		}
		respMap["message"] = msg + "（东财服务端错误，请稍后重试）"
	}
	return respMap
}

func (s SearchStockApi) SearchStock(pageSize int) map[string]any {
	res := s.searchCode("https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code", "web_ai_select_stocks", pageSize)
	// 东财 AI 选股服务故障（503）时回退同花顺问财（需配置 IwencaiApiKey）
	if code, _ := res["code"].(string); code == "503" {
		if fallback := s.searchStockViaIwencai(pageSize); fallback != nil {
			return fallback
		}
	}
	return res
}

// splitIwencaiStockCode 问财股票代码 "600519.SH"/"000001.SZ" → ("600519","SH")
func splitIwencaiStockCode(code string) (pureCode, market string) {
	if idx := strings.LastIndex(code, "."); idx > 0 && idx < len(code)-1 {
		return code[:idx], strings.ToUpper(code[idx+1:])
	}
	return code, ""
}

// searchStockViaIwencai 东财 AI 选股 503 时回退同花顺问财，将结果转换为东财 search-code 响应格式。
// 未配置问财 API key 或查询失败返回 nil（调用方保留原始 503 错误）。
func (s SearchStockApi) searchStockViaIwencai(pageSize int) map[string]any {
	api := NewIwencaiAPI()
	if api.config == nil || api.config.Settings == nil || api.config.Settings.IwencaiApiKey == "" {
		return nil
	}
	// 问财 API 有配额限制，pageSize 封顶防单次耗尽
	limit := pageSize
	if limit > 200 {
		limit = 200
	}
	result, err := api.Query(s.words, 1, limit)
	if err != nil || result == nil || len(result.Datas) == 0 {
		if err != nil {
			logger.SugaredLogger.Warnf("问财回退查询失败: %v", err)
		}
		return nil
	}

	// 收集所有列（并集），映射到东财列格式
	seen := map[string]bool{}
	var colKeys []string
	for _, row := range result.Datas {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				colKeys = append(colKeys, k)
			}
		}
	}

	// 东财前端操作列（K线/关注）依赖 SECURITY_CODE/SECURITY_SHORT_NAME/MARKET_SHORT_NAME 三个 key
	columns := []any{}
	dataList := []any{}
	for _, k := range colKeys {
		switch k {
		case "股票代码":
			columns = append(columns, map[string]any{"title": "代码", "key": "SECURITY_CODE"})
			if !seen["__market"] { // 从代码后缀提取市场简称
				columns = append(columns, map[string]any{"title": "市场简称", "key": "MARKET_SHORT_NAME"})
				seen["__market"] = true
			}
		case "股票简称", "名称":
			if !seen["__name"] {
				columns = append(columns, map[string]any{"title": "名称", "key": "SECURITY_SHORT_NAME"})
				seen["__name"] = true
			}
		default:
			columns = append(columns, map[string]any{"title": k, "key": k})
		}
	}
	for _, row := range result.Datas {
		tmp := map[string]any{}
		for k, v := range row {
			switch k {
			case "股票代码":
				code := convertor.ToString(v)
				pure, market := splitIwencaiStockCode(code)
				tmp["SECURITY_CODE"] = pure
				tmp["MARKET_SHORT_NAME"] = market
			case "股票简称", "名称":
				tmp["SECURITY_SHORT_NAME"] = v
			default:
				tmp[k] = v
			}
		}
		dataList = append(dataList, tmp)
	}

	return map[string]any{
		"code": "100",
		"msg":  "解析成功（数据来源：同花顺问财，东财AI选股服务暂不可用）",
		"data": map[string]any{
			"result": map[string]any{
				"columns":  columns,
				"dataList": dataList,
				"meta":     map[string]any{"total": len(dataList)},
			},
			"traceInfo": map[string]any{
				"showText": fmt.Sprintf("已通过同花顺问财完成「%s」选股（东财AI选股服务暂时不可用，已自动切换备用数据源）", s.words),
			},
		},
	}
}

func (s SearchStockApi) SearchBk(pageSize int) map[string]any {
	return s.searchCode("https://np-tjxg-b.eastmoney.com/api/smart-tag/bkc/v3/pw/search-code", "web_ai_select_bkcs", pageSize)
}

func (s SearchStockApi) SearchETF(pageSize int) map[string]any {
	return s.searchCode("https://np-tjxg-b.eastmoney.com/api/smart-tag/etf/v3/pw/search-code", "web_ai_select_etfs", pageSize)
}

func (s SearchStockApi) HotStrategy() map[string]any {
	url := fmt.Sprintf("https://np-ipick.eastmoney.com/recommend/stock/heat/ranking?count=20&trace=%d&client=web&biz=web_smart_tag", time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-ipick.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotStrategy-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	return respMap
}

func (s SearchStockApi) HotStrategyTable() string {
	markdownTable := ""
	res := s.HotStrategy()
	bytes, _ := json.Marshal(res)
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable = util.MarkdownTableWithTitle("当前热门选股策略", strategy.Data)
	return markdownTable
}

func (s SearchStockApi) StrategySquare() map[string]any {
	//https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword=
	url := "https://backtest.10jqka.com.cn/strategysquare/list?order=desc&page=1&pageNum=10&sortType=hot&keyword="
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "backtest.10jqka.com.cn").
		SetHeader("Origin", "https://backtest.10jqka.com.cn").
		SetHeader("Referer", "https://backtest.10jqka.com.cn/strategysquare/list").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("StrategySquare-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}
