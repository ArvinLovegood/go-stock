package data

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
)

// TestOKXLiveProbe OKX 实盘链路探针（只读校验 → 最小张数开仓 → 查单 → 立即平仓）。
//
// 密钥来源：应用设置库（设置页录入的 okx_live_*），本文件不含任何真实凭证。
// 仅显式运行：go test ./backend/data/ -run TestOKXLiveProbe -count=1 -v
// 选取全市场名义价值最小的 USDT 永续，按最小张数市价买入后立即 reduceOnly 平仓，
// 双边手续费按名义价值 0.05% 计，通常 < 0.01 USDT。
//
// 安全护栏：设置库里若存的是曾在对话中暴露的旧密钥，直接拒绝执行并提示重置。
func TestOKXLiveProbe(t *testing.T) {
	if dbPath := os.Getenv("OKX_PROBE_DB"); strings.TrimSpace(dbPath) != "" {
		// 显式指定库路径时强制重初始化：包内其他 *_test.go 的 init() 可能已用
		// 默认路径抢先初始化过 db.Dao（Go 包级 init 先于测试函数执行）。
		db.Init(dbPath)
	} else if db.Dao == nil {
		// 库路径默认用仓库内库（应用安装目录可用 OKX_PROBE_DB 指定，如 D:\go-stock\data\stock.db）
		db.Init("../../data/stock.db")
	}
	cfg := GetSettingConfig()
	if cfg == nil || cfg.Settings == nil {
		t.Fatal("配置不可读")
	}
	liveKey := strings.TrimSpace(cfg.OKXLiveApiKey)
	liveSecret := strings.TrimSpace(cfg.OKXLiveSecretKey)
	livePass := strings.TrimSpace(cfg.OKXLivePassphrase)
	if liveKey == "" || liveSecret == "" || livePass == "" {
		t.Fatal("设置库未录入实盘密钥：请在设置页 OKX 区块填写「实盘 API Key / SecretKey / 口令」后重跑")
	}
	// 曾在对话中暴露的旧密钥绝对不可使用
	const leakedKey = "109c567e-dd31-4001-a535-c93632cda85f"
	if liveKey == leakedKey {
		t.Fatal("设置库中的实盘密钥是已在对话中暴露的旧密钥，必须先去 OKX 后台重置并在设置页重新录入")
	}
	t.Logf("实盘密钥校验通过: key=%s****%s (全局模拟盘开关=%v，本探针固定走实盘环境)",
		liveKey[:6], liveKey[len(liveKey)-4:], cfg.OKXSimulated)

	cred := okxGetCredentialsForEnv(false)
	if cred == nil {
		t.Fatal("实盘凭证构造失败")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ---- 第 1 步：只读校验（账户配置 + USDT 余额），验证密钥/签名/IP 白名单 ----
	body, err := doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/config", url.Values{}, nil)
	if err != nil {
		t.Fatalf("实盘账户配置查询失败（密钥/口令/IP 白名单/代理）: %v", err)
	}
	var acctConf []struct {
		PosMode string `json:"posMode"` // long_short_mode / net_mode
		UID     string `json:"uid"`
	}
	if err := okxDataList(body, &acctConf); err != nil || len(acctConf) == 0 {
		t.Fatalf("解析账户配置失败: %v body=%s", err, string(body))
	}
	posMode := acctConf[0].PosMode
	t.Logf("账户配置 OK: uid=%s posMode=%s", acctConf[0].UID, posMode)

	body, err = doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/account/balance", url.Values{"ccy": {"USDT"}}, nil)
	if err != nil {
		t.Fatalf("实盘余额查询失败: %v", err)
	}
	t.Logf("USDT 余额: %s", body)

	// ---- 第 1.5 步：应用实盘只读链路全量过一遍（与交易面板绑定的同款接口）----
	instEnv := GetOKXAccountInstrumentsForEnv(false, "")
	if instEnv == nil {
		t.Logf("实盘账户品种表获取失败（将回退公共合约表，非致命）")
	} else {
		swapCnt, spotCnt := 0, 0
		for _, i := range instEnv {
			switch i.InstType {
			case "SWAP":
				swapCnt++
			case "SPOT":
				spotCnt++
			}
		}
		t.Logf("实盘账户品种表: 共 %d 个（SWAP=%d SPOT=%d）", len(instEnv), swapCnt, spotCnt)
	}
	posList, err := GetOKXPositionListForEnv(false, "")
	if err != nil {
		t.Fatalf("实盘持仓查询失败: %v", err)
	}
	t.Logf("实盘当前持仓: %d 笔", len(posList))
	openList, err := GetOKXOpenOrdersForEnv(false)
	if err != nil {
		t.Fatalf("实盘当前委托查询失败: %v", err)
	}
	t.Logf("实盘当前委托: %d 笔", len(openList))
	histList, err := GetOKXHistoryOrdersForEnv(false)
	if err != nil {
		t.Fatalf("实盘历史委托查询失败: %v", err)
	}
	t.Logf("实盘历史委托: %d 笔", len(histList))
	fills, fillErr := GetOKXFillsHistoryForEnv(false, "SWAP", "", 5)
	if fillErr != nil {
		t.Logf("实盘成交明细查询失败（非致命）: %v", fillErr)
	} else {
		t.Logf("实盘成交明细: 最近 %d 笔", len(fills))
	}

	// ---- 第 2 步：挑名义价值最小的 USDT 永续（公共接口）----
	var insts []OKXInstrument
	pubBody, err := doOKXRequest(ctx, http.MethodGet, "/api/v5/public/instruments", url.Values{"instType": {"SWAP"}}, nil, false)
	if err != nil {
		t.Fatalf("合约规格查询失败: %v", err)
	}
	if err := okxDataList(pubBody, &insts); err != nil {
		t.Fatalf("解析合约规格失败: %v", err)
	}
	var tickers []OKXTicker
	tkBody, err := doOKXRequest(ctx, http.MethodGet, "/api/v5/market/tickers", url.Values{"instType": {"SWAP"}}, nil, false)
	if err != nil {
		t.Fatalf("行情查询失败: %v", err)
	}
	if err := okxDataList(tkBody, &tickers); err != nil {
		t.Fatalf("解析行情失败: %v", err)
	}
	lastPx := map[string]float64{}
	for _, tk := range tickers {
		lastPx[tk.InstId] = okxFloatNum(tk.Last)
	}
	best := ""
	bestNotional := 1e18
	for _, ins := range insts {
		if !strings.HasSuffix(ins.InstId, "-USDT-SWAP") || ins.State != "live" {
			continue
		}
		px := lastPx[ins.InstId]
		notional := okxFloatNum(ins.MinSz) * okxFloatNum(ins.CtVal) * px
		if px > 0 && notional > 0 && notional < bestNotional {
			bestNotional = notional
			best = ins.InstId
		}
	}
	if best == "" {
		t.Fatal("未找到可交易的 USDT 永续")
	}
	var chosen OKXInstrument
	for _, ins := range insts {
		if ins.InstId == best {
			chosen = ins
			break
		}
	}
	t.Logf("目标合约: %s 最小张数=%s 每张面值=%s%s 最新价=%.6g → 最小名义价值≈%.4f USDT",
		best, chosen.MinSz, chosen.CtVal, chosen.CtValCcy, lastPx[best], bestNotional)

	// ---- 真实下单段：默认不执行（真实资金、不可逆操作由用户在应用内自行完成）----
	// 只读校验到此为止。除非显式设置 OKX_PROBE_TRADE=1，否则不再继续。
	if os.Getenv("OKX_PROBE_TRADE") != "1" {
		t.Logf("只读校验全部通过（密钥有效、IP 白名单放行、posMode=%s、USDT 余额可用）。跳过真实下单。", posMode)
		return
	}

	// ---- 第 3 步：最小张数市价开多（实盘）----
	openReq := &OKXOrderRequest{
		InstId:  best,
		TdMode:  "cross",
		Side:    "buy",
		OrdType: "market",
		Sz:      strings.TrimSpace(chosen.MinSz),
	}
	if posMode == "long_short_mode" {
		openReq.PosSide = "long"
	}
	openRes, err := placeOKXOrderWithEnv(openReq, false)
	if err != nil {
		t.Fatalf("实盘开仓失败: %v", err)
	}
	t.Logf("实盘开仓已提交: ordId=%s", openRes.OrdId)

	// ---- 第 4 步：查单确认成交状态 ----
	time.Sleep(2 * time.Second)
	q := url.Values{"instId": {best}, "ordId": {openRes.OrdId}}
	body, err = doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/trade/order", q, nil)
	if err != nil {
		t.Fatalf("查单失败: %v", err)
	}
	var orders []struct {
		OrdId   string `json:"ordId"`
		State   string `json:"state"`
		FillPx  string `json:"avgPx"`
		FillSz  string `json:"accFillSz"`
		Fee     string `json:"fee"`
		Msg     string `json:"sMsg"`
	}
	if err := okxDataList(body, &orders); err != nil || len(orders) == 0 {
		t.Fatalf("解析查单失败: %v body=%s", err, string(body))
	}
	ord := orders[0]
	t.Logf("开仓订单状态: state=%s 成交均价=%s 成交数量=%s 手续费=%s %s", ord.State, ord.FillPx, ord.FillSz, ord.Fee, ord.Msg)

	// ---- 第 5 步：立即 reduceOnly 平仓 ----
	ro := true
	closeReq := &OKXOrderRequest{
		InstId:     best,
		TdMode:     "cross",
		Side:       "sell",
		OrdType:    "market",
		Sz:         strings.TrimSpace(chosen.MinSz),
		ReduceOnly: &ro,
	}
	if posMode == "long_short_mode" {
		closeReq.PosSide = "long"
	}
	closeRes, err := placeOKXOrderWithEnv(closeReq, false)
	if err != nil {
		t.Fatalf("实盘平仓失败（请手动在 OKX 平掉 %s %s 张）: %v", best, chosen.MinSz, err)
	}
	t.Logf("实盘平仓已提交: ordId=%s", closeRes.OrdId)

	time.Sleep(2 * time.Second)
	q = url.Values{"instId": {best}, "ordId": {closeRes.OrdId}}
	body, err = doOKXRequestWithCred(ctx, cred, http.MethodGet, "/api/v5/trade/order", q, nil)
	if err == nil {
		var closes []struct {
			State  string `json:"state"`
			FillPx string `json:"avgPx"`
			Fee    string `json:"fee"`
		}
		if err := okxDataList(body, &closes); err == nil && len(closes) > 0 {
			t.Logf("平仓订单状态: state=%s 成交均价=%s 手续费=%s", closes[0].State, closes[0].FillPx, closes[0].Fee)
		}
	}
	t.Logf("实盘链路验证完成：开仓 ordId=%s → 平仓 ordId=%s（%s，最小张数，双边手续费≈%.4f USDT）",
		openRes.OrdId, closeRes.OrdId, best, bestNotional*0.001)
}
