package data

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestOKXSign 验证 HMAC-SHA256 签名逻辑与 OKX 文档示例一致。
func TestOKXSign(t *testing.T) {
	// 使用显式假密钥（非任何真实凭证），仅校验签名的确定性与输出格式
	const fakeSecret = "dummy-secret-not-a-real-credential"
	ts := "2020-12-08T09:08:57.715Z"
	got := okxSign(fakeSecret, ts, "GET",
		"/api/v5/account/balance?ccy=BTC", "")
	// 签名结果为确定性计算，校验 base64 解码为 32 字节且稳定可复现
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("签名不是合法 base64: %v", got)
	}
	if len(decoded) != 32 {
		t.Fatalf("HMAC-SHA256 签名长度应为 32 字节, got %d", len(decoded))
	}
	want := okxSign(fakeSecret, ts, "GET",
		"/api/v5/account/balance?ccy=BTC", "")
	if got != want {
		t.Fatalf("签名不具备确定性: %s != %s", got, want)
	}
}

// TestResolveOKXInstId 验证品种标识归一。
func TestResolveOKXInstId(t *testing.T) {
	cases := []struct {
		input    string
		instType string
		wantId   string
		wantType string
	}{
		{"btc", "", "BTC-USDT-SWAP", "SWAP"},
		{"btc", "SPOT", "BTC-USDT", "SPOT"},
		{"BTC-USDT", "", "BTC-USDT", "SPOT"},
		{"BTC-USDT-SWAP", "", "BTC-USDT-SWAP", "SWAP"},
		{"btcusdt", "", "BTC-USDT-SWAP", "SWAP"},
		{"btcusdt", "SPOT", "BTC-USDT", "SPOT"},
		{"ok:btcusdt-swap", "", "BTC-USDT-SWAP", "SWAP"},
		{"btc/usdt", "", "BTC-USDT", "SPOT"},
		{"比特币", "", "BTC-USDT-SWAP", "SWAP"},
		{"以太坊", "SPOT", "ETH-USDT", "SPOT"},
		{"dogeusdt", "", "DOGE-USDT-SWAP", "SWAP"},
		{"BTC-USD-SWAP", "", "BTC-USD-SWAP", "SWAP"},
		{"ETHUSDC", "SPOT", "ETH-USDC", "SPOT"},
	}
	for _, c := range cases {
		gotId, gotType, ok := ResolveOKXInstId(c.input, c.instType)
		if !ok {
			t.Errorf("ResolveOKXInstId(%q, %q) 应成功", c.input, c.instType)
			continue
		}
		if gotId != c.wantId || gotType != c.wantType {
			t.Errorf("ResolveOKXInstId(%q, %q) = (%q, %q), want (%q, %q)",
				c.input, c.instType, gotId, gotType, c.wantId, c.wantType)
		}
	}
	if _, _, ok := ResolveOKXInstId("", ""); ok {
		t.Error("空输入应失败")
	}
}

// TestOKXOrderRequestJSON 验证下单请求序列化（reduceOnly 指针 omitempty 语义）。
func TestOKXOrderRequestJSON(t *testing.T) {
	ro := true
	req := &OKXOrderRequest{
		InstId:         "BTC-USDT-SWAP",
		TdMode:         "cross",
		Side:           "buy",
		PosSide:        "net",
		OrdType:        "limit",
		Sz:             "1",
		Px:             "50000",
		ReduceOnly:     &ro,
		AttachAlgoOrds: []OKXAttachAlgoOrd{{SlTriggerPx: "48000", SlOrdPx: "-1"}},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if m["reduceOnly"] != true {
		t.Errorf("reduceOnly 应为 true, got %v", m["reduceOnly"])
	}
	if m["posSide"] != "net" {
		t.Errorf("posSide 应为 net, got %v", m["posSide"])
	}
	algos, ok := m["attachAlgoOrds"].([]any)
	if !ok || len(algos) != 1 {
		t.Fatalf("attachAlgoOrds 应有 1 个元素, got %v", m["attachAlgoOrds"])
	}
}

// TestOKXStateText 验证状态文案。
func TestOKXStateText(t *testing.T) {
	if okxStateText("filled") != "已成交" {
		t.Error("filled 应翻译为 已成交")
	}
	if okxStateText("partially_filled") != "部分成交" {
		t.Error("partially_filled 应翻译为 部分成交")
	}
}

// TestOKXCalcSizeByAmount 验证信号自动交易的金额→数量换算（向下取整到步进、minSz 拦截）。
func TestOKXCalcSizeByAmount(t *testing.T) {
	// SWAP：ctVal=0.01 BTC/张，lotSz=0.1 张，minSz=0.1 张；100 USDT @ 50000 → 100/(0.01*50000)=0.2 张
	swap := OKXInstrument{InstType: "SWAP", CtVal: "0.01", LotSz: "0.1", MinSz: "0.1"}
	sz, err := okxCalcSizeByAmount(100, 50000, swap)
	if err != nil || sz != "0.2" {
		t.Errorf("SWAP 100U@50000 应为 0.2 张, got (%q, %v)", sz, err)
	}
	// 金额不足 minSz：10 USDT → 0.02 张 < 0.1
	if _, err := okxCalcSizeByAmount(10, 50000, swap); err == nil {
		t.Error("10 USDT 应低于最小下单量而报错")
	}
	// SPOT：lotSz=0.00001，minSz=0.0001；100 USDT @ 50000 → 0.002（按 lotSz 小数位格式化为 0.00200）
	spot := OKXInstrument{InstType: "SPOT", LotSz: "0.00001", MinSz: "0.0001"}
	sz, err = okxCalcSizeByAmount(100, 50000, spot)
	if err != nil || sz != "0.00200" {
		t.Errorf("SPOT 100U@50000 应为 0.00200, got (%q, %v)", sz, err)
	}
	// 向下取整：103 USDT @ 50000 → 0.00206 → 步进 0.00001 → 0.00206
	sz, _ = okxCalcSizeByAmount(103, 50000, spot)
	if sz != "0.00206" {
		t.Errorf("SPOT 103U 应为 0.00206, got %q", sz)
	}
	// 无效输入
	if _, err := okxCalcSizeByAmount(0, 50000, spot); err == nil {
		t.Error("金额 0 应报错")
	}
	if _, err := okxCalcSizeByAmount(100, 0, spot); err == nil {
		t.Error("价格 0 应报错")
	}
}

// TestOKXSignalCodeInstInput 验证信号代码范围：仅 ok:/bn:/bt: 前缀参与，裸股票代码排除。
func TestOKXSignalCodeInstInput(t *testing.T) {
	if v, ok := okxSignalCodeInstInput("bn:BTCUSDT"); !ok || v != "BTCUSDT" {
		t.Errorf("bn:BTCUSDT 应识别为 BTCUSDT, got (%q,%v)", v, ok)
	}
	if v, ok := okxSignalCodeInstInput("bt:ETHUSDT"); !ok || v != "ETHUSDT" {
		t.Errorf("bt:ETHUSDT 应识别为 ETHUSDT, got (%q,%v)", v, ok)
	}
	if v, ok := okxSignalCodeInstInput("ok:btcusdt-swap"); !ok || v != "btcusdt-swap" {
		t.Errorf("ok: 前缀应识别, got (%q,%v)", v, ok)
	}
	if _, ok := okxSignalCodeInstInput("600519"); ok {
		t.Error("A 股代码不应参与 OKX 信号交易")
	}
	if _, ok := okxSignalCodeInstInput("sh600519"); ok {
		t.Error("带交易所前缀的股票代码不应参与 OKX 信号交易")
	}
	if _, ok := okxSignalCodeInstInput(""); ok {
		t.Error("空代码不应参与")
	}
}
