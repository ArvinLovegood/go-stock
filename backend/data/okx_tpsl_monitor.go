package data

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// 每笔持仓止盈止损监控。
//
// 设置页开启「止盈止损(持仓收益)」后，后台每 10 秒分别拉取 OKX 模拟盘与实盘两个
// 独立账户的全部持仓（两环境密钥各自独立、互不可见），任一持仓达到平仓线即立即
// 市价 reduceOnly 平仓。阈值两种模式（同一组设置参数两环境共用）：
//
//  1. fixed 固定收益率（默认）：用 OKX uplRatio（按保证金计、含杠杆、多空同号）
//     与固定百分比比较：收益率 ≥ 止盈% → 止盈；≤ -止损% → 止损。
//
//  2. vol 波动率自适应：按各合约近 N 根参考周期 K 线的平均振幅（(high-low)/open×100）
//     动态计算，止盈线 = 振幅×止盈倍数、止损线 = 振幅×止损倍数；收益口径用
//     「未实现盈亏 ÷ 开仓名义价值」（价格口径，与杠杆无关），与振幅口径一致。
//     波动大的合约阈值自动放宽、波动小的自动收紧，避免固定百分比失配。
//
// 手动开仓（合约行情页模拟盘/实盘页签）、信号自动下单开出的仓一视同仁。
// 平仓失败按指数退避重试（60s→2min→…封顶 30min）；OKX 51169（该方向无仓可平）冻结 10 分钟再试，
// 防止对已不存在的持仓死循环重试刷失败流水。平仓成功后持仓消失自然不再触发。
// 某环境密钥未配置时跳过该环境并只警告一次，避免每 10 秒刷屏。
//
// 与下单时附带的止盈止损（AttachAlgoOrd，交易所侧价格触发）互补。

const (
	okxTPSLInterval     = 10 * time.Second
	okxTPSLRetryGap     = 60 * time.Second
	okxTPSLMaxGap       = 30 * time.Minute // 连续失败的退避上限
	okxTPSLGhostTTL     = 10 * time.Minute // OKX 51169（无仓可平）的冻结时长
	okxTPSLVolTTL       = 5 * time.Minute  // 单合约波动率缓存
	okxTPSLDefaultKlt   = "15m"
	okxTPSLDefaultBars  = 20
	okxTPSLDefaultTpMul = 1.5
	okxTPSLDefaultSlMul = 1.0
)

var okxTPSLStartOnce sync.Once

// StartOKXTPSLMonitor 启动止盈止损监控守护协程（幂等，应用启动时调用一次）。
func StartOKXTPSLMonitor() {
	okxTPSLStartOnce.Do(func() {
		go okxTPSLMonitorLoop()
	})
}

// okxTPSLKey 平仓重试间隔的去重键：环境 + 合约 + 方向（两环境账户独立，同合约互不干扰）。
type okxTPSLKey struct {
	simulated bool
	instId    string
	posSide   string
}

// okxTPSLAttempt 单持仓的重试状态：上次尝试时间 + 连续失败次数（指数退避）+ 无仓可平冻结期。
// 实测教训：SOXL 实盘一笔持仓对 OKX 51169（该方向无仓可平）每 60s 死循环重试，
// 两天刷出 1800+ 条失败流水且永不停止——冻结 + 退避是必须的。
type okxTPSLAttempt struct {
	last       time.Time
	fails      int
	ghostUntil time.Time
}

// okxTPSLCredWarned 某环境凭证缺失只警告一次的标记（避免每 10 秒刷屏）。
var okxTPSLCredWarned = struct {
	sync.Mutex
	m map[bool]bool
}{m: make(map[bool]bool)}

// okxTPSLWarnCredMissing 环境凭证缺失警告：每环境只记一次。
func okxTPSLWarnCredMissing(simulated bool) {
	okxTPSLCredWarned.Lock()
	defer okxTPSLCredWarned.Unlock()
	if okxTPSLCredWarned.m[simulated] {
		return
	}
	okxTPSLCredWarned.m[simulated] = true
	env := "模拟盘"
	if !simulated {
		env = "实盘"
	}
	logger.SugaredLogger.Warnf("[止盈止损] %s环境未配置 API 凭证，跳过该环境监控", env)
}

// okxTPSLEnvTag 环境中文名（日志前缀用）。
func okxTPSLEnvTag(simulated bool) string {
	if simulated {
		return "模拟盘"
	}
	return "实盘"
}

func okxTPSLMonitorLoop() {
	lastAttempt := make(map[okxTPSLKey]okxTPSLAttempt)
	for {
		time.Sleep(okxTPSLInterval)
		if db.Dao == nil {
			continue
		}
		s := GetSettingConfig()
		if s == nil || s.Settings == nil || !s.OKXTPSLEnable {
			continue
		}
		cfg := okxTPSLConfig()
		if cfg.Mode == "fixed" && cfg.TpPct <= 0 && cfg.SlPct <= 0 {
			continue
		}
		// 模拟盘与实盘是两个独立账户（密钥各自独立、持仓互不可见），
		// 阈值参数共用同一组设置，每轮分别检查两环境持仓并各自平仓。
		for _, simulated := range []bool{true, false} {
			cred := okxGetCredentialsForEnv(simulated)
			if cred == nil {
				okxTPSLWarnCredMissing(simulated)
				continue
			}
			positions, err := okxPositionsForEnv("", simulated)
			if err != nil {
				// 凭证/网络问题静默重试，避免刷日志
				continue
			}
			now := time.Now()
			envTag := okxTPSLEnvTag(simulated)
			for _, p := range positions {
				action := ""
				line := 0.0
				roi := 0.0
				if cfg.Mode == "vol" {
					amp := okxAvgAmplitudePct(p.InstId, cfg.VolKlt, cfg.VolBars)
					if amp <= 0 {
						continue
					}
					roi = okxPriceRoiPct(p)
					tpLine := amp * cfg.TpMult
					slLine := -amp * cfg.SlMult
					if roi >= tpLine {
						action = "止盈"
						line = tpLine
					} else if roi <= slLine {
						action = "止损"
						line = slLine
					}
				} else {
					roi = okxPctNum(p.UplRatio) * 100 // uplRatio 为小数（0.05 = 5%）
					if cfg.TpPct > 0 && roi >= cfg.TpPct {
						action = "止盈"
						line = cfg.TpPct
					} else if cfg.SlPct > 0 && roi <= -cfg.SlPct {
						action = "止损"
						line = -cfg.SlPct
					}
				}
				if action == "" {
					continue
				}
				key := okxTPSLKey{simulated: simulated, instId: p.InstId, posSide: p.PosSide}
				if st, ok := lastAttempt[key]; ok {
					// 无仓可平冻结期（OKX 51169）：交易所已明确该方向没有可减/可平仓位，
					// 原地重试只会刷失败流水，冻结到期后若持仓仍存在且仍触发再恢复尝试。
					if now.Before(st.ghostUntil) {
						continue
					}
					// 连续失败指数退避：60s→2min→4min…封顶 30min，持久性错误不占满 API 频率
					shift := st.fails
					if shift > 5 {
						shift = 5
					}
					gap := okxTPSLRetryGap * (1 << shift)
					if gap > okxTPSLMaxGap {
						gap = okxTPSLMaxGap
					}
					if now.Sub(st.last) < gap {
						continue
					}
				}
				res, err := CloseOKXPositionForEnv(simulated, p.InstId, p.PosSide, p.Pos)
				// 平仓流水：family=tpsl 与开仓（buysell）/反向平仓（reverse）区分，成功失败均记录
				//（失败按指数退避重试、51169 冻结，不会刷屏）；参考价用持仓标记价。
				closeRec := okxTPSLCloseRecord(simulated, p, action, roi, line)
				if err != nil || res == nil || res.SCode != "0" {
					msg := ""
					if err != nil {
						msg = err.Error()
					} else if res != nil {
						msg = res.SMsg
					}
					closeRec.Status = "failed"
					closeRec.ErrMsg = fmt.Sprintf("%s: %s", closeRec.ErrMsg, msg)
					okxSaveSignalTradeRecord(closeRec)
					st := lastAttempt[key]
					st.last, st.fails = now, st.fails+1
					// OKX 51169：交易所确认该方向无仓可平——持仓多半已不存在（手动平仓/交易所侧
					// 止盈止损已触发）或持仓模式不匹配（净持仓 vs 双向），重试无意义，冻结一段时长。
					if strings.Contains(msg, "51169") {
						st.ghostUntil = now.Add(okxTPSLGhostTTL)
						logger.SugaredLogger.Warnf("[止盈止损·%s] %s 平仓报 OKX 51169（该方向无仓可平），冻结重试 %s；请核对交易所实际持仓与持仓模式设置",
							envTag, p.InstId, okxTPSLGhostTTL)
					}
					lastAttempt[key] = st
					logger.SugaredLogger.Errorf("[止盈止损·%s] %s平仓失败 %s posSide=%s 收益=%.2f%% 平仓线=%.2f%%: %s",
						envTag, action, p.InstId, p.PosSide, roi, line, msg)
					continue
				}
				lastAttempt[key] = okxTPSLAttempt{last: now} // 成功：重置退避与冻结
				closeRec.Status = "success"
				closeRec.OrdId = res.OrdId
				okxSaveSignalTradeRecord(closeRec)
				// 撤销交易所侧孤儿条件单：持仓已由软件侧市价平掉，未触发的 attachAlgoOrds
				// 若不撤销，同合约冷却后再开仓时会被旧条件单误触发平掉新仓。
				go okxCancelInstPendingAlgos(simulated, p.InstId)
				// 登记同向开仓冷却：被平仓头寸的开仓方向（多头=buy/空头=sell）在冷却期内不再自动开仓
				openDir := "sell"
				if p.PosSide == "long" || (p.PosSide == "net" && okxPctNum(p.Pos) > 0) {
					openDir = "buy"
				}
				if cd := okxSignalTradeConfig().ReentryCooldown; cd > 0 {
					okxMarkTPSLCooldown(simulated, p.InstId, openDir, cd)
				}
				logger.SugaredLogger.Infof("[止盈止损·%s] %s平仓已提交 %s posSide=%s 收益=%.2f%% 平仓线=%.2f%% 数量=%s 订单号=%s 同向冷却至 %s",
					envTag, action, p.InstId, p.PosSide, roi, line, p.Pos, res.OrdId,
					time.Now().Add(okxSignalTradeConfig().ReentryCooldown).Format("15:04:05"))
			}
		}
	}
}

// okxTPSLCloseRecord 构造止盈止损平仓流水（family=tpsl，kind=tp/sl）。
// 平仓方向：多头/净多卖出（sell）、空头/净空买入（buy）；参考价用持仓标记价；
// 代码按 OKX 口径反推（ok: 前缀剥 -SWAP 后缀），无原始信号上下文故 klt/评分为空。
func okxTPSLCloseRecord(simulated bool, p OKXPosition, action string, roi, line float64) models.SignalTradeRecord {
	posAbs := math.Abs(okxPctNum(p.Pos))
	closeSide := "sell"
	switch {
	case p.PosSide == "short":
		closeSide = "buy"
	case p.PosSide == "net" && okxPctNum(p.Pos) < 0:
		closeSide = "buy"
	}
	instType := ""
	if strings.HasSuffix(strings.ToUpper(p.InstId), "-SWAP") {
		instType = "SWAP"
	}
	// kind 统一用英文枚举（tp/sl），供前端面板按类型渲染（action 为中文标签仅用于日志）
	kind := "sl"
	if strings.Contains(action, "盈") {
		kind = "tp"
	}
	refPrice := okxPctNum(p.MarkPx)
	return models.SignalTradeRecord{
		Code:       "ok:" + strings.TrimSuffix(p.InstId, "-SWAP"),
		InstId:     p.InstId,
		InstType:   instType,
		Family:     "tpsl",
		Kind:       kind,
		BarTime:    time.Now().Unix(),
		Name:       p.InstId,
		Side:       closeSide,
		OrdType:    "market",
		Sz:         strconv.FormatFloat(posAbs, 'f', -1, 64),
		RefPrice:   refPrice,
		Env:        okxTradeEnvLabel(simulated),
		ErrMsg:     fmt.Sprintf("收益=%.2f%% 平仓线=%.2f%%", roi, line), // 触发上下文；失败时追加错误原因
		AmountUSDT: okxNotionalBySz(p.InstId, strconv.FormatFloat(posAbs, 'f', -1, 64), refPrice),
		At:         time.Now().UnixMilli(),
	}
}

// ===== 交易所侧附带止盈止损（开仓 attachAlgoOrds）=====
// 与软件侧 10 秒轮询互补：应用退出/断网期间由交易所侧价格触发兜底。
// 触发价与软件监控同一套配置（fixed/vol）计算，双方谁先到线谁先平；
// 软件侧市价平仓成功后须撤销交易所侧孤儿条件单（okxCancelInstPendingAlgos），
// 否则同合约再开仓时旧条件单误触发会平掉新仓。

// okxAlignPxToTick 按 tickSz 对齐价格（取最近档位），tickSz 无效时原样返回。
// 经固定小数位舍入消除浮点乘法尾数（如 104*0.001=0.10400000000000001），
// 否则触发价字符串带长尾会被 OKX 以价格精度错误拒绝（51006）。
func okxAlignPxToTick(px, tickSz float64) float64 {
	if tickSz <= 0 || px <= 0 {
		return px
	}
	aligned := math.Round(px/tickSz) * tickSz
	d := 0
	if i := strings.IndexByte(strconv.FormatFloat(tickSz, 'f', -1, 64), '.'); i >= 0 {
		d = len(strconv.FormatFloat(tickSz, 'f', -1, 64)) - i - 1
	}
	clean, err := strconv.ParseFloat(strconv.FormatFloat(aligned, 'f', d, 64), 64)
	if err != nil {
		return aligned
	}
	return clean
}

// okxComputeTPSLPx 按方向与价格口径阈值计算止盈/止损触发价（% → 价格）。
// 开多：TP 在上方、SL 在下方；开空相反。任一侧阈值 <=0 表示该侧不挂。
// 返回 ok=false 表示计算结果非法（阈值方向错误或触发价不高于 0），调用方应放弃附加。
func okxComputeTPSLPx(side string, refPrice, tpPct, slPct, tickSz float64) (tpPx, slPx float64, ok bool) {
	if refPrice <= 0 {
		return 0, 0, false
	}
	if tpPct > 0 {
		if side == "buy" {
			tpPx = okxAlignPxToTick(refPrice*(1+tpPct/100), tickSz)
		} else {
			tpPx = okxAlignPxToTick(refPrice*(1-tpPct/100), tickSz)
		}
	}
	if slPct > 0 {
		if side == "buy" {
			slPx = okxAlignPxToTick(refPrice*(1-slPct/100), tickSz)
		} else {
			slPx = okxAlignPxToTick(refPrice*(1+slPct/100), tickSz)
		}
	}
	if (tpPct > 0 && tpPx <= 0) || (slPct > 0 && slPx <= 0) {
		return 0, 0, false
	}
	// 方向校验：触发价必须与参考价分处正确两侧（tick 对齐后可能贴回参考价，也属非法）
	if tpPx > 0 && ((side == "buy" && tpPx <= refPrice) || (side == "sell" && tpPx >= refPrice)) {
		return 0, 0, false
	}
	if slPx > 0 && ((side == "buy" && slPx >= refPrice) || (side == "sell" && slPx <= refPrice)) {
		return 0, 0, false
	}
	return tpPx, slPx, true
}

// okxAttachTPSLForOrder 按当前 TPSL 配置为信号开仓计算交易所侧附带条件单。
// 仅 SWAP；fixed 模式阈值是保证金口径 ROI，须除以实际杠杆换算为价格口径（与软件监控判定口径一致），
// 杠杆未知（<=0）时无法可靠换算，放弃附加；vol 模式阈值为价格口径，振幅取不到时放弃附加。
// 返回 nil = 不附加（裸单）。
func okxAttachTPSLForOrder(simulated bool, instId, side string, refPrice, lever float64, cfg okxTPSLConf, ins OKXInstrument) *OKXAttachAlgoOrd {
	s := GetSettingConfig()
	if s == nil || s.Settings == nil || !s.OKXTPSLEnable {
		return nil
	}
	var tpPct, slPct float64
	if cfg.Mode == "fixed" {
		if lever <= 0 {
			return nil
		}
		tpPct, slPct = cfg.TpPct/lever, cfg.SlPct/lever
	} else {
		amp := okxAvgAmplitudePct(instId, cfg.VolKlt, cfg.VolBars)
		if amp <= 0 {
			return nil
		}
		tpPct, slPct = amp*cfg.TpMult, amp*cfg.SlMult
	}
	if tpPct <= 0 && slPct <= 0 {
		return nil
	}
	tpPx, slPx, ok := okxComputeTPSLPx(side, refPrice, tpPct, slPct, okxFloatNum(ins.TickSz))
	if !ok {
		logger.SugaredLogger.Warnf("[止盈止损] 交易所侧TPSL触发价计算非法，本笔放弃附加: instId=%s side=%s ref=%.8g tp%%=%.4f sl%%=%.4f", instId, side, refPrice, tpPct, slPct)
		return nil
	}
	ord := &OKXAttachAlgoOrd{
		TpOrdPx: "-1", // 市价止盈
		SlOrdPx: "-1", // 市价止损
	}
	if tpPx > 0 {
		ord.TpTriggerPx = strconv.FormatFloat(tpPx, 'f', -1, 64)
	} else {
		ord.TpOrdPx = ""
	}
	if slPx > 0 {
		ord.SlTriggerPx = strconv.FormatFloat(slPx, 'f', -1, 64)
	} else {
		ord.SlOrdPx = ""
	}
	return ord
}

// okxCancelInstPendingAlgos 撤销某合约在指定环境的全部未触发条件单（孤儿单清理）。
// 软件侧市价平仓成功后调用；查询/撤销失败仅告警不阻断（下一轮监控与冷却机制兜底）。
func okxCancelInstPendingAlgos(simulated bool, instId string) {
	list, err := GetOKXPendingAlgoOrdersEnv(simulated, instId)
	if err != nil {
		logger.SugaredLogger.Warnf("[止盈止损·%s] 查询孤儿条件单失败: instId=%s err=%v", okxTPSLEnvTag(simulated), instId, err)
		return
	}
	n := 0
	for _, a := range list {
		if err := CancelOKXAlgoOrderEnv(simulated, a.AlgoId, a.InstId); err != nil {
			logger.SugaredLogger.Warnf("[止盈止损·%s] 撤销孤儿条件单失败: instId=%s algoId=%s err=%v", okxTPSLEnvTag(simulated), a.InstId, a.AlgoId, err)
			continue
		}
		n++
	}
	if n > 0 {
		logger.SugaredLogger.Infof("[止盈止损·%s] 已撤销 %s 的 %d 张孤儿条件单（持仓已由软件侧平掉）", okxTPSLEnvTag(simulated), instId, n)
	}
}

// okxVolCache 单合约平均振幅缓存（公共 K 线接口 5 分钟一份，避免每 10 秒重复拉取）。
type okxVolEntry struct {
	ampPct    float64
	fetchedAt time.Time
}

var okxTPSLVolMu sync.Mutex
var okxTPSLVolCache = make(map[string]okxVolEntry)

func okxAvgAmplitudePct(instId, bar string, bars int) float64 {
	key := instId + "|" + bar
	okxTPSLVolMu.Lock()
	if e, ok := okxTPSLVolCache[key]; ok && time.Since(e.fetchedAt) < okxTPSLVolTTL {
		v := e.ampPct
		okxTPSLVolMu.Unlock()
		return v
	}
	okxTPSLVolMu.Unlock()

	candles := GetOKXCandles(instId, bar, bars+1)
	if len(candles) == 0 {
		return 0
	}
	sum, n := 0.0, 0
	for _, c := range candles {
		if c.Open <= 0 || c.High < c.Low {
			continue
		}
		sum += (c.High - c.Low) / c.Open * 100
		n++
	}
	amp := 0.0
	if n > 0 {
		amp = sum / float64(n)
	}
	okxTPSLVolMu.Lock()
	okxTPSLVolCache[key] = okxVolEntry{ampPct: amp, fetchedAt: time.Now()}
	okxTPSLVolMu.Unlock()
	return amp
}

// okxTPSLConf 平仓线配置（每轮循环按最新设置重建）。
type okxTPSLConf struct {
	Mode           string  // fixed / vol
	TpPct, SlPct   float64 // fixed 模式阈值
	VolKlt         string
	VolBars        int
	TpMult, SlMult float64 // vol 模式倍数
}

func okxTPSLConfig() okxTPSLConf {
	c := okxTPSLConf{
		Mode: "fixed", VolKlt: okxTPSLDefaultKlt, VolBars: okxTPSLDefaultBars,
		TpMult: okxTPSLDefaultTpMul, SlMult: okxTPSLDefaultSlMul,
	}
	if db.Dao == nil {
		return c
	}
	s := GetSettingConfig()
	if s == nil || s.Settings == nil {
		return c
	}
	c.TpPct = okxPctNum(s.OKXTakeProfitPct)
	c.SlPct = okxPctNum(s.OKXStopLossPct)
	if strings.EqualFold(strings.TrimSpace(s.OKXTPSLMode), "vol") {
		c.Mode = "vol"
	}
	if k := strings.TrimSpace(s.OKXTPSLVolKlt); k != "" {
		c.VolKlt = k
	}
	if b := int(okxPctNum(s.OKXTPSLVolBars)); b > 2 && b <= 300 {
		c.VolBars = b
	}
	if m := okxPctNum(s.OKXTPSLTpMult); m > 0 {
		c.TpMult = m
	}
	if m := okxPctNum(s.OKXTPSLSlMult); m > 0 {
		c.SlMult = m
	}
	return c
}

// okxPriceRoiPct 价格口径收益率（%）：未实现盈亏 ÷ 开仓名义价值。
// 开仓名义 = |持仓数量| × 合约面值 × 开仓均价（非 SWAP 面值按 1 计）。
func okxPriceRoiPct(p OKXPosition) float64 {
	upl := okxPctNum(p.Upl)
	avgPx := okxPctNum(p.AvgPx)
	posAbs := okxPctNum(p.Pos)
	if posAbs < 0 {
		posAbs = -posAbs
	}
	if upl == 0 || avgPx <= 0 || posAbs == 0 {
		return 0
	}
	ctVal := 1.0
	if strings.HasSuffix(strings.ToUpper(p.InstId), "-SWAP") {
		if spec := okxLookupInstrument(p.InstId); spec != nil && spec.CtVal != "" {
			if v := okxPctNum(spec.CtVal); v > 0 {
				ctVal = v
			}
		}
	}
	openNotional := posAbs * ctVal * avgPx
	if openNotional <= 0 {
		return 0
	}
	return upl / openNotional * 100
}

// okxPctNum 宽松解析数值，非法/空返回 0。
func okxPctNum(v string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return 0
	}
	return n
}
