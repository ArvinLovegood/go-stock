/**
 * 后台买卖点信号监控引擎。
 *
 * 为什么节拍放在 Go：窗口最小化时 Chromium 会把前端的 setInterval 深度节流
 * （intensive throttling 可达 1 次/分钟），而 Wails 事件走消息派发不受定时器节流影响。
 * 因此由 Go 侧 cron 每 60s 发 `signalMonitorTick`，前端收到后跑一轮扫描；
 * 前端自身只保留一个「Go 节拍疑似掉线」时的兜底定时器。
 *
 * 买卖点算法只存在一份 TypeScript 实现（calc.ts），本引擎直接复用，
 * 不移植到 Go —— 否则将永久双份维护且必然漂移。
 *
 * 硬约束：每只票必须取到 >= MIN_BARS 根 K 线。买点门控用 MA250、卖点波动门控用近 250 根 ATR 基准，
 * 数据不足时门控会**静默失效**，导致面板信号与图上箭头对不上。故取数 limit 与图表保持完全一致。
 */
import { reactive, watch } from 'vue'
import { EventsOn } from '../../../wailsjs/runtime'
import {
  ClearSignalRecords,
  GetEffectiveSponsorVip,
  GetSignalRecordPage,
  GetSignalStats,
  GetStockKLineWithFallback,
  IsHKTradingTime,
  IsTradingTime,
  IsUSTradingTime,
  NotifySignal,
  SaveSignalRecords,
} from '../../../wailsjs/go/main/App'
import { buySellPointsValues, temaTurnPointsValues } from './calc'
import { extractOHLCV } from './bars'
import { BUY_SELL_SCORE_OPTIONS, CN_TZ, DAILY_LIKE_KLT, DEFAULT_ADJUST, INTERVALS } from './constants'
import { chartTimeToUtcMs } from './time'
import { claimAlertToneOnce, playBuySellAlertTone, playTemaConfirmAlertTone, primeAlertAudio } from './alertSound'

/** 监控池上限（单轮取数总量上限，防止把 tick 预算撑爆） */
export const SIGNAL_POOL_LIMIT = 50
/** 单只票所需的最少 K 线根数：低于此值门控会静默失效，宁可跳过也不出假信号 */
const MIN_BARS = 300
/** 取数并发 */
const FETCH_CONCURRENCY = 4
/** 单轮 tick 预算：超时后不再发起新的取数，剩余股票留给下一轮 */
const TICK_BUDGET_MS = 25000
/** 单轮最多执行的任务数（一只票 × 一个周期 = 一个任务），超出部分按轮转留给下一轮，保证长池也能公平覆盖 */
const TICK_TASK_LIMIT = 120
/** 流水每页条数的可选值 */
export const SIGNAL_PAGE_SIZE_OPTIONS = [20, 50, 100]
/** 连播间隔：买卖点音约 0.34s、T确音 1s，留出间隙避免糊在一起 */
const TONE_GAP_MS = 800
/** 兜底定时器判定阈值：超过该时长没收到 Go 节拍，才由前端自行触发 */
const GO_TICK_STALE_MS = 150000

const PERSIST_KEY = 'signal-monitor-settings'
const TICK_EVENT = 'signalMonitorTick'

/** 监控的信号族 */
export const SIGNAL_FAMILY_OPTIONS = [
  { value: 'buysell', label: '买卖点（9 路共振）' },
  { value: 'tema', label: 'TEMA 转折（T确）' },
]

/** 提醒渠道（app=系统通知+应用内横幅） */
export const SIGNAL_CHANNEL_OPTIONS = [
  { value: 'app', label: '系统通知' },
  { value: 'feishu', label: '飞书' },
  { value: 'dingding', label: '钉钉' },
]

/** 可监控周期：年K/季K 根数太少（40/120 < 300）必然触发门控失效，直接排除 */
export const SIGNAL_INTERVAL_OPTIONS = INTERVALS.filter((it) => it.limit >= MIN_BARS)

export const signalMonitorState = reactive({
  /** 总开关 */
  enabled: false,
  /** 监控池：[{ code, name, klts }]，klts 为空数组表示继承默认周期；每只票可配多个周期，各自独立出信号 */
  pool: [],
  /** 默认监控周期 klt：票未单独配周期时用这个 */
  klt: '101',
  /** 买卖点灵敏度（与图表同一套档位定义） */
  minScore: 4,
  /** 启用的信号族 */
  families: ['buysell'],
  /** 提醒渠道 */
  channels: ['app'],
  /** 应用内提示音 */
  sound: true,
  /** 信号流水当前页（最新在前） */
  signals: [],
  /** 流水筛选：keyword 匹配代码/名称，start/end 为命中时刻（毫秒），null 表示不限 */
  filter: { keyword: '', start: null, end: null },
  /** 流水分页状态 */
  page: 1,
  pageSize: 20,
  total: 0,
  totalPages: 1,
  /** 查询进行中 */
  loading: false,
  /** 自上次打开面板以来新到的信号数（分页后无法用当前页推断未读） */
  newCount: 0,
  /** 最近一轮扫描完成时间（毫秒） */
  lastTickAt: 0,
  /** 本轮命中的信号数 */
  lastTickCount: 0,
  /** 是否正在扫描 */
  running: false,
  /** 最近一次错误信息 */
  lastError: '',
  /** 收益统计：整体 + 按股票 */
  stats: { overall: emptySignalStat(), stocks: [] },
  /** 统计区间：days 为快捷天数（1=今日、3/5/10/20），选自定义区间时 days=0 而看 start/end（毫秒） */
  statsRange: { days: 1, start: null, end: null },
  /** 上面这份统计实际覆盖的 K 线时间区间（Unix 秒），用于标注 */
  statsFrom: 0,
  statsTo: 0,
})

/** 收益统计条目的空值（overall 与 stocks 同构，便于直接渲染） */
function emptySignalStat(code = '', name = '') {
  return { code, name, trades: 0, wins: 0, winRate: 0, return: 0, open: 0 }
}

// ===== 持久化 =====

/** 旧版本遗留在 localStorage 的信号流水，仅在首次切换到 SQLite 时用于迁移 */
let legacySignals = []

/** 过滤出可监控的周期：月/季/年K 根数不足（已在选项里排除），顺便去重 */
function sanitizeKlts(v) {
  if (!Array.isArray(v)) return []
  return Array.from(new Set(v.filter((k) => SIGNAL_INTERVAL_OPTIONS.some((it) => it.klt === k))))
}

function loadSettings() {
  try {
    const raw = localStorage.getItem(PERSIST_KEY)
    if (!raw) return
    const d = JSON.parse(raw) || {}
    if (typeof d.enabled === 'boolean') signalMonitorState.enabled = d.enabled
    if (Array.isArray(d.pool)) {
      signalMonitorState.pool = d.pool
        .filter((e) => e && typeof e.code === 'string' && e.code.trim())
        .slice(0, SIGNAL_POOL_LIMIT)
        .map((e) => ({
          code: e.code.trim(),
          name: typeof e.name === 'string' ? e.name : '',
          // 空数组 = 继承全局默认周期；旧版本没有该字段，读出来即为空
          klts: sanitizeKlts(e.klts),
        }))
    }
    if (SIGNAL_INTERVAL_OPTIONS.some((it) => it.klt === d.klt)) signalMonitorState.klt = d.klt
    if (BUY_SELL_SCORE_OPTIONS.some((o) => o.value === d.minScore)) signalMonitorState.minScore = d.minScore
    if (Array.isArray(d.families)) {
      const valid = d.families.filter((f) => SIGNAL_FAMILY_OPTIONS.some((o) => o.value === f))
      if (valid.length) signalMonitorState.families = valid
    }
    if (Array.isArray(d.channels)) {
      signalMonitorState.channels = d.channels.filter((c) => SIGNAL_CHANNEL_OPTIONS.some((o) => o.value === c))
    }
    if (typeof d.sound === 'boolean') signalMonitorState.sound = d.sound
    // 旧版本把信号流水存在 localStorage，改由 SQLite 承载后仅用于一次性迁移
    if (Array.isArray(d.signals)) {
      legacySignals = d.signals.filter((s) => s && typeof s.code === 'string' && typeof s.time === 'number')
    }
  } catch { /* 脏数据忽略，用默认值 */ }
}

let persistTimer = null
function persistSettings() {
  if (persistTimer) clearTimeout(persistTimer)
  persistTimer = setTimeout(() => {
    persistTimer = null
    try {
      const { enabled, pool, klt, minScore, families, channels, sound } = signalMonitorState
      localStorage.setItem(PERSIST_KEY, JSON.stringify({ enabled, pool, klt, minScore, families, channels, sound }))
    } catch { /* 忽略配额异常 */ }
  }, 200)
}

loadSettings()

// ===== 信号流水（持久化在后端 SQLite）=====

/** 后端记录 → 面板条目；id 与运行期拼法一致，供 v-for key 使用 */
function toSignalItem(r) {
  return {
    id: `${r.code}|${r.klt}|${r.family === 'tema' ? 'tema' : 'bs'}|${r.kind}|${r.time}`,
    code: r.code,
    name: r.name || r.code,
    klt: r.klt,
    family: r.family,
    kind: r.kind,
    time: Number(r.time),
    price: r.price != null ? Number(r.price) : null,
    score: r.score != null ? Number(r.score) : null,
    detail: r.detail || '',
    at: Number(r.at) || 0,
  }
}

/** 面板条目 → 后端记录；不带 id/createdAt，由 SQLite 自增与自动填充 */
function toSignalRecord(s, at) {
  return {
    code: s.code,
    klt: s.klt,
    family: s.family,
    kind: s.kind,
    // time 会落到 Go 的 int64 字段，浮点会被 JSON 反序列化拒绝
    time: Math.round(s.time),
    name: s.name,
    price: s.price != null ? s.price : null,
    score: s.score != null ? s.score : null,
    detail: s.detail || '',
    at,
  }
}

/** 当前筛选条件 → 后端查询参数 */
function buildQuery() {
  const { filter, page, pageSize } = signalMonitorState
  return {
    keyword: (filter.keyword || '').trim(),
    // 区间筛的是「信号所在 K 线时间」，库里存的是 Unix 秒，日期控件给的是毫秒
    startTime: filter.start ? Math.floor(filter.start / 1000) : 0,
    endTime: filter.end ? Math.floor(filter.end / 1000) : 0,
    page,
    pageSize,
  }
}

/**
 * 查询当前页流水。resetPage 用于筛选条件变化时回到第 1 页。
 * 失败只记错误并保留原列表，避免面板内容突然空掉。
 */
export async function querySignals(opts = {}) {
  if (opts.resetPage) signalMonitorState.page = 1
  signalMonitorState.loading = true
  try {
    const res = await GetSignalRecordPage(buildQuery())
    const list = res && Array.isArray(res.list) ? res.list : []
    signalMonitorState.signals = list.map(toSignalItem)
    signalMonitorState.total = res && typeof res.total === 'number' ? res.total : list.length
    signalMonitorState.totalPages = res && res.totalPages > 0 ? res.totalPages : 1
    if (res && res.page > 0) signalMonitorState.page = res.page
  } catch (e) {
    signalMonitorState.lastError = `查询信号流水失败: ${e && e.message ? e.message : e}`
  } finally {
    signalMonitorState.loading = false
  }
}

/** 东八区某天 00:00 对应的 Unix 秒 */
function dayStartSec(ymd) {
  return Math.floor(Date.parse(`${ymd}T00:00:00+08:00`) / 1000)
}

/** 收益统计的快捷区间：value 为回溯天数，1 表示仅今日 */
export const SIGNAL_STATS_PRESETS = [
  { value: 1, label: '今日' },
  { value: 3, label: '3日' },
  { value: 5, label: '5日' },
  { value: 10, label: '10日' },
  { value: 20, label: '20日' },
]

/**
 * 收益统计区间 → K 线时间的 Unix 秒闭区间。
 * days 为快捷天数（1=今日…20日，按东八区自然日回溯，UTC+8 无夏令时可直接用 86400）；
 * days=0 时用自定义区间（毫秒时间戳），两个端点缺一即回落到「今日」。
 */
function statsSecondsRange() {
  const { days, start, end } = signalMonitorState.statsRange
  if (!days && start && end) {
    const from = Math.min(start, end)
    const to = Math.max(start, end)
    return { start: Math.floor(from / 1000), end: Math.floor(to / 1000) }
  }
  const n = days > 0 ? days : 1
  const today = dayStartSec(signalStatsDayLabel())
  return { start: today - (n - 1) * 86400, end: today + 86400 - 1 }
}

/** 统计口径所在的自然日（东八区 yyyy-MM-dd），前端只用于标注 */
export function signalStatsDayLabel() {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date())
}

/** 切换收益统计的快捷区间（天） */
export function setStatsPreset(days) {
  signalMonitorState.statsRange = { days, start: null, end: null }
  return loadSignalStats()
}

/** 使用自定义日期区间统计（毫秒时间戳，来自日期控件） */
export function setStatsCustomRange(start, end) {
  signalMonitorState.statsRange =
    start && end ? { days: 0, start, end } : { days: 1, start: null, end: null }
  return loadSignalStats()
}

/**
 * 刷新按买卖点信号操作的收益与胜率（整体 + 按股票）。
 * 配对与汇总在 Go 侧做（只算 buysell 族），前端只负责圈定区间。
 */
export async function loadSignalStats() {
  const { start, end } = statsSecondsRange()
  try {
    const res = await GetSignalStats({ startTime: start, endTime: end })
    const overall = res && res.overall ? res.overall : emptySignalStat()
    signalMonitorState.stats = {
      overall: { ...emptySignalStat(), ...overall },
      stocks: res && Array.isArray(res.stocks) ? res.stocks : [],
    }
    signalMonitorState.statsFrom = start
    signalMonitorState.statsTo = end
  } catch (e) {
    signalMonitorState.lastError = `统计信号收益失败: ${e && e.message ? e.message : e}`
  }
}

/** 面板启动时调用：先补一次旧版数据迁移，再查首页 */
export async function loadSignals() {
  try {
    if (legacySignals.length) {
      const probe = await GetSignalRecordPage({ page: 1, pageSize: 1 })
      // 只在库里确实为空时迁移，避免与已有数据重复
      if (!probe || !probe.total) {
        const legacy = legacySignals.slice(0, 500)
        await SaveSignalRecords(legacy.map((s) => toSignalRecord(s, Number(s.at) || 0)))
        // 覆盖 localStorage，清掉旧版流水字段，避免下次启动重复迁移
        persistSettings()
      }
      legacySignals = []
    }
  } catch { /* 迁移失败不影响正常查询 */ }
  await querySignals({ resetPage: true })
}

/** 设置筛选条件并重新查询（缺省字段保持原值，空值表示不限） */
export function setSignalFilter(filter = {}) {
  const f = signalMonitorState.filter
  if (typeof filter.keyword === 'string') f.keyword = filter.keyword
  if ('start' in filter) f.start = filter.start || null
  if ('end' in filter) f.end = filter.end || null
  return querySignals({ resetPage: true })
}

/** 翻页：pageSize 变化时回到第 1 页 */
export function setSignalPage(page, pageSize) {
  if (pageSize && pageSize !== signalMonitorState.pageSize) {
    signalMonitorState.pageSize = pageSize
    return querySignals({ resetPage: true })
  }
  signalMonitorState.page = page
  return querySignals()
}

// ===== 监控池维护 =====

export function addPoolEntry(code, name = '', klts = []) {
  const c = String(code || '').trim()
  if (!c) return { ok: false, msg: '代码不能为空' }
  if (signalMonitorState.pool.some((e) => e.code === c)) return { ok: false, msg: '该代码已在监控池中' }
  if (signalMonitorState.pool.length >= SIGNAL_POOL_LIMIT) {
    return { ok: false, msg: `监控池最多 ${SIGNAL_POOL_LIMIT} 只` }
  }
  signalMonitorState.pool.push({ code: c, name: String(name || '').trim(), klts: sanitizeKlts(klts) })
  persistSettings()
  return { ok: true, msg: '' }
}

/**
 * 某只票实际要监控的周期：未单独设置（空数组）时继承全局默认周期。
 */
export function entryKlts(entry) {
  return entry && Array.isArray(entry.klts) && entry.klts.length ? entry.klts : [signalMonitorState.klt]
}

/** 修改某只票的监控周期（传空数组 = 回到继承全局默认） */
export function setEntryKlts(code, klts) {
  const e = signalMonitorState.pool.find((it) => it.code === code)
  if (!e) return
  e.klts = sanitizeKlts(klts)
  // 周期变了，该票的基线一律作废，避免把已经存在的历史箭头当新信号
  dropBaselines(code)
  persistSettings()
}

export function removePoolEntry(code) {
  const i = signalMonitorState.pool.findIndex((e) => e.code === code)
  if (i < 0) return
  signalMonitorState.pool.splice(i, 1)
  // 池变化后基线失效，避免下次重新加入时把历史箭头当新信号
  dropBaselines(code)
  persistSettings()
}

export function clearPool() {
  signalMonitorState.pool.splice(0)
  baselines.clear()
  persistSettings()
}

export function clearSignals() {
  signalMonitorState.signals.splice(0)
  signalMonitorState.total = 0
  signalMonitorState.totalPages = 1
  signalMonitorState.page = 1
  signalMonitorState.newCount = 0
  legacySignals = []
  // 后端一并清空，否则重启后会被重新读回来
  ClearSignalRecords()
    .then(() => loadSignalStats())
    .catch(() => {})
  // persistSettings 写出的 JSON 已不含 signals 字段，顺带覆盖掉 localStorage 里的旧版残留
  persistSettings()
}

export function cycleSensitivity() {
  const i = BUY_SELL_SCORE_OPTIONS.findIndex((o) => o.value === signalMonitorState.minScore)
  signalMonitorState.minScore = BUY_SELL_SCORE_OPTIONS[(i + 1) % BUY_SELL_SCORE_OPTIONS.length].value
  persistSettings()
}

/**
 * 总开关。开启时清空基线：开监控前就已存在的历史箭头一律不补播（避免一开就响一片）。
 */
export function setMonitorEnabled(v) {
  const next = !!v
  if (next === signalMonitorState.enabled) return
  signalMonitorState.enabled = next
  if (next) baselines.clear()
  persistSettings()
}

// ===== 权限（VIP2 及以上）=====

/** 使用门槛：信号监控只对 VIP2 及以上赞助用户开放 */
export const SIGNAL_MONITOR_VIP_LEVEL = 2

/**
 * 当前生效的 VIP 等级：仅在赞助有效期内为解密等级，否则为 0。
 * 刻意不做缓存 —— 后端每次同步本地解密并判断有效期（无网络 IO），
 * 缓存会在启动早期读到空值并把 0 固化，导致 VIP2 用户被误拦。
 */
export async function currentVipLevel() {
  try {
    const res = await GetEffectiveSponsorVip()
    const lvl = Number(res?.vipLevel ?? 0)
    const active = res?.active !== false
    return active && !Number.isNaN(lvl) ? lvl : 0
  } catch {
    return 0
  }
}

/** 是否具备使用信号监控的权限 */
export async function canUseSignalMonitor() {
  return (await currentVipLevel()) >= SIGNAL_MONITOR_VIP_LEVEL
}

/** 供面板在任意设置变更后调用 */
export function persistMonitorSettings() {
  persistSettings()
}

// ===== 信号检测 =====

/**
 * 基线（最新一次已知信号所在 K 线时间），key 为 `代码|周期` ——
 * 同一只票的多个周期各记一份，互不干扰。
 * ctx 变化（换灵敏度、日内/日线口径切换）时该条目重建且不响铃 ——
 * 与图表 checkBuySellAlert 的语义保持一致：历史箭头永远不补播。
 */
const baselines = new Map()

/** 删除某只票在所有周期上的基线（票被移出池、或该票周期被改时调用） */
function dropBaselines(code) {
  const prefix = `${code}|`
  for (const k of Array.from(baselines.keys())) {
    if (k.startsWith(prefix)) baselines.delete(k)
  }
}

// 默认周期变了，继承默认的那些票换了周期，旧基线一律作废，
// 否则改回来时会把历史箭头当新信号补播一次
watch(
  () => signalMonitorState.klt,
  () => {
    for (const e of signalMonitorState.pool) {
      if (!e.klts || !e.klts.length) dropBaselines(e.code)
    }
  },
)

function latestTime(times, arr, pick = null) {
  if (!Array.isArray(arr) || !arr.length) return null
  for (let k = arr.length - 1; k >= 0; k--) {
    const item = arr[k]
    if (pick && !pick(item)) continue
    const t = times[item.i]
    if (typeof t === 'number') return t
  }
  return null
}

function latestItem(times, arr, pick = null) {
  if (!Array.isArray(arr) || !arr.length) return null
  for (let k = arr.length - 1; k >= 0; k--) {
    const item = arr[k]
    if (pick && !pick(item)) continue
    if (typeof times[item.i] === 'number') return item
  }
  return null
}

/** 信号所在 K 线时间的可读形式（yyyy-MM-dd HH:mm，东八区）。与筛选框的日期格式保持一致 */
export function formatSignalTime(t) {
  const ms = chartTimeToUtcMs(t)
  if (!Number.isFinite(ms)) return ''
  // Intl 的 zh-CN 用 “/” 分隔（2026/09/17），换成 “-” 才能和 NDatePicker 的 yyyy-MM-dd 对上
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(ms)).replace(/\//g, '-')
}

/** 计算单只票在指定周期上的信号并返回「新增」信号（无新增返回空数组） */
function detectEntry(entry, rows, klt) {
  const minScore = signalMonitorState.minScore
  if (rows.length < MIN_BARS) return []

  const { times, highs, lows, closes, vols, days } = extractOHLCV(rows)
  if (closes.length < MIN_BARS) return []

  const intraday = !DAILY_LIKE_KLT.has(klt)
  const opt = BUY_SELL_SCORE_OPTIONS.find((o) => o.value === minScore) || BUY_SELL_SCORE_OPTIONS[1]
  // 与图表 checkBuySellAlert 的 ctx 拼法一致（不含复权），后台与图表抢占同一提示音键
  const bsCtx = `${entry.code}|${klt}|${minScore}`
  const temaCtx = `${entry.code}|${klt}`
  const key = `${entry.code}|${klt}`
  const ctx = `${minScore}|${intraday ? 'i' : 'd'}`

  let st = baselines.get(key)
  if (!st || st.ctx !== ctx) {
    st = { ctx, buy: null, sell: null, temaBuy: null, temaSell: null }
    baselines.set(key, st)
  }
  const fresh = !st.ready
  st.ready = true

  const out = []

  if (signalMonitorState.families.includes('buysell')) {
    const { buys, sells } = buySellPointsValues(highs, lows, closes, vols, {
      minScore: opt.minScore,
      minGroups: opt.minGroups,
      dayKeys: days,
      intraday,
    })
    const buyTime = latestTime(times, buys)
    const sellTime = latestTime(times, sells)
    const pushIfNew = (time, item, kind) => {
      const baseField = kind === 'buy' ? 'buy' : 'sell'
      const prev = st[baseField]
      st[baseField] = time
      if (fresh || time == null || prev == null || time <= prev) return
      out.push({
        id: `${entry.code}|${klt}|bs|${kind}|${time}`,
        code: entry.code,
        name: entry.name || entry.code,
        klt,
        family: 'buysell',
        kind,
        time,
        price: typeof item.price === 'number' ? item.price : null,
        score: typeof item.score === 'number' ? item.score : null,
        detail: Array.isArray(item.reasons) && item.reasons.length ? item.reasons.join('、') : '',
        claim: `${bsCtx}|bs|${kind}|${time}`,
      })
    }
    pushIfNew(buyTime, latestItem(times, buys) || {}, 'buy')
    pushIfNew(sellTime, latestItem(times, sells) || {}, 'sell')
  }

  if (signalMonitorState.families.includes('tema')) {
    const { buys, sells } = temaTurnPointsValues(highs, lows, closes, { intraday })
    // 与图表一致：只认「T确」(kind==='conf')，T预/T强/T急 不提醒
    const onlyConf = (it) => it.kind === 'conf'
    const buyItem = latestItem(times, buys, onlyConf)
    const sellItem = latestItem(times, sells, onlyConf)
    const pushIfNew = (item, kind) => {
      const time = item ? times[item.i] : null
      const baseField = kind === 'buy' ? 'temaBuy' : 'temaSell'
      const prev = st[baseField]
      st[baseField] = time
      if (fresh || time == null || prev == null || time <= prev) return
      out.push({
        id: `${entry.code}|${klt}|tema|${kind}|${time}`,
        code: entry.code,
        name: entry.name || entry.code,
        klt,
        family: 'tema',
        kind,
        time,
        // TEMA 信号本身不带价格（只产出第几根 K 线），取该根收盘价，让流水各行信息量一致
        price: typeof closes[item.i] === 'number' ? closes[item.i] : null,
        score: null,
        detail: 'TEMA 速度温和穿越零轴',
        claim: `${temaCtx}|tema|${kind}|${time}`,
      })
    }
    pushIfNew(buyItem, 'buy')
    pushIfNew(sellItem, 'sell')
  }

  return out
}

// ===== 取数与调度 =====

let running = false
/** 任务轮转游标：池/周期数超出单轮上限时，下一轮从这里接着扫，保证公平覆盖 */
let tickCursor = 0

async function anyMarketOpen() {
  try {
    const [a, hk, us] = await Promise.all([
      IsTradingTime().catch(() => false),
      IsHKTradingTime().catch(() => false),
      IsUSTradingTime().catch(() => false),
    ])
    return !!(a || hk || us)
  } catch {
    return false
  }
}

/**
 * 复权口径必须与图表的 activeAdjust 默认值一致，否则同一只票的信号会与图上箭头对不上：
 * 场内 ETF / 港股 / 中证指数 / 海外指数默认不复权，其余个股默认前复权。
 */
function adjustForCode(code) {
  const raw = String(code || '')
  const upper = raw.toUpperCase()
  const digits = raw.replace(/[^\d]/g, '')
  const isEtf = digits.length >= 6 && ['15', '16', '50', '51', '52', '53', '56', '58'].includes(digits.substring(0, 2))
  const isHk = upper.endsWith('.HK') || upper.startsWith('HK')
  const isCsi = upper.endsWith('.CSI')
  const suffix = upper.startsWith('100.') ? upper.slice(4) : ''
  const isGlobalIndex = !!suffix && !/[0-9]/.test(suffix)
  return isEtf || isHk || isCsi || isGlobalIndex ? 'none' : DEFAULT_ADJUST
}

async function evaluateEntry(entry, klt) {
  const meta = INTERVALS.find((it) => it.klt === klt) || INTERVALS.find((it) => it.klt === '101')
  // 取数 limit 与复权口径都与图表默认一致（分时周期复权传空串）
  const adjust = DAILY_LIKE_KLT.has(meta.klt) ? adjustForCode(entry.code) : ''
  const res = await GetStockKLineWithFallback(entry.code, entry.name || '', meta.klt, meta.limit, adjust)
  const rows = Array.isArray(res?.data) ? res.data : []
  return detectEntry(entry, rows, meta.klt)
}

/**
 * 跑一轮扫描。并发 FETCH_CONCURRENCY、总预算 TICK_BUDGET_MS；
 * 单只失败只记错误，不影响其余股票。
 */
export async function runSignalTick(reason = 'manual') {
  if (!signalMonitorState.enabled) return
  if (!signalMonitorState.pool.length) return
  // 权限兜底：赞助到期后即使开关还亮着也不再扫描
  if (!(await canUseSignalMonitor())) return
  if (running) return
  running = true
  signalMonitorState.running = true
  try {
    if (!(await anyMarketOpen())) return
    // 一只票 × 一个周期 = 一个任务；轮转游标让超出单轮上限的部分在下一轮优先补上
    const tasks = []
    for (const entry of signalMonitorState.pool.slice(0, SIGNAL_POOL_LIMIT)) {
      for (const klt of entryKlts(entry)) tasks.push({ entry, klt })
    }
    if (!tasks.length) return
    const batch = Math.min(tasks.length, TICK_TASK_LIMIT)
    const from = tickCursor % tasks.length
    const picked = []
    for (let i = 0; i < batch; i++) picked.push(tasks[(from + i) % tasks.length])
    tickCursor = (from + batch) % tasks.length

    const startedAt = Date.now()
    let cursor = 0
    const hit = []
    const errors = []
    const workers = new Array(Math.min(FETCH_CONCURRENCY, picked.length)).fill(0).map(async () => {
      while (true) {
        if (Date.now() - startedAt > TICK_BUDGET_MS) return
        const idx = cursor++
        if (idx >= picked.length) return
        const { entry, klt } = picked[idx]
        try {
          const found = await evaluateEntry(entry, klt)
          if (found.length) hit.push(...found)
        } catch (e) {
          errors.push(`${entry.code} ${klt}: ${e && e.message ? e.message : e}`)
        }
      }
    })
    await Promise.all(workers)
    signalMonitorState.lastTickCount = hit.length
    signalMonitorState.lastError = errors.length ? errors.slice(0, 3).join('; ') : ''
    if (hit.length) dispatchSignals(hit)
  } catch (e) {
    signalMonitorState.lastError = e && e.message ? e.message : String(e)
  } finally {
    running = false
    signalMonitorState.running = false
    signalMonitorState.lastTickAt = Date.now()
  }
}

// ===== 提醒派发 =====

let toneQueue = []
let drainingTone = false

function queueTone(tone) {
  if (!signalMonitorState.sound) return
  toneQueue.push(tone)
  if (drainingTone) return
  drainingTone = true
  const playNext = () => {
    if (!toneQueue.length) {
      drainingTone = false
      return
    }
    const t = toneQueue.shift()
    if (t === 'tema-buy') playTemaConfirmAlertTone('buy')
    else if (t === 'tema-sell') playTemaConfirmAlertTone('sell')
    else playBuySellAlertTone(t)
    setTimeout(playNext, TONE_GAP_MS)
  }
  playNext()
}

function buildMessage(signals) {
  const labelOf = (s) => {
    if (s.family === 'tema') return s.kind === 'buy' ? 'T确买点' : 'T确卖点'
    return s.kind === 'buy' ? '买点' : '卖点'
  }
  const kltLabel = (SIGNAL_INTERVAL_OPTIONS.find((it) => it.klt === signalMonitorState.klt) || {}).label || '日K'
  if (signals.length === 1) {
    const s = signals[0]
    const scoreText = s.score != null ? `\n- **共振强度**: ${Math.round((s.score / 9) * 100)}%（${s.score}/9 路）` : ''
    const detailText = s.detail ? `\n- **命中信号**: ${s.detail}` : ''
    const priceText = s.price != null ? `\n- **信号价格**: ${s.price.toFixed(2)}` : ''
    const title = `【${labelOf(s)}】${s.name}`
    const content = `## ${labelOf(s)} · ${s.name}\n\n- **股票代码**: ${s.code}\n- **周期**: ${kltLabel}\n- **信号时间**: ${formatSignalTime(s.time)}${priceText}${scoreText}${detailText}\n\n⚠️ 共振信号只是概率优势，请结合仓位与止损执行`
    const plain = `${s.name}(${s.code}) ${labelOf(s)} ${formatSignalTime(s.time)}${s.price != null ? ` 价格 ${s.price.toFixed(2)}` : ''}`
    return { title, content, plain }
  }
  const lines = signals.map(
    (s) => `- **${s.name}**（${s.code}）${labelOf(s)} · ${formatSignalTime(s.time)}${s.price != null ? ` · ${s.price.toFixed(2)}` : ''}`,
  )
  const title = `【买卖点信号】${signals.length} 条`
  const content = `## 后台监控信号（${kltLabel}）\n\n${lines.join('\n')}\n\n⚠️ 共振信号只是概率优势，请结合仓位与止损执行`
  const plain = signals.map((s) => `${s.name}(${s.code}) ${labelOf(s)}`).join('；')
  return { title, content, plain }
}

async function dispatchSignals(hit) {
  hit.sort((a, b) => a.time - b.time)
  // at = 墙钟时间（与 K 线时间区分），供面板未读角标使用
  const now = Date.now()
  // 落库（SQLite）：不阻塞提醒流程；同一根 K 线重复上报由唯一索引兜住
  // 落库完成后再重算收益统计，否则新信号还没进库，统计会少一笔
  const hasBuySell = hit.some((s) => s.family === 'buysell')
  SaveSignalRecords(hit.map((s) => toSignalRecord(s, now)))
    .then(() => {
      if (hasBuySell) return loadSignalStats()
      return null
    })
    .catch((e) => {
      signalMonitorState.lastError = `信号落库失败: ${e && e.message ? e.message : e}`
    })

  signalMonitorState.newCount += hit.length
  // 仅在「第 1 页且无筛选」时把新信号并进列表；否则只累加新信号数，不打断正在浏览的结果
  const curFilter = signalMonitorState.filter
  const noFilter = !(curFilter.keyword || '').trim() && !curFilter.start && !curFilter.end
  if (signalMonitorState.page === 1 && noFilter) {
    for (const s of hit) signalMonitorState.signals.unshift({ ...s, at: now })
    if (signalMonitorState.signals.length > signalMonitorState.pageSize) {
      signalMonitorState.signals.splice(signalMonitorState.pageSize)
    }
    signalMonitorState.total += hit.length
    signalMonitorState.totalPages = Math.max(1, Math.ceil(signalMonitorState.total / signalMonitorState.pageSize))
  }

  // 提示音：先按「同一只票同一根 K 线买卖同现」合并，再与图表抢占同一提示音键
  const tones = []
  const bsHits = new Set()
  for (const s of hit) {
    if (s.family === 'buysell' && signalMonitorState.sound && claimAlertToneOnce(s.claim)) {
      bsHits.add(`${s.code}|${s.kind}`)
    } else if (s.family === 'tema' && signalMonitorState.sound && claimAlertToneOnce(s.claim)) {
      tones.push(s.kind === 'buy' ? 'tema-buy' : 'tema-sell')
    }
  }
  // 买卖点：同票买卖同现播三音，其余按方向播双音
  const bsCodes = new Set([...bsHits].map((k) => k.split('|')[0]))
  for (const code of bsCodes) {
    const hasBuy = bsHits.has(`${code}|buy`)
    const hasSell = bsHits.has(`${code}|sell`)
    if (hasBuy && hasSell) tones.push('both')
    else if (hasBuy) tones.push('buy')
    else if (hasSell) tones.push('sell')
  }
  if (tones.length) {
    primeAlertAudio()
    for (const t of tones) queueTone(t)
  }

  // 多通道推送：同一轮 tick 的命中合并成一条，避免刷屏
  if (signalMonitorState.channels.length) {
    const { title, content, plain } = buildMessage(hit)
    try {
      await NotifySignal(title, content, plain, signalMonitorState.channels)
    } catch (e) {
      signalMonitorState.lastError = `推送失败: ${e && e.message ? e.message : e}`
    }
  }
}

// ===== 生命周期 =====

let started = false
let lastGoTickAt = 0

/** 注册 Go 节拍监听与兜底定时器（幂等，重复调用无副作用） */
export async function startSignalMonitor() {
  if (started) return
  started = true
  // 权限不足时连监听都不注册：后台不做任何扫描，也不占取数配额
  if (!(await canUseSignalMonitor())) {
    started = false
    if (signalMonitorState.enabled) {
      // 赞助到期/未激活：把持久化里的开关一并关掉，避免面板显示「监控中」
      signalMonitorState.enabled = false
      persistSettings()
    }
    return
  }
  loadSignals()
  loadSignalStats()
  EventsOn(TICK_EVENT, () => {
    lastGoTickAt = Date.now()
    runSignalTick('go')
  })
  setInterval(() => {
    // Go 节拍正常时前端不做任何事，避免同一轮扫两遍
    if (Date.now() - lastGoTickAt < GO_TICK_STALE_MS) return
    runSignalTick('timer')
  }, 60000)
}