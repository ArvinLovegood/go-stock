package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/tidwall/gjson"
)

// OKX 交易的 AI 工具 handler 与共享 markdown 构建。
// 供 OpenAI 直连工具（registerToolHandler）与 eino Agent 工具（agent/tools）共用，
// 保证两条链路的输出一致。

func init() {
	registerToolHandler("GetOKXAccount", handleGetOKXAccount)
	registerToolHandler("GetOKXPositions", handleGetOKXPositions)
	registerToolHandler("GetOKXOrders", handleGetOKXOrders)
	registerToolHandler("PlaceOKXOrder", handlePlaceOKXOrder)
	registerToolHandler("CancelOKXOrder", handleCancelOKXOrder)
}

func okxToolProgress(ctx *ToolContext, funcArguments string, tip string) {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：" + ctx.FuncName + "，" + tip + "，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}
}

// okxErrorMarkdown 统一的失败提示。
func okxErrorMarkdown(what string, err error) (string, error) {
	return fmt.Sprintf("获取OKX%s失败：%v。请检查设置页对应环境的 OKX API 凭证（APIKey/SecretKey/Passphrase 口令，模拟盘/实盘两套）与网络（国内可能需要配置合约代理）。", what, err), nil
}

// okxParseEnv 解析工具参数里的 env：sim=模拟盘（默认），live=实盘。
// 空/未知值一律回退模拟盘，避免模型传错值时误触真实资金。
func okxParseEnv(funcArguments string) (simulated bool, envTag string) {
	env := strings.ToLower(strings.TrimSpace(gjson.Get(funcArguments, "env").String()))
	switch env {
	case "live", "real", "实盘":
		return true, "实盘"
	}
	return false, "模拟盘"
}

// okxEnvCredMissing 指定环境凭证未配置的提示（模拟盘/实盘是两套独立密钥）。
func okxEnvCredMissing(envTag string) string {
	return fmt.Sprintf("未配置 OKX %s环境的 API 凭证。模拟盘与实盘是两套独立密钥，请在设置页填写%s对应的 APIKey/SecretKey/Passphrase（口令）后再操作。",
		envTag, map[bool]string{true: "实盘密钥", false: "模拟盘密钥"}[envTag == "实盘"])
}

func okxTable(items []map[string]any) (string, error) {
	jsonData, err := json.Marshal(&items)
	if err != nil {
		return "", fmt.Errorf("数据格式化失败：%w", err)
	}
	return JSONToMarkdownTable(jsonData)
}

// ===== GetOKXAccount =====

// OKXAccountMarkdown 生成 OKX 账户余额 markdown（入参为工具 JSON 参数）。
// env 参数指定账户环境：sim=模拟盘（默认），live=实盘（真实资金）。
func OKXAccountMarkdown(funcArguments string) (string, error) {
	simulated, envTag := okxParseEnv(funcArguments)
	if okxGetCredentialsForEnv(simulated) == nil {
		return okxEnvCredMissing(envTag), nil
	}

	bal, err := GetOKXBalanceForEnv(simulated)
	if err != nil {
		return okxErrorMarkdown("账户余额（"+envTag+"）", err)
	}

	var sb strings.Builder
	sb.WriteString("\r\n ### OKX 账户余额（" + envTag + "）：\r\n")
	sb.WriteString(fmt.Sprintf("- 总权益：约 %s USD\r\n", okxTrimNum(bal.TotalEq)))
	if cfg, err := GetOKXAccountConfigForEnv(simulated); err == nil {
		sb.WriteString(fmt.Sprintf("- 持仓模式：%s\r\n", okxPosModeText(cfg.PosMode)))
	}

	items := make([]map[string]any, 0, len(bal.Details))
	for _, d := range bal.Details {
		eq := okxFloatNum(d.Eq)
		avail := okxFloatNum(d.AvailEq)
		if avail == 0 {
			avail = okxFloatNum(d.AvailBal)
		}
		if eq == 0 && avail == 0 && okxFloatNum(d.FrozenBal) == 0 {
			continue
		}
		items = append(items, map[string]any{
			"币种":      d.Ccy,
			"权益(USD)": okxTrimNum(d.Eq),
			"可用":      okxTrimNum(availableOf(d)),
			"冻结":      okxTrimNum(d.FrozenBal),
		})
	}
	if len(items) > 0 {
		table, err := okxTable(items)
		if err != nil {
			return "", err
		}
		sb.WriteString("\r\n" + table + "\r\n")
	}
	sb.WriteString("\r\n提示：如需查看杠杆仓位与未实现盈亏，可继续调用 GetOKXPositions。\r\n")
	return sb.String(), nil
}

// availableOf 取可用（先 availEq，退 availBal）。
func availableOf(d OKXBalanceDetail) string {
	if strings.TrimSpace(d.AvailEq) != "" {
		return d.AvailEq
	}
	return d.AvailBal
}

func handleGetOKXAccount(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	okxToolProgress(ctx, funcArguments, "查询OKX账户余额")
	res, err := OKXAccountMarkdown(funcArguments)
	return okxAppendResult(ctx, funcArguments, res, err)
}

// ===== GetOKXPositions =====

// OKXPositionsMarkdown 生成 OKX 持仓 markdown（入参为工具 JSON 参数）。
// env 参数指定账户环境：sim=模拟盘（默认），live=实盘（真实资金）。
func OKXPositionsMarkdown(funcArguments string) (string, error) {
	simulated, envTag := okxParseEnv(funcArguments)
	instId := ""
	if raw := strings.TrimSpace(gjson.Get(funcArguments, "instId").String()); raw != "" {
		resolved, _, ok := ResolveOKXInstId(raw, "")
		if !ok {
			return fmt.Sprintf("%s：无法识别的 OKX 品种标识（如 btc、BTC-USDT-SWAP、BTC-USDT）。", raw), nil
		}
		instId = resolved
	}

	if okxGetCredentialsForEnv(simulated) == nil {
		return okxEnvCredMissing(envTag), nil
	}
	positions, err := GetOKXPositionListForEnv(simulated, instId)
	if err != nil {
		return okxErrorMarkdown("持仓（"+envTag+"）", err)
	}
	if len(positions) == 0 {
		return envTag + "账户当前无持仓（或该品种无持仓）。", nil
	}

	items := make([]map[string]any, 0, len(positions))
	for _, p := range positions {
		uplRatio := okxFloatNum(p.UplRatio) * 100
		lever := strings.TrimSpace(p.Lever)
		if lever == "" {
			lever = "-"
		}
		posNote := okxTrimNum(p.Pos)
		if strings.HasSuffix(p.InstId, "-SWAP") {
			posNote += " 张"
		} else if p.PosCcy != "" {
			posNote += " " + p.PosCcy
		}
		items = append(items, map[string]any{
			"品种":        OKXInstDisplayName(p.InstId),
			"标识":        p.InstId,
			"方向":        okxPosSideText(p.PosSide),
			"数量":        posNote,
			"开仓均价":      okxTrimNum(p.AvgPx),
			"标记价":       okxTrimNum(p.MarkPx),
			"未实现盈亏":     okxTrimNum(p.Upl),
			"收益率(%)":    convertor.ToString(round4(uplRatio)),
			"杠杆":        lever,
			"保证金模式":     mapMarginMode(p.MgnMode),
			"预估强平价":     okxTrimNum(p.LiqPx),
			"名义价值(USD)": okxTrimNum(p.NotionalUsd),
		})
	}
	table, err := okxTable(items)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("\r\n ### OKX 当前持仓 · %s（共 %d 条）：\r\n%s\r\n", envTag, len(items), table), nil
}

func mapMarginMode(mgnMode string) string {
	switch mgnMode {
	case "cross":
		return "全仓"
	case "isolated":
		return "逐仓"
	case "cash":
		return "现货"
	}
	return mgnMode
}

func handleGetOKXPositions(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	okxToolProgress(ctx, funcArguments, "查询OKX持仓")
	res, err := OKXPositionsMarkdown(funcArguments)
	return okxAppendResult(ctx, funcArguments, res, err)
}

// ===== GetOKXOrders =====

// OKXOrdersMarkdown 生成 OKX 订单 markdown（入参为工具 JSON 参数）。
// env 参数指定账户环境：sim=模拟盘（默认），live=实盘（真实资金）。
func OKXOrdersMarkdown(funcArguments string) (string, error) {
	simulated, envTag := okxParseEnv(funcArguments)
	status := strings.ToLower(strings.TrimSpace(gjson.Get(funcArguments, "status").String()))
	if status == "" {
		status = "pending"
	}
	pending := status == "pending" || status == "live" || status == "待成交" || status == "当前"

	instType := strings.TrimSpace(gjson.Get(funcArguments, "instType").String())
	instId := ""
	if raw := strings.TrimSpace(gjson.Get(funcArguments, "instId").String()); raw != "" {
		resolved, resolvedType, ok := ResolveOKXInstId(raw, "")
		if !ok {
			return fmt.Sprintf("%s：无法识别的 OKX 品种标识（如 btc、BTC-USDT-SWAP、BTC-USDT）。", raw), nil
		}
		instId = resolved
		if instType == "" {
			instType = resolvedType
		}
	}
	limit := int(gjson.Get(funcArguments, "limit").Int())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	cred := okxGetCredentialsForEnv(simulated)
	if cred == nil {
		return okxEnvCredMissing(envTag), nil
	}
	var orders []OKXOrder
	var err error
	if pending {
		orders, err = okxOrdersByAPIWithCred(true, instType, instId, limit, cred)
	} else {
		orders, err = okxOrdersByAPIWithCred(false, instType, instId, limit, cred)
	}
	if err != nil {
		return okxErrorMarkdown("订单（"+envTag+"）", err)
	}
	if len(orders) == 0 {
		return envTag + "账户未查询到" + mapOrderScope(pending) + "订单。", nil
	}

	items := make([]map[string]any, 0, len(orders))
	for _, od := range orders {
		items = append(items, map[string]any{
			"时间":   okxMsToTime(od.UTime),
			"品种":   OKXInstDisplayName(od.InstId),
			"标识":   od.InstId,
			"方向":   okxPosSideText(od.PosSide) + "·" + okxSideText(od.Side),
			"类型":   okxOrdTypeText(od.OrdType),
			"委托价":  okxTrimNum(od.Px),
			"数量":   okxTrimNum(od.Sz),
			"已成交":  okxTrimNum(od.AccFillSz),
			"成交均价": okxTrimNum(od.AvgPx),
			"状态":   okxStateText(od.State),
			"只减仓":  od.ReduceOnly,
			"订单号":  od.OrdId,
		})
	}
	table, err := okxTable(items)
	if err != nil {
		return "", err
	}
	title := mapOrderScope(pending)
	return fmt.Sprintf("\r\n ### OKX %s订单 · %s（共 %d 条）：\r\n%s\r\n", title, envTag, len(items), table), nil
}

func mapOrderScope(pending bool) string {
	if pending {
		return "待成交"
	}
	return "历史"
}

func okxSideText(side string) string {
	if side == "buy" {
		return "买入"
	}
	if side == "sell" {
		return "卖出"
	}
	return side
}

func okxOrdTypeText(ordType string) string {
	switch ordType {
	case "limit":
		return "限价"
	case "market":
		return "市价"
	case "post_only":
		return "只做maker"
	case "fok":
		return "FOK"
	case "ioc":
		return "IOC"
	}
	return ordType
}

func handleGetOKXOrders(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	okxToolProgress(ctx, funcArguments, "查询OKX订单")
	res, err := OKXOrdersMarkdown(funcArguments)
	return okxAppendResult(ctx, funcArguments, res, err)
}

// ===== PlaceOKXOrder =====

// OKXPlaceOrderMarkdown 执行下单并生成结果 markdown（入参为工具 JSON 参数）。
// env 参数指定交易环境：sim=模拟盘（默认），live=实盘（真实资金，下单前必须与用户确认）。
func OKXPlaceOrderMarkdown(funcArguments string) (string, error) {
	simulated, envTag := okxParseEnv(funcArguments)
	instIdRaw := strings.TrimSpace(gjson.Get(funcArguments, "instId").String())
	if instIdRaw == "" {
		return "参数 instId 不能为空，请传入品种标识（如 btc、BTC-USDT-SWAP、BTC-USDT）。", nil
	}
	instTypeHint := strings.TrimSpace(gjson.Get(funcArguments, "instType").String())
	instId, instType, ok := ResolveOKXInstId(instIdRaw, instTypeHint)
	if !ok {
		return fmt.Sprintf("%s：无法识别的 OKX 品种标识（如 btc、BTC-USDT-SWAP、BTC-USDT）。", instIdRaw), nil
	}

	side := strings.ToLower(strings.TrimSpace(gjson.Get(funcArguments, "side").String()))
	if side != "buy" && side != "sell" {
		return "参数 side 必须为 buy（买入/开多）或 sell（卖出/开空）。", nil
	}
	sz := strings.TrimSpace(gjson.Get(funcArguments, "sz").String())
	if sz == "" {
		return "参数 sz（下单数量）不能为空。注意：SWAP 以「张」为单位（一张=ctVal 个币），SPOT 以基础币数量为单位。", nil
	}

	req := &OKXOrderRequest{
		InstId:  instId,
		Side:    side,
		Sz:      sz,
		OrdType: strings.ToLower(strings.TrimSpace(gjson.Get(funcArguments, "ordType").String())),
		Px:      okxTrimNum(gjson.Get(funcArguments, "px").String()),
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "tdMode").String()); v != "" {
		req.TdMode = v
	}
	if v := strings.TrimSpace(gjson.Get(funcArguments, "posSide").String()); v != "" {
		req.PosSide = strings.ToLower(v)
	}
	if gjson.Get(funcArguments, "reduceOnly").Exists() {
		ro := gjson.Get(funcArguments, "reduceOnly").Bool()
		req.ReduceOnly = &ro
	}
	// 附带止盈止损
	slTrigger := strings.TrimSpace(gjson.Get(funcArguments, "slTriggerPx").String())
	tpTrigger := strings.TrimSpace(gjson.Get(funcArguments, "tpTriggerPx").String())
	if slTrigger != "" || tpTrigger != "" {
		algo := OKXAttachAlgoOrd{}
		if slTrigger != "" {
			algo.SlTriggerPx = okxTrimNum(slTrigger)
			algo.SlOrdPx = "-1" // 市价止损
		}
		if tpTrigger != "" {
			algo.TpTriggerPx = okxTrimNum(tpTrigger)
			algo.TpOrdPx = "-1" // 市价止盈
		}
		req.AttachAlgoOrds = []OKXAttachAlgoOrd{algo}
	}

	// env=live 时走实盘密钥动真实资金，输出与提示里明确标注环境
	res, err := SubmitOKXOrderForEnv(simulated, req)
	if err != nil {
		return fmt.Sprintf("OKX下单失败（%s，%s %s %s）：%v。"+
			"常见原因：%s环境未配置凭证、交易开关未开启、Passphrase 口令错误、账户为双向持仓模式但未传 posSide（long/short）、数量或价格不符合规格、IP 不在白名单。",
			envTag, instId, okxSideText(side), sz, err, envTag), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\r\n ### ✅ OKX 下单成功（%s，订单号 %s）\r\n", envTag, res.OrdId))
	sb.WriteString(fmt.Sprintf("- 品种：%s（%s）\r\n", OKXInstDisplayName(instId), instId))
	sb.WriteString(fmt.Sprintf("- 方向：%s\r\n", okxSideText(side)))
	if req.PosSide != "" {
		sb.WriteString(fmt.Sprintf("- 持仓方向：%s\r\n", okxPosSideText(req.PosSide)))
	}
	sb.WriteString(fmt.Sprintf("- 类型：%s\r\n", okxOrdTypeText(req.OrdType)))
	sb.WriteString(fmt.Sprintf("- 数量：%s（%s）\r\n", sz, instType))
	if req.Px != "" {
		sb.WriteString(fmt.Sprintf("- 委托价：%s\r\n", req.Px))
	}
	sb.WriteString(fmt.Sprintf("- 保证金模式：%s\r\n", mapMarginMode(req.TdMode)))
	if envTag == "实盘" {
		sb.WriteString("- ⚠️ 本次下单使用**实盘真实资金**\r\n")
	}
	if req.ReduceOnly != nil && *req.ReduceOnly {
		sb.WriteString("- 只减仓：是\r\n")
	}
	if len(req.AttachAlgoOrds) > 0 {
		algo := req.AttachAlgoOrds[0]
		if algo.SlTriggerPx != "" {
			sb.WriteString(fmt.Sprintf("- 止损触发价：%s（市价止损）\r\n", algo.SlTriggerPx))
		}
		if algo.TpTriggerPx != "" {
			sb.WriteString(fmt.Sprintf("- 止盈触发价：%s（市价止盈）\r\n", algo.TpTriggerPx))
		}
	}
	sb.WriteString("\r\n可调用 GetOKXOrders（status=pending）查看委托，或 GetOKXPositions 查看持仓。\r\n")
	return sb.String(), nil
}

func handlePlaceOKXOrder(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	okxToolProgress(ctx, funcArguments, "OKX下单")
	res, err := OKXPlaceOrderMarkdown(funcArguments)
	return okxAppendResult(ctx, funcArguments, res, err)
}

// ===== CancelOKXOrder =====

// OKXCancelOrderMarkdown 执行撤单并生成结果 markdown（入参为工具 JSON 参数）。
// env 参数指定交易环境：sim=模拟盘（默认），live=实盘（真实资金）。
func OKXCancelOrderMarkdown(funcArguments string) (string, error) {
	simulated, envTag := okxParseEnv(funcArguments)
	instIdRaw := strings.TrimSpace(gjson.Get(funcArguments, "instId").String())
	ordId := strings.TrimSpace(gjson.Get(funcArguments, "ordId").String())
	if instIdRaw == "" || ordId == "" {
		return "撤单需要 instId 与 ordId 两个参数。可先调用 GetOKXOrders（status=pending）获取待成交订单号。", nil
	}
	instId, _, ok := ResolveOKXInstId(instIdRaw, "")
	if !ok {
		return fmt.Sprintf("%s：无法识别的 OKX 品种标识。", instIdRaw), nil
	}

	res, err := RevokeOKXOrderForEnv(simulated, instId, ordId)
	if err != nil {
		return fmt.Sprintf("OKX撤单失败（%s，instId=%s ordId=%s）：%v。常见原因：%s环境未配置凭证、交易开关未开启、订单已成交/已撤销、Passphrase 口令错误。",
			envTag, instId, ordId, err, envTag), nil
	}
	return fmt.Sprintf("\r\n ### ✅ OKX 撤单请求已提交（%s）\r\n- 品种：%s\r\n- 订单号：%s\r\n\r\n可调用 GetOKXOrders（status=history）确认最终状态。\r\n",
		envTag, OKXInstDisplayName(instId), res.OrdId), nil
}

func handleCancelOKXOrder(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	okxToolProgress(ctx, funcArguments, "OKX撤单")
	res, err := OKXCancelOrderMarkdown(funcArguments)
	return okxAppendResult(ctx, funcArguments, res, err)
}

// ===== 公共 =====

// okxAppendResult 统一的 tool 消息追加。
func okxAppendResult(ctx *ToolContext, funcArguments, res string, err error) error {
	msg := res
	if err != nil {
		msg = err.Error()
	}
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(),
		ctx.CurrentCallID, ctx.FuncName, funcArguments, msg)
	return nil
}
