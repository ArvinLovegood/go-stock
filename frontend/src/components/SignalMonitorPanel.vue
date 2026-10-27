<script setup>
/**
 * 后台买卖点信号监控面板（三页签：股票监控 + OKX 模拟盘池 + OKX 实盘池）。
 *
 * 三套监控完全独立：各自的监控池、总开关、周期/灵敏度/提醒设置与流水视角；
 * OKX 双池分环境：模拟盘池的信号只在模拟盘自动下单、实盘池只在实盘下单（各受设置页对应开关约束）。
 * 本组件只负责交互与展示，引擎（kline/signalMonitor.ts 三实例）跟随应用生命周期常驻。
 * 抽屉常驻渲染（与 AI 助手抽屉同款做法），关闭态仅隐藏不销毁，避免重开时重建 DOM。
 */
import { computed, onBeforeMount, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NButton, NCard, NCheckbox, NCheckboxGroup, NDatePicker, NEmpty, NFlex, NIcon, NInput, NModal,
  NPopconfirm, NScrollbar, NSelect, NSwitch, NTabPane, NTabs, NTag, NText, NTooltip, useMessage,
} from 'naive-ui'
import { CloseOutline, PulseOutline, StatsChartOutline } from '@vicons/ionicons5'
import { GetStockList, GetConfig, GetVersionInfo, GetBinanceFuturesSymbols, GetBitgetFuturesSymbols, GetOKXTradingOverview, GetOkxInstruments, GetSignalTradePage, ClearSignalTradeRecords } from '../../wailsjs/go/main/App'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import MarketSignalScanPanel from './MarketSignalScanPanel.vue'
import { BUY_SELL_SCORE_OPTIONS } from './kline/constants'
import { BUY_SELL_MAX_SCORE } from './kline/calc'
import { alertSpeechAvailable, primeAlertSpeech, speakAlertText } from './kline/alertSound'
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from './kline/useKlineModalFit'
import {
  SIGNAL_CHANNEL_OPTIONS, SIGNAL_FAMILY_OPTIONS, SIGNAL_INTERVAL_OPTIONS, SIGNAL_POOL_LIMIT,
  SIGNAL_STATS_PRESETS, SCAN_INTERVAL_OPTIONS, applySignalScanTick, canUseSignalMonitor, formatSignalTime, persistMonitorSettings,
  stockMonitor, okxMonitor, okxLiveMonitor, startSignalMonitors,
} from './kline/signalMonitor'

const message = useMessage()
const visible = ref(false)
/** 定制版本（-tags custom）：不含 OKX/合约功能，隐藏 OKX 双池页签与自动交易流水，不加载加密合约候选 */
const isCustomBuild = ref(false)
/** 当前页签：stock=股票监控 / okx=OKX 模拟盘池 / okxLive=OKX 实盘池，三套引擎实例互不影响 */
const activeTab = ref('stock')
const monitors = { stock: stockMonitor, okx: okxMonitor, okxLive: okxLiveMonitor }
const mon = computed(() => monitors[activeTab.value])
/** OKX 双页签（模拟盘池/实盘池）共用的展示判断 */
const isOkxTab = computed(() => activeTab.value === 'okx' || activeTab.value === 'okxLive')
/** 选股候选：与「关注」一致，来自全市场（A股/指数/港美股/场内基金），不限于自选股（仅股票页签） */
const stockOptions = ref([])
/** 加密合约候选（ok:=OKX、bn:=币安、bt:=Bitget）：仅用于 OKX 双页签的搜索联想 */
const contractOptions = ref([])
let stockSearchTimer = null
let stockSearchSeq = 0
const STOCK_SEARCH_LIMIT = 20
/** 流水筛选的输入态：回车或点「查询」后才提交给后端，避免每敲一个字就查一次库 */
const keywordInput = ref('')
const rangeInput = ref(null)
/** 流水方向筛选（''=全部 / buy=仅买点 / sell=仅卖点）：选择即生效，与关键词/区间一并提交 */
const kindInput = ref('')
const kindOptions = [
  { label: '全部', value: '' },
  { label: '买点', value: 'buy' },
  { label: '卖点', value: 'sell' },
]
/** 收益统计的自定义区间输入态（毫秒），与快捷区间互斥 */
const statsRangeInput = ref(null)
/** K 线弹窗：点击信号行的「K线」按钮时打开，代码已转为东方财富格式 */
const klineShow = ref(false)
const klineCode = ref('')
const klineName = ref('')
const darkTheme = ref(false)

// K 线弹窗尺寸与图表高度自适应：与全站其他 K 线弹窗共用同一套实现
const klineWrapRef = ref(null)
const {
  chartHeight: klineChartHeight,
  attach: attachKlineFit,
  detach: detachKlineFit,
} = useKlineModalFit(klineWrapRef)

watch(klineShow, (v) => {
  if (v) {
    attachKlineFit()
    return
  }
  detachKlineFit()
})

/** OKX 模拟交易流水：面板打开时加载，独立于信号流水的筛选与分页（仅 OKX 页签展示）；滚动分页触底追加 */
const tradeList = ref([])
const tradeTotal = ref(0)
const tradePage = ref(1)
const tradePageSize = ref(20)
const tradeTotalPages = ref(1)
const tradeLoading = ref(false)

watch(visible, (v) => {
  if (v && !isCustomBuild.value) loadTrades()
})

/** 当前 OKX 页签对应的交易环境（模拟盘池/实盘池各看各的流水） */
const tradeEnv = () => (activeTab.value === 'okxLive' ? '实盘' : '模拟盘')

async function loadTrades(opts = {}) {
  tradeLoading.value = true
  try {
    const res = await GetSignalTradePage({
      keyword: '', status: '', env: tradeEnv(), startTime: 0, endTime: 0,
      page: tradePage.value, pageSize: tradePageSize.value,
    })
    const list = res && res.list ? res.list : []
    // append=true：触底加载下一页追加到尾部；否则整页替换（刷新/切环境/清空后重查）
    tradeList.value = opts.append ? tradeList.value.concat(list) : list
    tradeTotal.value = res && res.total ? Number(res.total) : 0
    tradeTotalPages.value = res && res.totalPages > 0 ? res.totalPages : 1
    if (res && res.page > 0) tradePage.value = res.page
  } catch (e) {
    if (!opts.append) {
      tradeList.value = []
      tradeTotal.value = 0
      tradeTotalPages.value = 1
    }
  } finally {
    tradeLoading.value = false
  }
}

/** 交易流水滚动分页：加载下一页追加（查询中/最后一页直接忽略） */
function loadMoreTrades() {
  if (tradeLoading.value || tradePage.value >= tradeTotalPages.value) return
  tradePage.value += 1
  loadTrades({ append: true })
}

/** 交易流水触底判定：距底部 40px 内触发（与信号流水同款） */
function onTradeScroll(e) {
  const el = e.target
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 40) {
    loadMoreTrades()
  }
}

async function clearTrades() {
  await ClearSignalTradeRecords(tradeEnv())
  tradePage.value = 1
  await loadTrades()
}

/** 墙钟毫秒 → MM-DD HH:mm:ss */
function tradeTimeText(ms) {
  const n = Number(ms) || 0
  if (!n) return ''
  const d = new Date(n)
  const p = (x) => String(x).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

const kltOptions = SIGNAL_INTERVAL_OPTIONS.map((it) => ({ label: it.label, value: it.klt }))
const scoreOptions = BUY_SELL_SCORE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))

/** 未读角标：三套监控的未读求和；面板打开时全部清零 */
const unread = computed(() => stockMonitor.state.newCount + okxMonitor.state.newCount + okxLiveMonitor.state.newCount)
/** 监控中数量：三套池里「开关已开启实例」的票数之和 */
const activeCount = computed(() =>
  (stockMonitor.state.enabled ? stockMonitor.state.pool.length : 0) +
  (okxMonitor.state.enabled ? okxMonitor.state.pool.length : 0) +
  (okxLiveMonitor.state.enabled ? okxLiveMonitor.state.pool.length : 0),
)

const sourceText = computed(() => {
  const st = mon.value.state
  if (!st.pool.length) return '监控池为空，请先添加标的'
  if (!st.enabled) return `已配置 ${st.pool.length} 只，监控未开启`
  return `监控中 · ${st.pool.length} 只`
})

const lastTickText = computed(() => {
  const st = mon.value.state
  if (!st.lastTickAt) return '尚未扫描'
  const d = new Date(st.lastTickAt)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  return `${hh}:${mm}:${ss}${st.lastTickCount ? ` · 命中 ${st.lastTickCount} 条` : ''}`
})

function labelOf(s) {
  if (s.family === 'tema') return s.kind === 'buy' ? 'T确买' : 'T确卖'
  if (s.family === 'hull') return s.kind === 'buy' ? 'H确买' : 'H确卖'
  return s.kind === 'buy' ? '买点' : '卖点'
}

/** 周期中文名：同一只票可能配了多个周期，流水里必须能区分 */
function kltLabel(klt) {
  const it = SIGNAL_INTERVAL_OPTIONS.find((o) => o.klt === klt)
  return it ? it.label : String(klt || '')
}

/** 收益率显示：+1.23% / -0.45% */
function pct(v) {
  const n = Number(v) || 0
  return `${n >= 0 ? '+' : ''}${(n * 100).toFixed(2)}%`
}

/** 胜率显示：没成交笔数时给个占位符，避免显示 0% 误导 */
function winRateText(it) {
  if (!it || !it.trades) return '—'
  return `${(it.winRate * 100).toFixed(1)}%（${it.wins}/${it.trades}）`
}

/** 收益配色按 A 股习惯：涨红跌绿，持平不染色 */
function returnColor(v) {
  const n = Number(v) || 0
  if (n > 0) return '#d03050'
  if (n < 0) return '#18a058'
  return undefined
}

/** Unix 秒 → 东八区 yyyy-MM-dd */
function dayLabel(sec) {
  const n = Number(sec) || 0
  if (!n) return ''
  const d = new Date(n * 1000)
  const p = (x) => String(x).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

/** 统计实际覆盖的 K 线时间区间：同一天只显示一个日期 */
const statsRangeText = computed(() => {
  const st = mon.value.state
  const a = dayLabel(st.statsFrom)
  if (!a) return '—'
  const b = dayLabel(st.statsTo)
  return a === b ? a : `${a} ~ ${b}`
})

/** 切到快捷区间：清掉自定义输入态，避免两处状态同时亮着 */
function onStatsPreset(days) {
  statsRangeInput.value = null
  mon.value.setStatsPreset(days)
}

/** 自定义区间：清空时回到「今日」 */
function onStatsDateRange(range) {
  statsRangeInput.value = Array.isArray(range) ? range : null
  if (Array.isArray(range) && range[0] && range[1]) {
    mon.value.setStatsCustomRange(Number(range[0]), Number(range[1]))
    return
  }
  mon.value.setStatsPreset(1)
}

/**
 * 打开/关闭面板。打开前实时校验 VIP（后端同步本地解密，微秒级，不影响打开速度），
 * 权限不足时提示并保持关闭，避免用户进了面板才发现用不了。
 */
async function togglePanel() {
  try {
    if (!visible.value && !(await ensureVip())) return
  } catch (e) {
    message.error(`打开信号监控面板失败: ${e}`)
    return
  }
  visible.value = !visible.value
}

/** VIP 门槛校验：权限不足时给出统一的引导提示 */
async function ensureVip() {
  if (await canUseSignalMonitor()) return true
  message.warning('后台买卖点信号监控仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
  return false
}

/** 总开关（当前页签的实例）：开启前先校验权限，拒绝时不改状态（开关受控于实例 state.enabled，不会误亮） */
async function onToggleMonitor(v) {
  if (v && !(await ensureVip())) return
  mon.value.setMonitorEnabled(v)
}

/**
 * 语音报名称开关。开启时借这次点击的手势预热语音合成器（自动播放策略要求），
 * 并在语音列表异步加载完成后再确认一次，避免「开了却没声」。
 */
function onToggleVoice(v) {
  mon.value.state.voice = v
  if (!v) return
  primeAlertSpeech()
  setTimeout(() => {
    if (!alertSpeechAvailable()) {
      message.warning('系统未检测到中文语音包，将只播放提示音。可在「时间和语言 → 语言」中添加中文语音后重启应用。')
      return
    }
    // 试听一句示例，与提示音开关的「开即试听」保持一致
    speakAlertText('贵州茅台 买点')
  }, 400)
}

/**
 * 提交筛选条件（空值表示不限）。
 * range 由日期控件的 update:value 透传（清空时为 null，选择时为数组）；
 * 从按钮/回车进来时拿到的是事件对象，此时回退读输入态。
 */
function applyFilter(range) {
  const r = Array.isArray(range) || range === null ? range : rangeInput.value
  const list = Array.isArray(r) ? r : []
  mon.value.setSignalFilter({ keyword: keywordInput.value, start: list[0] || null, end: list[1] || null, kind: kindInput.value })
}

/** 方向筛选选择即生效：只改方向，关键词/区间输入态原样带上 */
function onKindFilterChange(v) {
  kindInput.value = v || ''
  applyFilter()
}

function resetFilter() {
  keywordInput.value = ''
  rangeInput.value = null
  kindInput.value = ''
  mon.value.setSignalFilter({ keyword: '', start: null, end: null, kind: '' })
}

/** 信号流水滚动分页：距底部 40px 内触发加载下一页（loadMoreSignals 内部自防抖：查询中/最后一页直接忽略） */
function onSignalScroll(e) {
  const el = e.target
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 40) {
    mon.value.loadMoreSignals()
  }
}

// ===== 收益统计列表滚动分页（纯前端渐进渲染：数据已整份在内存，每次多渲染 20 只） =====
const STATS_STEP = 20
const statsShown = ref(STATS_STEP)
const visibleStats = computed(() => {
  const m = mon.value
  return m ? m.state.stats.stocks.slice(0, statsShown.value) : []
})
// 统计数据变化（切页签/切区间重查）时回到首屏；全市场扫描页签下 mon 为空，getter 需可空
watch(() => {
  const m = mon.value
  return m ? m.state.stats : null
}, () => { statsShown.value = STATS_STEP })
/** 统计列表触底：多渲染一批（已全部展示时忽略） */
function onStatsScroll(e) {
  const el = e.target
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 40) {
    if (statsShown.value < mon.value.state.stats.stocks.length) {
      statsShown.value += STATS_STEP
    }
  }
}

// 切页签：筛选输入态跟随目标实例回显，并刷新其流水
watch(activeTab, () => {
  // 全市场扫描页签独立于监控实例（无 mon 状态），切换时不动监控
  if (activeTab.value === 'marketScan') return
  const st = mon.value.state
  keywordInput.value = st.filter.keyword || ''
  rangeInput.value = st.filter.start && st.filter.end ? [st.filter.start, st.filter.end] : null
  kindInput.value = st.filter.kind || ''
  const r = st.statsRange
  statsRangeInput.value = r.days === 0 && r.start && r.end ? [r.start, r.end] : null
  mon.value.querySignals({ resetPage: true })
  // OKX 双池页签互切：交易流水按环境重查（回到第 1 页，避免停留在另一环境的翻页位置）
  if (isOkxTab.value) {
    tradePage.value = 1
    loadTrades()
  }
})

// 面板上的所有设置变更都落盘（各实例各自的设置各自持久化）
watch(
  () => [
    stockMonitor.state.enabled, stockMonitor.state.klt, stockMonitor.state.minScore, stockMonitor.state.families,
    stockMonitor.state.channels, stockMonitor.state.sound, stockMonitor.state.voice, stockMonitor.state.pool.length,
    stockMonitor.state.scanIntervalSec,
    okxMonitor.state.enabled, okxMonitor.state.klt, okxMonitor.state.minScore, okxMonitor.state.families,
    okxMonitor.state.channels, okxMonitor.state.sound, okxMonitor.state.voice, okxMonitor.state.pool.length,
    okxMonitor.state.scanIntervalSec,
    okxLiveMonitor.state.enabled, okxLiveMonitor.state.klt, okxLiveMonitor.state.minScore, okxLiveMonitor.state.families,
    okxLiveMonitor.state.channels, okxLiveMonitor.state.sound, okxLiveMonitor.state.voice, okxLiveMonitor.state.pool.length,
    okxLiveMonitor.state.scanIntervalSec,
  ],
  () => persistMonitorSettings(),
  { deep: true },
)

// 打开时清三套未读并补统计；关闭时刷新当前页签回第 1 页，避免下次打开还停在上次的翻页结果
// 定制版本无 OKX 双池，只处理股票监控实例
watch(visible, (v) => {
  if (v) {
    stockMonitor.state.newCount = 0
    stockMonitor.loadSignalStats()
    if (!isCustomBuild.value) {
      okxMonitor.state.newCount = 0
      okxLiveMonitor.state.newCount = 0
      okxMonitor.loadSignalStats()
      okxLiveMonitor.loadSignalStats()
    }
    return
  }
  // 关闭时刷新当前页签回第 1 页；全市场扫描页签无监控实例，跳过
  if (activeTab.value !== 'marketScan') mon.value.querySignals({ resetPage: true })
})

// 面板开着时新信号已直接并入列表，不必再累计未读角标
watch(
  () => [stockMonitor.state.newCount, okxMonitor.state.newCount, okxLiveMonitor.state.newCount],
  () => {
    if (!visible.value) return
    stockMonitor.state.newCount = 0
    okxMonitor.state.newCount = 0
    okxLiveMonitor.state.newCount = 0
  },
)

function closePanel() {
  visible.value = false
}

async function scanNow() {
  if (!(await ensureVip())) return
  const m = mon.value
  if (!m.state.pool.length) {
    message.warning(m.id === 'stock' ? '监控池为空，请先添加股票' : 'OKX 监控池为空，请先添加合约')
    return
  }
  if (!m.state.enabled) {
    message.warning('请先开启后台监控')
    return
  }
  await m.runSignalTick('manual')
  if (m.state.lastError) message.error(`扫描异常：${m.state.lastError}`)
  else if (!m.state.lastTickCount) message.info('本轮未发现新信号')
}

/**
 * 信号里的代码是 600519.SH 这类后缀格式，但历史流水里可能残留 sh600519 / 600519。
 * K 线组件的 code 需要东方财富格式（600519.SH），这里统一归一化。
 */
function toChartCode(code) {
  const c = String(code || '').trim()
  if (!c) return ''
  // 加密标的：非东财体系，按 ok:/bn:/bt: 前缀直通（行情组件会走对应数据源）
  const lower = c.toLowerCase()
  if (lower.startsWith('ok:')) return `ok:${c.slice(3).toUpperCase()}`
  if (lower.startsWith('bn:')) return `bn:${c.slice(3).toUpperCase()}`
  if (lower.startsWith('bt:')) return `bt:${c.slice(3).toUpperCase()}`
  if (/\.(SH|SZ|BJ|HK|US|SS|CSI)$/i.test(c)) return c.toUpperCase()
  if (/^100\.[A-Za-z]+$/.test(c)) return c.toUpperCase()
  if (lower.startsWith('sh')) return `${lower.slice(2)}.SH`
  if (lower.startsWith('sz')) return `${lower.slice(2)}.SZ`
  if (lower.startsWith('bj')) return `${lower.slice(2)}.BJ`
  if (lower.startsWith('hk')) return `${lower.slice(2).toUpperCase()}.HK`
  if (lower.startsWith('us')) return `${lower.slice(2).toUpperCase()}.US`
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return `${c}.SH`
    if (d === '4' || d === '8' || d === '9') return `${c}.BJ`
    return `${c}.SZ`
  }
  return ''
}

/** 打开 K 线弹窗：信号流水里随时核对信号对应的 K 线 */
function openKline(s) {
  const em = toChartCode(s.code)
  if (!em) {
    message.warning('该代码暂不支持K线图')
    return
  }
  klineCode.value = em
  klineName.value = s.name && s.name !== s.code ? s.name : ''
  klineShow.value = true
}

function onAddCode(code, name = '') {
  const r = mon.value.addPoolEntry(code, name)
  if (!r.ok) message.warning(r.msg)
  return r.ok
}

function onPickStock(code) {
  if (!code) return
  const opt = pickerOptions.value.find((o) => o.value === code)
  onAddCode(code, opt ? opt.name : '')
  // 选择器只当「添加」用，选完即复位，避免与监控池状态耦合
  pickCode.value = null
}

const pickCode = ref(null)

function toStockOption(item) {
  const code = item && (item.ts_code || item.TsCode) ? (item.ts_code || item.TsCode) : ''
  const name = item && (item.name || item.Name) ? (item.name || item.Name) : ''
  return { value: code, name, label: `${name} - ${code}`.trim() }
}

function loadAllStocks() {
  GetStockList('').then((list) => {
    stockOptions.value = (list || []).map(toStockOption).filter((o) => o.value)
  }).catch(() => { /* 全量列表拉取失败不影响在线搜索 */ })
}

/** 预载加密合约清单（ok:/bn:/bt:），供 OKX 页签联想；任一数据源失败只影响自己（不影响股票监控） */
function loadContracts() {
  // OKX 候选以公共全量品种表为主（含 SOXL 等美股永续；账户品种表受环境限制，模拟盘只有主流币），
  // 账户总览作兜底补充，按 instId 去重后合并；过滤 live 状态 + USDT 计价
  Promise.allSettled([GetBitgetFuturesSymbols(), GetBinanceFuturesSymbols(), GetOkxInstruments(), GetOKXTradingOverview()]).then(([btR, bnR, okxPubR, ovR]) => {
    const bt = btR.status === 'fulfilled' ? btR.value : null
    const bn = bnR.status === 'fulfilled' ? bnR.value : null
    const okxPub = okxPubR.status === 'fulfilled' ? okxPubR.value : null
    const ov = ovR.status === 'fulfilled' ? ovR.value : null
    const list = []
    for (const s of bt || []) {
      const sym = String(s.symbol || '').toUpperCase()
      if (!sym) continue
      const name = s.displayName || sym
      list.push({ value: `bt:${sym}`, name, label: `${name} - bt:${sym}` })
    }
    for (const s of bn || []) {
      const sym = String(s.symbol || '').toUpperCase()
      if (!sym) continue
      const name = s.displayName || sym
      list.push({ value: `bn:${sym}`, name, label: `${name} - bn:${sym}` })
    }
    // OKX 合约（含币安/Bitget 未上架的品种）：名称即 instId，信号联动 OKX 自动交易最直接
    const okxSeen = {}
    for (const ins of (ov && ov.instruments) || []) {
      const instId = String(inst_id_of(ins) || '').toUpperCase()
      if (!instId || okxSeen[instId]) continue
      okxSeen[instId] = true
      list.push({ value: `ok:${instId}`, name: instId, label: `${instId} - ok:${instId}` })
    }
    for (const ins of okxPub || []) {
      const instId = String(inst_id_of(ins) || '').toUpperCase()
      if (!instId || okxSeen[instId]) continue
      const state = String(ins.state || '').toLowerCase()
      if (state && state !== 'live') continue
      const isSwap = String(ins.instType || '').toUpperCase() === 'SWAP' || /-SWAP$/.test(instId)
      const ccy = String((isSwap ? ins.settleCcy : ins.quoteCcy) || '').toUpperCase()
      if (ccy && ccy !== 'USDT') continue
      okxSeen[instId] = true
      list.push({ value: `ok:${instId}`, name: instId, label: `${instId} - ok:${instId}` })
    }
    contractOptions.value = list
  })
}

function inst_id_of(ins) {
  return ins && (ins.instId || ins.InstId) ? (ins.instId || ins.InstId) : ''
}

/** 当前页签的联想候选：股票页签用全市场股票，OKX 双池页签用加密合约本地清单 */
const pickerOptions = computed(() => (isOkxTab.value ? contractOptions.value : stockOptions.value))

/**
 * 股票页签：与「关注」一致的选股方式，直接搜全市场名称/代码（不限自选股），
 * 输入防抖 300ms 调后端模糊搜索。OKX 页签：候选为本地合约清单，NSelect 自带本地过滤即可。
 */
function onSearchStock(keyword) {
  if (activeTab.value !== 'stock') return
  const k = String(keyword || '').trim()
  if (stockSearchTimer) clearTimeout(stockSearchTimer)
  if (!k) {
    loadAllStocks()
    return
  }
  const seq = ++stockSearchSeq
  stockSearchTimer = setTimeout(() => {
    GetStockList(k).then((res) => {
      if (seq !== stockSearchSeq) return
      const existing = new Set(stockOptions.value.map((o) => o.value))
      const extra = []
      for (const item of res || []) {
        const opt = toStockOption(item)
        if (opt.value && !existing.has(opt.value)) {
          extra.push(opt)
          existing.add(opt.value)
        }
        if (extra.length >= STOCK_SEARCH_LIMIT) break
      }
      if (extra.length) stockOptions.value = stockOptions.value.concat(extra)
    }).catch(() => { /* 搜索失败保留原候选 */ })
  }, 300)
}

/** OKX 自动交易双环境开关（设置页「模拟盘自动下单」「实盘自动下单」），相互独立可同时启用，仅展示用 */
const tradeSimOn = ref(false)
const tradeLiveOn = ref(false)

// 定制版本标记（后端 CustomBuild 编译开关）：隐藏 OKX 双池页签与自动交易流水，不加载加密合约候选
const customBuildReady = GetVersionInfo()
  .then((result) => { isCustomBuild.value = result?.customBuild === true })
  .catch(() => { /* 查询失败按普通版本渲染 */ })

onBeforeMount(() => {
  // K 线弹窗跟随应用主题（与 AI 助手取法一致）；同时取自动交易双环境开关做展示
  GetConfig()
    .then((cfg) => {
      darkTheme.value = !!(cfg && cfg.darkTheme)
      tradeSimOn.value = cfg?.okxSignalTradeEnable === true
      tradeLiveOn.value = cfg?.okxSignalTradeLive === true
    })
    .catch(() => { /* 读取失败按浅色渲染 */ })
})

onMounted(() => {
  // 先等版本标记就绪（GetVersionInfo 为异步，避免竞态）：定制版仅启动股票信号监控
  customBuildReady.then(() => {
    startSignalMonitors()
    loadAllStocks()
    if (!isCustomBuild.value) loadContracts()
  })
})

onBeforeUnmount(() => {
  if (stockSearchTimer) clearTimeout(stockSearchTimer)
})
</script>

<template>
  <div
    class="edge-trigger"
    :class="{ 'edge-trigger-on': activeCount > 0 }"
    :title="activeCount > 0 ? `信号监控中（${activeCount} 只）` : '买卖点信号监控'"
    @click="togglePanel"
  >
    <div class="edge-trigger-inner">
      <NIcon :component="PulseOutline" size="18" />
      <span class="edge-trigger-text">信号监控</span>
      <div v-if="unread" class="edge-trigger-badge">{{ unread > 99 ? '99+' : unread }}</div>
    </div>
  </div>

  <div :class="['drawer-wrap', { 'drawer-open': visible }]">
    <div class="drawer-mask" @click="closePanel" />
    <div class="drawer-panel monitor-drawer" @click.stop>
      <NCard
        size="small"
        class="panel-card"
        :bordered="false"
        content-style="padding: 0; display: flex; flex-direction: column; min-height: 0; overflow: hidden;"
      >
        <template #header>
          <div class="panel-header">
            <span class="panel-title">后台买卖点信号监控</span>
            <div class="panel-actions">
              <!-- 全市场扫描页签无监控实例（mon），「立即扫描」仅对三套监控页签显示 -->
              <NButton v-if="activeTab !== 'marketScan'" size="small" quaternary :loading="mon.state.running" @click="scanNow">立即扫描</NButton>
              <NButton size="small" quaternary circle title="关闭" @click="closePanel">
                <template #icon>
                  <NIcon :component="CloseOutline" />
                </template>
              </NButton>
            </div>
          </div>
        </template>

        <!-- 页签：股票监控与 OKX 双池（模拟盘/实盘）完全独立（独立池/开关/参数/流水），互不影响 -->
        <div class="tab-bar">
          <NTabs v-model:value="activeTab" type="segment" size="small" :animated="false">
            <NTabPane name="stock" tab="股票监控" />
            <!-- 定制版本不含 OKX/合约功能，隐藏 OKX 双池页签 -->
            <NTabPane v-if="!isCustomBuild" name="okx" tab="OKX 模拟盘池" />
            <NTabPane v-if="!isCustomBuild" name="okxLive" tab="OKX 实盘池" />
            <NTabPane name="marketScan" tab="全市场扫描" />
          </NTabs>
        </div>

        <NScrollbar class="panel-body">
          <!-- 全市场扫描页签：独立组件，不依赖上面三套监控实例 -->
          <MarketSignalScanPanel v-if="activeTab === 'marketScan'" />
          <template v-else>
          <div class="section">
            <NFlex justify="space-between" align="center">
              <NFlex :size="6" align="center">
                <NText strong>后台监控</NText>
                <NTooltip trigger="hover" :z-index="10002" style="max-width: 300px">
                  <template #trigger>
                    <NText depth="3" style="font-size: 12px; cursor: help;">窗口最小化也会持续提醒</NText>
                  </template>
                  节拍由 Go 侧每 60 秒派发（窗口最小化时浏览器会节流前端定时器）；
                  关闭窗口即退出应用，后台监控随之停止。
                  {{ isOkxTab ? '加密合约 7x24 交易，不受交易时段限制。' : '' }}
                </NTooltip>
              </NFlex>
              <NSwitch :value="mon.state.enabled" @update:value="onToggleMonitor" />
            </NFlex>
            <NFlex :size="6" align="center" style="margin-top: 6px;">
              <NText depth="3" style="font-size: 12px;">{{ sourceText }}</NText>
              <NText depth="3" style="font-size: 12px;">· {{ lastTickText }}</NText>
            </NFlex>
            <NText v-if="mon.state.lastError" type="error" style="font-size: 12px;">{{ mon.state.lastError }}</NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NText strong>监控池</NText>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">{{ mon.state.pool.length }}/{{ SIGNAL_POOL_LIMIT }}</NText>
                <NButton v-if="mon.state.pool.length" size="tiny" quaternary @click="mon.clearPool">清空</NButton>
              </NFlex>
            </NFlex>
            <NFlex :size="6" align="center" style="margin-top: 6px;" :wrap="false">
              <NSelect
                v-model:value="pickCode"
                :options="pickerOptions"
                filterable
                clearable
                :placeholder="isOkxTab ? '搜索加密合约（ok:/bn:/bt:，如 BTC）' : '搜索股票（名称/代码）'"
                :z-index="10002"
                style="flex: 1;"
                @search="onSearchStock"
                @update:value="onPickStock"
              />
            </NFlex>
            <div v-if="mon.state.pool.length" class="pool-list">
              <div v-for="e in mon.state.pool" :key="e.code" class="pool-row">
                <NText
                  class="pool-name"
                  style="font-size: 13px;"
                  :title="`${e.code} 点击在K线图上查看`"
                  @click="openKline(e)"
                >
                  {{ e.name || e.code }}
                </NText>
                <NText depth="3" style="font-size: 12px; flex: none;">{{ e.code }}</NText>
                <NSelect
                  multiple
                  clearable
                  size="tiny"
                  :value="mon.entryKlts(e)"
                  :options="kltOptions"
                  :max-tag-count="2"
                  :z-index="10002"
                  placeholder="继承默认周期"
                  style="flex: 1; min-width: 120px;"
                  @update:value="(v) => mon.setEntryKlts(e.code, v)"
                />
                <NButton size="tiny" quaternary circle title="移出监控池" @click="mon.removePoolEntry(e.code)">
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
            <NText v-else depth="3" style="font-size: 12px;">
              {{ isOkxTab
                ? (activeTab === 'okxLive'
                    ? '实盘池内合约的买卖点信号只在实盘环境自动下单（真实资金，需在设置页开启「实盘自动下单」）'
                    : '模拟盘池内合约的买卖点信号只在模拟盘自动下单（不动真实资金）；两池可放同一合约，两环境各下一笔')
                : '与自选股解耦，可搜索全市场股票（名称/代码）；加密标的请移步「OKX 模拟盘池 / 实盘池」页签' }}
            </NText>
          </div>

          <div class="section">
            <NText strong>检测参数</NText>
            <NFlex :size="8" align="center" style="margin-top: 6px;" :wrap="false">
              <div class="field">
                <NText depth="3" style="font-size: 12px;">默认周期</NText>
                <NSelect v-model:value="mon.state.klt" :options="kltOptions" :z-index="10002" style="width: 100%;" />
              </div>
              <div class="field">
                <NText depth="3" style="font-size: 12px;">灵敏度</NText>
                <NSelect v-model:value="mon.state.minScore" :options="scoreOptions" :z-index="10002" style="width: 100%;" />
              </div>
              <div class="field">
                <NText depth="3" style="font-size: 12px;">扫描周期</NText>
                <NSelect
                  v-model:value="mon.state.scanIntervalSec"
                  :options="SCAN_INTERVAL_OPTIONS"
                  :z-index="10002"
                  style="width: 100%;"
                  @update:value="applySignalScanTick"
                />
              </div>
            </NFlex>
            <NCheckboxGroup v-model:value="mon.state.families" style="margin-top: 8px;">
              <NFlex :size="14">
                <NCheckbox v-for="o in SIGNAL_FAMILY_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
              </NFlex>
            </NCheckboxGroup>
            <NText depth="3" style="font-size: 12px;">
              默认周期对所有票生效；监控池里可给单只票勾选多个周期，各自独立出信号
            </NText>
            <br>
            <NText depth="3" style="font-size: 12px;">
              与图表同口径：每只票取同日K根数上限的 K 线（≥300 根），门控才不会静默失效
            </NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NText strong>提醒方式</NText>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">提示音</NText>
                <NSwitch v-model:value="mon.state.sound" size="small" />
                <NTooltip trigger="hover" :z-index="10002" style="max-width: 340px">
                  <template #trigger>
                    <NText depth="3" style="font-size: 12px; cursor: help;">语音报名称</NText>
                  </template>
                  短音放完再念一句「标的名称 + 方向」，多只票同时命中时不用回到屏幕也能分辨是哪一只。
                  依赖系统中文语音包（Windows 可在「时间和语言 → 语言」中添加中文语音）；
                  开启后连播间隔会放宽。关闭提示音时语音不单独启用；打开开关时会试听一句示例。
                </NTooltip>
                <NSwitch :value="mon.state.voice" :disabled="!mon.state.sound" size="small" @update:value="onToggleVoice" />
              </NFlex>
            </NFlex>
            <NCheckboxGroup v-model:value="mon.state.channels" style="margin-top: 6px;">
              <NFlex :size="14">
                <NCheckbox v-for="o in SIGNAL_CHANNEL_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
              </NFlex>
            </NCheckboxGroup>
            <NText depth="3" style="font-size: 12px;">
              同一轮命中的多只票会合并成一条消息推送；两套监控的提醒设置各自独立
            </NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NFlex :size="6" align="center">
                <NText strong>信号收益统计</NText>
                <NTooltip trigger="hover" :z-index="10002" style="max-width: 320px">
                  <template #trigger>
                    <NText depth="3" style="font-size: 12px; cursor: help;">按买卖点操作</NText>
                  </template>
                  口径：买点开仓、卖点平仓逐笔配对（只算买卖点，TEMA 转折不参与）。
                  每笔按信号所在 K 线的收盘价满仓计算，累计收益为各笔收益率之和。
                  加密标的按自动交易语义：卖点开空、买点平空开多，空单跌了才算盈利。
                  区间内平仓的卖点会回看区间之前最多 120 天的买点来配对；区间内开仓、还没等到卖点的记为「持仓中」，不计入胜率。
                  区间按信号的 K 线时间统计。
                </NTooltip>
              </NFlex>
              <NText depth="3" style="font-size: 12px;">K线时间 {{ statsRangeText }}</NText>
            </NFlex>

            <NFlex :size="6" align="center" style="margin-top: 6px;">
              <NButton
                v-for="p in SIGNAL_STATS_PRESETS"
                :key="p.value"
                size="tiny"
                :type="mon.state.statsRange.days === p.value ? 'primary' : 'default'"
                :quaternary="mon.state.statsRange.days !== p.value"
                @click="onStatsPreset(p.value)"
              >
                {{ p.label }}
              </NButton>
              <NDatePicker
                v-model:value="statsRangeInput"
                type="daterange"
                size="small"
                format="yyyy-MM-dd"
                :actions="['clear', 'confirm']"
                :to="'.monitor-drawer'"
                placeholder="自定义区间"
                style="flex: 1; min-width: 180px;"
                @update:value="onStatsDateRange"
              />
            </NFlex>

            <NFlex :size="18" align="center" style="margin-top: 8px;">
              <div class="stat-block">
                <NText depth="3" style="font-size: 12px;">累计收益</NText>
                <NText strong :style="{ color: returnColor(mon.state.stats.overall.return) }">
                  {{ pct(mon.state.stats.overall.return) }}
                </NText>
              </div>
              <div class="stat-block">
                <NText depth="3" style="font-size: 12px;">胜率</NText>
                <NText strong>{{ winRateText(mon.state.stats.overall) }}</NText>
              </div>
              <div class="stat-block">
                <NText depth="3" style="font-size: 12px;">笔数</NText>
                <NText strong>{{ mon.state.stats.overall.trades }}</NText>
              </div>
              <div v-if="mon.state.stats.overall.open" class="stat-block">
                <NText depth="3" style="font-size: 12px;">持仓中</NText>
                <NText strong>{{ mon.state.stats.overall.open }}</NText>
              </div>
            </NFlex>

            <!-- 统计列表滚动分页：纯前端渐进渲染（每次触底多展示 20 只），数据随统计区间整体重查 -->
            <div v-if="mon.state.stats.stocks.length" class="signal-scroll" @scroll.passive="onStatsScroll">
              <div class="stat-list">
                <!-- 与信号流水同款：名称靠左，数据靠右，行内 space-between -->
                <div v-for="it in visibleStats" :key="it.code" class="stat-row">
                  <NText
                    class="stat-name"
                    :title="`${it.code} 点击在K线图上查看`"
                    @click="openKline(it)"
                  >
                    {{ it.name || it.code }}
                  </NText>
                  <NFlex :size="10" align="center" class="stat-meta">
                    <NText class="stat-col stat-col-count" depth="3">{{ it.trades }} 笔</NText>
                    <NText class="stat-col stat-col-win" depth="3">胜率 {{ winRateText(it) }}</NText>
                    <NText class="stat-col stat-col-ret" :style="{ color: returnColor(it.return) }">
                      {{ pct(it.return) }}
                    </NText>
                    <NText v-if="it.open" class="stat-col" depth="3">持仓 {{ it.open }}</NText>
                  </NFlex>
                </div>
              </div>
              <div v-if="mon.state.stats.stocks.length > STATS_STEP" class="scroll-load-hint">
                <span v-if="visibleStats.length >= mon.state.stats.stocks.length">已到底 · 共 {{ mon.state.stats.stocks.length }} 只</span>
                <span v-else>上滑加载更多（{{ visibleStats.length }}/{{ mon.state.stats.stocks.length }}）</span>
              </div>
            </div>
            <NText v-else depth="3" style="font-size: 12px;">该区间内暂无买卖点配对成交</NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NText strong>信号流水</NText>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">共 {{ mon.state.total }} 条</NText>
                <NPopconfirm v-if="mon.state.total" :z-index="10002" @positive-click="mon.clearSignals">
                  <template #trigger>
                    <NButton size="tiny" quaternary>清空</NButton>
                  </template>
                  确认清空{{ isOkxTab ? ' OKX 加密' : '股票' }}监控的全部信号流水？只影响本页签视角，不可恢复
                </NPopconfirm>
              </NFlex>
            </NFlex>

            <!-- 查询条件：关键词匹配代码/名称，时间区间按命中时刻筛，方向筛买卖点，三者同排 -->
            <div class="filter-row">
              <NFlex :size="6" align="center" :wrap="false">
                <NInput
                  v-model:value="keywordInput"
                  size="small"
                  clearable
                  placeholder="代码 / 名称"
                  style="width: 150px; flex: none;"
                  @keyup.enter="applyFilter"
                />
                <NDatePicker
                  v-model:value="rangeInput"
                  type="datetimerange"
                  size="small"
                  format="yyyy-MM-dd HH:mm"
                  :actions="['clear', 'confirm']"
                  :to="'.monitor-drawer'"
                  style="flex: 1; min-width: 0;"
                  @update:value="applyFilter"
                />
                <NSelect
                  :value="kindInput"
                  size="small"
                  :options="kindOptions"
                  :z-index="10002"
                  placeholder="全部方向"
                  style="width: 92px; flex: none;"
                  @update:value="onKindFilterChange"
                />
                <NButton size="small" type="primary" :loading="mon.state.loading" style="flex: none;" @click="applyFilter">查询</NButton>
                <NButton size="small" quaternary style="flex: none;" @click="resetFilter">重置</NButton>
              </NFlex>
            </div>

            <NEmpty
              v-if="!mon.state.signals.length"
              size="small"
              :description="mon.state.loading ? '查询中…' : '暂无信号'"
              style="margin-top: 8px;"
            />
            <div v-else class="signal-scroll" @scroll.passive="onSignalScroll">
              <div class="signal-list">
                <div v-for="s in mon.state.signals" :key="s.id" class="signal-row">
                  <NFlex :size="6" align="center">
                    <NTag size="tiny" :bordered="false" :type="s.kind === 'buy' ? 'error' : 'success'">
                      {{ labelOf(s) }}
                    </NTag>
                    <NText strong style="font-size: 13px;">{{ s.name }}</NText>
                    <NText depth="3" style="font-size: 12px;">{{ s.code }}</NText>
                    <NText depth="3" style="font-size: 12px;">{{ kltLabel(s.klt) }}</NText>
                  </NFlex>
                  <NFlex :size="6" align="center" class="signal-meta">
                    <NText depth="3" style="font-size: 12px;">
                      {{ formatSignalTime(s.time) }}
                      <span v-if="s.price != null"> · {{ s.price.toFixed(2) }}</span>
                      <span v-if="s.score != null"> · {{ Math.round((s.score / BUY_SELL_MAX_SCORE) * 100) }}%</span>
                    </NText>
                    <NButton
                      size="tiny"
                      quaternary
                      title="在K线图上查看"
                      @click="openKline(s)"
                    >
                      <template #icon>
                        <NIcon :component="StatsChartOutline" />
                      </template>
                    </NButton>
                  </NFlex>
                </div>
              </div>
              <div v-if="mon.state.total > 0" class="scroll-load-hint">
                <span v-if="mon.state.loading">加载中…</span>
                <span v-else-if="mon.state.signals.length >= mon.state.total">已到底 · 共 {{ mon.state.total }} 条</span>
                <span v-else>上滑加载更多（{{ mon.state.signals.length }}/{{ mon.state.total }}）</span>
              </div>
            </div>
          </div>

          <div v-if="isOkxTab" class="section">
            <NFlex justify="space-between" align="center">
              <NFlex :size="6" align="center">
                <NText strong>OKX 自动交易</NText>
                <NTag size="tiny" :bordered="false" :type="tradeSimOn ? 'info' : 'default'">
                  模拟盘 · {{ tradeSimOn ? '启用' : '暂停' }}
                </NTag>
                <NTag size="tiny" :bordered="false" :type="tradeLiveOn ? 'error' : 'default'">
                  实盘 · {{ tradeLiveOn ? '启用' : '暂停' }}
                </NTag>
              </NFlex>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">共 {{ tradeTotal }} 条</NText>
                <NButton size="tiny" quaternary :loading="tradeLoading" @click="loadTrades">刷新</NButton>
                <NPopconfirm v-if="tradeTotal" :z-index="10002" @positive-click="clearTrades">
                  <template #trigger>
                    <NButton size="tiny" quaternary>清空</NButton>
                  </template>
                  确认清空「{{ tradeEnv() }}」环境的全部自动交易流水？只影响本页签环境，不可恢复
                </NPopconfirm>
              </NFlex>
            </NFlex>
            <NText depth="3" style="font-size: 12px; display: block; margin-top: 2px;">
              信号只在来源池对应的环境下单：模拟盘池 → 模拟盘、实盘池 → 实盘（各受设置页对应开关约束，可同时启用或各自暂停）；
              同一合约放进两池时，同一信号两个环境各下一笔、流水分别标注环境。
              {{ tradeLiveOn ? '实盘已启用：实盘池信号将使用真实资金，请谨慎！' : '实盘未启用，不会动真实资金。' }}
            </NText>

            <NEmpty
              v-if="!tradeList.length"
              size="small"
              :description="tradeLoading ? '查询中…' : '暂无自动交易记录'"
              style="margin-top: 8px;"
            />
            <div v-else class="signal-scroll" @scroll.passive="onTradeScroll">
              <div class="signal-list">
                <div v-for="t in tradeList" :key="t.id" class="signal-row">
                <NFlex :size="6" align="center">
                  <NTag size="tiny" :bordered="false" :type="t.side === 'buy' ? 'error' : 'success'">
                    {{ t.side === 'buy' ? '买入' : '卖出' }}
                  </NTag>
                  <NTag v-if="t.family === 'reverse'" size="tiny" :bordered="false" type="warning">
                    反向平仓
                  </NTag>
                  <NTag v-else-if="t.family === 'tpsl'" size="tiny" :bordered="false" :type="t.kind === 'tp' ? 'info' : 'error'">
                    {{ t.kind === 'tp' ? '止盈平仓' : '止损平仓' }}
                  </NTag>
                  <NText strong style="font-size: 13px;">{{ t.name || t.instId }}</NText>
                  <NText depth="3" style="font-size: 12px;">{{ t.instId }}</NText>
                  <NText v-if="t.klt" depth="3" style="font-size: 12px;">{{ kltLabel(t.klt) }}</NText>
                  <NTag size="tiny" :bordered="false" :type="t.env === '实盘' ? 'error' : 'default'">
                    {{ t.env || '模拟盘' }}
                  </NTag>
                  <NTag size="tiny" :bordered="false" :type="t.status === 'success' ? 'info' : (t.status === 'skipped' ? 'default' : 'warning')">
                    {{ t.status === 'success' ? '已下单' : (t.status === 'skipped' ? '已跳过' : '失败') }}
                  </NTag>
                </NFlex>
                <NFlex :size="6" align="center" class="signal-meta">
                  <NText depth="3" style="font-size: 12px;" :title="t.status === 'success' ? `订单号 ${t.ordId || '—'}${t.errMsg ? ' · ' + t.errMsg : ''}` : (t.errMsg || '')">
                    <template v-if="t.status === 'success'">
                      {{ t.sz }} @≈{{ t.refPrice }}<template v-if="t.errMsg">（{{ t.errMsg }}）</template> · {{ tradeTimeText(t.at) }}
                    </template>
                    <template v-else>
                      {{ t.errMsg || (t.status === 'skipped' ? '已跳过' : '下单失败') }} · {{ tradeTimeText(t.at) }}
                    </template>
                  </NText>
                </NFlex>
                </div>
              </div>
              <div v-if="tradeTotal > 0" class="scroll-load-hint">
                <span v-if="tradeLoading">加载中…</span>
                <span v-else-if="tradeList.length >= tradeTotal">已到底 · 共 {{ tradeTotal }} 条</span>
                <span v-else>上滑加载更多（{{ tradeList.length }}/{{ tradeTotal }}）</span>
              </div>
            </div>
          </div>
          </template>
        </NScrollbar>
      </NCard>
    </div>
  </div>

  <!-- 信号对应的 K 线弹窗：宽度按窗口宽、图表高度按窗口高自适应 -->
  <NModal
    v-model:show="klineShow"
    :title="`${klineName || klineCode} — K线`"
    preset="card"
    :z-index="10010"
    :style="KLINE_MODAL_STYLE"
    :content-style="KLINE_MODAL_CONTENT_STYLE"
  >
    <div ref="klineWrapRef">
      <StockLightweightKlineChart
        v-if="klineShow"
        :key="'monitor-kline-' + klineCode"
        :code="klineCode"
        :stock-name="klineName"
        :dark-theme="darkTheme"
        :chart-height="klineChartHeight"
      />
    </div>
  </NModal>
</template>

<style scoped>
.edge-trigger {
  position: fixed;
  top: calc(50% + 130px);
  right: 0;
  z-index: 9997;
  transform: translateY(-50%);
  width: 32px;
  height: 96px;
  border-radius: 12px 0 0 12px;
  background: linear-gradient(135deg, #2f7d63 0%, #1f5f7a 100%);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: -2px 0 12px rgba(31, 95, 122, 0.4);
  transition: width 0.2s ease, box-shadow 0.2s ease;
}
.edge-trigger-on {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.75);
}
.edge-trigger:hover {
  width: 40px;
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.edge-trigger-text {
  font-size: 13px;
  writing-mode: vertical-rl;
  letter-spacing: 1px;
  line-height: 1;
  white-space: nowrap;
}
.edge-trigger-badge {
  position: absolute;
  top: -4px;
  left: -6px;
  min-width: 16px;
  height: 16px;
  padding: 0 3px;
  border-radius: 8px;
  background: #f97316;
  color: #fff;
  font-size: 10px;
  line-height: 16px;
  text-align: center;
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.9);
}

.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
  visibility: hidden;
  transition: visibility 0s 0.25s;
}
.drawer-wrap.drawer-open {
  visibility: visible;
  transition: visibility 0s;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.25s ease;
}
.drawer-wrap.drawer-open .drawer-mask {
  opacity: 1;
}
.drawer-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 42vw;
  min-width: 380px;
  max-width: calc(100vw - 48px);
  background: var(--n-color-modal);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
  /* 日期面板挂在抽屉上（NDatePicker 的 to），需要可见溢出，否则展开时会被裁掉 */
  overflow: visible;
  display: flex;
  flex-direction: column;
  transform: translateX(100%);
  transition: transform 0.25s ease;
}
.drawer-wrap.drawer-open .drawer-panel {
  transform: translateX(0);
}
.panel-card {
  height: 100%;
  border-radius: 0;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  /* NCard small 的 header 左右内边距是 16px，补 4px 让标题与下方区块内容左边缘对齐 */
  padding: 0 4px;
}
.panel-title {
  font-size: 15px;
  font-weight: 600;
}
.panel-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
/* 页签固定在滚动区上方，切页签时滚动位置留在各自内容上 */
.tab-bar {
  flex: none;
  padding: 4px 20px 8px;
}
.panel-body {
  flex: 1;
  min-height: 0;
}
/* 左右内边距放在每个区块上：NScrollbar 的根元素带 overflow:hidden，
   加在其上的 padding 不会把内容推开，会表现为内容紧贴面板左边缘 */
.section {
  padding: 10px 20px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}
.section:first-child {
  padding-top: 12px;
}
.section:last-child {
  border-bottom: none;
  padding-bottom: 24px;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.pool-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}
/* 每行：名称可点看 K 线 + 该票独立的周期多选 + 移出 */
.pool-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
}
.pool-name {
  cursor: pointer;
  flex: none;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pool-name:hover {
  opacity: 0.75;
}
/* 今日收益：指标块 + 按股票明细 */
.stat-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.stat-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 10px;
}
.stat-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 0;
  border-bottom: 1px dashed rgba(128, 128, 128, 0.15);
}
/* 名称居左、数据居右；数据列固定宽度，各行数字纵向对齐 */
.stat-name {
  flex: none;
  max-width: 40%;
  font-size: 13px;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stat-name:hover {
  opacity: 0.75;
}
.stat-meta {
  flex: none;
  white-space: nowrap;
}
.stat-col {
  flex: none;
  font-size: 12px;
  white-space: nowrap;
}
.stat-col-count {
  width: 46px;
}
.stat-col-win {
  width: 118px;
}
.stat-col-ret {
  width: 64px;
  text-align: right;
}
.signal-list {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
}
.signal-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 5px 0;
  border-bottom: 1px dashed rgba(128, 128, 128, 0.15);
}
/* 信号流水滚动分页容器：限高出滚动条，触底自动加载下一页 */
.signal-scroll {
  max-height: 420px;
  overflow-y: auto;
  padding-right: 4px;
}
.scroll-load-hint {
  text-align: center;
  font-size: 12px;
  color: #999;
  padding: 6px 0 2px;
}
/* 时间列整行显示不折行；左侧代码/名称过长时先让位 */
.signal-row > .n-flex:first-child {
  min-width: 0;
  overflow: hidden;
}
.signal-meta {
  flex: none;
  white-space: nowrap;
}
.filter-row {
  margin-top: 8px;
}

.pager {
  margin-top: 10px;
  display: flex;
  justify-content: center;
}
</style>
