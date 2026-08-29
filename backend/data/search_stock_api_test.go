package data

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/random"
)

func TestSearchStock(t *testing.T) {
	db.Init("../../data/stock.db")

	e := convertor.ToString(math.Floor(float64(9*random.RandFloat(0, 1, 12) + 1)))
	for i := 0; i < 19; i++ {
		e += convertor.ToString(math.Floor(float64(9 * random.RandFloat(0, 1, 12))))
	}
	logger.SugaredLogger.Infof("e:%s", e)

	res := NewSearchStockApi("量比大于2，基本面优秀，2025年三季报已披露，主力连续3日净流入，非创业板非科创板非ST").SearchStock(20)
	//res := NewSearchStockApi("今日涨幅前5的概念板块").SearchBk(50)
	//res := NewSearchStockApi("今日涨幅前15的ETF").SearchETF(50)

	logger.SugaredLogger.Infof("res:%+v", res)
	data := res["data"].(map[string]any)
	result := data["result"].(map[string]any)
	dataList := result["dataList"].([]any)
	columns := result["columns"].([]any)
	headers := map[string]string{}
	for _, v := range columns {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		//logger.SugaredLogger.Infof("key:%s title:%s dateMsg:%s unit:%s", d["key"], d["title"], d["dateMsg"], d["unit"])
		title := convertor.ToString(d["title"])
		if convertor.ToString(d["dateMsg"]) != "" {
			title = title + "[" + convertor.ToString(d["dateMsg"]) + "]"
		}
		if convertor.ToString(d["unit"]) != "" {
			title = title + "(" + convertor.ToString(d["unit"]) + ")"
		}
		headers[d["key"].(string)] = title
	}
	table := &[]map[string]any{}
	for _, v := range dataList {
		//logger.SugaredLogger.Infof("v:%+v", v)
		d := v.(map[string]any)
		tmp := map[string]any{}
		for key, title := range headers {
			//logger.SugaredLogger.Infof("%s:%s", title, convertor.ToString(d[key]))
			tmp[title] = convertor.ToString(d[key])
		}
		*table = append(*table, tmp)
		//logger.SugaredLogger.Infof("--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------")
	}
	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	logger.SugaredLogger.Infof("markdownTable=\n%s", markdownTable)
}

// TestGenerateRequestId 验证 requestId 格式：32位随机字母数字 + 13位毫秒时间戳（2026-08 东财契约）
func TestGenerateRequestId(t *testing.T) {
	id := generateRequestId()
	if len(id) != 45 {
		t.Fatalf("requestId 长度应为 45（32随机+13毫秒时间戳），实际 %d: %s", len(id), id)
	}
	for i, c := range id {
		if i < 32 {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
				t.Fatalf("requestId 前 32 位应全为字母数字，第 %d 位为 %q", i, c)
			}
		} else if c < '0' || c > '9' {
			t.Fatalf("requestId 后 13 位应为毫秒时间戳数字，第 %d 位为 %q", i, c)
		}
	}
	// 尾部时间戳与当前时间偏差应在 5 秒内
	ts, err := strconv.ParseInt(id[32:], 10, 64)
	if err != nil {
		t.Fatalf("requestId 尾部时间戳解析失败: %v", err)
	}
	if diff := time.Now().UnixMilli() - ts; diff < -5000 || diff > 5000 {
		t.Fatalf("requestId 时间戳与当前时间偏差 %dms，超出 ±5s", diff)
	}
	// 两次生成应不同（随机性）
	if generateRequestId() == id {
		t.Fatal("两次生成的 requestId 不应相同")
	}
}

// TestSearchCodeApi 集成测试：真实调用东财智能选股 Stock/Bk/ETF 三个 search-code 接口。
// 使用临时数据库自包含运行：qgqp_b_id 可通过环境变量 SEARCH_CODE_QGQP_BID 指定（默认用内置测试值）。
// 东财 AI 解析服务故障时返回 code=503，此时验证错误透传逻辑（message 附加服务端错误提示）；
// 服务恢复时验证 dataList 结构。
func TestSearchCodeApi(t *testing.T) {
	// 不用 t.TempDir：db.Init 启动的后台 goroutine 会持有库文件句柄，Windows 下 TempDir 清理失败会误报测试失败
	tmpDir, err := os.MkdirTemp("", "search-code-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	db.Init(tmpDir + "/stock.db")
	// settings 表的迁移在应用 main.go 中，测试库需自行建表并插入首行
	if err := db.Dao.AutoMigrate(&Settings{}); err != nil {
		t.Fatalf("迁移 settings 表失败: %v", err)
	}
	if err := db.Dao.Exec("INSERT OR IGNORE INTO settings (id) VALUES (1)").Error; err != nil {
		t.Fatalf("插入 settings 首行失败: %v", err)
	}
	qgqpBId := os.Getenv("SEARCH_CODE_QGQP_BID")
	if qgqpBId == "" {
		qgqpBId = "02efa8944b1f90fbfe050e1e695a480d" // 内置测试标识
	}
	if err := db.Dao.Model(&Settings{}).Where("id = ?", 1).Update("qgqp_b_id", qgqpBId).Error; err != nil {
		t.Fatalf("写入测试 qgqp_b_id 失败: %v", err)
	}
	if NewSettingsApi().Config.QgqpBId == "" {
		t.Skip("qgqp_b_id 写入后仍未生效，跳过集成测试")
	}
	cases := []struct {
		name string
		call func(pageSize int) map[string]any
	}{
		{"Stock", NewSearchStockApi("市盈率低于20").SearchStock},
		{"Bk", NewSearchStockApi("今日涨幅前5的概念板块").SearchBk},
		{"ETF", NewSearchStockApi("今日涨幅前15的ETF").SearchETF},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := c.call(10)
			if res == nil {
				t.Fatal("返回结果为空")
			}
			code := convertor.ToString(res["code"])
			switch code {
			case "-1":
				t.Fatalf("客户端构造/请求失败: %s", convertor.ToString(res["message"]))
			case "503":
				// 东财服务端 AI 解析故障：验证 msg 已透传并附加提示
				msg := convertor.ToString(res["message"])
				if msg == "" {
					t.Fatal("code=503 时 message 不应为空")
				}
				if !strings.Contains(msg, "东财服务端错误") {
					t.Fatalf("code=503 时 message 应附加服务端错误提示，实际: %s", msg)
				}
				t.Logf("东财服务端故障（预期可能发生）: %s", msg)
			default:
				data, ok := res["data"].(map[string]any)
				if !ok {
					t.Fatalf("code=%s 但 data 结构异常: %+v", code, res["data"])
				}
				result, ok := data["result"].(map[string]any)
				if !ok {
					t.Fatalf("data.result 结构异常: %+v", data["result"])
				}
				dataList, _ := result["dataList"].([]any)
				columns, _ := result["columns"].([]any)
				t.Logf("查询成功: %d 条结果, %d 列", len(dataList), len(columns))
				if len(dataList) == 0 {
					t.Log("警告: dataList 为空")
				}
			}
		})
	}
}

// TestSearchStockIwencaiFallback 端到端测试东财 503 时问财自动回退：
// 需 IWENCAI_API_KEY 环境变量；东财恢复时走正常路径，两条路径都应返回
// SECURITY_CODE/SECURITY_SHORT_NAME 供前端操作列使用。
func TestSearchStockIwencaiFallback(t *testing.T) {
	apiKey := os.Getenv("IWENCAI_API_KEY")
	if apiKey == "" {
		t.Skip("未设置 IWENCAI_API_KEY 环境变量，跳过回退测试")
	}
	tmpDir, err := os.MkdirTemp("", "iwencai-fallback-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	db.Init(tmpDir + "/stock.db")
	if err := db.Dao.AutoMigrate(&Settings{}, &AIConfig{}); err != nil {
		t.Fatalf("迁移表失败: %v", err)
	}
	if err := db.Dao.Exec("INSERT OR IGNORE INTO settings (id) VALUES (1)").Error; err != nil {
		t.Fatalf("插入 settings 首行失败: %v", err)
	}
	if err := db.Dao.Model(&Settings{}).Where("id = ?", 1).Updates(map[string]any{
		"qgqp_b_id":       "02efa8944b1f90fbfe050e1e695a480d",
		"iwencai_api_key": apiKey,
	}).Error; err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}

	res := NewSearchStockApi("市盈率低于20，市值大于100亿").SearchStock(10)
	code := convertor.ToString(res["code"])
	if code != "100" {
		t.Fatalf("期望 code=100（东财成功或问财回退），实际: %s, msg: %s", code, convertor.ToString(res["message"])+convertor.ToString(res["msg"]))
	}
	msg := convertor.ToString(res["msg"])
	if strings.Contains(msg, "问财") {
		t.Log("走问财回退路径（东财仍 503）")
	} else {
		t.Log("走东财正常路径（服务已恢复）")
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 结构异常: %+v", res["data"])
	}
	result, _ := data["result"].(map[string]any)
	dataList, _ := result["dataList"].([]any)
	if len(dataList) == 0 {
		t.Fatal("dataList 为空")
	}
	first, _ := dataList[0].(map[string]any)
	// 前端 K线/关注操作列依赖的三个 key 必须存在
	for _, key := range []string{"SECURITY_CODE", "SECURITY_SHORT_NAME"} {
		if convertor.ToString(first[key]) == "" {
			t.Fatalf("首行数据缺少 %s: %+v", key, first)
		}
	}
	t.Logf("返回 %d 条，首条: 代码=%s 名称=%s", len(dataList), first["SECURITY_CODE"], first["SECURITY_SHORT_NAME"])
	if traceInfo, ok := data["traceInfo"].(map[string]any); ok {
		t.Logf("traceInfo: %s", convertor.ToString(traceInfo["showText"]))
	}
}

// TestSplitIwencaiStockCode 验证问财代码拆分
func TestSplitIwencaiStockCode(t *testing.T) {
	cases := []struct{ in, code, market string }{
		{"600519.SH", "600519", "SH"},
		{"000001.SZ", "000001", "SZ"},
		{"830799.BJ", "830799", "BJ"},
		{"00700.HK", "00700", "HK"},
		{"600519", "600519", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		code, market := splitIwencaiStockCode(c.in)
		if code != c.code || market != c.market {
			t.Fatalf("splitIwencaiStockCode(%q) = (%q,%q), want (%q,%q)", c.in, code, market, c.code, c.market)
		}
	}
}

func TestGetStockFinancialInfo(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockFinancialInfo("300390.SZ")
	MD := util.MarkdownTableWithTitle("300390.SZ股票财报信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}
func TestGetStockHolderNum(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewStockDataApi().GetStockHolderNum("300390.SZ")
	MD := util.MarkdownTableWithTitle("股票股东人数信息", res.Result.Data)
	logger.SugaredLogger.Infof("res:\n%s", MD)
}

func TestSearchStockApi_HotStrategy(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").HotStrategy()
	bytes, err := json.Marshal(res)
	if err != nil {
		return
	}
	strategy := &models.HotStrategy{}
	json.Unmarshal(bytes, strategy)
	for _, data := range strategy.Data {
		data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
	}
	markdownTable := util.MarkdownTable(strategy.Data)
	logger.SugaredLogger.Infof("res:%s", markdownTable)
	//dataList := res["data"].([]any)
	//for _, v := range dataList {
	//	d := v.(map[string]any)
	//	logger.SugaredLogger.Infof("v:%+v", d)
	//}
}
func TestSearchStockApi_HotStrategyTable(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewSearchStockApi("").StrategySquare()
	logger.SugaredLogger.Infof("res:%+v", res)
}
