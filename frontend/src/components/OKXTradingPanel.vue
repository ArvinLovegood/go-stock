<script setup lang="ts">
// OKX 风格交易面板：合约行情 → 模拟盘/实盘 页签（同一面板两个实例，按 simulated 固定环境）
// - 左侧下单区：合约选择、买/卖方向、限价/市价、数量（按可用余额百分比换算张数）、可选止盈止损
// - 右侧：合约最新行情 + 当前持仓（市价平仓）/ 当前委托（撤单）/ 历史委托
// - 账户/交易接口走 Env 版绑定，显式传入 props.simulated 指定环境，不读取设置页「模拟盘」全局开关：
//   simulated=true 用模拟盘密钥（模拟账户），false 用实盘密钥（真实资金，页面红色警示）。
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, nextTick, watch } from 'vue'
import { NButton, NSelect, NSwitch, NTag, useMessage } from 'naive-ui'
import { createChart, CandlestickSeries, HistogramSeries, LineSeries, ColorType, createSeriesMarkers, LineStyle } from 'lightweight-charts'
import { emaFinite, trixValues, emaLeadingNull } from './kline/calc'
import {
  GetOKXTradingOverviewEnv,
  GetOKXCandleList,
  GetSignalRecordPage,
  GetOKXInstTicker,
  GetOKXWalletEnv,
  GetOKXAccountConfigVMEnv,
  GetOKXPositionListEnv,
  GetOKXPositionTPSLEnv,
  GetOKXOpenOrdersEnv,
  GetOKXFillsHistoryEnv,
  GetOKXHistoryOrdersEnv,
  SubmitOKXOrderEnv,
  RevokeOKXOrderEnv,
  CloseOKXPositionEnv,
  SetOKXLeverageEnv,
  GetConfig,
} from '../../wailsjs/go/main/App'
import { BUY_SELL_SCORE_OPTIONS } from './kline/constants'
import { AUTO_TREND_INTERVAL_OPTIONS, canUseSignalMonitor, okxMonitor, okxLiveMonitor, SIGNAL_INTERVAL_OPTIONS, SIGNAL_POOL_LIMIT } from './kline/signalMonitor'
import { runOkxTrendScan, TREND_SCAN_MIN_SCORES } from './kline/okxTrendScanner'

const props = defineProps({
  darkTheme: { type: Boolean, default: false },
  /** 固定交易环境：true=OKX 模拟盘（模拟盘密钥），false=OKX 实盘（实盘密钥、真实资金） */
  simulated: { type: Boolean, default: true },
  /** 页签是否激活：非激活时暂停轮询定时器，避免模拟盘/实盘两个面板实例同时轮询翻倍请求 */
  active: { type: Boolean, default: true },
})
const message = useMessage()

// ===== 行情与合约 =====
const instMap = ref<Record<string, any>>({})
const tickerMap = ref<Record<string, any>>({})
const instOptions = ref<any[]>([])
const selectedInst = ref('BTC-USDT-SWAP')
const overviewLoading = ref(false)

const curTicker = ref<any>(null)
let tickerTimer: any = null
let dataTimer: any = null

// ===== 账户 =====
const wallet = ref<any>(null)
const acctCfg = ref<any>(null)
const positions = ref<any[]>([])
// 持仓止盈止损线视图（instId|posSide → view），随持仓一起轮询
const posTpslMap = ref<Map<string, any>>(new Map())
const openOrders = ref<any[]>([])
const historyOrders = ref<any[]>([])

// ===== 下单表单 =====
const form = reactive({
  side: 'buy' as 'buy' | 'sell',
  ordType: 'limit' as 'limit' | 'market',
  px: '' as string,
  sz: '' as string,
  pct: 0,
  lever: '', // 杠杆：''=账户默认；选择后立即同步到交易所（cross 全仓，仅 SWAP）
  useTpSl: false,
  tpPx: '' as string,
  slPx: '' as string,
})
const submitting = ref(false)

// 杠杆选项与持久化（按面板环境记忆）
const leverOptions = [
  { label: '账户默认', value: '' },
  { label: '1x', value: '1' },
  { label: '2x', value: '2' },
  { label: '3x', value: '3' },
  { label: '5x', value: '5' },
  { label: '10x', value: '10' },
  { label: '20x', value: '20' },
]
const LEVER_KEY_PREFIX = 'okxPanelLever:'
const LEVER_ENV = props.simulated ? 'sim' : 'live'
try { form.lever = localStorage.getItem(LEVER_KEY_PREFIX + LEVER_ENV) || '' } catch {}

/** 把所选杠杆同步到交易所（当前合约）；失败抛错由调用方提示 */
async function applyLeverage(instId: string, lever: string, silent = false) {
  if (!lever || !instId || !instId.endsWith('-SWAP')) return
  try {
    await SetOKXLeverageEnv(props.simulated, instId, lever)
    if (!silent) message.success(`已设置 ${instId} 杠杆 ${lever}x（${props.simulated ? '模拟盘' : '实盘'} · 全仓）`)
  } catch (e: any) {
    if (silent) {
      message.warning(`杠杆同步失败（${instId} → ${lever}x）：${e?.message || e}，将以账户当前杠杆下单`)
    } else {
      throw e
    }
  }
}

async function onLeverChange(v: string) {
  try {
    localStorage.setItem(LEVER_KEY_PREFIX + LEVER_ENV, v)
  } catch {}
  if (!v || !selectedInst.value) return
  try {
    await applyLeverage(selectedInst.value, v)
  } catch (e: any) {
    message.error(`杠杆设置失败：${e?.message || e}，已回退为账户默认`)
    form.lever = ''
  }
  onPctChange(form.pct) // 杠杆变化影响预估保证金
}

const curInst = computed(() => instMap.value[selectedInst.value] || null)
const lastPrice = computed(() => {
  const v = Number(curTicker.value?.last)
  return Number.isFinite(v) && v > 0 ? v : 0
})
const availUSDT = computed(() => {
  const d = wallet.value?.details || []
  const usdt = d.find((x: any) => x.ccy === 'USDT')
  const v = Number(usdt?.availBal || usdt?.availEq || 0)
  return Number.isFinite(v) ? v : 0
})
const totalEq = computed(() => Number(wallet.value?.totalEq || 0))

const chg24h = computed(() => {
  const t = curTicker.value
  if (!t?.last || !t?.open24h) return 0
  const last = Number(t.last), open = Number(t.open24h)
  if (!open) return 0
  return ((last - open) / open) * 100
})

const lotStep = computed(() => Math.max(Number(curInst.value?.lotSz) || 1, 0.00000001))
const minSz = computed(() => Number(curInst.value?.minSz) || 1)
const ctVal = computed(() => Number(curInst.value?.ctVal) || 1)
const tickSz = computed(() => Number(curInst.value?.tickSz) || 0.1)

const estValue = computed(() => {
  const sz = Number(form.sz)
  if (!Number.isFinite(sz) || sz <= 0 || !lastPrice.value) return 0
  return sz * ctVal.value * lastPrice.value
})

const estMargin = computed(() => {
  // 按所选杠杆估算保证金（名义价值/杠杆）；未选杠杆时按 OKX 全仓常见默认 3x 近似，实际以下单校验为准
  const lever = Number(form.lever) || 3
  return estValue.value / lever
})

function fmtNum(v: any, digits = 4) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '--'
  return n.toLocaleString('en-US', { maximumFractionDigits: digits })
}

function roundToStep(value: number, step: number) {
  if (!Number.isFinite(value) || value <= 0 || step <= 0) return 0
  return Math.floor(value / step) * step
}

function trimNum(v: number) {
  if (!v) return ''
  return String(Number(v.toFixed(8)))
}

// 合约下拉：USDT 永续 + 币币现货 分组，按 24h 成交额排序，展示最新价
function buildOptions() {
  const swap: any[] = []
  const spot: any[] = []
  for (const [id, ins] of Object.entries(instMap.value)) {
    const t = tickerMap.value[id]
    const last = t?.last ? Number(t.last) : 0
    const opt = {
      label: `${id}  ${last > 0 ? last : '--'}`,
      value: id,
      vol: Number(t?.volCcy24h) || 0,
      search: id.toLowerCase(),
    }
    if ((ins as any).instType === 'SPOT') spot.push(opt)
    else swap.push(opt)
  }
  const sortDesc = (a: any, b: any) => b.vol - a.vol
  swap.sort(sortDesc)
  spot.sort(sortDesc)
  // 全量下发（按成交额降序）：交易面板的品种下拉要支持搜索任意可交易品种，
  // 此前只取每类成交额前 150 导致大量小币种（如 BAT/SKDD）搜不到
  const mk = (list: any[]) => list.map((o) => ({ label: o.label, value: o.value, search: o.search }))
  instOptions.value = []
  if (swap.length) instOptions.value.push({ type: 'group', label: 'USDT 永续', key: 'swap', children: mk(swap) })
  if (spot.length) instOptions.value.push({ type: 'group', label: '币币现货', key: 'spot', children: mk(spot) })
}

function firstOptionValue(): string {
  for (const g of instOptions.value) {
    if (g.children?.length) return g.children[0].value
  }
  return ''
}

function filterOptions(pattern: string, option: any) {
  return option.search.includes((pattern || '').toLowerCase())
}

function onInstChange(id: string) {
  selectedInst.value = id
  form.px = ''
  form.sz = ''
  form.pct = 0
  refreshTicker()
  loadCandles()
  // 杠杆按合约逐个生效：切换合约时把已选杠杆静默同步到新合约（失败仅提示，不回退选择）
  if (form.lever) applyLeverage(id, form.lever, true)
}

// ===== 趋势扫描（全市场 OKX 永续：5 路趋势证据等权计分，手动触发的低频工具） =====
/** 扫描周期选项：趋势工具以日线为主，辅以中短周期 */
const TREND_SCAN_BAR_OPTIONS = ['1D', '4H', '1H', '15m']
const trendScanShow = ref(false)
const trendScanBar = ref('1D')
/** 门槛档（3/4/5 路）：只过滤已扫描结果，不重发请求，可随时松紧切换 */
const trendScanMinScore = ref(4)
const trendScanRunning = ref(false)
let trendScanAbort: { aborted: boolean } | null = null
const trendScanDone = ref(0)
const trendScanTotal = ref(0)
const trendScanAll = ref<any[]>([])
const trendScanFinished = ref(false)
/** 结果 = 全量评分按当前门槛现算（同一轮扫描结果可即时切换松紧档） */
const trendScanRows = computed(() => trendScanAll.value.filter((r: any) => r.score >= trendScanMinScore.value))

async function startTrendScan() {
  if (trendScanRunning.value) return
  trendScanRunning.value = true
  trendScanFinished.value = false
  trendScanAll.value = []
  trendScanDone.value = 0
  trendScanTotal.value = 0
  trendScanAbort = { aborted: false }
  try {
    const res = await runOkxTrendScan({
      bar: trendScanBar.value,
      abort: trendScanAbort,
      onProgress: (d: number, t: number) => {
        trendScanDone.value = d
        trendScanTotal.value = t
      },
    })
    trendScanAll.value = res.rows || []
    trendScanFinished.value = !trendScanAbort.aborted
  } catch (e: any) {
    message.error(`趋势扫描失败: ${e?.message || e}`)
  } finally {
    trendScanRunning.value = false
  }
}

function stopTrendScan() {
  if (trendScanAbort) trendScanAbort.aborted = true
}

/** 弹窗关闭即停止在途扫描 */
watch(trendScanShow, (v) => {
  if (!v && trendScanRunning.value) stopTrendScan()
})

/** 点击结果行：切换面板品种（复用 onInstChange 完整联动），K 线同步切到扫描周期 */
function onTrendPick(r: any) {
  if (!r?.instId) return
  trendScanShow.value = false
  if (klineBar.value !== trendScanBar.value) onBarChange(trendScanBar.value)
  onInstChange(r.instId)
}

const trendScanColumns = [
  { title: '品种', key: 'instId', width: 150, render: (r: any) => h('b', null, { default: () => r.instId }) },
  {
    title: '趋势分', key: 'score', width: 80,
    render: (r: any) => h(NTag,
      { type: r.score >= 5 ? 'error' : r.score >= 4 ? 'warning' : 'default', size: 'small', bordered: false },
      { default: () => `${r.score}/5` }),
  },
  { title: '现价', key: 'close', width: 110, render: (r: any) => fmtNum(r.close, 4) },
  {
    title: '单根涨跌', key: 'pct', width: 90,
    render: (r: any) => h('span', { style: `color:${r.pct >= 0 ? upColor.value : downColor.value}` },
      { default: () => `${r.pct >= 0 ? '+' : ''}${(r.pct * 100).toFixed(2)}%` }),
  },
  { title: 'ADX', key: 'adx', width: 70, render: (r: any) => (r.adx != null ? r.adx.toFixed(1) : '—') },
  { title: '距20期高', key: 'roomToHigh', width: 90, render: (r: any) => (r.roomToHigh != null ? `+${(r.roomToHigh * 100).toFixed(1)}%` : '—') },
  {
    // 快速入池：加入本面板环境对应的监控池（模拟盘面板→okxMonitor、实盘→okxLiveMonitor），
    // 与信号监控面板对应页签共享同一实例与持久化；pool 为 reactive，入池/删池后按钮态自动刷新
    title: '操作', key: 'op', width: 86,
    render: (r: any) => {
      if (mon.value.state.pool.some((e: any) => e.code === `ok:${r.instId}`)) {
        return h(NButton, { size: 'tiny', quaternary: true, disabled: true }, { default: () => '已入池' })
      }
      return h(NButton, {
        size: 'tiny', quaternary: true, type: 'primary',
        onClick: (e: MouseEvent) => {
          e.stopPropagation() // 不触发行点击切品种/关弹窗，入池后可继续浏览扫描结果
          addInstToPool(r.instId)
        },
      }, { default: () => '+ 监控' })
    },
  },
]
const trendScanRowProps = (r: any) => ({ style: 'cursor:pointer', onClick: () => onTrendPick(r) })

// ===== 自动交易监控池（直接绑定与本面板环境一致的实例：模拟盘面板→模拟盘池 okxMonitor，
// 实盘面板→实盘池 okxLiveMonitor；与信号监控面板对应页签共享同一实例与持久化） =====

/** 设置页「模拟盘自动下单」「实盘自动下单」双环境开关：只做展示提示，不在本面板改它 */
const autoTradeOn = ref(false)
const autoTradeLiveOn = ref(false)
/** 设置页「自动调池·模拟盘/实盘」总闸：趋势跟随的双重确认之一，面板开启前先查它 */
const autoPoolOn = ref(false)
const autoPoolLiveOn = ref(false)

/** 面板环境对应的监控池实例：由 props.simulated 固定决定，面板内不再切换 */
const mon = computed(() => (props.simulated ? okxMonitor : okxLiveMonitor))

/** 信号监控默认周期的可选值（与监控面板同一来源） */
const monKltOptions = SIGNAL_INTERVAL_OPTIONS.map((it) => ({ label: it.label, value: it.klt }))
/** 灵敏度可选值（与监控面板同一来源：灵敏/标准/严格 → 最低信号路数） */
const monScoreOptions = BUY_SELL_SCORE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))

async function loadAutoTradeMeta() {
  try {
    const cfg = await GetConfig()
    autoTradeOn.value = !!cfg?.okxSignalTradeEnable
    autoTradeLiveOn.value = !!cfg?.okxSignalTradeLive
    autoPoolOn.value = !!cfg?.okxAutoPoolEnable
    autoPoolLiveOn.value = !!cfg?.okxAutoPoolLive
  } catch {
    autoTradeOn.value = false
    autoTradeLiveOn.value = false
    autoPoolOn.value = false
    autoPoolLiveOn.value = false
  }
}

/** 监控总开关：开启前校验 VIP 门槛（与信号监控面板同一套提示） */
async function onToggleMonitor(v: boolean) {
  if (v && !(await canUseSignalMonitor())) {
    message.warning('信号监控仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
    return
  }
  mon.value.setMonitorEnabled(v)
}

/** 把合约加入本面板环境的自动交易监控池（ok: 前缀代码，池上限与信号监控页签一致）；VIP 门槛与信号监控面板一致，供左侧「+ 监控池」与趋势扫描结果行共用 */
async function addInstToPool(instId: string) {
  if (!instId) return
  if (!(await canUseSignalMonitor())) {
    message.warning('信号监控仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
    return
  }
  const r = mon.value.addPoolEntry(`ok:${instId}`, instId)
  if (!r.ok) {
    message.warning(r.msg)
    return
  }
  message.success(`已加入${props.simulated ? '模拟盘池' : '实盘池（真实资金监控，请谨慎）'}：${instId}`)
}

/** 把左侧当前所选合约加入监控池 */
function addCurrentInstToPool() {
  addInstToPool(selectedInst.value)
}

// ===== 趋势跟随（引擎 runTrendFollowTick 定时驱动：命中入池；失趋势时无持仓移除、有持仓暂留待自动交易平仓；这里只做配置与状态展示） =====

const autoTrendBarOptions = TREND_SCAN_BAR_OPTIONS.map((b: string) => ({ label: b, value: b }))
const autoTrendIntervalOptions = AUTO_TREND_INTERVAL_OPTIONS.map((o: any) => ({ label: o.label, value: o.value }))
/** 入池门槛选项：>=N/5 路趋势证据 */
const autoTrendInOptions = [5, 4, 3].map((v) => ({ label: `≥${v} 路`, value: v }))
/** 出池门槛选项：跌破 N 路移除（setAutoTrend 强制其 < 入池门槛形成滞回） */
const autoTrendExitOptions = [4, 3, 2, 1].map((v) => ({ label: `<${v} 路`, value: v }))

/** 趋势跟随总开关：双重确认——设置页「自动调池」总闸 + 本面板开关都开才生效；开启前另校验 VIP */
async function onToggleAutoTrend(v: boolean) {
  if (!v) {
    mon.value.setAutoTrend({ enabled: false })
    return
  }
  if (!(await canUseSignalMonitor())) {
    message.warning('信号监控仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
    return
  }
  await loadAutoTradeMeta()
  if (!(props.simulated ? autoPoolOn.value : autoPoolLiveOn.value)) {
    message.warning(`请先在设置页开启「自动调池·${props.simulated ? '模拟盘' : '实盘'}」总闸，再启用趋势跟随。`)
    return
  }
  mon.value.setAutoTrend({ enabled: true })
  const t = mon.value.state.autoTrend
  message.success(`趋势跟随已开启：每 ${t.intervalMin} 分钟扫描 ${t.bar}，≥${t.minScore}/5 路入池、<${t.exitScore} 路进入移除流程`)
  if (!props.simulated) {
    message.warning('实盘池趋势跟随不会主动平仓：失趋势且有持仓的品种会暂留池中，由买卖点自动交易平仓后再移除。')
  }
}

/** 趋势跟随上次运行时间 → MM-DD HH:mm */
function autoTrendTime(ms: number) {
  if (!ms) return ''
  const d = new Date(ms)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** 池条目代码 → 合约 instId（ok:XXX → XXX），供点击行切换 K 线 */
function poolInstId(code: string) {
  return String(code || '').replace(/^ok:/i, '')
}

function onPoolRowClick(e: any) {
  const inst = poolInstId(e.code)
  if (inst && inst !== selectedInst.value) onInstChange(inst)
}

// 百分比滑块 → 市价按最新价、限价按委托价换算张数（保守按无杠杆名义价值）
function onPctChange(v: number) {
  form.pct = v
  const refPx = form.ordType === 'limit' ? Number(form.px) : lastPrice.value
  if (!refPx || refPx <= 0 || !availUSDT.value) return
  const budget = (availUSDT.value * v) / 100
  const sz = roundToStep(budget / (ctVal.value * refPx), lotStep.value)
  if (sz < minSz.value) {
    form.sz = ''
    return
  }
  form.sz = trimNum(sz)
}

// 仓位快捷档位：1/4、1/3、1/2、全仓（按可用余额百分比换算张数）
const pctQuick = [
  { label: '1/4', v: 25 },
  { label: '1/3', v: 33 },
  { label: '1/2', v: 50 },
  { label: '全仓', v: 100 },
]

function onOrdTypeChange(t: string) {
  form.ordType = t
  if (t === 'market') {
    form.px = ''
  } else if (!form.px && lastPrice.value) {
    form.px = trimNum(lastPrice.value)
  }
}

// ===== 数据加载 =====
async function loadOverview() {
  overviewLoading.value = true
  try {
    // 按面板环境取各自账户的品种表：模拟盘=demo 品种表（仅主流币），实盘=实盘全量品种表
    const res = await GetOKXTradingOverviewEnv(props.simulated)
    const map: Record<string, any> = {}
    for (const it of res.instruments || []) map[it.instId] = it
    instMap.value = map
    const tmap: Record<string, any> = {}
    for (const t of res.tickers || []) tmap[t.instId] = t
    tickerMap.value = tmap
    buildOptions()
    applyDefaultInst()
    refreshTicker()
    loadCandles()
  } catch (e) {
    console.error('loadOverview error:', e)
  } finally {
    overviewLoading.value = false
  }
}

async function refreshTicker() {
  if (!selectedInst.value) return
  try {
    curTicker.value = await GetOKXInstTicker(selectedInst.value)
  } catch (e) {
    console.error('refreshTicker error:', e)
  }
}

async function loadWallet() {
  try {
    wallet.value = await GetOKXWalletEnv(props.simulated)
  } catch (e) {
    console.error('loadWallet error:', e)
  }
}

async function loadAccount() {
  try {
    acctCfg.value = await GetOKXAccountConfigVMEnv(props.simulated)
  } catch (e) {
    acctCfg.value = null
  }
}

async function loadPositions() {
  try {
    positions.value = await GetOKXPositionListEnv(props.simulated, '')
    applyDefaultInst()
  } catch (e) {
    console.error('loadPositions error:', e)
  }
  // 止盈止损线视图与持仓同轮刷新：监控口径在后台计算（fixed=保证金收益率、vol=振幅×杠杆），
  // 未开启止盈止损时后端返回空数组，持仓行显示「未开启」。
  try {
    const views = await GetOKXPositionTPSLEnv(props.simulated)
    posTpslMap.value = new Map(views.map((v: any) => [`${v.instId}|${v.posSide}`, v]))
    // 表格行渲染依赖 posTpslMap：map 更新后重建 positions 引用触发 data-table 重渲染，避免行内容滞留上一轮
    positions.value = [...positions.value]
  } catch (e) {
    console.error('loadPositionTPSL error:', e)
  }
}

/** 初始默认品种：有持仓默认选中第一笔持仓的品种，无持仓默认 BTC-USDT-SWAP（不在可交易列表则取列表第一个）。
 * 只在面板初始化（品种与持仓首次加载）时应用一次，后续轮询与下单不会把用户手选的品种顶掉。 */
let defaultInstApplied = false
function applyDefaultInst() {
  if (defaultInstApplied) return
  if (!Object.keys(instMap.value).length) return
  defaultInstApplied = true
  const posInst = positions.value.find((p: any) => instMap.value[p.instId])?.instId
  if (posInst) {
    if (posInst !== selectedInst.value) onInstChange(posInst)
    return
  }
  if (instMap.value['BTC-USDT-SWAP']) {
    if (selectedInst.value !== 'BTC-USDT-SWAP') onInstChange('BTC-USDT-SWAP')
  } else if (!instMap.value[selectedInst.value]) {
    const first = firstOptionValue()
    if (first) selectedInst.value = first.value
  }
}

async function loadOrders() {
  try {
    openOrders.value = await GetOKXOpenOrdersEnv(props.simulated)
    historyOrders.value = await GetOKXHistoryOrdersEnv(props.simulated)
  } catch (e) {
    console.error('loadOrders error:', e)
  }
}

// ===== 已实现盈亏统计（fills-history 的 fillPnl 按订单聚合；只统计合约，现货无已实现盈亏） =====
const fills = ref<any[]>([])

async function loadFills() {
  try {
    fills.value = await GetOKXFillsHistoryEnv(props.simulated, 'SWAP', '', 300)
  } catch (e) {
    console.error('loadFills error:', e)
  }
}

/** 订单号 → 聚合结果：净盈亏（fillPnl + fillFee，费用为负）、成仓名义价值 */
const ordPnlMap = computed(() => {
  const m = new Map<string, { pnl: number; notional: number }>()
  for (const f of fills.value) {
    const ordId = String(f.ordId || '')
    if (!ordId) continue
    const e = m.get(ordId) || { pnl: 0, notional: 0 }
    const px = Number(f.fillPx) || 0
    const sz = Number(f.fillSz) || 0
    const inst = instMap.value[String(f.instId || '')]
    const mult = inst?.instType === 'SWAP' ? (Number(inst.ctVal) || 1) : 1
    e.pnl += (Number(f.fillPnl) || 0) + (Number(f.fillFee) || 0)
    e.notional += px * sz * mult
    m.set(ordId, e)
  }
  return m
})

/** 整体统计：只统计有盈亏明细的已成交订单（名义价值 > 0），与历史委托列表对齐 */
const tradeStats = computed(() => {
  let pnl = 0, notional = 0, wins = 0, losses = 0, rateSum = 0, n = 0
  for (const o of historyOrders.value) {
    const e = ordPnlMap.value.get(String(o.ordId))
    if (!e || e.notional <= 0) continue
    n++
    pnl += e.pnl
    notional += e.notional
    if (e.pnl > 0) wins++
    else if (e.pnl < 0) losses++
    rateSum += e.pnl / e.notional
  }
  return {
    n, pnl, wins, losses,
    winRate: wins + losses > 0 ? wins / (wins + losses) : 0,
    totalRate: notional > 0 ? pnl / notional : 0,
    avgRate: n > 0 ? rateSum / n : 0,
  }
})

/** 盈亏着色：涨红跌绿，与面板方向列一致 */
function pnlColor(v: number) {
  return v > 0 ? upColor.value : v < 0 ? downColor.value : 'inherit'
}

function refreshAll() {
  loadWallet()
  loadPositions()
  loadOrders()
  loadFills()
}

// ===== 交易动作 =====
function validateForm(): string {
  if (!curInst.value) return '请选择合约'
  if (form.ordType === 'limit' && !(Number(form.px) > 0)) return '请输入委托价格'
  const sz = Number(form.sz)
  if (!Number.isFinite(sz) || sz <= 0) return '请输入下单数量（张）'
  if (sz < minSz.value) return `低于最小下单量 ${minSz.value} 张`
  if (form.useTpSl && form.tpPx === '' && form.slPx === '') return '已勾选止盈止损，请至少填写一项'
  return ''
}

async function submitOrder() {
  const err = validateForm()
  if (err) {
    message.warning(err)
    return
  }
  submitting.value = true
  try {
    const req: any = {
      instId: selectedInst.value,
      tdMode: curInst.value?.instType === 'SPOT' ? 'cash' : 'cross',
      side: form.side,
      ordType: form.ordType,
      sz: String(Number(form.sz)),
    }
    if (form.ordType === 'limit') req.px = String(Number(form.px))
    // 双向持仓合约账户需显式 posSide；单向持仓与现货不传
    if (acctCfg.value?.posMode === 'long_short_mode' && curInst.value?.instType !== 'SPOT') {
      req.posSide = form.side === 'buy' ? 'long' : 'short'
    }
    if (form.useTpSl) {
      const algo: any = { slOrdPx: '-1', tpOrdPx: '-1' }
      if (form.tpPx !== '') algo.tpTriggerPx = String(Number(form.tpPx))
      if (form.slPx !== '') algo.slTriggerPx = String(Number(form.slPx))
      req.attachAlgoOrds = [algo]
    }
    const res = await SubmitOKXOrderEnv(props.simulated, req)
    if (res?.sCode === '0' && res?.ordId) {
      message.success(`下单成功 · 订单号 ${res.ordId}`)
      form.pct = 0
      refreshAll()
    } else {
      message.error(`下单失败：${res?.sMsg || '未知错误'}`)
    }
  } catch (e: any) {
    message.error(`下单失败：${e?.message || e}`)
  } finally {
    submitting.value = false
  }
}

async function closePosition(row: any) {
  try {
    const res = await CloseOKXPositionEnv(props.simulated, row.instId, row.posSide, row.pos)
    if (res?.sCode === '0' && res?.ordId) {
      message.success(`平仓单已提交 · ${row.instId} 订单号 ${res.ordId}`)
    } else {
      message.error(`平仓失败：${res?.sMsg || '未知错误'}`)
    }
    refreshAll()
  } catch (e: any) {
    message.error(`平仓失败：${e?.message || e}`)
  }
}

async function cancelOrder(row: any) {
  try {
    const res = await RevokeOKXOrderEnv(props.simulated, row.instId, row.ordId)
    if (res?.sCode === '0') {
      message.success(`已撤销 ${row.instId} 委托 ${row.ordId}`)
    } else {
      message.error(`撤单失败：${res?.sMsg || '未知错误'}`)
    }
    loadOrders()
  } catch (e: any) {
    message.error(`撤单失败：${e?.message || e}`)
  }
}

// ===== 右区高度：JS 实测视口剩余高度，K线占 70%、订单页签占 30% =====
const panelRef = ref<HTMLElement | null>(null)
const rightH = ref(640)
const narrow = ref(false)
let panelRO: any = null

function measureHeight() {
  const el = panelRef.value
  if (!el) return
  narrow.value = window.innerWidth < 900
  const rect = el.getBoundingClientRect()
  if (rect.height < 50 || rect.top < 0) return // 面板隐藏（页签未激活）时不测量
  const h = Math.max(Math.floor(window.innerHeight - rect.top - 16), 560)
  rightH.value = h
}

// ===== K 线区与订单区高度分割：手柄拖动调节（25%~80%），按面板环境持久化 =====
const rightAreaRef = ref<HTMLElement | null>(null)
const SPLIT_KEY_PREFIX = 'okxPanelSplitPct:'
const splitEnv = computed(() => (props.simulated ? 'sim' : 'live'))
const splitPct = ref(clampSplitPct(Number(localStorage.getItem(SPLIT_KEY_PREFIX + splitEnv.value)) || 70))
let splitDragging = false

function clampSplitPct(v: number): number {
  if (!Number.isFinite(v)) return 70
  return Math.min(80, Math.max(25, v))
}

function onSplitDown(e: MouseEvent) {
  const area = rightAreaRef.value
  if (!area) return
  splitDragging = true
  document.body.style.userSelect = 'none'
  const rect = area.getBoundingClientRect() // 拖动期间右区总高不变，捕获一次即可
  const move = (ev: MouseEvent) => {
    if (!splitDragging) return
    const pct = ((ev.clientY - rect.top) / rect.height) * 100
    splitPct.value = clampSplitPct(Math.round(pct * 10) / 10)
  }
  const up = () => {
    splitDragging = false
    document.body.style.userSelect = ''
    try { localStorage.setItem(SPLIT_KEY_PREFIX + splitEnv.value, String(splitPct.value)) } catch {}
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
  e.preventDefault()
}

// ===== 订单列表分页：默认 10 条，滚动到底部加载更多 =====
const ORDER_PAGE_SIZE = 10
const openVisible = ref(ORDER_PAGE_SIZE)
const histVisible = ref(ORDER_PAGE_SIZE)
const openShown = computed(() => openOrders.value.slice(0, openVisible.value))
const histShown = computed(() => historyOrders.value.slice(0, histVisible.value))

function onOrdersScroll(which: 'open' | 'history', e: Event) {
  const el = e.target as HTMLElement
  if (!el) return
  if (el.scrollTop + el.clientHeight < el.scrollHeight - 8) return
  const total = which === 'open' ? openOrders.value.length : historyOrders.value.length
  const cur = which === 'open' ? openVisible.value : histVisible.value
  if (cur < total) {
    if (which === 'open') openVisible.value = Math.min(cur + ORDER_PAGE_SIZE, total)
    else histVisible.value = Math.min(cur + ORDER_PAGE_SIZE, total)
  }
}

function shownHint(which: 'open' | 'history'): string {
  const total = which === 'open' ? openOrders.value.length : historyOrders.value.length
  const cur = which === 'open' ? openShown.value.length : histShown.value.length
  if (!total) return ''
  return cur < total ? `已显示 ${cur}/${total} 条 · 下拉到底加载更多` : `已全部加载 ${total} 条`
}

// ===== 表格列 =====
// 点击任意订单行（持仓/委托/历史）自动把左侧合约与 K 线切换到该订单品种
function rowProps(r: any) {
  return {
    style: 'cursor: pointer;',
    title: '点击切换 K 线到该合约',
    onClick: () => {
      if (r.instId && r.instId !== selectedInst.value) onInstChange(r.instId)
    },
  }
}

const posColumns = [
  { title: '合约', key: 'instId', width: 150, render: (r: any) => r.instId },
  {
    title: '方向', key: 'posSide', width: 80,
    render: (r: any) => {
      const long = r.posSide === 'long' || (r.posSide === 'net' && Number(r.pos) >= 0)
      // 多=涨方向、空=跌方向，颜色跟随涨跌色习惯（红涨绿跌时多=红）
      return h(NTag, { size: 'tiny', bordered: false, type: long ? (upColorMode.value ? 'error' : 'success') : (upColorMode.value ? 'success' : 'error') },
        { default: () => (r.posSide === 'net' ? (long ? '多' : '空') : long ? '开多' : '开空') })
    },
  },
  { title: '数量(张)', key: 'pos', width: 90, render: (r: any) => fmtNum(Math.abs(Number(r.pos)), 4) },
  { title: '开仓均价', key: 'avgPx', width: 100, render: (r: any) => fmtNum(r.avgPx) },
  { title: '标记价格', key: 'markPx', width: 100, render: (r: any) => fmtNum(r.markPx) },
  {
    title: '未实现盈亏', key: 'upl', width: 110,
    render: (r: any) => {
      const v = Number(r.upl)
      const color = v > 0 ? upColor.value : v < 0 ? downColor.value : undefined
      return h('span', { style: { color, fontSize: '12px' } }, `${v >= 0 ? '+' : ''}${fmtNum(v, 2)} (${(Number(r.uplRatio) * 100).toFixed(2)}%)`)
    },
  },
  { title: '杠杆', key: 'lever', width: 60, render: (r: any) => `${r.lever || '--'}x` },
  { title: '保证金', key: 'margin', width: 90, render: (r: any) => fmtNum(r.margin, 2) },
  { title: '强平价', key: 'liqPx', width: 100, render: (r: any) => (r.liqPx ? fmtNum(r.liqPx) : '--') },
  {
    // 止盈止损线视图（与后台持仓监控同款口径）：现=当前账户收益率，差=距触发还需的收益率百分点，价=按标记价估算的触发价
    title: '止盈止损', key: 'tpsl', width: 185,
    render: (r: any) => {
      const v = posTpslMap.value.get(`${r.instId}|${r.posSide}`)
      if (!v) {
        return h('span', { style: { fontSize: '11px', color: '#999' }, title: '未开启止盈止损、或波动率数据暂不可用' }, '未开启/暂不可用')
      }
      const f = (n: number, d = 1) => Number(n).toFixed(d)
      const px = (n: number) => (n > 0 ? ` 价${fmtNum(n)}` : '')
      const lines: any[] = [
        h('div', { style: { fontSize: '10px', color: '#999', marginBottom: '1px' } },
          `现 ${f(v.roi)}% · ${v.mode === 'vol' ? '波动率' : '固定'}口径`),
      ]
      if (v.hasTp) {
        lines.push(h('div', { style: { fontSize: '11px', color: upColor.value } }, `TP ${f(v.tpLine)}% 差${f(v.distTp)}%${px(v.tpPx)}`))
      } else {
        lines.push(h('div', { style: { fontSize: '11px', color: '#999' } }, 'TP 未启用'))
      }
      if (v.hasSl) {
        lines.push(h('div', { style: { fontSize: '11px', color: downColor.value } }, `SL ${f(v.slLine)}% 差${f(v.distSl)}%${px(v.slPx)}`))
      } else {
        lines.push(h('div', { style: { fontSize: '11px', color: '#999' } }, 'SL 未启用'))
      }
      if (v.trailArmed) {
        lines.push(h('div', { style: { fontSize: '11px', color: '#f0a020' } }, `移动止盈 ${f(v.trailLine)}%`))
      }
      return h('div', { title: '账户口径（保证金收益率）· 差 = 距触发还需的收益率百分点' }, lines)
    },
  },
  {
    title: '操作', key: 'op', width: 80,
    render: (r: any) => h(NButton, {
      size: 'tiny', type: 'warning', quaternary: true,
      onClick: () => closePosition(r),
    }, { default: () => '市价平仓' }),
  },
]

const orderColumns = (history: boolean) => [
  { title: '合约', key: 'instId', width: 150, render: (r: any) => r.instId },
  // 历史委托在合约后先展示每笔已实现盈亏与收益率（fills-history 聚合，无明细时显示 --）
  ...(history
    ? [
        {
          title: '盈亏', key: 'pnl', width: 105,
          render: (r: any) => {
            const e = ordPnlMap.value.get(String(r.ordId))
            if (!e || e.notional <= 0) return '--'
            return h('span', { style: { color: pnlColor(e.pnl), fontWeight: 600 } },
              `${e.pnl >= 0 ? '+' : ''}${e.pnl.toFixed(4)}`)
          },
        },
        {
          title: '收益率', key: 'pnlRate', width: 90,
          render: (r: any) => {
            const e = ordPnlMap.value.get(String(r.ordId))
            if (!e || e.notional <= 0) return '--'
            const rate = (e.pnl / e.notional) * 100
            return h('span', { style: { color: pnlColor(rate) } },
              `${rate >= 0 ? '+' : ''}${rate.toFixed(2)}%`)
          },
        },
      ]
    : []),
  {
    title: '方向', key: 'side', width: 70,
    render: (r: any) => h(NTag, { size: 'tiny', bordered: false, type: r.side === 'buy' ? 'error' : 'success' },
      { default: () => (r.side === 'buy' ? '买入' : '卖出') }),
  },
  { title: '类型', key: 'ordType', width: 70, render: (r: any) => (r.ordType === 'market' ? '市价' : '限价') },
  { title: '价格', key: 'px', width: 100, render: (r: any) => (r.px ? fmtNum(r.px) : '市价') },
  { title: '数量(张)', key: 'sz', width: 90, render: (r: any) => fmtNum(r.sz, 4) },
  {
    title: history ? '状态' : '成交',
    key: 'state', width: history ? 80 : 100,
    render: (r: any) => {
      if (!history) return `${fmtNum(r.accFillSz, 4)}/${fmtNum(r.sz, 4)}`
      const m: Record<string, string> = { live: '待成交', partially_filled: '部分成交', filled: '已成交', canceled: '已撤销' }
      return m[r.state] || r.state
    },
  },
  {
    title: '时间', key: 'cTime', width: 100,
    render: (r: any) => {
      const n = Number(r.cTime)
      if (!n) return '--'
      const d = new Date(n)
      const p = (x: number) => String(x).padStart(2, '0')
      return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
    },
  },
]

const openOrderColumns = [
  ...orderColumns(false),
  {
    title: '操作', key: 'op', width: 70,
    render: (r: any) => h(NButton, { size: 'tiny', quaternary: true, type: 'error', onClick: () => cancelOrder(r) },
      { default: () => '撤单' }),
  },
]
const historyOrderColumns = orderColumns(true)

// ===== K 线图（lightweight-charts，全部周期级别） =====
const BAR_LEVELS = ['1m', '3m', '5m', '15m', '30m', '1H', '2H', '4H', '6H', '12H', '1D', '1W', '1M']
/** 监控周期（东财 klt 码）→ OKX K 线周期映射；不在映射内的周期回退 15m */
const MON_KLT_TO_BAR: Record<string, string> = { '1': '1m', '5': '5m', '15': '15m', '30': '30m', '60': '1H', '101': '1D', '102': '1W' }
function monKltToBar(klt: string): string {
  return MON_KLT_TO_BAR[String(klt || '')] || '15m'
}
/** K 线周期默认跟随本环境监控周期；用户手动切换后以手动为准（不再被监控周期变化顶掉） */
let klineBarManual = false
const klineBar = ref(monKltToBar(mon.value.state.klt))
watch(() => mon.value.state.klt, (k) => {
  if (klineBarManual) return
  const b = monKltToBar(k)
  if (klineBar.value !== b) {
    klineBar.value = b
    loadCandles()
  }
})
const klineLoading = ref(false)
const chartRef = ref<HTMLElement | null>(null)
let chart: any = null
let candleSeries: any = null
let volumeSeries: any = null
let resizeObserver: any = null
let chartTimer: any = null

// ===== 主图 EMA 叠加 + TRIX(15) 子图 =====
const showEMA = ref(true)
const showTRIX = ref(true)
const EMA_LEVELS = [
  { p: 7, color: '#f59e0b' },
  { p: 25, color: '#3b82f6' },
  { p: 99, color: '#a855f7' },
]
let emaSeries: any[] = []
let trixLine: any = null
let trixSignal: any = null
let lastTimes: number[] = []
let lastCloses: number[] = []
// 本地 K 线缓存（升序）：轮询只拉最近一小段，与缓存合并后增量更新，不再每 30s 全量 setData+fitContent
let candleCache: any[] = []
let volumeCache: any[] = []
let candleKey = ''
/** 增量更新路径单次最多回放的尾部根数上限（超过则退回全量 setData） */
const MAX_TAIL_UPDATE_BARS = 10

// ===== K 线上的买卖点标注（信号流水，来源=加密监控池） =====
/** OKX K 线周期 → 监控周期（东财 klt 码）；信号按周期生成，标注只取与当前 K 线同周期的买卖点 */
const BAR_TO_KLT: Record<string, string> = { '1m': '1', '5m': '5', '15m': '15', '30m': '30', '1H': '60', '1D': '101', '1W': '102' }
/** 历史买卖点标注句柄（createSeriesMarkers 绑定在 candleSeries 上，图表随组件存活） */
let signalMarkersApi: any = null
/** 最新买卖点水平价线句柄 */
let lastSignalLine: any = null
/** 最新买卖点（水平线标题里的收益率随最新价刷新） */
const lastSignal = ref<{ price: number; kind: string } | null>(null)

/** 价格显示小数位：按合约 tickSz 自适应（与标题行最新价一致） */
function priceDigits(): number {
  const t = Number(tickSz.value) || 0
  if (t > 0 && t < 1) return Math.max(0, Math.min(6, Math.round(-Math.log10(t))))
  return 2
}

/** 最新买卖点水平线标题：方向 + 信号价 + 按最新价计算的方向收益率（买=开多涨赚 / 卖=开空跌赚，与收益统计口径一致） */
function signalLineTitle(): string {
  const s = lastSignal.value
  if (!s) return ''
  const last = lastPrice.value
  const p = s.price
  let rate = 0
  if (last > 0 && p > 0) rate = s.kind === 'buy' ? (last - p) / p : (p - last) / p
  return `${s.kind === 'buy' ? '买' : '卖'} ${p.toFixed(priceDigits())} ${rate >= 0 ? '+' : ''}${(rate * 100).toFixed(2)}%`
}

/**
 * 加载当前品种的买卖点：历史信号打 K 线标注（买=红上箭头 / 卖=绿下箭头，只渲染落在已加载 K 线内的），
 * 最新一条画水平虚线价线，标题随行情轮询刷新收益率；随 loadCandles（品种/周期切换 + 定时轮询）重查。
 */
async function loadSignalMarks() {
  if (!candleSeries || !selectedInst.value) return
  const klt = BAR_TO_KLT[klineBar.value]
  try {
    const res = await GetSignalRecordPage({
      keyword: selectedInst.value, startTime: 0, endTime: 0,
      page: 1, pageSize: 200, source: 'crypto',
    })
    const list: any[] = res && Array.isArray(res.list) ? res.list : []
    if (!candleSeries) return // 卸载竞态：等待期间图表已销毁
    // 同一信号两池都命中只存一条（唯一索引），无需去重；API 按 bar_time 倒序，hits[0] 即最新
    const hits = list.filter((r) => r.family === 'buysell' && r.price && String(r.klt) === klt)
    const markers = hits.slice(0, 100).reverse().map((r) => ({
      time: Number(r.time),
      position: r.kind === 'buy' ? 'belowBar' : 'aboveBar',
      color: r.kind === 'buy' ? upColor.value : downColor.value,
      shape: r.kind === 'buy' ? 'arrowUp' : 'arrowDown',
      text: r.kind === 'buy' ? '买' : '卖',
    }))
    if (!signalMarkersApi) signalMarkersApi = createSeriesMarkers(candleSeries, markers)
    else signalMarkersApi.setMarkers(markers)
    const latest = hits[0]
    if (lastSignalLine) {
      candleSeries.removePriceLine(lastSignalLine)
      lastSignalLine = null
    }
    lastSignal.value = latest && Number(latest.price) > 0 ? { price: Number(latest.price), kind: latest.kind } : null
    if (lastSignal.value) {
      lastSignalLine = candleSeries.createPriceLine({
        price: lastSignal.value.price,
        color: lastSignal.value.kind === 'buy' ? upColor.value : downColor.value,
        lineWidth: 1,
        lineStyle: LineStyle.Dashed,
        axisLabelVisible: true,
        title: signalLineTitle(),
      })
    }
  } catch (e) {
    console.error('loadSignalMarks error:', e)
  }
}

// 最新价变动 → 刷新水平线标题里的收益率
watch(() => lastPrice.value, () => {
  if (lastSignalLine && lastSignal.value) lastSignalLine.applyOptions({ title: signalLineTitle() })
})

// ===== K 线上的止盈止损线（监控口径，与持仓表同数据源 posTpslMap，随持仓轮询刷新） =====
let tpslLines: any[] = []

/** 画当前品种持仓的止盈/止损/移动止盈虚线（估算触发价），无持仓或未开启时清空 */
function drawTpslLines() {
  if (!candleSeries) return
  tpslLines.forEach((l) => { try { candleSeries.removePriceLine(l) } catch (e) { /* 图表已销毁 */ } })
  tpslLines = []
  const views = [...posTpslMap.value.values()].filter((v: any) => v.instId === selectedInst.value)
  for (const v of views) {
    // 同合约双向持仓时各画各的，标题带方向区分
    const side = v.posSide === 'long' ? '(多)' : v.posSide === 'short' ? '(空)' : ''
    if (v.hasTp && Number(v.tpPx) > 0) {
      tpslLines.push(candleSeries.createPriceLine({
        price: Number(v.tpPx), color: upColor.value, lineWidth: 1, lineStyle: LineStyle.Dashed,
        axisLabelVisible: true, title: `止盈${side} ${v.tpLine >= 0 ? '+' : ''}${Number(v.tpLine).toFixed(1)}%`,
      }))
    }
    if (v.hasSl && Number(v.slPx) > 0) {
      tpslLines.push(candleSeries.createPriceLine({
        price: Number(v.slPx), color: downColor.value, lineWidth: 1, lineStyle: LineStyle.Dashed,
        axisLabelVisible: true, title: `止损${side} ${Number(v.slLine).toFixed(1)}%`,
      }))
    }
    if (v.trailArmed && Number(v.trailPx) > 0) {
      tpslLines.push(candleSeries.createPriceLine({
        price: Number(v.trailPx), color: '#f59e0b', lineWidth: 1, lineStyle: LineStyle.Dashed,
        axisLabelVisible: true, title: `移动止盈${side} +${Number(v.trailLine).toFixed(1)}%`,
      }))
    }
  }
}

// 持仓 TPSL 视图刷新（每轮 loadPositions）→ 重画线；品种切换时由 loadCandles 兜底重画
watch(posTpslMap, drawTpslLines)

// ===== 涨跌颜色习惯：红涨绿跌（A股，默认）/ 绿涨红跌（国际） =====
// 全局记忆（localStorage），模拟盘/实盘两个面板实例通过 window 事件同步切换。
const COLOR_STORAGE_KEY = 'okx-up-color'
const COLORChangeEvent = 'okx-up-color-change'
const upColorMode = ref(localStorage.getItem(COLOR_STORAGE_KEY) !== 'intl') // true=红涨绿跌（A股）
const upColor = computed(() => (upColorMode.value ? '#d03050' : '#18a058'))
const downColor = computed(() => (upColorMode.value ? '#18a058' : '#d03050'))
const upVolColor = computed(() => (upColorMode.value ? 'rgba(208,48,80,0.45)' : 'rgba(24,160,88,0.45)'))
const downVolColor = computed(() => (upColorMode.value ? 'rgba(24,160,88,0.45)' : 'rgba(208,48,80,0.45)'))

/** 颜色习惯切换后重绘所有着色元素：K 线蜡烛/成交量、买卖点标注、止盈止损线、持仓表 */
function applyColorMode() {
  if (candleSeries) {
    candleSeries.applyOptions({ upColor: upColor.value, downColor: downColor.value, wickUpColor: upColor.value, wickDownColor: downColor.value })
    // 成交量颜色在数据点里，需按新配色重放着色（视图缩放/平移不受影响）
    if (volumeSeries && volumeCache.length && volumeCache.length === candleCache.length) {
      volumeCache = volumeCache.map((v: any, i: number) => ({
        ...v,
        color: candleCache[i] && candleCache[i].close >= candleCache[i].open ? upVolColor.value : downVolColor.value,
      }))
      volumeSeries.setData(volumeCache)
    }
  }
  loadSignalMarks()
  drawTpslLines()
  // 持仓表行内色在 render 函数里，重建 positions 引用触发 data-table 重渲染（与 posTpslMap 更新同款手法）
  positions.value = [...positions.value]
}

function toggleColorMode() {
  upColorMode.value = !upColorMode.value
  localStorage.setItem(COLOR_STORAGE_KEY, upColorMode.value ? 'cn' : 'intl')
  applyColorMode()
  window.dispatchEvent(new CustomEvent(COLORChangeEvent))
}

function syncColorMode() {
  const v = localStorage.getItem(COLOR_STORAGE_KEY) !== 'intl'
  if (v !== upColorMode.value) {
    upColorMode.value = v
    applyColorMode()
  }
}

function toLine(times: number[], vals: any[]): any[] {
  const out: any[] = []
  for (let i = 0; i < vals.length && i < times.length; i++) {
    const v = vals[i]
    if (v == null || !Number.isFinite(Number(v))) continue
    out.push({ time: times[i], value: Number(v) })
  }
  return out
}

function ensureEmaSeries() {
  if (!chart || emaSeries.length) return
  emaSeries = EMA_LEVELS.map((c) =>
    chart.addSeries(LineSeries, {
      color: c.color,
      lineWidth: 1,
      title: `EMA${c.p}`,
      lastValueVisible: false,
      priceLineVisible: false,
      crosshairMarkerVisible: false,
    }, 0),
  )
}

function removeEmaSeries() {
  if (!chart) return
  emaSeries.forEach((s) => chart.removeSeries(s))
  emaSeries = []
}

function ensureTrixSeries() {
  if (!chart || trixLine) return
  chart.panes()[0]?.setStretchFactor(3)
  const pf = { type: 'price', precision: 2, minMove: 0.01 }
  trixLine = chart.addSeries(LineSeries, {
    color: '#3b82f6', lineWidth: 2, title: 'TRIX',
    lastValueVisible: false, priceLineVisible: false, priceFormat: pf,
  }, 1)
  trixSignal = chart.addSeries(LineSeries, {
    color: '#ef4444', lineWidth: 1, title: 'Signal',
    lastValueVisible: false, priceLineVisible: false, priceFormat: pf,
  }, 1)
  chart.panes()[1]?.setStretchFactor(1)
}

function removeTrixSeries() {
  if (!chart) return
  if (trixLine) { chart.removeSeries(trixLine); trixLine = null }
  if (trixSignal) { chart.removeSeries(trixSignal); trixSignal = null }
  while (chart.panes().length > 1) chart.removePane(chart.panes().length - 1)
  chart.panes()[0]?.setStretchFactor(1)
}

function applyIndicators() {
  if (!chart || !candleSeries || !lastCloses.length) return
  if (showEMA.value) {
    ensureEmaSeries()
    EMA_LEVELS.forEach((c, i) => {
      emaSeries[i]?.setData(toLine(lastTimes, emaFinite(lastCloses, c.p)))
    })
  }
  if (showTRIX.value) {
    ensureTrixSeries()
    const trix = trixValues(lastCloses, 15)
    trixLine?.setData(toLine(lastTimes, trix))
    trixSignal?.setData(toLine(lastTimes, emaLeadingNull(trix, 9)))
  }
}

function onToggleEMA() {
  showEMA.value = !showEMA.value
  if (!showEMA.value) removeEmaSeries()
  else applyIndicators()
}

function onToggleTRIX() {
  showTRIX.value = !showTRIX.value
  if (!showTRIX.value) removeTrixSeries()
  else applyIndicators()
}

function chartColors() {
  const dark = props.darkTheme
  return {
    text: dark ? '#9ca3af' : '#666666',
    grid: dark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
    bg: dark ? '#101014' : '#ffffff',
  }
}

function initChart() {
  if (!chartRef.value || chart) return
  const c = chartColors()
  chart = createChart(chartRef.value, {
    height: 320,
    autoSize: true,
    layout: { background: { type: ColorType.Solid, color: c.bg }, textColor: c.text, fontSize: 11 },
    grid: { vertLines: { color: c.grid }, horzLines: { color: c.grid } },
    rightPriceScale: { borderColor: c.grid },
    timeScale: { borderColor: c.grid, timeVisible: true, secondsVisible: false, rightOffset: 8 },
    crosshair: { mode: 0 },
  })
  candleSeries = chart.addSeries(CandlestickSeries, {
    upColor: upColor.value,
    downColor: downColor.value,
    borderVisible: false,
    wickUpColor: upColor.value,
    wickDownColor: downColor.value,
  })
  volumeSeries = chart.addSeries(HistogramSeries, {
    priceFormat: { type: 'volume' },
    priceScaleId: 'vol',
    lastValueVisible: false,
    priceLineVisible: false,
  })
  chart.priceScale('vol').applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } })
  loadCandles()
}

async function loadCandles() {
  if (!selectedInst.value || !candleSeries) return
  // 买卖点标注/最新水平线与 K 线同源刷新（品种/周期切换 + 定时轮询），不阻塞 K 线加载
  loadSignalMarks()
  // 止盈止损线随品种切换重画（旧品种的线必须清掉），数据更新由 posTpslMap watch 驱动
  drawTpslLines()
  // 品种/周期变化时重置缓存走全量；同品种同周期的轮询走增量
  const key = `${selectedInst.value}|${klineBar.value}`
  const fullLoad = key !== candleKey
  if (fullLoad) {
    candleKey = key
    candleCache = []
    volumeCache = []
  }
  klineLoading.value = fullLoad || !candleCache.length
  try {
    // 首次/切换品种拉 200 根建仓；轮询只拉最近 30 根（最后一根在变动 + 可能新收线），请求体积约为原来的 15%
    const limit = candleCache.length ? 30 : 200
    const list = await GetOKXCandleList(selectedInst.value, klineBar.value, limit)
    const bars = []
    const vols = []
    for (const c of list || []) {
      if (!c.ts || c.ts <= 0) continue
      const time = Math.floor(c.ts / 1000)
      bars.push({ time, open: c.open, high: c.high, low: c.low, close: c.close })
      vols.push({
        time,
        value: c.volCcy > 0 ? c.volCcy : c.vol,
        color: c.close >= c.open ? upVolColor.value : downVolColor.value,
      })
    }
    bars.sort((a: any, b: any) => a.time - b.time)
    if (!bars.length) return
    if (!candleCache.length) {
      candleCache = bars
      volumeCache = vols
      candleSeries.setData(bars)
      volumeSeries.setData(vols)
      // 默认视图：展示最近 ~120 根，右侧留 8 根空白不贴边（EMA99 预热线也在窗口内）；用户可自由缩放平移
      chart?.timeScale().setVisibleLogicalRange({ from: Math.max(0, bars.length - 120), to: bars.length - 1 + 8 })
      lastTimes = bars.map((b: any) => b.time)
      lastCloses = bars.map((b: any) => b.close)
      applyIndicators()
      return
    }
    // 合并进缓存，返回值：'none'=无变化跳过重绘；'tail'=只更新/追加了尾部（可增量）；'full'=结构变化需全量
    const prevLen = candleCache.length
    const res = mergeCandles(bars, vols)
    if (res === 'none') return
    lastTimes = candleCache.map((b) => b.time)
    lastCloses = candleCache.map((b) => b.close)
    if (res === 'tail') {
      // series.update() 只允许改末根或追加，回放必须从「合并前的最后一根」开始
      //（覆盖：进行中末根变动 + 其后新收线追加），不打断用户缩放/平移，也不触发 fitContent
      const fromIdx = Math.max(0, prevLen - 1)
      const count = candleCache.length - fromIdx
      if (count <= MAX_TAIL_UPDATE_BARS) {
        for (let i = fromIdx; i < candleCache.length; i++) {
          candleSeries.update(candleCache[i])
          volumeSeries.update(volumeCache[i])
        }
        updateIndicatorTails(fromIdx)
        return
      }
    }
    candleSeries.setData(candleCache)
    volumeSeries.setData(volumeCache)
    applyIndicators()
  } catch (e) {
    console.error('loadCandles error:', e)
  } finally {
    klineLoading.value = false
  }
}

/**
 * 把轮询取回的最近一段 K 线合并进本地缓存（缓存与新数据均按时间升序）。
 * 返回：
 * - 'none'：新数据与缓存尾部完全一致（盘面无变化），无需重绘；
 * - 'tail'：只更新了已存在的最后一根（进行中的 bar）或在其后追加了新收线 bar，可安全走 series.update()；
 * - 'full'：出现中间历史被改写/时间交叉等结构变化，需整体 setData。
 */
function mergeCandles(bars: any[], vols: any[]): 'none' | 'tail' | 'full' {
  const lastT = candleCache[candleCache.length - 1].time
  const firstT = bars[0].time
  if (firstT > lastT) {
    // 无重叠（如应用长时间挂起后恢复，轮询窗口整体前移）：追加进缓存后走全量重绘，避免缓存冻结
    candleCache.push(...bars)
    volumeCache.push(...vols)
    return 'full'
  }
  let changed = false
  let tailOnly = true
  let j = 0
  // 对齐：跳过缓存中新窗口之前的部分，从重叠区开始逐根比对
  let i = 0
  while (i < candleCache.length && candleCache[i].time < firstT) i++
  while (j < bars.length) {
    const nb = bars[j]
    const nv = vols[j]
    if (i >= candleCache.length) {
      candleCache.push(nb)
      volumeCache.push(nv)
      changed = true
      i++
      j++
      continue
    }
    const ob = candleCache[i]
    if (ob.time === nb.time) {
      const changedVol = volumeCache[i].value !== nv.value
      if (ob.open !== nb.open || ob.high !== nb.high || ob.low !== nb.low || ob.close !== nb.close || changedVol) {
        candleCache[i] = nb
        volumeCache[i] = nv
        changed = true
        // 改写的是历史中间根（非末根）时不能走增量 update()（lightweight-charts 只允许动末根/追加）
        if (i < candleCache.length - 1) tailOnly = false
      }
      i++
      j++
      continue
    }
    // 时间交叉（缓存该位置的时间与新数据不一致）→ 数据结构异常，保守全量
    return 'full'
  }
  if (!changed) return 'none'
  return tailOnly ? 'tail' : 'full'
}

/** 增量更新 EMA/TRIX 折线：只回放 fromIdx 起的点位（指标全量重算，但只 update 变动尾部） */
function updateIndicatorTails(fromIdx: number) {
  if (!chart) return
  if (showEMA.value) {
    ensureEmaSeries()
    EMA_LEVELS.forEach((c, i) => {
      const vals = emaFinite(lastCloses, c.p)
      for (let j = fromIdx; j < vals.length && j < lastTimes.length; j++) {
        const v = vals[j]
        if (v == null || !Number.isFinite(Number(v))) continue
        emaSeries[i]?.update({ time: lastTimes[j], value: Number(v) })
      }
    })
  }
  if (showTRIX.value) {
    ensureTrixSeries()
    const trix = trixValues(lastCloses, 15)
    const sig = emaLeadingNull(trix, 9)
    for (let j = fromIdx; j < trix.length && j < lastTimes.length; j++) {
      if (Number.isFinite(Number(trix[j]))) trixLine?.update({ time: lastTimes[j], value: Number(trix[j]) })
      if (Number.isFinite(Number(sig[j]))) trixSignal?.update({ time: lastTimes[j], value: Number(sig[j]) })
    }
  }
}

function onBarChange(b: string) {
  if (klineBar.value === b) return
  klineBarManual = true // 手动选择后不再跟随监控周期
  klineBar.value = b
  loadCandles()
}

// 生命周期
// ===== 轮询定时器：页签非激活时暂停，激活时恢复，避免模拟盘/实盘两个面板实例同时轮询翻倍请求 =====
/** 数据刷新基准间隔（秒）：行情=基准，账户/持仓/委托=基准×3，K线=基准×6；默认 5s 对应原固定 5/15/30s */
const REFRESH_BASE_OPTIONS = [3, 5, 10, 15, 30, 60].map((s) => ({ label: `${s}秒`, value: s }))
const refreshBase = ref(5)
try {
  const saved = Number(localStorage.getItem(`okx-panel-refresh-base-${props.simulated ? 'sim' : 'live'}`))
  if (REFRESH_BASE_OPTIONS.some((o) => o.value === saved)) refreshBase.value = saved
} catch { /* 脏数据忽略 */ }
function onRefreshBaseChange(v: number) {
  refreshBase.value = v
  try {
    localStorage.setItem(`okx-panel-refresh-base-${props.simulated ? 'sim' : 'live'}`, String(v))
  } catch { /* 存储失败忽略 */ }
  // 重启定时器让新间隔立即生效；页签非激活时本来就没有定时器在跑
  stopTimers()
  if (props.active) startTimers()
}

function startTimers() {
  if (tickerTimer || dataTimer || chartTimer) return // 已在运行，避免重复叠加
  const base = refreshBase.value * 1000
  tickerTimer = setInterval(refreshTicker, base)
  dataTimer = setInterval(refreshAll, base * 3)
  chartTimer = setInterval(loadCandles, base * 6)
}

function stopTimers() {
  if (tickerTimer) { clearInterval(tickerTimer); tickerTimer = null }
  if (dataTimer) { clearInterval(dataTimer); dataTimer = null }
  if (chartTimer) { clearInterval(chartTimer); chartTimer = null }
}

watch(() => props.active, (v) => {
  if (v) {
    startTimers()
    // 每次切回本面板都重拉设置页开关：用户可能刚在设置页开了「自动调池」总闸，
    // 不刷新会让「总闸未开」红字残留、趋势跟随开启被误拒
    loadAutoTradeMeta()
  } else {
    stopTimers()
  }
})

onMounted(() => {
  loadOverview()
  loadAccount()
  refreshAll()
  loadAutoTradeMeta()
  // 另一环境面板切换涨跌色习惯时同步本面板
  window.addEventListener(COLORChangeEvent, syncColorMode)
  if (props.active) startTimers()
  nextTick(() => {
    initChart()
    measureHeight()
    panelRO = new ResizeObserver(() => measureHeight())
    if (panelRef.value) panelRO.observe(panelRef.value)
    window.addEventListener('resize', measureHeight)
  })
})

onBeforeUnmount(() => {
  window.removeEventListener(COLORChangeEvent, syncColorMode)
  if (tickerTimer) clearInterval(tickerTimer)
  if (dataTimer) clearInterval(dataTimer)
  if (chartTimer) clearInterval(chartTimer)
  if (panelRO) {
    panelRO.disconnect()
    panelRO = null
  }
  window.removeEventListener('resize', measureHeight)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (chart) {
    emaSeries = []
    trixLine = null
    trixSignal = null
    signalMarkersApi = null
    lastSignalLine = null
    lastSignal.value = null
    tpslLines = []
    chart.remove()
    chart = null
    candleSeries = null
    volumeSeries = null
  }
})
</script>

<template>
  <div ref="panelRef" class="okx-trade-panel" :class="{ dark: props.darkTheme, 'color-intl': !upColorMode }">
    <n-spin :show="overviewLoading">
      <div class="trade-layout">
        <!-- 左侧下单区 -->
        <div class="order-card">
          <div class="row">
            <n-select
              :value="selectedInst"
              :options="instOptions"
              filterable
              size="small"
              placeholder="搜索合约，如 BTC"
              :render-label="(option: any) => option.label"
              :filter="filterOptions"
              @update:value="onInstChange"
            />
          </div>

          <div class="ticker-line">
            <span class="last" :class="chg24h >= 0 ? 'up' : 'down'">{{ fmtNum(curTicker?.last, 4) }}</span>
            <span class="chg" :class="chg24h >= 0 ? 'up' : 'down'">{{ chg24h >= 0 ? '+' : '' }}{{ chg24h.toFixed(2) }}%</span>
          </div>
          <div class="ticker-sub">
            <span>24h高 {{ fmtNum(curTicker?.high24h, 2) }}</span>
            <span>24h低 {{ fmtNum(curTicker?.low24h, 2) }}</span>
            <span>成交额 {{ fmtNum(Number(curTicker?.volCcy24h || 0) / 1e6, 1) }}M</span>
          </div>

          <div class="avail-line">
            <span>可用</span>
            <b>{{ fmtNum(availUSDT, 2) }}</b>
            <span>USDT · 总权益 ${{ fmtNum(totalEq, 2) }}</span>
          </div>

          <div class="side-btns">
            <button class="side-btn buy" :class="{ active: form.side === 'buy' }" @click="form.side = 'buy'">买入(做多)</button>
            <button class="side-btn sell" :class="{ active: form.side === 'sell' }" @click="form.side = 'sell'">卖出(做空)</button>
          </div>

          <div class="type-tabs">
            <span :class="{ active: form.ordType === 'limit' }" @click="onOrdTypeChange('limit')">限价</span>
            <span :class="{ active: form.ordType === 'market' }" @click="onOrdTypeChange('market')">市价</span>
            <n-tag size="tiny" :bordered="false" style="margin-left: auto;">全仓</n-tag>
          </div>

          <div class="field">
            <span class="label">价格 (USDT)</span>
            <n-input-number
              v-model:value="form.px"
              size="small"
              :disabled="form.ordType === 'market'"
              :placeholder="form.ordType === 'market' ? '以市价成交' : '委托价格'"
              :step="tickSz"
              :min="0"
              :button-placement="`both`"
              style="width: 100%;"
            >
              <template #suffix>
                <n-button text size="tiny" @click="form.px = curTicker?.last || ''">最新</n-button>
              </template>
            </n-input-number>
          </div>

          <div class="field">
            <span class="label">杠杆</span>
            <n-select
              v-model:value="form.lever"
              size="small"
              :options="leverOptions"
              placeholder="账户默认"
              @update:value="onLeverChange"
            />
          </div>

          <div class="field">
            <span class="label">数量 (张)</span>
            <n-input-number
              v-model:value="form.sz"
              size="small"
              :step="lotStep"
              :min="0"
              placeholder="下单张数"
              style="width: 100%;"
            />
          </div>

          <div class="pct-quick">
            <button v-for="q in pctQuick" :key="q.v" class="pct-btn" :class="{ active: form.pct === q.v }"
                    type="button" @click="onPctChange(q.v)">{{ q.label }}</button>
          </div>
          <n-slider v-model:value="form.pct" :step="1" :format-tooltip="(v: number) => `${v}%`" :marks="{ 0: '0%', 25: '25%', 50: '50%', 75: '75%', 100: '100%' }" @update:value="onPctChange"/>

          <div class="est-line">
            <span>预估价值 <b>{{ fmtNum(estValue, 2) }}</b> USDT</span>
            <span>约保证金 <b>{{ fmtNum(estMargin, 2) }}</b> USDT</span>
          </div>

          <div class="tpsl">
            <n-checkbox v-model:checked="form.useTpSl" size="small">止盈止损</n-checkbox>
            <div v-if="form.useTpSl" class="tpsl-fields">
              <n-input-number v-model:value="form.tpPx" size="tiny" :min="0" placeholder="止盈触发价" style="width: 100%;"/>
              <n-input-number v-model:value="form.slPx" size="tiny" :min="0" placeholder="止损触发价" style="width: 100%;"/>
            </div>
          </div>

          <button class="submit-btn" :class="form.side" :disabled="submitting" @click="submitOrder">
            {{ submitting ? '提交中…' : form.side === 'buy' ? '买入(做多)' : '卖出(做空)' }}
          </button>
          <div v-if="!props.simulated" class="live-warn">实盘环境：下单/平仓将使用真实资金，请谨慎操作</div>

          <!-- 自动交易监控池：直接绑定与本面板环境一致的实例（模拟盘面板→模拟盘池，实盘面板→实盘池） -->
          <div class="auto-mon">
            <div class="auto-mon-head">
              <span class="label">自动交易监控池</span>
              <n-tag size="tiny" :bordered="false" :type="props.simulated ? 'info' : 'error'">
                {{ props.simulated ? '模拟盘池' : '实盘池' }}
              </n-tag>
              <n-switch
                size="small"
                :value="mon.state.enabled"
                @update:value="onToggleMonitor"
              />
            </div>
            <div class="auto-mon-sub">
              <span>{{ mon.state.pool.length }}/{{ SIGNAL_POOL_LIMIT }} 只</span>
              <n-tag v-if="props.simulated" size="tiny" :bordered="false" :type="autoTradeOn ? 'success' : 'default'">
                模拟盘自动下单 {{ autoTradeOn ? '启用' : '暂停' }}
              </n-tag>
              <n-tag v-else size="tiny" :bordered="false" :type="autoTradeLiveOn ? 'error' : 'default'">
                实盘自动下单 {{ autoTradeLiveOn ? '启用' : '暂停' }}
              </n-tag>
              <n-button
                v-if="mon.state.pool.length"
                size="tiny"
                quaternary
                :title="props.simulated ? '清空模拟盘监控池（不动真实资金）' : '清空实盘监控池（仅停止实盘信号监控）'"
                @click="mon.clearPool()"
              >清空</n-button>
            </div>
            <n-button size="tiny" dashed block :disabled="!selectedInst" @click="addCurrentInstToPool">
              + 添加当前合约{{ selectedInst ? `（${selectedInst}）` : '' }}
            </n-button>
            <!-- 趋势跟随：引擎定时全市场扫描，≥门槛路入池；跌破出池线时无持仓移除、有持仓暂留池中（由买卖点自动交易平仓后再移除）；与上方手动「趋势扫描」同判定同数据源 -->
            <div class="auto-trend">
              <div class="auto-trend-head">
                <span class="label">趋势跟随</span>
                <span v-if="mon.state.autoTrend.enabled && !(props.simulated ? autoPoolOn : autoPoolLiveOn)" class="at-gate">总闸未开（设置页）</span>
                <span v-if="mon.state.autoTrend.running" class="at-running">扫描中…</span>
                <n-switch size="small" :value="mon.state.autoTrend.enabled" @update:value="onToggleAutoTrend"/>
              </div>
              <template v-if="mon.state.autoTrend.enabled">
                <div class="at-opts">
                  <label>周期<n-select size="tiny" style="width: 70px" :value="mon.state.autoTrend.bar" :options="autoTrendBarOptions" @update:value="(v: string) => mon.setAutoTrend({ bar: v })"/></label>
                  <label>入池<n-select size="tiny" style="width: 76px" :value="mon.state.autoTrend.minScore" :options="autoTrendInOptions" @update:value="(v: number) => mon.setAutoTrend({ minScore: v })"/></label>
                  <label>出池<n-select size="tiny" style="width: 76px" :value="mon.state.autoTrend.exitScore" :options="autoTrendExitOptions" @update:value="(v: number) => mon.setAutoTrend({ exitScore: v })"/></label>
                  <label>间隔<n-select size="tiny" style="width: 84px" :value="mon.state.autoTrend.intervalMin" :options="autoTrendIntervalOptions" @update:value="(v: number) => mon.setAutoTrend({ intervalMin: v })"/></label>
                </div>
                <div class="at-status">
                  {{ mon.state.autoTrend.lastRunAt
                    ? `上次 ${autoTrendTime(mon.state.autoTrend.lastRunAt)} · ${mon.state.autoTrend.lastMsg}`
                    : '开启后等待下轮节拍扫描（池空也会自动入池）' }}
                </div>
              </template>
            </div>
            <div v-if="mon.state.pool.length" class="auto-mon-list">
              <div
                v-for="e in mon.state.pool"
                :key="e.code"
                class="auto-mon-row"
                :title="`${e.code} · 点击切换 K 线`"
                @click="onPoolRowClick(e)"
              >
                <span class="name">{{ e.name || poolInstId(e.code) }}</span>
                <n-button
                  size="tiny"
                  quaternary
                  circle
                  class="rm"
                  title="移出监控池"
                  @click.stop="mon.removePoolEntry(e.code)"
                >✕</n-button>
              </div>
            </div>
            <div v-else class="auto-mon-empty">
              {{ props.simulated
                ? '模拟盘池为空：池内合约的买卖点信号只在模拟盘自动下单，不动真实资金'
                : '实盘池为空：池内合约的买卖点信号将用真实资金在实盘自动下单，请谨慎' }}
            </div>
            <div class="auto-mon-foot">
              <span>监控周期</span>
              <n-select
                v-model:value="mon.state.klt"
                :options="monKltOptions"
                size="tiny"
                style="flex: 1;"
                @update:value="mon.persist()"
              />
            </div>
            <div class="auto-mon-foot">
              <span>灵敏度</span>
              <n-select
                v-model:value="mon.state.minScore"
                :options="monScoreOptions"
                size="tiny"
                style="flex: 1;"
                @update:value="mon.persist()"
              />
            </div>
            <div class="auto-mon-hint">
              {{ props.simulated
                ? (autoTradeOn
                    ? '模拟盘池已启用：池内合约的买卖点信号在模拟盘自动市价下单，不动真实资金'
                    : '当前为模拟盘池，但设置页「模拟盘自动下单」未开启，信号不会实际下单')
                : (autoTradeLiveOn
                    ? '实盘池已启用：池内合约的买卖点信号将用真实资金自动市价下单，请谨慎！'
                    : '当前为实盘池，但设置页「实盘自动下单」未开启，信号不会实际下单') }}
            </div>
          </div>
        </div>

        <!-- 右侧行情与持仓 -->
        <div class="right-area" ref="rightAreaRef" :style="narrow ? {} : { height: rightH + 'px' }">
          <div class="account-bar">
            <span>账户总权益 <b>${{ fmtNum(totalEq, 2) }}</b></span>
            <span>可用 USDT <b>{{ fmtNum(availUSDT, 2) }}</b></span>
            <n-tag v-if="!props.simulated" size="small" type="error" :bordered="false">实盘 · 真实资金</n-tag>
            <n-tag v-else size="small" type="info" :bordered="false">模拟盘</n-tag>
            <span class="refresh-ctrl" title="数据刷新基准间隔：行情=所选间隔，账户/持仓/委托=3倍，K线=6倍；仅对本面板生效并独立记忆">
              刷新
              <n-select
                v-model:value="refreshBase"
                :options="REFRESH_BASE_OPTIONS"
                size="tiny"
                style="width: 74px;"
                @update:value="onRefreshBaseChange"
              />
            </span>
            <n-button
              size="tiny"
              quaternary
              :title="upColorMode ? '当前：红涨绿跌（A股习惯），点击切换为绿涨红跌（国际习惯）；模拟盘/实盘面板共用，重启后保留' : '当前：绿涨红跌（国际习惯），点击切换为红涨绿跌（A股习惯）；模拟盘/实盘面板共用，重启后保留'"
              @click="toggleColorMode"
            >{{ upColorMode ? '红涨绿跌' : '绿涨红跌' }}</n-button>
            <n-button size="tiny" quaternary @click="refreshAll">刷新</n-button>
          </div>

          <!-- 已实现盈亏统计（合约成交，按订单聚合） -->
          <div class="trade-stats">
            <span>已实现盈亏
              <b :style="{ color: pnlColor(tradeStats.pnl) }">
                {{ tradeStats.pnl >= 0 ? '+' : '' }}{{ tradeStats.pnl.toFixed(4) }} USDT
              </b>
            </span>
            <span>总收益率
              <b :style="{ color: pnlColor(tradeStats.totalRate * 100) }">
                {{ tradeStats.totalRate >= 0 ? '+' : '' }}{{ (tradeStats.totalRate * 100).toFixed(2) }}%
              </b>
            </span>
            <span>平均收益率
              <b :style="{ color: pnlColor(tradeStats.avgRate * 100) }">
                {{ tradeStats.avgRate >= 0 ? '+' : '' }}{{ (tradeStats.avgRate * 100).toFixed(2) }}%
              </b>
            </span>
            <span>胜率
              <b>{{ (tradeStats.winRate * 100).toFixed(1) }}%</b>
              <i class="wl">{{ tradeStats.wins }}胜 / {{ tradeStats.losses }}负</i>
            </span>
            <span>成交 <b>{{ tradeStats.n }}</b> 笔</span>
            <i class="hint">按最近成交明细聚合，收益率 ≈ 净盈亏 / 成仓名义</i>
          </div>

          <div class="kline-block" :style="narrow ? {} : { flexBasis: splitPct + '%' }">
            <div class="kline-toolbar">
              <!-- 当前品种 + 最新价 + 24h 涨跌幅（随行情轮询刷新） -->
              <div class="kline-symbol">
                <span class="sym-name">{{ curInst?.instId || selectedInst }}</span>
                <span v-if="lastPrice" class="sym-last" :style="{ color: chg24h >= 0 ? upColor : downColor }">
                  {{ fmtNum(lastPrice, tickSz ? Math.max(0, Math.min(6, Math.round(-Math.log10(tickSz)))) : 4) }}
                </span>
                <span v-if="curTicker?.last" class="sym-chg" :style="{ color: chg24h >= 0 ? upColor : downColor }">
                  {{ chg24h >= 0 ? '+' : '' }}{{ chg24h.toFixed(2) }}%
                </span>
              </div>
              <div class="bar-levels">
                <button
                  v-for="b in BAR_LEVELS"
                  :key="b"
                  class="bar-btn"
                  :class="{ active: klineBar === b }"
                  @click="onBarChange(b)"
                >{{ b }}</button>
              </div>
              <div class="ind-levels">
                <button class="bar-btn ind-btn ema" :class="{ active: showEMA }" @click="onToggleEMA">EMA</button>
                <button class="bar-btn ind-btn trix" :class="{ active: showTRIX }" @click="onToggleTRIX">TRIX</button>
                <button class="bar-btn ind-btn" @click="trendScanShow = true">趋势扫描</button>
              </div>
              <n-spin v-if="klineLoading" :size="14"/>
            </div>
            <div ref="chartRef" class="kline-chart"></div>
          </div>

          <div v-if="!narrow" class="split-handle" title="拖动调节K线与订单区高度" @mousedown="onSplitDown"><i class="grip"></i></div>

          <n-tabs type="line" size="small" animated default-value="positions" class="trade-tabs">
            <n-tab-pane name="positions" tab="当前持仓">
              <div class="orders-scroll">
                <n-empty v-if="!positions.length" size="small" description="暂无持仓"/>
                <n-data-table
                  v-else
                  :columns="posColumns"
                  :data="positions"
                  :row-key="(r: any) => r.instId + r.posSide"
                  :row-props="rowProps"
                  size="small"
                  :single-line="false"
                  :scroll-x="950"
                />
              </div>
            </n-tab-pane>
            <n-tab-pane name="orders" tab="当前委托">
              <div class="orders-scroll" @scroll="onOrdersScroll('open', $event)">
                <n-empty v-if="!openShown.length" size="small" description="暂无委托"/>
                <template v-else>
                  <n-data-table
                    :columns="openOrderColumns"
                    :data="openShown"
                    :row-key="(r: any) => r.ordId"
                    :row-props="rowProps"
                    size="small"
                    :single-line="false"
                    :scroll-x="820"
                  />
                  <div class="load-hint">{{ shownHint('open') }}</div>
                </template>
              </div>
            </n-tab-pane>
            <n-tab-pane name="history" tab="历史委托">
              <div class="orders-scroll" @scroll="onOrdersScroll('history', $event)">
                <n-empty v-if="!histShown.length" size="small" description="暂无历史委托"/>
                <template v-else>
                  <n-data-table
                    :columns="historyOrderColumns"
                    :data="histShown"
                    :row-key="(r: any) => r.ordId"
                    :row-props="rowProps"
                    size="small"
                    :single-line="false"
                    :scroll-x="1010"
                  />
                  <div class="load-hint">{{ shownHint('history') }}</div>
                </template>
              </div>
            </n-tab-pane>
          </n-tabs>
        </div>
      </div>
    </n-spin>

    <!-- 趋势扫描：全市场 OKX 永续 5 路趋势证据评分（多头排列/MA20向上/Supertrend翻多/ADX强/通道上半区） -->
    <n-modal v-model:show="trendScanShow" preset="card" title="OKX 趋势扫描" :style="{ width: '760px', maxWidth: '94vw' }">
      <div class="okx-ts-bar">
        <div class="okx-ts-group">
          <span class="okx-ts-label">周期</span>
          <button v-for="b in TREND_SCAN_BAR_OPTIONS" :key="b" class="okx-ts-btn" :class="{ active: trendScanBar === b }"
            :disabled="trendScanRunning" @click="trendScanBar = b">{{ b }}</button>
        </div>
        <div class="okx-ts-group">
          <span class="okx-ts-label">门槛</span>
          <button v-for="o in TREND_SCAN_MIN_SCORES" :key="o.value" class="okx-ts-btn" :class="{ active: trendScanMinScore === o.value }"
            @click="trendScanMinScore = o.value">{{ o.label }}</button>
        </div>
        <div class="okx-ts-spacer"/>
        <n-button v-if="!trendScanRunning" size="small" type="primary" :loading="false" @click="startTrendScan">开始扫描</n-button>
        <n-button v-else size="small" @click="stopTrendScan">停止</n-button>
      </div>
      <div v-if="trendScanRunning || trendScanTotal" class="okx-ts-progress">
        <n-progress type="line" :percentage="trendScanTotal ? Math.round((trendScanDone / trendScanTotal) * 100) : 0" :height="8" :show-indicator="false"/>
        <span class="okx-ts-hint">{{ trendScanDone }}/{{ trendScanTotal || '…' }}{{ trendScanRunning ? ' 扫描中…' : ' 完成' }}</span>
      </div>
      <div v-if="trendScanRows.length" class="okx-ts-result">
        <n-data-table
          :columns="trendScanColumns"
          :data="trendScanRows"
          :row-key="(r: any) => r.instId"
          :row-props="trendScanRowProps"
          size="small"
          :single-line="false"
          :max-height="420"
        />
        <div class="okx-ts-hint okx-ts-foot">共 {{ trendScanRows.length }} 只命中 · 门槛 {{ trendScanMinScore }}/5 · 点击行切换到该合约（K 线同步切周期）</div>
      </div>
      <n-empty
        v-else-if="trendScanFinished && !trendScanRunning"
        size="small"
        :description="`当前门槛（${trendScanMinScore}/5）无命中品种，可放宽门槛或换周期后重扫`"
      />
    </n-modal>
  </div>
</template>

<style scoped>
.okx-trade-panel {
  height: 100%;
  overflow: auto;
  padding: 4px 2px;
  /* 涨跌语义色：默认红涨绿跌（A股习惯）；.color-intl 反转为绿涨红跌（跟随账户栏颜色习惯开关） */
  --up-c: #d03050;
  --down-c: #18a058;
}

.okx-trade-panel.color-intl {
  --up-c: #18a058;
  --down-c: #d03050;
}

.trade-layout {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.order-card {
  width: 300px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.row {
  display: flex;
  gap: 8px;
}

.ticker-line {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.ticker-line .last {
  font-size: 20px;
  font-weight: 700;
}

.ticker-line .chg {
  font-size: 13px;
}

.up {
  color: var(--up-c);
}

.down {
  color: var(--down-c);
}

.ticker-sub {
  display: flex;
  gap: 10px;
  font-size: 11px;
  opacity: 0.75;
}

.avail-line {
  display: flex;
  gap: 6px;
  font-size: 12px;
  opacity: 0.9;
}

.side-btns {
  display: flex;
  gap: 8px;
}

.side-btn {
  flex: 1;
  padding: 8px 0;
  border-radius: 4px;
  border: 1px solid transparent;
  cursor: pointer;
  font-size: 13px;
  background: transparent;
  opacity: 0.55;
}

.side-btn.buy {
  border-color: var(--up-c);
  color: var(--up-c);
}

.side-btn.sell {
  border-color: var(--down-c);
  color: var(--down-c);
}

.side-btn.buy.active {
  background: var(--up-c);
  color: #fff;
  opacity: 1;
}

.side-btn.sell.active {
  background: var(--down-c);
  color: #fff;
  opacity: 1;
}

.type-tabs {
  display: flex;
  gap: 14px;
  font-size: 13px;
  align-items: center;
}

.type-tabs span {
  cursor: pointer;
  opacity: 0.6;
  padding-bottom: 2px;
}

.type-tabs span.active {
  opacity: 1;
  font-weight: 600;
  border-bottom: 2px solid #18a058;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.field .label {
  font-size: 12px;
  opacity: 0.7;
}

/* 仓位快捷档位按钮 */
.pct-quick {
  display: flex;
  gap: 4px;
  margin: 4px 0 2px;
}

.pct-btn {
  flex: 1;
  padding: 2px 0;
  font-size: 11px;
  border: 1px solid rgba(128, 128, 128, 0.35);
  border-radius: 4px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  transition: all 0.15s;
}

.pct-btn:hover {
  border-color: #18a058;
  color: #18a058;
}

.pct-btn.active {
  background: #18a058;
  border-color: #18a058;
  color: #fff;
}

.est-line {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  opacity: 0.8;
}

.tpsl {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.tpsl-fields {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.submit-btn {
  padding: 10px 0;
  border: none;
  border-radius: 4px;
  color: #fff;
  font-size: 14px;
  cursor: pointer;
}

.submit-btn.buy {
  background: var(--up-c);
}

.submit-btn.sell {
  background: var(--down-c);
}

.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* 实盘环境警示（真实资金） */
.live-warn {
  font-size: 11px;
  color: #e05252;
  line-height: 1.5;
  text-align: center;
}

/* 自动交易监控池 */
.auto-mon {
  border-top: 1px dashed rgba(128, 128, 128, 0.35);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.auto-mon-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.auto-mon-head .label {
  font-size: 13px;
  font-weight: 600;
}

.auto-mon-sub {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  opacity: 0.85;
}

.auto-mon-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 180px;
  overflow: auto;
}

.auto-mon-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}

.auto-mon-row:hover {
  background: rgba(24, 160, 88, 0.1);
}

/* 趋势跟随（引擎定时扫描：命中入池；失趋势无持仓移除、有持仓暂留待自动交易平仓） */
.auto-trend {
  border-top: 1px dashed rgba(128, 128, 128, 0.35);
  padding-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.auto-trend-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.auto-trend-head .label {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
}

.at-running {
  font-size: 11px;
  color: #18a058;
}

.at-gate {
  font-size: 11px;
  color: #d03050;
}

.at-opts {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  font-size: 11px;
}

.at-opts label {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0.9;
}

.at-status {
  font-size: 11px;
  opacity: 0.75;
  line-height: 1.4;
}

.auto-mon-row .name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auto-mon-row .rm {
  flex: none;
  opacity: 0.55;
}

.auto-mon-row .rm:hover {
  opacity: 1;
}

.auto-mon-empty {
  font-size: 12px;
  opacity: 0.6;
  line-height: 1.5;
}

.auto-mon-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  opacity: 0.85;
}

.auto-mon-hint {
  font-size: 11px;
  opacity: 0.6;
  line-height: 1.5;
}

.right-area {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
}

/* 已实现盈亏统计条 */
.trade-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 16px;
  font-size: 12px;
  padding: 5px 10px;
  border: 1px solid rgba(128, 128, 128, 0.25);
  border-radius: 6px;
}

.trade-stats b {
  font-weight: 600;
}

.trade-stats .wl {
  font-style: normal;
  opacity: 0.65;
  margin-left: 4px;
}

.trade-stats .hint {
  font-style: normal;
  opacity: 0.55;
  margin-left: auto;
  font-size: 11px;
}

.account-bar {
  display: flex;
  gap: 16px;
  align-items: center;
  font-size: 13px;
  flex-shrink: 0;
}

.refresh-ctrl {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  font-size: 12px;
  color: #888;
}

.kline-symbol {
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
  margin-right: 4px;
  min-width: 0;
}

.kline-symbol .sym-name {
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.kline-symbol .sym-last {
  font-size: 13px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.kline-symbol .sym-chg {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.kline-block {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 0 0 70%;
  min-height: 240px;
}

.kline-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap; /* 窄窗口下品种行情与周期按钮换行显示，避免挤压溢出 */
}

.bar-levels {
  display: flex;
  gap: 2px;
  flex-wrap: wrap;
}

.bar-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 3px;
  opacity: 0.65;
  color: inherit;
}

.bar-btn.active {
  background: #18a058;
  color: #fff;
  opacity: 1;
}

.ind-levels {
  display: flex;
  gap: 2px;
  margin-left: auto;
}

.ind-btn.ema.active {
  background: #f59e0b;
  color: #fff;
  opacity: 1;
}

.ind-btn.trix.active {
  background: #3b82f6;
  color: #fff;
  opacity: 1;
}

.kline-chart {
  width: 100%;
  flex: 1;
  min-height: 0;
}

/* K 线区与订单区之间的拖拽分割手柄 */
.split-handle {
  flex: 0 0 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: row-resize;
  margin: -2px 0;
}

.split-handle .grip {
  width: 42px;
  height: 3px;
  border-radius: 2px;
  background: rgba(128, 128, 128, 0.35);
  transition: background 0.15s;
}

.split-handle:hover .grip,
.split-handle:active .grip {
  background: #18a058;
}

.trade-tabs {
  flex: 1 1 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.trade-tabs :deep(.n-tabs-pane-wrapper) {
  flex: 1;
  min-height: 0;
}

.trade-tabs :deep(.n-tab-pane) {
  height: 100%;
  padding: 8px 0 4px;
}

.orders-scroll {
  height: 100%;
  overflow: auto;
}

.load-hint {
  text-align: center;
  font-size: 11px;
  opacity: 0.55;
  padding: 6px 0 2px;
}

.trade-tabs :deep(.n-data-table-th) {
  font-size: 12px;
}

.trade-tabs :deep(.n-data-table-td) {
  font-size: 12px;
}

@media (max-width: 900px) {
  .trade-layout {
    flex-direction: column;
  }

  .order-card {
    width: 100%;
  }

  .kline-block {
    flex: none;
  }

  .kline-chart {
    flex: none;
    height: 320px;
  }

  .trade-tabs {
    flex: none;
  }

  .orders-scroll {
    max-height: 420px;
  }
}
</style>

<style>
/* 趋势扫描弹窗全局样式：n-modal 传送至 body，scoped 样式无法作用，用 okx-ts- 前缀防冲突 */
.okx-ts-bar {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  margin-bottom: 10px;
}

.okx-ts-group {
  display: flex;
  align-items: center;
  gap: 4px;
}

.okx-ts-label {
  font-size: 12px;
  opacity: 0.6;
  margin-right: 2px;
}

.okx-ts-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 3px;
  opacity: 0.65;
  color: inherit;
}

.okx-ts-btn.active {
  background: #18a058;
  color: #fff;
  opacity: 1;
}

.okx-ts-btn:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.okx-ts-spacer {
  flex: 1;
}

.okx-ts-progress {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.okx-ts-progress .n-progress {
  flex: 1;
}

.okx-ts-hint {
  font-size: 12px;
  opacity: 0.6;
  white-space: nowrap;
}

.okx-ts-foot {
  text-align: center;
  padding-top: 6px;
}

.okx-ts-result .n-data-table-th,
.okx-ts-result .n-data-table-td {
  font-size: 12px;
}
</style>
