/**
 * 后台买卖点信号监控引擎（双实例：股票监控 + OKX 加密监控）。
 *
 * 为什么是两套独立实例：股票信号监控与 OKX 加密信号监控完全解耦——
 * 各自独立的监控池、总开关、周期/灵敏度/提醒设置与流水视角（后端按 source 过滤），
 * A 股监控关着不影响 OKX 监控跑，反之亦然。共享同一套买卖点算法与提醒通道。
 * 引擎实例由 createSignalMonitor 工厂创建，localStorage 持久化 key 各自独立；
 * 旧版混在股票池里的加密标的会在首次加载时自动迁移到 OKX 池。
 *
 * 为什么节拍放在 Go：窗口最小化时 Chromium 会把前端的 setInterval 深度节流
 * （intensive throttling 可达 1 次/分钟），而 Wails 事件走消息派发不受定时器节流影响。
 * 因此由 Go 侧 cron 每 60s 发 `signalMonitorTick`，前端收到后两个实例各跑一轮扫描；
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
  GetConfig,
  GetEffectiveSponsorVip,
  GetOKXPositionListEnv,
  GetSignalRecordPage,
  GetSignalStats,
  GetStockKLineWithFallback,
  GetVersionInfo,
  IsHKTradingTime,
  IsTradingTime,
  IsUSTradingTime,
  GetSignalScanInterval,
  SetSignalScanInterval,
  NotifySignal,
  SaveSignalRecords,
} from '../../../wailsjs/go/main/App'
import { buySellPointsValues, temaTurnPointsValues, hullTurnPointsValues, BUY_SELL_MAX_SCORE } from './calc'
import { runOkxTrendScan, scanOneTrend } from './okxTrendScanner'
import { extractOHLCV } from './bars'
import { BUY_SELL_SCORE_OPTIONS, CN_TZ, DAILY_LIKE_KLT, DEFAULT_ADJUST, INTERVALS } from './constants'
import { chartTimeToUtcMs } from './time'
import { claimAlertToneOnce, playBuySellAlertTone, playTemaConfirmAlertTone, primeAlertAudio, primeAlertSpeech, speakAlertText } from './alertSound'

/** 监控池上限（单轮取数总量上限，防止把 tick 预算撑爆；股票/OKX 池各自独立计数） */
export const SIGNAL_POOL_LIMIT = 50
/** 单只票所需的最少 K 线根数：低于此值门控会静默失效，宁可跳过也不出假信号 */
const MIN_BARS = 300
/** 取数并发 */
const FETCH_CONCURRENCY = 4
/** 单轮 tick 预算：超时后不再发起新的取数，剩余股票留给下一轮 */
const TICK_BUDGET_MS = 25000
/** 单轮最多执行的任务数（一只票 × 一个周期 = 一个任务），超出部分按轮转留给下一轮，保证长池也能公平覆盖 */
const TICK_TASK_LIMIT = 120
/** 单轮 tick 硬超时看门狗：预算检查只发生在取数之间，若某个取数 Promise 永不结算（wails IPC 异常挂起），
 * running 锁会永久卡死、此后每轮 tick 都被「上一轮进行中」跳过，面板恒显「尚未扫描」。硬超时强制释放锁并把原因写入 lastError */
const TICK_HARD_TIMEOUT_MS = 90000
/** 趋势跟随单轮硬超时看门狗：出池循环是顺序 await（池上限 50 × 每次取数/持仓查询各 12s 级），弱网下单轮可能很长；
 * 超时释放 running 锁让下一轮照常调度（在途循环自行收尾，已出的入池/出池结果不丢失） */
const TREND_ROUND_TIMEOUT_MS = 10 * 60000
/** 流水每页条数的可选值 */
export const SIGNAL_PAGE_SIZE_OPTIONS = [20, 50, 100]
/** 连播间隔：买卖点音约 0.34s、T确音 1s，留出间隙避免糊在一起 */
const TONE_GAP_MS = 800
/** 开启语音播报时的连播间隔：多留出念股票名称的时间，否则下一只票的短音会打断上一句 */
const TONE_GAP_MS_SPEECH = 1800
/** 扫描节拍间隔（秒）：由 Go cron 驱动真实节奏（1~600s，面板「扫描间隔」可热更），
 * 前端兜底定时器与「Go 节拍失联」判定阈值都跟随该值 */
let scanIntervalSec = 60
/** 兜底定时器判定阈值：超过该时长没收到 Go 节拍，才由前端自行触发（间隔 + 90s 余量，防网络/休眠误判） */
const goTickStaleMs = () => scanIntervalSec * 1000 + 90000

/** 扫描节拍间隔可选值（秒）：每套监控可单独设置；Go tick 按三套的最小值唤醒，各实例再按自己的周期节流。
 *  1/5/10 秒高频档位面向加密 7x24 行情；一轮扫描可能跑不满一个高频周期，running 锁与按周期节流会自动跳过多余 tick */
export const SCAN_INTERVAL_OPTIONS = [
  { label: '1 秒', value: 1 },
  { label: '5 秒', value: 5 },
  { label: '10 秒', value: 10 },
  { label: '30 秒', value: 30 },
  { label: '1 分钟', value: 60 },
  { label: '2 分钟', value: 120 },
  { label: '5 分钟', value: 300 },
]

/**
 * 趋势跟随自动扫描间隔（分钟，仅 OKX 双池）：全市场扫描一轮要拉百余品种 × 200 根 K 线，
 * 间隔太密会持续占用取数带宽、也会让「失趋势即移除」对盘中噪音过度敏感。
 * 下限 5 分钟：加密 7x24 快节奏下的用户自选项；running 锁保证一轮没跑完时自动跳过
 * 下一轮（不会重入叠加），但 5 分钟档会长期高频占用取数带宽，且失趋势判定更贴近盘中噪音。
 */
export const AUTO_TREND_INTERVAL_OPTIONS = [
  { label: '5 分钟', value: 5 },
  { label: '15 分钟', value: 15 },
  { label: '30 分钟', value: 30 },
  { label: '1 小时', value: 60 },
  { label: '2 小时', value: 120 },
  { label: '4 小时', value: 240 },
  { label: '12 小时', value: 720 },
]

const STOCK_PERSIST_KEY = 'signal-monitor-settings'
const OKX_PERSIST_KEY = 'okx-signal-monitor-settings'
const OKX_LIVE_PERSIST_KEY = 'okx-live-signal-monitor-settings'
const TICK_EVENT = 'signalMonitorTick'

/** 监控的信号族 */
export const SIGNAL_FAMILY_OPTIONS = [
  { value: 'buysell', label: '买卖点（14 路共振）' },
  { value: 'tema', label: 'TEMA 转折（T确）' },
  { value: 'hull', label: 'Hull 转折（H确）' },
]

/** 提醒渠道（app=系统通知+应用内横幅） */
export const SIGNAL_CHANNEL_OPTIONS = [
  { value: 'app', label: '系统通知' },
  { value: 'feishu', label: '飞书' },
  { value: 'dingding', label: '钉钉' },
]

/** 可监控周期：年K/季K 根数太少（40/120 < 300）必然触发门控失效，直接排除 */
export const SIGNAL_INTERVAL_OPTIONS = INTERVALS.filter((it) => it.limit >= MIN_BARS)

/** 加密标的代码：ok:=OKX、bn:=币安、bt:=Bitget 前缀 */
export function isCryptoCode(code) {
  return /^(ok|bn|bt):/i.test(String(code || '').trim())
}

/**
 * 复权口径必须与图表的 activeAdjust 默认值一致，否则同一只票的信号会与图上箭头对不上：
 * 场内 ETF / 港股 / 中证指数 / 海外指数默认不复权，其余个股默认前复权。
 */
export function adjustForCode(code) {
  const raw = String(code || '')
  const upper = raw.toUpperCase()
  // 永续合约不做复权（与图表对 ok:/bn:/bt: 的默认口径一致）
  if (isCryptoCode(raw)) return 'none'
  const digits = raw.replace(/[^\d]/g, '')
  const isEtf = digits.length >= 6 && ['15', '16', '50', '51', '52', '53', '56', '58'].includes(digits.substring(0, 2))
  const isHk = upper.endsWith('.HK') || upper.startsWith('HK')
  const isCsi = upper.endsWith('.CSI')
  const suffix = upper.startsWith('100.') ? upper.slice(4) : ''
  const isGlobalIndex = !!suffix && !/[0-9]/.test(suffix)
  return isEtf || isHk || isCsi || isGlobalIndex ? 'none' : DEFAULT_ADJUST
}

/** 收益统计条目的空值（overall 与 stocks 同构，便于直接渲染） */
function emptySignalStat(code = '', name = '') {
  return { code, name, trades: 0, wins: 0, winRate: 0, return: 0, open: 0 }
}

/** 收益统计的快捷区间：value 为回溯天数，1 表示仅今日 */
export const SIGNAL_STATS_PRESETS = [
  { value: 1, label: '今日' },
  { value: 3, label: '3日' },
  { value: 5, label: '5日' },
  { value: 10, label: '10日' },
  { value: 20, label: '20日' },
]

/** 东八区某天 00:00 对应的 Unix 秒 */
function dayStartSec(ymd) {
  return Math.floor(Date.parse(`${ymd}T00:00:00+08:00`) / 1000)
}

/** 毫秒时间戳 → 它所在的东八区自然日（yyyy-MM-dd） */
function cnDayLabel(ms) {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(ms))
}

/** 过滤出可监控的周期：月/季/年K 根数不足（已在选项里排除），顺便去重 */
function sanitizeKlts(v) {
  if (!Array.isArray(v)) return []
  return Array.from(new Set(v.filter((k) => SIGNAL_INTERVAL_OPTIONS.some((it) => it.klt === k))))
}

/**
 * 一次性迁移：旧版把加密标的（ok:/bn:/bt:）混在股票监控池里，拆分后归位到 OKX 池。
 * 仅在 OKX 池还没有持久化配置时执行；迁移改写两份 localStorage，之后各自独立演进。
 */
function migrateCryptoPoolFromStock() {
  try {
    if (localStorage.getItem(OKX_PERSIST_KEY)) return
    const raw = localStorage.getItem(STOCK_PERSIST_KEY)
    if (!raw) return
    const d = JSON.parse(raw) || {}
    if (!Array.isArray(d.pool)) return
    const crypto = d.pool.filter((e) => e && typeof e.code === 'string' && isCryptoCode(e.code))
    if (!crypto.length) return
    d.pool = d.pool.filter((e) => !crypto.includes(e))
    const { enabled, klt, minScore, families, channels, sound, voice } = d
    localStorage.setItem(STOCK_PERSIST_KEY, JSON.stringify({ enabled, pool: d.pool, klt, minScore, families, channels, sound, voice }))
    localStorage.setItem(OKX_PERSIST_KEY, JSON.stringify({
      enabled: false, pool: crypto, klt: '101', minScore: 6, families: ['buysell'], channels: ['app'], sound: true, voice: true,
    }))
  } catch { /* 脏数据忽略，不做迁移 */ }
}
migrateCryptoPoolFromStock()

/**
 * 一次性迁移：OKX 自动交易池监控周期 5m → 15m。
 * 回测依据：裸算法在 5m 上被手续费拖垮（-33.9%/窗），降频到 15m 信号密度降为 1/3、
 * 费用拖累同比降 2/3，叠加 MA144 过滤后全池拉回≈打平（+1.18%/窗）。
 * 带迁移标记只执行一次：迁移后用户手动改回 5m 或换其他周期，不再强制覆盖。
 */
function migrateOkxPoolTo15m() {
  const FLAG = 'okx-pool-15m-migrated'
  try {
    if (localStorage.getItem(FLAG)) return
    localStorage.setItem(FLAG, '1')
    for (const key of [OKX_PERSIST_KEY, OKX_LIVE_PERSIST_KEY]) {
      const raw = localStorage.getItem(key)
      if (!raw) continue
      const d = JSON.parse(raw) || {}
      if (d.klt === '5') d.klt = '15'
      if (Array.isArray(d.pool)) {
        d.pool = d.pool.map((e) => (e && Array.isArray(e.klts)
          ? { ...e, klts: sanitizeKlts(e.klts).map((k) => (k === '5' ? '15' : k)) }
          : e))
      }
      localStorage.setItem(key, JSON.stringify(d))
    }
  } catch { /* 脏数据忽略，不做迁移 */ }
}
migrateOkxPoolTo15m()

// ===== 权限（VIP2 及以上，两套监控共用同一门槛） =====

/** 使用门槛：信号监控只对 VIP2 及以上赞助用户开放 */
export const SIGNAL_MONITOR_VIP_LEVEL = 2

/**
 * 当前生效的 VIP 等级：仅在赞助有效期内为解密等级，否则为 0。
 * 刻意不做缓存 —— 后端每次同步本地解密并判断有效期（无网络 IO），
 * 缓存会在启动早期读到空值并把 0 固化，导致 VIP2 用户被误拦。
 */
export async function currentVipLevel() {
  try {
    // 3 秒兜底：绑定调用若异常挂起（不 reject 也不 resolve），不能让面板点击陷入无响应
    const res = await Promise.race([
      GetEffectiveSponsorVip(),
      new Promise((_, rej) => setTimeout(() => rej(new Error('VIP 查询超时（绑定调用无响应）')), 3000)),
    ])
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

/** 给 Promise 套硬超时：超时 reject 但不打断在途任务（其结果自行收尾），保证调用方的锁必然释放 */
function withTimeout(promise, ms, label) {
  let timer = null
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${label}超时中止（${Math.round(ms / 1000)}s 无响应）`)), ms)
  })
  return Promise.race([promise, timeout]).finally(() => { if (timer) clearTimeout(timer) })
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

/** 后端记录 → 面板条目；id 与运行期拼法一致，供 v-for key 使用 */
function toSignalItem(r) {
  return {
    id: `${r.code}|${r.klt}|${r.family === 'tema' || r.family === 'hull' ? r.family : 'bs'}|${r.kind}|${r.time}`,
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
function toSignalRecord(s, at, envs = '') {
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
    // 来源监控池环境标注（sim=模拟盘池 / live=实盘池），后端据此决定在哪个环境自动下单
    envs,
  }
}

// ===== 提醒音（两套实例共享一条连播队列） =====

let toneQueue = []
let drainingTone = false

/**
 * 短音时长（毫秒）：语音要等音调放完再开口，否则两者叠在一起听不清方向。
 * 买卖点双音 0.30s、三音 0.47s、T确 1.0s，各留一点余量。
 */
const TONE_DUR_MS = { buy: 340, sell: 340, both: 510, 'tema-buy': 1060, 'tema-sell': 1060 }

/**
 * 播报用的标的称呼：优先中文名；名称缺失或与代码相同时退化为去掉市场前缀的代码。
 * 代码直接念会被逐字符读成「s z 零 零 二 二 四 五」，故必须剥掉前后缀。
 */
function speakName(s) {
  const raw = String(s.code || '')
  const name = (s.name || '').trim()
  if (name && name !== s.code) {
    // 合约展示名形如「苹果(AAPL)/USDT 美股永续」：只念中文别名，念出代码/单位很难听清
    if (/^(bn|bt):/i.test(raw)) return name.split('(')[0].split('/')[0].trim() || name
    // OKX 合约名即 instId（BTC-USDT-SWAP）：只念基础币种
    if (/^ok:/i.test(raw)) return name.split('-')[0].trim() || name
    return name
  }
  return raw
    .replace(/^(sh|sz|bj|hk|us|gb_)/i, '')
    .replace(/^(ok|bn|bt):/i, '')
    .replace(/\.(SH|SZ|BJ|HK|US)$/i, '')
    .split('-')[0]
    .trim()
}

/**
 * 提示音入队：tone 决定音调，say 是随后要念的一句话。
 * 开启语音播报时队列间隔放宽，好让名称念完再念下一只。
 */
function queueTone(soundOn, voiceOn, item) {
  if (!soundOn) return
  toneQueue.push(item)
  if (drainingTone) return
  drainingTone = true
  const playNext = () => {
    if (!toneQueue.length) {
      drainingTone = false
      return
    }
    const { tone, say } = toneQueue.shift()
    if (tone === 'tema-buy') playTemaConfirmAlertTone('buy')
    else if (tone === 'tema-sell') playTemaConfirmAlertTone('sell')
    else playBuySellAlertTone(tone)
    // 先短音、后语音：音调先把方向传出来，语音再补上标的名称
    if (voiceOn && say) {
      setTimeout(() => speakAlertText(say), TONE_DUR_MS[tone] || 340)
    }
    setTimeout(playNext, voiceOn ? TONE_GAP_MS_SPEECH : TONE_GAP_MS)
  }
  playNext()
}

// 语音播报与音频上下文同样受自动播放策略约束，需要在用户手势中预热。
// 引擎是应用级单例（面板由 App.vue 常驻挂载），故这里注册的手势监听无需卸载。
if (typeof window !== 'undefined') {
  window.addEventListener('pointerdown', primeAlertSpeech)
  window.addEventListener('keydown', primeAlertSpeech)
}

// ===== 引擎工厂 =====

interface MonitorOptions {
  /** 实例标识：stock=股票监控 / okx=OKX 模拟盘池 / okxLive=OKX 实盘池 */
  id: 'stock' | 'okx' | 'okxLive'
  persistKey: string
  /** 后端流水/统计的来源过滤：stock=仅股票，crypto=仅加密标的（ok:/bn:/bt:） */
  source: string
  /** 监控池代码白名单：股票池拒绝加密代码，OKX 池只收加密代码 */
  acceptCode: (code: string) => boolean
  /** 是否按 A股/港股/美股交易时段门控（加密 7x24 不停市，无需门控） */
  gateMarketOpen: boolean
  /** 交易环境标注：sim=模拟盘池 / live=实盘池，随信号上报给后端决定在哪个环境自动下单；股票池留空 */
  tradeEnv?: string
  /**
   * 趋势过滤（自动交易实例专用）：价格在 MA144 下方时丢弃 buy 信号、上方时丢弃 sell 信号。
   * 回测依据（三轮递进：5m 全期 4 合约 / 5m 随机窗 9 品种 108 窗 / 15m 随机窗 9 品种 104 天 108 窗）：
   * 无过滤全池确定亏损（5m -33.9%/窗、15m -13.5%/窗）；MA144 是全场景最稳健组件——
   * 5m 减亏 90%，15m 把全池拉回≈打平（+1.18%/窗），全面优于 MA50（15m 全池仍 -3.32%）。
   * 股票提醒实例不开启（提醒宁可多不可漏）。
   */
  trendFilter?: boolean
  /** 实例默认监控周期 klt（票未单独配周期时用；不传为日K '101'） */
  defaultKlt?: string
  /** 是否承接旧版 localStorage 信号流水的一次性迁移（仅股票实例） */
  legacySignalMigration?: boolean
  /** 启用趋势跟随（仅 OKX 双池）：定时全市场扫描，5 路命中自动入池；失趋势且有持仓的品种暂留池中，待自动交易平仓后再移除 */
  trendFollow?: boolean
}

export function createSignalMonitor(opts: MonitorOptions) {
  const state = reactive({
    /** 总开关 */
    enabled: false,
    /** 监控池：[{ code, name, klts }]，klts 为空数组表示继承默认周期；每只票可配多个周期，各自独立出信号 */
    pool: [],
    /** 默认监控周期 klt：票未单独配周期时用这个 */
    klt: opts.defaultKlt || '101',
    /** 买卖点灵敏度（与图表同一套档位定义） */
    minScore: 6,
    /** 启用的信号族 */
    families: ['buysell'],
    /** 提醒渠道 */
    channels: ['app'],
    /** 应用内提示音 */
    sound: true,
    /** 语音报名称：短音之后再念一句「股票名称 + 方向」，多只票同时命中时也能分辨（依赖系统中文语音包） */
    voice: true,
    /** 本套监控的扫描周期（秒）：Go tick 按三套的最小值唤醒，本实例按该周期节流（1~300） */
    scanIntervalSec: 60,
    /** 信号流水当前页（最新在前） */
    signals: [],
    /** 流水筛选：keyword 匹配代码/名称，start/end 为命中时刻（毫秒），null 表示不限；kind 筛方向（''=全部 / buy=买点 / sell=卖点） */
    filter: { keyword: '', start: null, end: null, kind: '' },
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
    /**
     * 趋势跟随（仅 OKX 双池，opts.trendFollow 开启的实例才有意义）：定时全市场扫描，
     * 命中 minScore 路趋势证据自动入池；池内跌破 exitScore 路进入移除流程——
     * 仍有持仓的品种暂留池中（交给买卖点自动交易平仓，平掉后下轮扫描自然移除），
     * 趋势跟随自身绝不主动平仓。入池/出池门槛刻意拉开（默认 5 入 3 出）形成滞回防横跳。
     */
    autoTrend: {
      enabled: false,
      /** 扫描/判分用的 OKX K 线周期 */
      bar: '1D',
      /** 入池门槛：>= minScore 路趋势证据才自动入池 */
      minScore: 5,
      /** 出池门槛：跌破 exitScore 路进入移除流程（须 < minScore 形成滞回） */
      exitScore: 3,
      /** 自动扫描间隔（分钟） */
      intervalMin: 60,
      /** 运行态（不持久化） */
      running: false,
      lastRunAt: 0,
      lastMsg: '',
    },
  })

  /** 基线（最新一次已知信号所在 K 线时间），key 为 `代码|周期` ——同票多周期各记一份，互不干扰 */
  const baselines = new Map()
  /** 单实例扫描锁与任务轮转游标（两实例并行扫描，互不挤占） */
  let running = false
  let tickCursor = 0
  let persistTimer = null
  /** localStorage 里是否持久化过扫描周期（未持久化时启动会从旧版全局设置迁移） */
  let scanIntervalLoaded = false
  /** 旧版本遗留在 localStorage 的信号流水，仅股票实例首次切换到 SQLite 时迁移 */
  let legacySignals = []

  function loadSettings() {
    try {
      const raw = localStorage.getItem(opts.persistKey)
      if (!raw) return
      const d = JSON.parse(raw) || {}
      if (typeof d.enabled === 'boolean') state.enabled = d.enabled
      if (Array.isArray(d.pool)) {
        state.pool = d.pool
          .filter((e) => e && typeof e.code === 'string' && e.code.trim())
          .filter((e) => opts.acceptCode(e.code))
          .slice(0, SIGNAL_POOL_LIMIT)
          .map((e) => ({
            code: e.code.trim(),
            name: typeof e.name === 'string' ? e.name : '',
            // 空数组 = 继承全局默认周期；旧版本没有该字段，读出来即为空
            klts: sanitizeKlts(e.klts),
          }))
      }
      if (SIGNAL_INTERVAL_OPTIONS.some((it) => it.klt === d.klt)) state.klt = d.klt
      if (BUY_SELL_SCORE_OPTIONS.some((o) => o.value === d.minScore)) state.minScore = d.minScore
      if (Array.isArray(d.families)) {
        const valid = d.families.filter((f) => SIGNAL_FAMILY_OPTIONS.some((o) => o.value === f))
        if (valid.length) state.families = valid
      }
      if (Array.isArray(d.channels)) {
        state.channels = d.channels.filter((c) => SIGNAL_CHANNEL_OPTIONS.some((o) => o.value === c))
      }
      if (typeof d.sound === 'boolean') state.sound = d.sound
      if (typeof d.voice === 'boolean') state.voice = d.voice
      if (SCAN_INTERVAL_OPTIONS.some((o) => o.value === d.scanIntervalSec)) {
        state.scanIntervalSec = d.scanIntervalSec
        scanIntervalLoaded = true
      }
      // 趋势跟随配置（仅 OKX 双池持久化了该节）；运行态（running/lastRunAt/lastMsg）不落盘
      if (d.autoTrend && typeof d.autoTrend === 'object') {
        const t = d.autoTrend
        if (typeof t.enabled === 'boolean') state.autoTrend.enabled = t.enabled
        if (typeof t.bar === 'string' && t.bar) state.autoTrend.bar = t.bar
        if (Number.isFinite(t.minScore) && t.minScore >= 1 && t.minScore <= 5) state.autoTrend.minScore = Number(t.minScore)
        if (Number.isFinite(t.exitScore) && t.exitScore >= 1 && t.exitScore < state.autoTrend.minScore) state.autoTrend.exitScore = Number(t.exitScore)
        if (AUTO_TREND_INTERVAL_OPTIONS.some((o) => o.value === t.intervalMin)) state.autoTrend.intervalMin = t.intervalMin
        // autoClose（移除前主动平仓）已按需求移除：趋势跟随绝不主动平仓，持久化里的旧字段直接忽略
      }
      // 旧版本把信号流水存在 localStorage，改由 SQLite 承载后仅用于一次性迁移
      if (opts.legacySignalMigration && Array.isArray(d.signals)) {
        legacySignals = d.signals.filter((s) => s && typeof s.code === 'string' && typeof s.time === 'number')
      }
    } catch { /* 脏数据忽略，用默认值 */ }
  }

  function persistSettings() {
    if (persistTimer) clearTimeout(persistTimer)
    persistTimer = setTimeout(() => {
      persistTimer = null
      try {
        const { enabled, pool, klt, minScore, families, channels, sound, voice, scanIntervalSec } = state
        const { enabled: atOn, bar: atBar, minScore: atMin, exitScore: atExit, intervalMin: atIv } = state.autoTrend
        localStorage.setItem(opts.persistKey, JSON.stringify({
          enabled, pool, klt, minScore, families, channels, sound, voice, scanIntervalSec,
          autoTrend: opts.trendFollow ? { enabled: atOn, bar: atBar, minScore: atMin, exitScore: atExit, intervalMin: atIv } : undefined,
        }))
      } catch { /* 忽略配额异常 */ }
    }, 200)
  }

  // 默认周期变了，继承默认的那些票换了周期，旧基线一律作废，
  // 否则改回来时会把历史箭头当新信号补播一次
  watch(
    () => state.klt,
    () => {
      for (const e of state.pool) {
        if (!e.klts || !e.klts.length) dropBaselines(e.code)
      }
    },
  )

  /** 删除某只票在所有周期上的基线（票被移出池、或该票周期被改时调用） */
  function dropBaselines(code) {
    const prefix = `${code}|`
    for (const k of Array.from(baselines.keys())) {
      if (k.startsWith(prefix)) baselines.delete(k)
    }
  }

  // ===== 信号流水（持久化在后端 SQLite，按 source 过滤互不串扰） =====

  /** 当前筛选条件 → 后端查询参数 */
  function buildQuery() {
    const { filter, page, pageSize } = state
    return {
      keyword: (filter.keyword || '').trim(),
      // 区间筛的是「信号所在 K 线时间」，库里存的是 Unix 秒，日期控件给的是毫秒
      startTime: filter.start ? Math.floor(filter.start / 1000) : 0,
      endTime: filter.end ? Math.floor(filter.end / 1000) : 0,
      page,
      pageSize,
      source: opts.source,
      // 方向筛选：''=全部、buy=仅买点、sell=仅卖点（买卖点/T确/H确 各族同按方向筛）
      kind: filter.kind === 'buy' || filter.kind === 'sell' ? filter.kind : '',
    }
  }

  /**
   * 查询当前页流水。resetPage 用于筛选条件变化时回到第 1 页；
   * append=true 时把本页结果追加到列表尾部（滚动分页加载更多），否则整页替换。
   * 失败只记错误并保留原列表，避免面板内容突然空掉。
   */
  async function querySignals(opts2 = {}) {
    if (opts2.resetPage) state.page = 1
    state.loading = true
    try {
      const res = await GetSignalRecordPage(buildQuery())
      const list = res && Array.isArray(res.list) ? res.list : []
      state.signals = opts2.append ? state.signals.concat(list.map(toSignalItem)) : list.map(toSignalItem)
      state.total = res && typeof res.total === 'number' ? res.total : list.length
      state.totalPages = res && res.totalPages > 0 ? res.totalPages : 1
      if (res && res.page > 0) state.page = res.page
    } catch (e) {
      state.lastError = `查询信号流水失败: ${e && e.message ? e.message : e}`
    } finally {
      state.loading = false
    }
  }

  /** 滚动分页：加载下一页并追加到列表尾部（已是最后一页或查询中时忽略） */
  function loadMoreSignals() {
    if (state.loading || state.page >= state.totalPages) return Promise.resolve()
    state.page += 1
    return querySignals({ append: true })
  }

  /** 收益统计区间 → K 线时间的 Unix 秒闭区间。 */
  function statsSecondsRange() {
    const { days, start, end } = state.statsRange
    if (!days && start && end) {
      const from = Math.min(start, end)
      const to = Math.max(start, end)
      // 日期控件的两个端点都是当日 00:00（naive-ui 对 daterange 做 startOfDay），
      // 而日K的 bar_time 落在当日 12:00：结束值不补到当日 23:59:59 会把结束日整天滤掉
      return { start: dayStartSec(cnDayLabel(from)), end: dayStartSec(cnDayLabel(to)) + 86400 - 1 }
    }
    const n = days > 0 ? days : 1
    const today = dayStartSec(cnDayLabel(Date.now()))
    return { start: today - (n - 1) * 86400, end: today + 86400 - 1 }
  }

  /** 统计口径所在的自然日（东八区 yyyy-MM-dd），前端只用于标注 */
  function signalStatsDayLabel() {
    return cnDayLabel(Date.now())
  }

  /** 切换收益统计的快捷区间（天） */
  function setStatsPreset(days) {
    state.statsRange = { days, start: null, end: null }
    return loadSignalStats()
  }

  /** 使用自定义日期区间统计（毫秒时间戳，来自日期控件） */
  function setStatsCustomRange(start, end) {
    state.statsRange =
      start && end ? { days: 0, start, end } : { days: 1, start: null, end: null }
    return loadSignalStats()
  }

  /**
   * 刷新按买卖点信号操作的收益与胜率（整体 + 按股票）。
   * 配对与汇总在 Go 侧做（只算 buysell 族），前端只负责圈定区间。
   */
  async function loadSignalStats() {
    const { start, end } = statsSecondsRange()
    try {
      const res = await GetSignalStats({ startTime: start, endTime: end, source: opts.source })
      const overall = res && res.overall ? res.overall : emptySignalStat()
      state.stats = {
        overall: { ...emptySignalStat(), ...overall },
        stocks: res && Array.isArray(res.stocks) ? res.stocks : [],
      }
      state.statsFrom = start
      state.statsTo = end
    } catch (e) {
      state.lastError = `统计信号收益失败: ${e && e.message ? e.message : e}`
    }
  }

  /** 面板启动时调用：先补一次旧版数据迁移，再查首页与统计 */
  async function boot() {
    try {
      if (legacySignals.length) {
        const probe = await GetSignalRecordPage({ page: 1, pageSize: 1, source: opts.source })
        // 只在库里确实为空时迁移，避免与已有数据重复
        if (!probe || !probe.total) {
          const legacy = legacySignals.slice(0, 500)
          await SaveSignalRecords(legacy.map((s) => toSignalRecord(s, Number(s.at) || 0, opts.tradeEnv || '')))
          // 覆盖 localStorage，清掉旧版流水字段，避免下次启动重复迁移
          persistSettings()
        }
        legacySignals = []
      }
    } catch { /* 迁移失败不影响正常查询 */ }
    await querySignals({ resetPage: true })
    await loadSignalStats()
  }

  /** 设置筛选条件并重新查询（缺省字段保持原值，空值表示不限） */
  function setSignalFilter(filter = {}) {
    const f = state.filter
    if (typeof filter.keyword === 'string') f.keyword = filter.keyword
    if ('start' in filter) f.start = filter.start || null
    if ('end' in filter) f.end = filter.end || null
    if ('kind' in filter) f.kind = filter.kind === 'buy' || filter.kind === 'sell' ? filter.kind : ''
    return querySignals({ resetPage: true })
  }

  // ===== 监控池维护 =====

  function addPoolEntry(code, name = '', klts = []) {
    const c = String(code || '').trim()
    if (!c) return { ok: false, msg: '代码不能为空' }
    if (!opts.acceptCode(c)) {
      return { ok: false, msg: opts.id === 'stock' ? '股票监控池不收加密标的（请放到 OKX 监控页签）' : 'OKX 监控池只收加密标的（ok:/bn:/bt:）' }
    }
    if (state.pool.some((e) => e.code === c)) return { ok: false, msg: '该代码已在监控池中' }
    if (state.pool.length >= SIGNAL_POOL_LIMIT) {
      return { ok: false, msg: `监控池最多 ${SIGNAL_POOL_LIMIT} 只` }
    }
    state.pool.push({ code: c, name: String(name || '').trim(), klts: sanitizeKlts(klts) })
    persistSettings()
    return { ok: true, msg: '' }
  }

  /**
   * 某只票实际要监控的周期：未单独设置（空数组）时继承全局默认周期。
   */
  function entryKlts(entry) {
    return entry && Array.isArray(entry.klts) && entry.klts.length ? entry.klts : [state.klt]
  }

  /** 修改某只票的监控周期（传空数组 = 回到继承全局默认） */
  function setEntryKlts(code, klts) {
    const e = state.pool.find((it) => it.code === code)
    if (!e) return
    e.klts = sanitizeKlts(klts)
    // 周期变了，该票的基线一律作废，避免把已经存在的历史箭头当新信号
    dropBaselines(code)
    persistSettings()
  }

  function removePoolEntry(code) {
    const i = state.pool.findIndex((e) => e.code === code)
    if (i < 0) return
    state.pool.splice(i, 1)
    // 池变化后基线失效，避免下次重新加入时把历史箭头当新信号
    dropBaselines(code)
    persistSettings()
  }

  function clearPool() {
    state.pool.splice(0)
    baselines.clear()
    persistSettings()
  }

  function clearSignals() {
    state.signals.splice(0)
    state.total = 0
    state.totalPages = 1
    state.page = 1
    state.newCount = 0
    legacySignals = []
    // 后端按来源一并清空（只清本实例视角的流水），否则重启后会被重新读回来
    ClearSignalRecords(opts.source)
      .then(() => loadSignalStats())
      .catch(() => {})
    persistSettings()
  }

  function cycleSensitivity() {
    const i = BUY_SELL_SCORE_OPTIONS.findIndex((o) => o.value === state.minScore)
    state.minScore = BUY_SELL_SCORE_OPTIONS[(i + 1) % BUY_SELL_SCORE_OPTIONS.length].value
    persistSettings()
  }

  /**
   * 总开关。开启时清空基线：开监控前就已存在的历史箭头一律不补播（避免一开就响一片）。
   * 自愈：启动时 VIP 校验瞬时失败（如启动高峰期 3s 兜底超时）会不注册节拍监听并强制关开关且不复位——
   * 用户此刻重新打开开关时再次拉起引擎（幂等），权限已恢复即可恢复定时扫描。
   */
  function setMonitorEnabled(v) {
    const next = !!v
    if (next === state.enabled) return
    state.enabled = next
    if (next) baselines.clear()
    persistSettings()
    if (next) startSignalMonitors()
  }

  // ===== 信号检测 =====

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

  /** 计算单只票在指定周期上的信号并返回「新增」信号（无新增返回空数组） */
  function detectEntry(entry, rows, klt) {
    const minScore = state.minScore
    if (rows.length < MIN_BARS) return []

    const { times, highs, lows, closes, vols, days } = extractOHLCV(rows)
    if (closes.length < MIN_BARS) return []

    const intraday = !DAILY_LIKE_KLT.has(klt)
    const opt = BUY_SELL_SCORE_OPTIONS.find((o) => o.value === minScore) || BUY_SELL_SCORE_OPTIONS[1]
    // 与图表 checkBuySellAlert 的 ctx 拼法一致（不含复权），后台与图表抢占同一提示音键
    const bsCtx = `${entry.code}|${klt}`
    const temaCtx = `${entry.code}|${klt}`
    const hullCtx = `${entry.code}|${klt}`
    const key = `${entry.code}|${klt}`
    const ctx = `${minScore}|${intraday ? 'i' : 'd'}`

    let st = baselines.get(key)
    if (!st || st.ctx !== ctx) {
      st = { ctx, buy: null, sell: null, temaBuy: null, temaSell: null, hullBuy: null, hullSell: null }
      baselines.set(key, st)
    }
    const fresh = !st.ready
    st.ready = true

    const out = []

    if (state.families.includes('buysell')) {
      const { buys, sells } = buySellPointsValues(highs, lows, closes, vols, {
        minScore: opt.minScore,
        minGroups: opt.minGroups,
        dayKeys: days,
        intraday,
      })
      const buyTime = latestTime(times, buys)
      const sellTime = latestTime(times, sells)
      // 同根 K 线方向冲突裁决：买卖点同根同时触发时只保留分数高的一方，同分视为无方向（都不发）。
      // 实测震荡行情下多路指标会同时给出买/卖共振（如 CCI/RSI 报买、KDJ/ADX 报卖），
      // 不裁决会同一分钟 buy/sell 双发，自动交易端直接形成一对对敲磨损单。
      let effBuyTime = buyTime
      let effSellTime = sellTime
      if (buyTime != null && sellTime != null && buyTime === sellTime) {
        const bs = (latestItem(times, buys) || {}).score ?? 0
        const ss = (latestItem(times, sells) || {}).score ?? 0
        if (bs > ss) effSellTime = null
        else if (ss > bs) effBuyTime = null
        else {
          effBuyTime = null
          effSellTime = null
        }
      }
      // 趋势过滤（仅自动交易实例开启）：信号根收盘价与 MA144 比较判断趋势方向，
      // 逆势信号直接丢弃——下跌趋势中的 buy 是接飞刀（SOXL 深跌途中 4 路高分 buy 连环错）。
      // 阈值取 MA144 而非 MA50：三轮回测中 MA144 全场景最优（15m 全池 ≈打平 vs MA50 仍 -3.32%/窗）。
      // 注意 baseline 照常更新（传原始 time），只拦截 out.push：否则基线卡 null 会吞掉后续所有新信号。
      const trendBlocked = (item) => {
        if (!opts.trendFilter || item.i == null || item.i + 1 < 144) return false
        let sum = 0
        for (let k = item.i - 143; k <= item.i; k++) sum += closes[k]
        const ma144 = sum / 144
        if (!(ma144 > 0)) return false
        const px = closes[item.i]
        return item.kind === 'buy' ? px < ma144 : px > ma144
      }
      const buyItem = latestItem(times, buys) || {}
      const sellItem = latestItem(times, sells) || {}
      const pushIfNew = (time, item, kind) => {
        const baseField = kind === 'buy' ? 'buy' : 'sell'
        const prev = st[baseField]
        st[baseField] = time
        if (fresh || time == null || prev == null || time <= prev) return
        if (trendBlocked({ ...item, kind })) return
        out.push({
          id: `${entry.code}|${klt}|bs|${kind}|${time}`,
          code: entry.code,
          name: entry.name || entry.code,
          klt,
          family: 'buysell',
          kind,
          time,
          // 流水里的价格是「可成交价」= 信号确认那根的收盘价，不是图上箭头坐标
          // （箭头为了不压 K 线取买点最低价/卖点最高价，直接拿来算收益会系统性高估）
          price: typeof closes[item.i] === 'number' ? closes[item.i] : null,
          score: typeof item.score === 'number' ? item.score : null,
          detail: Array.isArray(item.reasons) && item.reasons.length ? item.reasons.join('、') : '',
          claim: `${bsCtx}|bs|${kind}|${time}`,
        })
      }
      pushIfNew(effBuyTime, buyItem, 'buy')
      pushIfNew(effSellTime, sellItem, 'sell')
    }

    if (state.families.includes('tema')) {
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

    if (state.families.includes('hull')) {
      // Hull 转折与 TEMA 同为单指标转折族：HMA 斜率换向即确认点（函数内已做 minGap
      // 聚类与日K MA20 顺势门控），只产出 kind==='conf'，无需再过滤
      const { buys, sells } = hullTurnPointsValues(highs, lows, closes, { intraday })
      const buyItem = latestItem(times, buys)
      const sellItem = latestItem(times, sells)
      const pushIfNew = (item, kind) => {
        const time = item ? times[item.i] : null
        const baseField = kind === 'buy' ? 'hullBuy' : 'hullSell'
        const prev = st[baseField]
        st[baseField] = time
        if (fresh || time == null || prev == null || time <= prev) return
        out.push({
          id: `${entry.code}|${klt}|hull|${kind}|${time}`,
          code: entry.code,
          name: entry.name || entry.code,
          klt,
          family: 'hull',
          kind,
          time,
          // Hull 信号同样只产出第几根 K 线，取该根收盘价，与 TEMA 行信息量一致
          price: typeof closes[item.i] === 'number' ? closes[item.i] : null,
          score: null,
          detail: 'HMA 斜率换向确认',
          claim: `${hullCtx}|hull|${kind}|${time}`,
        })
      }
      pushIfNew(buyItem, 'buy')
      pushIfNew(sellItem, 'sell')
    }

    return out
  }

  // ===== 取数与调度 =====

  /** 池中是否存在永续合约（bn:=币安 / bt:=Bitget / ok:=OKX）；股票实例的时段门控据此放行 */
  function hasContractInPool() {
    return state.pool.some((e) => isCryptoCode(e && e.code))
  }

  async function anyMarketOpen() {
    if (!opts.gateMarketOpen) return true // OKX 加密监控 7x24，不做时段门控
    // 池中含永续合约时直接放行：加密合约 7x24 不停市；美股/传统金融永续休市时
    // K 线不再更新，本就不会产生新信号，无需再加时段门控（避免漏掉 24h 交易的品种）。
    if (hasContractInPool()) return true
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
   * 单只失败只记错误，不影响其余股票。整体套 TICK_HARD_TIMEOUT_MS 硬超时看门狗，
   * 防止个别取数 Promise 永不结算导致 running 锁卡死（此后每轮 tick 都被跳过）。
   */
  async function runSignalTick(reason = 'manual') {
    if (!state.enabled) return
    if (!state.pool.length) return
    // 本实例扫描周期节流：Go tick 按三套监控的最小周期唤醒，各实例按自己的周期跳过多余 tick
    //（'manual' 立即扫描不受限；500ms 余量防定时器抖动导致周期漂移）
    if (reason !== 'manual' && Date.now() - state.lastTickAt < state.scanIntervalSec * 1000 - 500) return
    // 权限兜底：赞助到期后即使开关还亮着也不再扫描
    if (!(await canUseSignalMonitor())) return
    if (running) return
    running = true
    state.running = true
    const body = async () => {
      if (!(await anyMarketOpen())) return
      // 一只票 × 一个周期 = 一个任务；轮转游标让超出单轮上限的部分在下一轮优先补上
      const tasks = []
      for (const entry of state.pool.slice(0, SIGNAL_POOL_LIMIT)) {
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
      state.lastTickCount = hit.length
      state.lastError = errors.length ? errors.slice(0, 3).join('; ') : ''
      if (hit.length) dispatchSignals(hit)
    }
    try {
      await withTimeout(body(), TICK_HARD_TIMEOUT_MS, '本轮扫描')
    } catch (e) {
      state.lastError = e && e.message ? e.message : String(e)
    } finally {
      running = false
      state.running = false
      state.lastTickAt = Date.now()
    }
  }

  // ===== 趋势跟随（仅 OKX 双池：定时全市场扫描，命中入池、失趋势移除，有持仓先平仓再移除） =====

  /** UI 修改趋势跟随配置的唯一入口：走 persistSettings 落盘，exitScore 强制 < minScore 保住滞回 */
  function setAutoTrend(patch) {
    const t = state.autoTrend
    const next = { ...t, ...patch }
    if (Number.isFinite(next.minScore)) next.minScore = Math.min(5, Math.max(1, Number(next.minScore)))
    if (Number.isFinite(next.exitScore)) next.exitScore = Math.min(5, Math.max(1, Number(next.exitScore)))
    if (next.exitScore >= next.minScore) next.exitScore = next.minScore - 1
    Object.assign(t, next)
    persistSettings()
    // 自愈：同 setMonitorEnabled——打开趋势跟随时确保引擎节拍链已在跑（幂等）
    if (patch && patch.enabled === true) startSignalMonitors()
  }

  /** 本实例对应交易环境是否模拟盘（GetOKXPositionListEnv 的第一参数口径） */
  const trendSimulated = opts.tradeEnv !== 'live'

  /**
   * 趋势跟随出池处置：绝不主动平仓——仍有持仓的品种暂留池中（买卖点自动交易扫的是同一个池，
   * 卖点信号平仓后，下轮趋势扫描再查一次自然移除）；无持仓才真正移除。
   * 持仓查询按合约过滤；查询失败时抛错，调用方按「保留不动」处理，宁可多留一轮。
   */
  async function trendExitRemove(instId, code) {
    let positions = []
    try {
      positions = (await GetOKXPositionListEnv(trendSimulated, instId)) || []
    } catch (e) {
      throw new Error(`持仓查询失败: ${e && e.message ? e.message : e}`)
    }
    if (positions.some((p) => String(p?.instId || '').toUpperCase() === instId)) {
      return false // 有持仓：暂留池中，等自动交易平仓后下轮再移除
    }
    removePoolEntry(code)
    return true
  }

  /**
   * 趋势跟随一轮：与信号扫描同一 Go tick 驱动，但独立节流（intervalMin 分钟）与独立 running 锁，
   * 不占信号扫描的 tick 预算、也不受「池空即返回」限制（池空恰是自动入池时机）。
   * 整体套 TREND_ROUND_TIMEOUT_MS 硬超时看门狗：出池循环顺序 await，弱网下单轮可能很长，
   * 超时释放 running 锁让下一轮照常调度（在途循环自行收尾，已出的入池/出池结果不丢失）。
   */
  async function runTrendFollowTick() {
    if (!opts.trendFollow) return
    const t = state.autoTrend
    if (!t.enabled || t.running) return
    if (Date.now() - t.lastRunAt < t.intervalMin * 60000 - 500) return
    // 设置页「自动调池」全局确认兜底：总闸未开则面板开关不生效（与信号自动下单同一套双重约束）。
    // GetConfig 是本地读库（微秒级），每轮查一次换「实盘平仓必有显式确认」值得
    try {
      const cfg = await GetConfig()
      const confirmed = opts.tradeEnv === 'live' ? cfg?.okxAutoPoolLive === true : cfg?.okxAutoPoolEnable === true
      if (!confirmed) return
    } catch { /* 读不到配置宁可不动池 */ return }
    // 权限兜底：与信号扫描同一套 VIP 门槛（本地解密微秒级）
    if (!(await canUseSignalMonitor())) return
    t.running = true
    const body = async () => {
      const { rows } = await runOkxTrendScan({ bar: t.bar })
      const scoreBy = new Map(rows.map((r) => [r.instId, r]))
      // 1) 自动入池：rows 已按 score 降序，低于门槛即止；池满让 addPoolEntry 的上限校验兜底
      let added = 0
      for (const r of rows) {
        if (r.score < t.minScore) break
        if (state.pool.length >= SIGNAL_POOL_LIMIT) break
        if (state.pool.some((e) => e.code === `ok:${r.instId}`)) continue
        if (addPoolEntry(`ok:${r.instId}`, r.instId).ok) added++
      }
      // 2) 自动出池：只管 ok: 前缀（bn:/bt: 是用户手动加的跨所品种，趋势跟随不越权处置）；
      //    不在全市场扫描结果里的（非 USDT 结算等）单独拉 K 线补判，判不出则保守跳过；
      //    有持仓的暂留池中（等买卖点自动交易平仓），无持仓才移除
      let removed = 0
      let held = 0
      const errors = []
      for (const e of state.pool.slice()) {
        const m = /^ok:([A-Z0-9-]+)$/i.exec(String(e.code || ''))
        if (!m) continue
        const instId = m[1].toUpperCase()
        const hit = scoreBy.get(instId)
        let score = hit ? hit.score : null
        if (!hit) {
          const one = await scanOneTrend(instId, t.bar)
          if (one == null) continue // 拉不到 K 线/根数不足：不据此移除
          score = one.score
        }
        if (score >= t.exitScore) continue
        try {
          if (await trendExitRemove(instId, e.code)) removed++
          else held++
        } catch (err) {
          errors.push(`${instId} ${err && err.message ? err.message : err}`)
        }
      }
      t.lastRunAt = Date.now()
      const parts = [`入池 ${added}`, `移除 ${removed}`]
      if (held) parts.push(`持仓暂留 ${held}（待自动交易平仓）`)
      if (errors.length) parts.push(`异常 ${errors.length}（已保留池内）`)
      t.lastMsg = parts.join(' · ') + (errors.length ? `：${errors.slice(0, 2).join('; ')}` : '')
    }
    try {
      await withTimeout(body(), TREND_ROUND_TIMEOUT_MS, '趋势跟随本轮')
    } catch (e) {
      t.lastRunAt = Date.now()
      t.lastMsg = `扫描失败: ${e && e.message ? e.message : e}`
    } finally {
      t.running = false
    }
  }

  // ===== 提醒派发 =====

  function buildMessage(signals) {
    const labelOf = (s) => {
      if (s.family === 'tema') return s.kind === 'buy' ? 'T确买点' : 'T确卖点'
      if (s.family === 'hull') return s.kind === 'buy' ? 'H确买点' : 'H确卖点'
      return s.kind === 'buy' ? '买点' : '卖点'
    }
    const kltLabel = (SIGNAL_INTERVAL_OPTIONS.find((it) => it.klt === state.klt) || {}).label || '日K'
    if (signals.length === 1) {
      const s = signals[0]
      const scoreText = s.score != null ? `\n- **共振强度**: ${Math.round((s.score / BUY_SELL_MAX_SCORE) * 100)}%（${s.score}/${BUY_SELL_MAX_SCORE} 路）` : ''
      const detailText = s.detail ? `\n- **命中信号**: ${s.detail}` : ''
      const priceText = s.price != null ? `\n- **信号价格**: ${s.price.toFixed(2)}` : ''
      const title = `【${labelOf(s)}】${s.name}`
      const content = `## ${labelOf(s)} · ${s.name}\n\n- **代码**: ${s.code}\n- **周期**: ${kltLabel}\n- **信号时间**: ${formatSignalTime(s.time)}${priceText}${scoreText}${detailText}\n\n⚠️ 共振信号只是概率优势，请结合仓位与止损执行`
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
    SaveSignalRecords(hit.map((s) => toSignalRecord(s, now, opts.tradeEnv || '')))
      .then(() => {
        if (hasBuySell) return loadSignalStats()
        return null
      })
      .catch((e) => {
        state.lastError = `信号落库失败: ${e && e.message ? e.message : e}`
      })

    state.newCount += hit.length
    // 仅在「第 1 页且无筛选」时把新信号并进列表；否则只累加新信号数，不打断正在浏览的结果。
    // 方向筛选生效时只并入同向信号，异向信号只计入未读，不混进当前视角
    const curFilter = state.filter
    const noFilter = !(curFilter.keyword || '').trim() && !curFilter.start && !curFilter.end
    const kindPass = (s) => !curFilter.kind || s.kind === curFilter.kind
    if (state.page === 1 && noFilter) {
      const merged = hit.filter(kindPass)
      for (const s of merged) state.signals.unshift({ ...s, at: now })
      if (state.signals.length > state.pageSize) {
        state.signals.splice(state.pageSize)
      }
      state.total += merged.length
      state.totalPages = Math.max(1, Math.ceil(state.total / state.pageSize))
    }

    // 提示音：先按「同一只票同一根 K 线买卖同现」合并，再与图表抢占同一提示音键
    const tones = []
    const bsHits = new Set()
    /** 代码 → 播报称呼：同票买卖同现时要念同一个名字 */
    const names = new Map()
    for (const s of hit) {
      names.set(s.code, speakName(s))
      if (!state.sound || !claimAlertToneOnce(s.claim)) continue
      if (s.family === 'tema' || s.family === 'hull') {
        // 不念「T确」：语音会把拉丁字母逐字读出来，不如「转折买点」清楚；
        // Hull 音译「赫尔」可念，前缀区分同票同时触发的 TEMA / Hull 转折
        tones.push({
          tone: s.kind === 'buy' ? 'tema-buy' : 'tema-sell',
          say: `${names.get(s.code)} ${s.family === 'hull' ? '赫尔' : ''}转折${s.kind === 'buy' ? '买点' : '卖点'}`,
        })
      } else {
        bsHits.add(`${s.code}|${s.kind}`)
      }
    }
    // 买卖点：同票买卖同现播三音，其余按方向播双音
    const bsCodes = new Set([...bsHits].map((k) => k.split('|')[0]))
    for (const code of bsCodes) {
      const hasBuy = bsHits.has(`${code}|buy`)
      const hasSell = bsHits.has(`${code}|sell`)
      const both = hasBuy && hasSell
      tones.push({
        tone: both ? 'both' : hasBuy ? 'buy' : 'sell',
        say: `${names.get(code)} ${both ? '买卖点' : hasBuy ? '买点' : '卖点'}`,
      })
    }
    if (tones.length) {
      primeAlertAudio()
      for (const t of tones) queueTone(state.sound, state.voice, t)
    }

    // 多通道推送：同一轮 tick 的命中合并成一条，避免刷屏
    if (state.channels.length) {
      const { title, content, plain } = buildMessage(hit)
      try {
        await NotifySignal(title, content, plain, state.channels)
      } catch (e) {
        state.lastError = `推送失败: ${e && e.message ? e.message : e}`
      }
    }
  }

  loadSettings()

  return {
    id: opts.id,
    source: opts.source,
    state,
    addPoolEntry,
    removePoolEntry,
    clearPool,
    entryKlts,
    setEntryKlts,
    setMonitorEnabled,
    runSignalTick,
    runTrendFollowTick,
    setAutoTrend,
    querySignals,
    loadMoreSignals,
    setSignalFilter,
    clearSignals,
    loadSignalStats,
    setStatsPreset,
    setStatsCustomRange,
    cycleSensitivity,
    persist: persistSettings,
    boot,
    /** localStorage 是否持久化过扫描周期（供启动时从旧版全局设置迁移） */
    isScanIntervalSet: () => scanIntervalLoaded,
    signalStatsDayLabel,
  }
}

// ===== 三实例 =====

/** 股票信号监控：A股/港股/美股/场内基金，按交易时段门控，流水视角=仅股票 */
export const stockMonitor = createSignalMonitor({
  id: 'stock',
  persistKey: STOCK_PERSIST_KEY,
  source: 'stock',
  acceptCode: (c) => !isCryptoCode(c),
  gateMarketOpen: true,
  legacySignalMigration: true,
})

/** OKX 模拟盘监控池：池内合约的买卖点信号仅在模拟盘环境自动下单（不动真实资金），7x24 不门控。
 *  默认周期 15m：信号密度约为 5m 的 1/3，费用拖累同比降 2/3（回测 15m+MA144 全池 ≈打平 vs 5m 确定亏损）。
 *  开启趋势过滤：MA144 逆势信号不上报（三轮回测全场景最稳健组件） */
export const okxMonitor = createSignalMonitor({
  id: 'okx',
  persistKey: OKX_PERSIST_KEY,
  source: 'crypto',
  acceptCode: isCryptoCode,
  gateMarketOpen: false,
  tradeEnv: 'sim',
  trendFollow: true,
  defaultKlt: '15',
  trendFilter: true,
})

/** OKX 实盘监控池：池内合约的买卖点信号仅在实盘环境自动下单（真实资金，需另开实盘开关），与模拟盘池完全独立。
 *  实盘宁可漏不可误，同样 15m 周期 + 趋势过滤挡掉逆势接飞刀信号 */
export const okxLiveMonitor = createSignalMonitor({
  id: 'okxLive',
  persistKey: OKX_LIVE_PERSIST_KEY,
  source: 'crypto',
  acceptCode: isCryptoCode,
  gateMarketOpen: false,
  tradeEnv: 'live',
  trendFollow: true,
  defaultKlt: '15',
  trendFilter: true,
})

/** 供面板在任意设置变更后调用（各实例各自的设置都落盘） */
export function persistMonitorSettings() {
  stockMonitor.persist()
  okxMonitor.persist()
  okxLiveMonitor.persist()
}

// ===== 生命周期 =====

let started = false
let lastGoTickAt = 0

/** 定制版本标记（后端 CustomBuild 编译开关）：仅启动/驱动股票监控，OKX 双池引擎完全惰性 */
let customBuild = false

/** 当前参与监控的实例集合（定制版只保留股票监控） */
function activeMonitors() {
  return customBuild ? [stockMonitor] : [stockMonitor, okxMonitor, okxLiveMonitor]
}

/** 三套（或定制版仅股票）实例的最小扫描周期，作为 Go tick 的注册节拍 */
function minActiveScanInterval() {
  const monitors = activeMonitors()
  let min = monitors[0].state.scanIntervalSec
  for (const m of monitors) min = Math.min(min, m.state.scanIntervalSec)
  return min
}

/**
 * 注册 Go 节拍监听与兜底定时器（幂等，重复调用无副作用）。各实例共享同一节拍，各自按开关与池决定是否扫描。
 * 启动时 VIP 校验失败会不注册监听并把 started 复位——用户重新打开监控开关/趋势跟随时由此再次拉起（自愈）。
 */
export async function startSignalMonitors() {
  if (started) return
  started = true
  // 定制版本探测需在 boot 前完成（决定 OKX 双池是否参与启动与节拍）
  try {
    const info = await GetVersionInfo()
    customBuild = info?.customBuild === true
  } catch { /* 查询失败按普通版本处理 */ }
  // 权限不足时连监听都不注册：后台不做任何扫描，也不占取数配额
  if (!(await canUseSignalMonitor())) {
    started = false
    for (const m of activeMonitors()) {
      if (m.state.enabled) {
        // 赞助到期/未激活：把持久化里的开关一并关掉，避免面板显示「监控中」
        m.state.enabled = false
        m.persist()
      }
    }
    return
  }
  for (const m of activeMonitors()) m.boot()
  // 三套实例各自的扫描周期：未持久化过的实例继承旧版全局设置（signal_scan_interval_sec），
  // 然后 Go tick 按三套的最小周期重注册（含落库，下次启动直接生效）
  try {
    const legacySec = await GetSignalScanInterval()
    for (const m of activeMonitors()) {
      if (!m.isScanIntervalSet()) m.state.scanIntervalSec = legacySec
    }
  } catch { /* 浏览器开发模式无 Go 运行时，忽略 */ }
  // 先注册间隔变更监听再触发重注册：SetSignalScanInterval 落库后 Go 会广播 signalScanIntervalChanged，
  // 若注册晚于调用会错过该事件，兜底定时器节奏与真实间隔不同步
  EventsOn('signalScanIntervalChanged', (sec: number) => applyScanInterval(sec))
  applyScanInterval(minActiveScanInterval())
  await applySignalScanTick()
  EventsOn(TICK_EVENT, () => {
    lastGoTickAt = Date.now()
    for (const m of activeMonitors()) m.runSignalTick('go')
    // 趋势跟随：仅 OKX 双池有该能力，独立节流与 running 锁（不占信号扫描的 tick 预算）
    if (!customBuild) {
      okxMonitor.runTrendFollowTick()
      okxLiveMonitor.runTrendFollowTick()
    }
  })
}

/** 任一实例扫描周期变化后调用：按三套最小周期重注册 Go tick（后端落库+热更 cron+发事件回排兜底定时器） */
export async function applySignalScanTick() {
  const min = minActiveScanInterval()
  try {
    await SetSignalScanInterval(min)
  } catch { /* 浏览器开发模式无 Go 运行时，仅前端兜底定时器在跑 */ }
}

/** 应用新的扫描节拍间隔：重排兜底定时器（Go cron 的热更由后端完成，这里只管前端侧节奏与失联阈值） */
let fallbackTimer: ReturnType<typeof setInterval> | null = null
function applyScanInterval(sec: number) {
  scanIntervalSec = sec > 0 ? sec : 60
  if (fallbackTimer) clearInterval(fallbackTimer)
  fallbackTimer = setInterval(() => {
    // Go 节拍正常时前端不做任何事，避免同一轮扫两遍
    if (Date.now() - lastGoTickAt < goTickStaleMs()) return
    // 定制版本仅驱动股票监控（OKX 双池引擎未启动）
    if (customBuild) {
      stockMonitor.runSignalTick('timer')
      return
    }
    stockMonitor.runSignalTick('timer')
    okxMonitor.runSignalTick('timer')
    okxLiveMonitor.runSignalTick('timer')
    okxMonitor.runTrendFollowTick()
    okxLiveMonitor.runTrendFollowTick()
  }, scanIntervalSec * 1000)
}

// ===== 快照式买卖点检测（供「股票推荐记录」等一次性展示复用同一套算法） =====

/**
 * 「最近一年」折算到各周期的 K 线根数（A股一年约 244 个交易日；分钟口径一天约 240 分钟）。
 * 推荐记录页一次性扫描用：窗口收窄到一年可大幅降低单只票取数量，从而支撑不限量扫描。
 */
const YEAR_BARS_BY_KLT = {
  '1': 244 * 240,
  '5': 244 * 48,
  '15': 244 * 16,
  '30': 244 * 8,
  '60': 244 * 4,
  '101': 244,
  '102': 49,
  '103': 12,
  '104': 4,
  '106': 1,
}

/**
 * 扫描取数根数：按「最近一年」折算，但不低于信号门控所需最少根数（低于则 MA250 门控静默失效），
 * 也不超过图表口径的 limit（数据源本身量级）。日K 一年约 244 根 < MIN_BARS(300)，故回落到 300。
 */
function scanLimitForKlt(klt) {
  const meta = INTERVALS.find((it) => it.klt === klt) || INTERVALS.find((it) => it.klt === '101')
  const yearBars = YEAR_BARS_BY_KLT[meta.klt] || meta.limit
  return Math.min(meta.limit, Math.max(MIN_BARS, yearBars))
}

/**
 * 一次性检测某只票在指定周期上「最近 recentDays 个交易日内」的买卖点信号。
 * 与后台监控共用买卖点算法（buySellPointsValues）与复权口径，取数窗口改为「最近一年」（见 scanLimitForKlt）。
 * 区别仅在于不依赖基线：监控只报「新增」信号，本函数返回窗口内命中的全部信号，供面板快照展示。
 * 窗口以「K 线最新一根所在的交易日」为基准回溯，周末/节假日/停牌都不会错位；
 * 分时周期下同一交易日的所有 K 线同属一个交易日，故「今日」= 当天全部分时信号。
 * 返回 { signals, bars, insufficient }：signals 为该票窗口内命中的全部买卖点（同方向可多条，不再只取最新一条）；
 * bars 为可用 K 线根数；insufficient 表示根数不足 MIN_BARS（门控会静默失效，故不出信号）。
 */
export async function detectRecentBuySell(code, name, klt, minScore, recentDays = 1) {
  const meta = INTERVALS.find((it) => it.klt === klt) || INTERVALS.find((it) => it.klt === '101')
  const adjust = DAILY_LIKE_KLT.has(meta.klt) ? adjustForCode(code) : ''
  const res = await GetStockKLineWithFallback(code, name || '', meta.klt, scanLimitForKlt(meta.klt), adjust)
  const rows = Array.isArray(res?.data) ? res.data : []
  if (rows.length < MIN_BARS) return { signals: [], bars: rows.length, insufficient: true }
  const { times, highs, lows, closes, vols, days } = extractOHLCV(rows)
  if (closes.length < MIN_BARS) return { signals: [], bars: closes.length, insufficient: true }

  const intraday = !DAILY_LIKE_KLT.has(meta.klt)
  const opt = BUY_SELL_SCORE_OPTIONS.find((o) => o.value === minScore) || BUY_SELL_SCORE_OPTIONS[1]
  const { buys, sells } = buySellPointsValues(highs, lows, closes, vols, {
    minScore: opt.minScore,
    minGroups: opt.minGroups,
    dayKeys: days,
    intraday,
  })

  const last = closes.length - 1
  // 最近 recentDays 个交易日（对 K 线出现过的自然日去重后取末尾），作为「当前有信号」的判定窗口
  const distinctDays = Array.from(new Set(days.filter(Boolean)))
  const allowDays = new Set(distinctDays.slice(-Math.max(1, Number(recentDays) || 1)))
  const pickAll = (arr, kind) => {
    const out = []
    for (const it of arr) {
      // 只保留最近窗口内的信号：更早的信号已在图上，不属于「当前有买卖点」
      if (it.i < 0 || it.i > last) continue
      if (!allowDays.has(days[it.i])) continue
      out.push({
        code,
        name: name || code,
        klt: meta.klt,
        kind,
        time: times[it.i],
        price: typeof closes[it.i] === 'number' ? closes[it.i] : null,
        score: typeof it.score === 'number' ? it.score : null,
        reasons: Array.isArray(it.reasons) ? it.reasons : [],
        lag: last - it.i,
      })
    }
    return out
  }
  const signals = [...pickAll(buys, 'buy'), ...pickAll(sells, 'sell')]
  return { signals, bars: closes.length, insufficient: false }
}
