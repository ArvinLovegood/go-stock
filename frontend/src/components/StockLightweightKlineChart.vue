<script setup>
import { GetStockEastMoneyKLine, GetStockEastMoneyKLinePage, GetStockKLineWithFallback, GetStockKLinePageWithFallback } from '../../wailsjs/go/main/App'
import {
  CandlestickSeries,
  createChart,
  HistogramSeries,
  LineSeries,
  LineStyle,
} from 'lightweight-charts'
import { NButton, NFlex, NInput, NSpin, NText, NTooltip } from 'naive-ui'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  smaValues, emaFinite, emaLeadingNull, weightedMaValues, bollingerBands, obvValues,
  macdBundle, kdjBundle, rsiBundle, atrValues, vwapValues, mfiValues, kamaValues,
  keltnerChannelValues, supertrendValues, ichimokuValues, cciValues, ttmSqueezeValues,
  sarValues, donchianChannelValues, adxValues, williamsRValues, stochRsiValues,
  cmfValues, aroonValues, cmoValues, forceIndexValues, pivotPointsValues, demaValues,
  zigzagValues, satsValues, alligatorValues, aoValues, hullMaValues, adValues,
  trixValues, rocValues, fractalValues, chopValues, elderRayValues, chaikinOscValues,
  vwapBandsValues, massIndexValues, ulcerIndexValues, coppockValues, temaValues, smiValues, smcValues,
  trixSlopeValues,
} from './kline/calc'
import { makeToggle } from './kline/indicators/toggle'
import { parseNumStr, formatPrice2, formatVolumeCn, formatAmountCn, formatPctField, formatSigned2 } from './kline/format'
import {
  eastMoneyDayToUnixSeconds, eastMoneyKlineFieldToUnixSeconds, chartTimeToUtcMs,
  formatTickTime, sortKey, toChartTime, mergeKlineRows, mergeRefreshWithLatest,
  extractYmdDatePart, barSecondsForMinuteKlt,
} from './kline/time'

import {
  CLR_RISE, CLR_FALL, DAILY_LIKE_KLT, CN_TZ,
  HISTORY_PAGE_SIZE, BARS_BEFORE_LOAD_MORE, DEFAULT_VISIBLE_BARS,
  DEFAULT_RIGHT_LOGICAL_GAP, SHOW_CHIP_TOOLBAR_BUTTON, INTERVALS,
} from './kline/constants'

const props = defineProps({
  code: { type: String, default: '' },
  stockName: { type: String, default: '' },
  darkTheme: { type: Boolean, default: false },
  chartHeight: { type: Number, default: 400 },
  /** 定时拉取当前周期最新 K 线，毫秒；0 关闭；默认 60 秒 */
  realtimeIntervalMs: { type: Number, default: 1000*60 },
  /** 多单开仓价；传入则与内部输入同步，未传入（undefined）时不向父组件 emit */
  longEntryPrice: { type: [String, Number], default: undefined },
  /** 多单止损价 */
  longStopLossPrice: { type: [String, Number], default: undefined },
  /** 多单止盈价 */
  longTakeProfitPrice: { type: [String, Number], default: undefined },
  /** 成本价 */
  costPrice: { type: [String, Number], default: undefined },
})

const emit = defineEmits([
  'update:longEntryPrice',
  'update:longStopLossPrice',
  'update:longTakeProfitPrice',
  'update:costPrice',
])
const chartContainerRef = ref(null)
/** 十字线当前 K 对应的原始行（东财字段） */
const hoverRawRow = ref(null)
/** 无十字线时展示：当前数据中时间最新的一根 K 线 */
const defaultLatestRawRow = ref(null)
const activeKlt = ref('101')
const showMA = ref(false)
const showBOLL = ref(false)
const showOBV = ref(false)
const showMACD = ref(false)
const showKDJ = ref(false)
const showRSI = ref(false)
const showATR = ref(false)
const showVWAP = ref(false)
const showMFI = ref(false)
const showKAMA = ref(false)
const showKeltner = ref(false)
const showSupertrend = ref(false)
const showEMA = ref(false)
const showIchimoku = ref(false)
const showCCI = ref(false)
const showTTMSqueeze = ref(false)
const showSAR = ref(false)
const showDonchian = ref(false)
const showADX = ref(false)
const showWilliamsR = ref(false)
const showStochRSI = ref(false)
const showCMF = ref(false)
const showAroon = ref(false)
const showCMO = ref(false)
const showForceIndex = ref(false)
const showPivot = ref(false)
const showDEMA = ref(false)
const showZigZag = ref(false)
const showSATS = ref(false)
const showAvgAmp = ref(false)
const showAlligator = ref(false)
const showAO = ref(false)
const showHullMA = ref(false)
const showAD = ref(false)
const showTRIX = ref(false)
const showTRIXSlope = ref(false)
const showROC = ref(false)
const showFractal = ref(false)
const showCHOP = ref(false)
const showElderRay = ref(false)
const showChaikinOsc = ref(false)
const showVWAPBands = ref(false)
const showMassIndex = ref(false)
const showUlcerIndex = ref(false)
const showCoppock = ref(false)
const showTEMA = ref(false)
const showSMI = ref(false)
const showSignalRatio = ref(false)
const showSMC = ref(false)
const showChip = ref(false)
const chipBins = ref(80)
const chipCanvasRef = ref(null)
const chipItems = ref([])
const chipMeta = ref({ avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 })
/** TradingView 风格「多单」：开仓 / 止损 / 止盈 价位线 */
const showLongPosition = ref(false)
const longEntryStr = ref('')
const longStopStr = ref('')
const longTakeProfitStr = ref('')
const longCostStr = ref('')
/** 在 K 线主区点击，按顺序写入开仓 → 止损 → 止盈（再点回到开仓） */
const longClickPickEnabled = ref(false)
const longClickNextField = ref('entry')
/** 点过开仓/止损/止盈输入框后，下一次主图点击写入对应价位（blur 延迟清除以兼容「先失焦后 click」） */
const longFocusedPriceField = ref(null)
/** 由 props 写入价位时抑制 emit，避免与 v-model 循环 */
const suppressLongPriceEmit = ref(false)
const loading = ref(false)
const loadingHistory = ref(false)
const errorText = ref('')
const activeDataSource = ref('')

let chart = null
let candleSeries = null
let volSeries = null
let pollTimer = null
/** 已合并的后端原始 K 线（按时间升序） */
let mergedRawRows = []
/** 每次 mergedRawRows 变更后递增，供 computed 感知变化 */
const mergedRawRowsVersion = ref(0)
const hasMoreOlder = ref(true)
let loadOlderDebounceTimer = null
/** Wails/WebView 下可见区回调偶发不触发，用轻量轮询兜底 */
let historyVisiblePollTimer = null
let logicalRangeHandler = null
let visibleTimeRangeHandler = null
let crosshairMoveHandler = null
let chartClickHandler = null
/** 上一次请求更早 K 线使用的 end，用于识别「重叠返回」避免误判无更多数据 */
let lastOlderHistoryEndTried = ''
/** >0 时表示由代码在改时间轴（fitContent / setData / setVisibleLogicalRange），不触发分页加载 */
let programmaticRangeDepth = 0
/** 多单标注：createPriceLine 返回的句柄，需在 dispose / 重绘前 remove */
let longPositionPriceLines = []
/** 各价位线句柄，用于命中与拖动时 applyOptions */
let longLineByKind = { entry: null, stop: null, takeProfit: null }
/** 正在拖动某条多单价位线时禁止 watch 里整表重建线条 */
let longPositionDragActive = false
let longDragKind = null
/** 命中线后抑制一次「图上点击设价」，避免拖/点冲突 */
let longSuppressChartClick = false
let longPaneDragEl = null
/** window 上拖动用监听器是否已挂（避免重复解绑 / 重复 up） */
let longDragWindowListenersOn = false
/** 拖动过程中最近一次指针 Y，用于松手后恢复「可拖」光标 */
let longLastPointerClientY = null
/** 垂直方向命中容差（px） */
const LONG_PRICE_LINE_HIT_PX = 12
/** 输入框 blur 后延迟清除「待图表取价」状态（ms），需大于 click 相对 blur 的间隔 */
const LONG_FOCUS_BLUR_CLEAR_MS = 600
let longFocusBlurTimer = null

/** 指标线系列（与主图生命周期同步） */
const ind = {
  ma5: null,
  ma10: null,
  ma20: null,
  ma60: null,
  bollU: null,
  bollM: null,
  bollL: null,
  obv: null,
  macdHist: null,
  macdDif: null,
  macdDea: null,
  kdjK: null,
  kdjD: null,
  kdjJ: null,
  rsi: null,
  atr: null,
  vwap: null,
  mfi: null,
  kama: null,
  keltnerU: null,
  keltnerM: null,
  keltnerL: null,
  supertrend: null,
  ema12: null,
  ema21: null,
  ichTenkan: null,
  ichKijun: null,
  ichSpanA: null,
  ichSpanB: null,
  ichChikou: null,
  cci: null,
  ttmHist: null,
  ttmDots: null,
  sar: null,
  donchianU: null,
  donchianM: null,
  donchianL: null,
  adx: null,
  adxDiP: null,
  adxDiM: null,
  williamsR: null,
  stochRsi: null,
  stochRsiD: null,
  cmf: null,
  aroonUp: null,
  aroonDown: null,
  cmo: null,
  forceIndex: null,
  pivotPP: null,
  pivotS1: null,
  pivotS2: null,
  pivotR1: null,
  pivotR2: null,
  dema: null,
  zigzag: null,
  satsLine: null,
  satsUpper: null,
  satsLower: null,
  avgAmp5: null,
  avgAmp10: null,
  avgAmp20: null,
  alligatorJaw: null,
  alligatorTeeth: null,
  alligatorLips: null,
  aoLine: null,
  aoHist: null,
  hullMA: null,
  adLine: null,
  trixLine: null,
  trixSignal: null,
  trixSlopeHist: null,
  rocLine: null,
  fractalHigh: null,
  fractalLow: null,
  chopLine: null,
  elderBull: null,
  elderBear: null,
  chaikinOscLine: null,
  vwapBandsU: null,
  vwapBandsM: null,
  vwapBandsL: null,
  massIndexLine: null,
  ulcerLine: null,
  coppockLine: null,
  temaLine: null,
  smiLine: null,
  smiSignal: null,
  smcSwingHigh: null,
  smcSwingLow: null,
  smcIntHigh: null,
  smcIntLow: null,
  smcBos: null,
  smcChoch: null,
  smcSwBos: null,
  smcSwChoch: null,
  smcFvgTop: null,
  smcFvgBot: null,
  smcObTop: null,
  smcObBot: null,
}

import { indicatorTips } from './kline/indicators/tips'

function removeSeriesSafe(api) {
  if (!api || !chart) return null
  try {
    chart.removeSeries(api)
  } catch {
    /* ignore */
  }
  return null
}



function extractOHLCV(rows) {
  const sorted = [...(rows || [])].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const times = []
  const opens = []
  const closes = []
  const highs = []
  const lows = []
  const vols = []
  const amplitudes = []
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    times.push(t)
    opens.push(o)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    vols.push(Number.isFinite(v) ? v : 0)
    const rawAmp = parseNumStr(r.amplitude)
    amplitudes.push(Number.isFinite(rawAmp) ? rawAmp : (o > 0 ? (h - l) / o * 100 : NaN))
  }
  return { times, opens, closes, highs, lows, vols, amplitudes }
}

function avgAmplitude(amplitudes, period) {
  if (!amplitudes || amplitudes.length < period) return NaN
  let s = 0, cnt = 0
  for (let i = amplitudes.length - period; i < amplitudes.length; i++) {
    const v = amplitudes[i]
    if (Number.isFinite(v)) { s += v; cnt++ }
  }
  return cnt === period ? s / cnt : NaN
}

function formatVolumeRatio(v) {
  if (v == null || v === '' || v === '--') return '--'
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(2) : '--'
}

function toLineData(times, values) {
  const arr = []
  for (let i = 0; i < times.length; i++) {
    const v = values[i]
    if (v != null && Number.isFinite(v)) arr.push({ time: times[i], value: v })
  }
  return arr
}

/** 单根 K 的近似「成本中枢」：优先日 VWAP（成交额/量），否则典型价，夹在 [L,H] */
import { chipBarCostCenter, addChipVolumeKernel, calcChipDistribution } from './kline/chip'

function drawChipCanvas() {
  const canvas = chipCanvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  const dpr = window.devicePixelRatio || 1
  const rect = canvas.getBoundingClientRect()
  const w = rect.width
  const h = rect.height
  canvas.width = w * dpr
  canvas.height = h * dpr
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, w, h)
  const isDark = props.darkTheme
  ctx.fillStyle = isDark ? '#141414' : '#ffffff'
  ctx.fillRect(0, 0, w, h)
  const items = chipItems.value
  if (!items.length) return
  const maxRatio = Math.max(...items.map((it) => it.ratio || 0), 1e-9)
  const barMaxW = w - 4
  const barH = Math.max(1, h / items.length)
  const cur = chipMeta.value.current || 0
  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    const y = i * barH
    const bw = Math.max(0, (it.ratio / maxRatio) * barMaxW)
    const isProfit = it.price <= cur
    if (isProfit) {
      ctx.fillStyle = isDark ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.6)'
    } else {
      ctx.fillStyle = isDark ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.6)'
    }
    ctx.fillRect(w - bw, y, bw, barH - 0.5)
  }
  if (cur > 0) {
    const minP = chipMeta.value.minPrice || 0
    const maxP = chipMeta.value.maxPrice || 0
    if (maxP > minP && cur >= minP && cur <= maxP) {
      const curY = ((cur - minP) / (maxP - minP)) * h
      ctx.strokeStyle = isDark ? '#fbbf24' : '#d97706'
      ctx.lineWidth = 1
      ctx.setLineDash([4, 3])
      ctx.beginPath()
      ctx.moveTo(0, curY)
      ctx.lineTo(w, curY)
      ctx.stroke()
      ctx.setLineDash([])
    }
  }
}

function tearDownAllSubPanes() {
  if (!chart) return
  ind.obv = removeSeriesSafe(ind.obv)
  ind.macdHist = removeSeriesSafe(ind.macdHist)
  ind.macdDif = removeSeriesSafe(ind.macdDif)
  ind.macdDea = removeSeriesSafe(ind.macdDea)
  ind.kdjK = removeSeriesSafe(ind.kdjK)
  ind.kdjD = removeSeriesSafe(ind.kdjD)
  ind.kdjJ = removeSeriesSafe(ind.kdjJ)
  ind.rsi = removeSeriesSafe(ind.rsi)
  ind.atr = removeSeriesSafe(ind.atr)
  ind.mfi = removeSeriesSafe(ind.mfi)
  ind.cci = removeSeriesSafe(ind.cci)
  ind.ttmHist = removeSeriesSafe(ind.ttmHist)
  ind.ttmDots = removeSeriesSafe(ind.ttmDots)
  ind.adx = removeSeriesSafe(ind.adx)
  ind.adxDiP = removeSeriesSafe(ind.adxDiP)
  ind.adxDiM = removeSeriesSafe(ind.adxDiM)
  ind.williamsR = removeSeriesSafe(ind.williamsR)
  ind.stochRsi = removeSeriesSafe(ind.stochRsi)
  ind.stochRsiD = removeSeriesSafe(ind.stochRsiD)
  ind.cmf = removeSeriesSafe(ind.cmf)
  ind.aroonUp = removeSeriesSafe(ind.aroonUp)
  ind.aroonDown = removeSeriesSafe(ind.aroonDown)
  ind.cmo = removeSeriesSafe(ind.cmo)
  ind.forceIndex = removeSeriesSafe(ind.forceIndex)
  ind.avgAmp5 = removeSeriesSafe(ind.avgAmp5)
  ind.avgAmp10 = removeSeriesSafe(ind.avgAmp10)
  ind.avgAmp20 = removeSeriesSafe(ind.avgAmp20)
  ind.aoHist = removeSeriesSafe(ind.aoHist)
  ind.aoLine = removeSeriesSafe(ind.aoLine)
  ind.adLine = removeSeriesSafe(ind.adLine)
  ind.trixLine = removeSeriesSafe(ind.trixLine)
  ind.trixSignal = removeSeriesSafe(ind.trixSignal)
  ind.trixSlopeHist = removeSeriesSafe(ind.trixSlopeHist)
  ind.rocLine = removeSeriesSafe(ind.rocLine)
  ind.chopLine = removeSeriesSafe(ind.chopLine)
  ind.elderBull = removeSeriesSafe(ind.elderBull)
  ind.elderBear = removeSeriesSafe(ind.elderBear)
  ind.chaikinOscLine = removeSeriesSafe(ind.chaikinOscLine)
  ind.massIndexLine = removeSeriesSafe(ind.massIndexLine)
  ind.ulcerLine = removeSeriesSafe(ind.ulcerLine)
  ind.coppockLine = removeSeriesSafe(ind.coppockLine)
  ind.smiLine = removeSeriesSafe(ind.smiLine)
  ind.smiSignal = removeSeriesSafe(ind.smiSignal)
  ind.signalRatioBullish = removeSeriesSafe(ind.signalRatioBullish)
  ind.signalRatioBearish = removeSeriesSafe(ind.signalRatioBearish)
  ind.signalRatioNet = removeSeriesSafe(ind.signalRatioNet)
  while (chart.panes().length > 1) {
    chart.removePane(chart.panes().length - 1)
  }
  chart.panes()[0]?.setStretchFactor(1)
}

const subLineOpts = {
  lineWidth: 1,
  lastValueVisible: false,
  priceLineVisible: false,
}

function syncSubPaneIndicators(times, closes, highs, lows, vols) {
  if (!chart) return
  tearDownAllSubPanes()

  const subs = []
  if (showOBV.value) subs.push('obv')
  if (showMACD.value) subs.push('macd')
  if (showKDJ.value) subs.push('kdj')
  if (showRSI.value) subs.push('rsi')
  if (showATR.value) subs.push('atr')
  if (showMFI.value) subs.push('mfi')
  if (showCCI.value) subs.push('cci')
  if (showTTMSqueeze.value) subs.push('ttmSqueeze')
  if (showADX.value) subs.push('adx')
  if (showWilliamsR.value) subs.push('williamsR')
  if (showStochRSI.value) subs.push('stochRsi')
  if (showCMF.value) subs.push('cmf')
  if (showAroon.value) subs.push('aroon')
  if (showCMO.value) subs.push('cmo')
  if (showForceIndex.value) subs.push('forceIndex')
  if (showAvgAmp.value) subs.push('avgAmp')
  if (showAO.value) subs.push('ao')
  if (showAD.value) subs.push('ad')
  if (showTRIX.value) subs.push('trix')
  if (showTRIXSlope.value) subs.push('trixSlope')
  if (showROC.value) subs.push('roc')
  if (showCHOP.value) subs.push('chop')
  if (showElderRay.value) subs.push('elderRay')
  if (showChaikinOsc.value) subs.push('chaikinOsc')
  if (showMassIndex.value) subs.push('massIndex')
  if (showUlcerIndex.value) subs.push('ulcerIndex')
  if (showCoppock.value) subs.push('coppock')
  if (showSMI.value) subs.push('smi')
  if (showSignalRatio.value) subs.push('signalRatio')
  if (subs.length === 0) return

  chart.panes()[0]?.setStretchFactor(3)

  let paneIdx = 1
  for (const key of subs) {
    if (key === 'obv') {
      const obv = obvValues(closes, vols)
      ind.obv = chart.addSeries(
        LineSeries,
        {
          color: '#22c55e',
          lineWidth: 1,
          title: 'OBV',
          lastValueVisible: true,
          priceLineVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.obv.setData(toLineData(times, obv))
    } else if (key === 'macd') {
      const { dif, dea, hist } = macdBundle(closes)
      ind.macdHist = chart.addSeries(
        HistogramSeries,
        {
          priceFormat: { type: 'price', precision: 4, minMove: 0.0001 },
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDif = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'DIF',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDea = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          title: 'DEA',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      const histData = []
      for (let i = 0; i < times.length; i++) {
        const hv = hist[i]
        if (hv == null || !Number.isFinite(hv)) continue
        histData.push({
          time: times[i],
          value: hv,
          color:
            hv >= 0
              ? 'rgba(239, 83, 80, 0.55)'
              : 'rgba(38, 166, 154, 0.55)',
        })
      }
      ind.macdHist.setData(histData)
      ind.macdDif.setData(toLineData(times, dif))
      ind.macdDea.setData(toLineData(times, dea))
    } else if (key === 'kdj') {
      const { K, D, J } = kdjBundle(highs, lows, closes, 9)
      ind.kdjK = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#f59e0b', title: 'K' },
        paneIdx,
      )
      ind.kdjD = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#3b82f6', title: 'D' },
        paneIdx,
      )
      ind.kdjJ = chart.addSeries(
        LineSeries,
        { ...subLineOpts, color: '#a855f7', title: 'J' },
        paneIdx,
      )
      ind.kdjK.setData(toLineData(times, K))
      ind.kdjD.setData(toLineData(times, D))
      ind.kdjJ.setData(toLineData(times, J))
    } else if (key === 'rsi') {
      const rsi = rsiBundle(closes, 14)
      ind.rsi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#d946ef',
          title: 'RSI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.rsi.setData(toLineData(times, rsi))
    } else if (key === 'atr') {
      const atr = atrValues(highs, lows, closes, 14)
      ind.atr = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'ATR14',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.atr.setData(toLineData(times, atr))
    } else if (key === 'mfi') {
      const mfi = mfiValues(highs, lows, closes, vols, 14)
      ind.mfi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'MFI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.mfi.setData(toLineData(times, mfi))
    } else if (key === 'cci') {
      const cci = cciValues(highs, lows, closes, 20)
      ind.cci = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#eab308',
          title: 'CCI20',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cci.setData(toLineData(times, cci))
    } else if (key === 'ttmSqueeze') {
      const { squeeze, momentum } = ttmSqueezeValues(highs, lows, closes)
      ind.ttmHist = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.ttmDots = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 2,
          title: 'SQZ',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const histData = []
      const dotData = []
      for (let i = 0; i < times.length; i++) {
        const mv = momentum[i]
        if (mv != null && Number.isFinite(mv)) {
          histData.push({
            time: times[i],
            value: mv,
            color: mv >= 0
              ? (squeeze[i] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.4)')
              : (squeeze[i] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.4)'),
          })
          dotData.push({
            time: times[i],
            value: 0,
            color: squeeze[i] ? '#eab308' : '#22c55e',
          })
        }
      }
      ind.ttmHist.setData(histData)
      ind.ttmDots.setData(dotData)
    } else if (key === 'adx') {
      const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
      ind.adxDiP = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          lineWidth: 1,
          title: '+DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiM = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          lineWidth: 1,
          title: '-DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adx = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'ADX',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiP.setData(toLineData(times, diP))
      ind.adxDiM.setData(toLineData(times, diM))
      ind.adx.setData(toLineData(times, adx))
    } else if (key === 'williamsR') {
      const wr = williamsRValues(highs, lows, closes, 14)
      ind.williamsR = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'W%R14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.williamsR.setData(toLineData(times, wr))
    } else if (key === 'stochRsi') {
      const { k, d } = stochRsiValues(closes, 14, 14, 3, 3)
      ind.stochRsi = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'StochRSI K',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsiD = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'StochRSI D',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsi.setData(toLineData(times, k))
      ind.stochRsiD.setData(toLineData(times, d))
    } else if (key === 'cmf') {
      const cmf = cmfValues(highs, lows, closes, vols, 20)
      ind.cmf = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#14b8a6',
          title: 'CMF20',
          priceFormat: { type: 'price', precision: 3, minMove: 0.001 },
        },
        paneIdx,
      )
      ind.cmf.setData(toLineData(times, cmf))
    } else if (key === 'aroon') {
      const { up, down } = aroonValues(highs, lows, 25)
      ind.aroonUp = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          title: 'Aroon Up',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonDown = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Aroon Down',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonUp.setData(toLineData(times, up))
      ind.aroonDown.setData(toLineData(times, down))
    } else if (key === 'cmo') {
      const cmo = cmoValues(closes, 14)
      ind.cmo = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'CMO14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cmo.setData(toLineData(times, cmo))
    } else if (key === 'forceIndex') {
      const fi = forceIndexValues(closes, vols, 13)
      ind.forceIndex = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'FI13',
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.forceIndex.setData(toLineData(times, fi))
    } else if (key === 'avgAmp') {
      const { amplitudes } = extractOHLCV(mergedRawRows)
      const aa5 = smaValues(amplitudes, 5)
      const aa10 = smaValues(amplitudes, 10)
      const aa20 = smaValues(amplitudes, 20)
      ind.avgAmp5 = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: '均幅5',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp10 = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          title: '均幅10',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp20 = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#a855f7',
          title: '均幅20',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp5.setData(toLineData(times, aa5))
      ind.avgAmp10.setData(toLineData(times, aa10))
      ind.avgAmp20.setData(toLineData(times, aa20))
    } else if (key === 'ao') {
      const ao = aoValues(highs, lows)
      ind.aoHist = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.aoLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          title: 'AO',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const aoHistData = []
      for (let i = 0; i < times.length; i++) {
        const v = ao[i]
        if (v != null && Number.isFinite(v)) {
          aoHistData.push({
            time: times[i],
            value: v,
            color: v >= 0
              ? (i > 0 && ao[i - 1] != null && v > ao[i - 1] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.35)')
              : (i > 0 && ao[i - 1] != null && v < ao[i - 1] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.35)'),
          })
        }
      }
      ind.aoHist.setData(aoHistData)
      ind.aoLine.setData(toLineData(times, ao))
    } else if (key === 'ad') {
      const ad = adValues(highs, lows, closes, vols)
      ind.adLine = chart.addSeries(
        LineSeries,
        {
          color: '#22c55e',
          lineWidth: 1,
          title: 'A/D',
          lastValueVisible: true,
          priceLineVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.adLine.setData(toLineData(times, ad))
    } else if (key === 'trix') {
      const trix = trixValues(closes, 15)
      const signal = emaLeadingNull(trix, 9)
      ind.trixLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'TRIX',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.trixSignal = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Signal',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.trixLine.setData(toLineData(times, trix))
      ind.trixSignal.setData(toLineData(times, signal))
    } else if (key === 'trixSlope') {
      const slope = trixSlopeValues(closes, 15)
      ind.trixSlopeHist = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const slopeData = []
      for (let i = 0; i < times.length; i++) {
        const sv = slope[i]
        if (sv != null && Number.isFinite(sv)) {
          slopeData.push({
            time: times[i],
            value: sv,
            color: sv >= 0
              ? (i > 0 && slope[i - 1] != null && sv > slope[i - 1] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.35)')
              : (i > 0 && slope[i - 1] != null && sv < slope[i - 1] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.35)'),
          })
        }
      }
      ind.trixSlopeHist.setData(slopeData)
    } else if (key === 'roc') {
      const roc = rocValues(closes, 12)
      ind.rocLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#d946ef',
          title: 'ROC12',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.rocLine.setData(toLineData(times, roc))
    } else if (key === 'chop') {
      const chop = chopValues(highs, lows, closes, 14)
      ind.chopLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'CHOP',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.chopLine.setData(toLineData(times, chop))
    } else if (key === 'elderRay') {
      const { bullPower, bearPower } = elderRayValues(highs, lows, closes, 13)
      ind.elderBull = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.elderBear = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const bullData = []
      const bearData = []
      for (let i = 0; i < times.length; i++) {
        const bv = bullPower[i]
        const brv = bearPower[i]
        if (bv != null && Number.isFinite(bv)) {
          bullData.push({
            time: times[i],
            value: bv,
            color: bv >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(239, 68, 68, 0.35)',
          })
        }
        if (brv != null && Number.isFinite(brv)) {
          bearData.push({
            time: times[i],
            value: brv,
            color: brv >= 0 ? 'rgba(34, 197, 94, 0.7)' : 'rgba(34, 197, 94, 0.35)',
          })
        }
      }
      ind.elderBull.setData(bullData)
      ind.elderBear.setData(bearData)
    } else if (key === 'chaikinOsc') {
      const co = chaikinOscValues(highs, lows, closes, vols, 3, 10)
      ind.chaikinOscLine = chart.addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      const coData = []
      for (let i = 0; i < times.length; i++) {
        const v = co[i]
        if (v != null && Number.isFinite(v)) {
          coData.push({
            time: times[i],
            value: v,
            color: v >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(34, 197, 94, 0.7)',
          })
        }
      }
      ind.chaikinOscLine.setData(coData)
    } else if (key === 'massIndex') {
      const mi = massIndexValues(highs, lows)
      ind.massIndexLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'Mass',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.massIndexLine.setData(toLineData(times, mi))
    } else if (key === 'ulcerIndex') {
      const ui = ulcerIndexValues(closes)
      ind.ulcerLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Ulcer',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.ulcerLine.setData(toLineData(times, ui))
    } else if (key === 'coppock') {
      const cp = coppockValues(closes)
      ind.coppockLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'Coppock',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.coppockLine.setData(toLineData(times, cp))
    } else if (key === 'smi') {
      const { smi: smiData, signal: smiSig } = smiValues(highs, lows, closes)
      ind.smiLine = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'SMI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.smiSignal = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Signal',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.smiLine.setData(toLineData(times, smiData))
      ind.smiSignal.setData(toLineData(times, smiSig))
    } else if (key === 'signalRatio') {
      // 逐根 K 线评估所有指标信号，计算看多/看空/中性/震荡比例
      const bullishArr = new Array(times.length).fill(null)
      const bearishArr = new Array(times.length).fill(null)
      const netArr = new Array(times.length).fill(null)
      // 预计算所有指标数组（只算一次）
      const ma5 = smaValues(closes, 5)
      const ma10 = smaValues(closes, 10)
      const ma20 = smaValues(closes, 20)
      const ma60 = smaValues(closes, 60)
      const ema12 = emaFinite(closes, 12)
      const ema21 = emaFinite(closes, 21)
      const { upper: bollU, mid: bollM, lower: bollL } = bollingerBands(closes, 20, 2)
      const vw = vwapValues(highs, lows, closes, vols, 20)
      const dema21 = demaValues(closes, 21)
      const tema21 = temaValues(closes, 21)
      const kama10 = kamaValues(closes, 10, 2, 30)
      const hull9 = hullMaValues(closes, 9)
      const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
      const { supertrend: stVal, direction: stDir } = supertrendValues(highs, lows, closes, 10, 3)
      const { tenkan: ichTen, kijun: ichKij, spanA: ichSA, senkouB: ichSB } = ichimokuValues(highs, lows, closes)
      const { sar: sarVal, direction: sarDir } = sarValues(highs, lows, closes, 0.02, 0.2)
      const { upper: dcU, mid: dcM, lower: dcL } = donchianChannelValues(highs, lows, 20)
      const { jaw: agJ, teeth: agT, lips: agL } = alligatorValues(highs, lows, closes)
      const { directions: zzDir } = zigzagValues(highs, lows, closes, 5)
      const { direction: satsDir } = satsValues(highs, lows, closes, vols)
      const { pp: pivPP, s1: pivS1, r1: pivR1 } = pivotPointsValues(highs, lows, closes)
      const { vwap: vbM, upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
      const { dif: macdDif, dea: macdDea, hist: macdHist } = macdBundle(closes)
      const rsi14 = rsiBundle(closes, 14)
      const { K: kdjK, D: kdjD, J: kdjJ } = kdjBundle(highs, lows, closes, 9)
      const cci20 = cciValues(highs, lows, closes, 20)
      const wr14 = williamsRValues(highs, lows, closes, 14)
      const { k: stochK, d: stochD } = stochRsiValues(closes, 14, 14, 3, 3)
      const { adx: adxVal, diP: adxP, diM: adxM } = adxValues(highs, lows, closes, 14)
      const { up: arUp, down: arDown } = aroonValues(highs, lows, 25)
      const cmo14 = cmoValues(closes, 14)
      const trix15 = trixValues(closes, 15)
      const trixSig = emaLeadingNull(trix15, 9)
      const roc12 = rocValues(closes, 12)
      const coppock = coppockValues(closes)
      const { smi: smiD, signal: smiS } = smiValues(highs, lows, closes)
      const ao534 = aoValues(highs, lows)
      const obv = obvValues(closes, vols)
      const mfi14 = mfiValues(highs, lows, closes, vols, 14)
      const cmf20 = cmfValues(highs, lows, closes, vols, 20)
      const adLine = adValues(highs, lows, closes, vols)
      const fi13 = forceIndexValues(closes, vols, 13)
      const co310 = chaikinOscValues(highs, lows, closes, vols, 3, 10)
      const atr14 = atrValues(highs, lows, closes, 14)
      const chop14 = chopValues(highs, lows, closes, 14)
      const miVal = massIndexValues(highs, lows)
      const uiVal = ulcerIndexValues(closes)
      const { squeeze: ttmSq, momentum: ttmMo } = ttmSqueezeValues(highs, lows, closes)
      const { bullPower: erBull, bearPower: erBear } = elderRayValues(highs, lows, closes, 13)
      // 辅助：读取数组在 i 位置的值
      const v = (arr, i) => (i >= 0 && i < arr.length && arr[i] != null && Number.isFinite(arr[i])) ? arr[i] : null
      const vPrev = (arr, i) => v(arr, i - 1)
      // 构建ZigZag方向缓存（找最近非零方向）
      const zzLastDir = new Array(times.length).fill(0)
      let lastNonZero = 0
      for (let i = 0; i < zzDir.length; i++) {
        if (zzDir[i] === 1 || zzDir[i] === -1) lastNonZero = zzDir[i]
        zzLastDir[i] = lastNonZero
      }
      // 逐根K线评估
      for (let i = 0; i < times.length; i++) {
        const c = closes[i]
        if (c == null || !Number.isFinite(c)) continue
        let bull = 0, bear = 0, neut = 0, osci = 0, cnt = 0
        // MA
        { const a5=v(ma5,i),a10=v(ma10,i),a20=v(ma20,i),a60=v(ma60,i)
          if(a5!=null&&a10!=null&&a20!=null&&a60!=null){cnt++;if(a5>a10&&a10>a20&&a20>a60)bull++;else if(a5<a10&&a10<a20&&a20<a60)bear++;else if((a5>a20&&a10<a60)||(a5<a20&&a10>a60))osci++;else neut++;} }
        // EMA
        { const e12=v(ema12,i),e21=v(ema21,i)
          if(e12!=null&&e21!=null){cnt++;if(e12>e21)bull++;else if(e12<e21)bear++;else neut++;} }
        // BOLL
        { const bu=v(bollU,i),bm=v(bollM,i),bl=v(bollL,i)
          if(bu!=null&&bm!=null&&bl!=null){cnt++;if(c>bu)bull++;else if(c<bl)bear++;else if(c>bm)osci++;else neut++;} }
        // VWAP
        { const vwv=v(vw,i);if(vwv!=null){cnt++;if(c>vwv)bull++;else if(c<vwv)bear++;else neut++;} }
        // DEMA
        { const dv=v(dema21,i);if(dv!=null){cnt++;if(c>dv)bull++;else if(c<dv)bear++;else neut++;} }
        // TEMA
        { const tv=v(tema21,i);if(tv!=null){cnt++;if(c>tv)bull++;else if(c<tv)bear++;else neut++;} }
        // KAMA
        { const kv=v(kama10,i),kp=vPrev(kama10,i)
          if(kv!=null&&kp!=null){cnt++;if(c>kv&&kv>kp)bull++;else if(c<kv&&kv<kp)bear++;else neut++;} }
        // HullMA
        { const hv=v(hull9,i),hp=vPrev(hull9,i)
          if(hv!=null&&hp!=null){cnt++;if(hv>hp)bull++;else if(hv<hp)bear++;else neut++;} }
        // Keltner
        { const ku=v(kU,i),kl=v(kL,i)
          if(ku!=null&&kl!=null){cnt++;if(c>ku)bull++;else if(c<kl)bear++;else osci++;} }
        // SuperTrend
        { const sd=v(stDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Ichimoku
        { const it=v(ichTen,i),ik=v(ichKij,i),isa=v(ichSA,i),isb=v(ichSB,i)
          if(it!=null&&ik!=null&&isa!=null&&isb!=null){const ct=Math.max(isa,isb),cb=Math.min(isa,isb);cnt++;if(c>ct&&it>ik)bull++;else if(c<cb&&it<ik)bear++;else if(c>=cb&&c<=ct)osci++;else neut++;} }
        // SAR
        { const sd=v(sarDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Donchian
        { const du=v(dcU,i),dl=v(dcL,i)
          if(du!=null&&dl!=null){cnt++;if(c>=du)bull++;else if(c<=dl)bear++;else osci++;} }
        // Alligator
        { const aj=v(agJ,i),at=v(agT,i),al=v(agL,i)
          if(aj!=null&&at!=null&&al!=null){cnt++;if(al>at&&at>aj)bull++;else if(al<at&&at<aj)bear++;else osci++;} }
        // ZigZag
        { const zd=zzLastDir[i];if(zd!==0){cnt++;if(zd===-1)bull++;else if(zd===1)bear++;else neut++;} }
        // SATS
        { const sd=v(satsDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Pivot
        { const pp=v(pivPP,i),s1=v(pivS1,i),r1=v(pivR1,i)
          if(pp!=null&&r1!=null&&s1!=null){cnt++;if(c>r1)bull++;else if(c<s1)bear++;else if(c>pp)osci++;else neut++;} }
        // VWAPBands
        { const vu=v(vbU,i),vl=v(vbL,i)
          if(vu!=null&&vl!=null){cnt++;if(c>vu)bull++;else if(c<vl)bear++;else osci++;} }
        // MACD
        { const md=v(macdDif,i),me=v(macdDea,i),mh=v(macdHist,i)
          if(md!=null&&me!=null&&mh!=null){cnt++;if(md>me&&mh>0)bull++;else if(md<me&&mh<0)bear++;else if((md>0&&mh<0)||(md<0&&mh>0))osci++;else neut++;} }
        // RSI
        { const rv=v(rsi14,i);if(rv!=null){cnt++;if(rv>70)osci++;else if(rv<30)osci++;else if(rv>50)bull++;else bear++;} }
        // KDJ
        { const kk=v(kdjK,i),kd=v(kdjD,i),kj=v(kdjJ,i)
          if(kk!=null&&kd!=null&&kj!=null){cnt++;if(kj>kk&&kk>kd&&kk<80)bull++;else if(kj<kk&&kk<kd&&kk>20)bear++;else if(kk>80)bear++;else if(kk<20)bull++;else osci++;} }
        // CCI
        { const cv=v(cci20,i);if(cv!=null){cnt++;if(cv>100)bull++;else if(cv<-100)bear++;else osci++;} }
        // W%R
        { const wv=v(wr14,i);if(wv!=null){cnt++;if(wv<-80)bull++;else if(wv>-20)bear++;else osci++;} }
        // StochRSI
        { const sk=v(stochK,i),sd2=v(stochD,i)
          if(sk!=null&&sd2!=null){cnt++;if(sk<20&&sd2<20&&sk>sd2)bull++;else if(sk>80&&sd2>80&&sk<sd2)bear++;else osci++;} }
        // ADX
        { const av=v(adxVal,i),ap=v(adxP,i),am=v(adxM,i)
          if(av!=null&&ap!=null&&am!=null){cnt++;if(av>25&&ap>am)bull++;else if(av>25&&ap<am)bear++;else osci++;} }
        // Aroon
        { const au=v(arUp,i),ad2=v(arDown,i)
          if(au!=null&&ad2!=null){cnt++;if(au>70&&ad2<30)bull++;else if(ad2>70&&au<30)bear++;else osci++;} }
        // CMO
        { const cv=cmo14[i];if(cv!=null&&Number.isFinite(cv)){cnt++;if(cv>50)bull++;else if(cv<-50)bear++;else osci++;} }
        // TRIX
        { const tv=v(trix15,i),ts=v(trixSig,i)
          if(tv!=null&&ts!=null){cnt++;if(tv>ts)bull++;else if(tv<ts)bear++;else neut++;} }
        // ROC
        { const rv=v(roc12,i);if(rv!=null){cnt++;if(rv>0)bull++;else if(rv<0)bear++;else neut++;} }
        // Coppock
        { const cv=v(coppock,i),cp=vPrev(coppock,i)
          if(cv!=null&&cp!=null){cnt++;if(cv>0&&cp<=0)bull++;else if(cv<0)bear++;else neut++;} }
        // SMI
        { const sv=v(smiD,i),ss=v(smiS,i)
          if(sv!=null&&ss!=null){cnt++;if(sv>ss&&sv>0)bull++;else if(sv<ss&&sv<0)bear++;else osci++;} }
        // AO
        { const av2=v(ao534,i),ap2=vPrev(ao534,i)
          if(av2!=null&&ap2!=null){cnt++;if(av2>0&&av2>ap2)bull++;else if(av2<0&&av2<ap2)bear++;else if(av2>0&&av2<ap2)osci++;else neut++;} }
        // OBV
        { const ov=v(obv,i),op=vPrev(obv,i)
          if(ov!=null&&op!=null){cnt++;if(ov>op)bull++;else if(ov<op)bear++;else neut++;} }
        // MFI
        { const mv=v(mfi14,i);if(mv!=null){cnt++;if(mv>80)osci++;else if(mv<20)osci++;else if(mv>50)bull++;else bear++;} }
        // CMF
        { const cv=v(cmf20,i);if(cv!=null){cnt++;if(cv>0.05)bull++;else if(cv<-0.05)bear++;else osci++;} }
        // A/D
        { const av3=v(adLine,i),ap3=vPrev(adLine,i)
          if(av3!=null&&ap3!=null){cnt++;if(av3>ap3)bull++;else if(av3<ap3)bear++;else neut++;} }
        // FI
        { const fv=v(fi13,i);if(fv!=null){cnt++;if(fv>0)bull++;else if(fv<0)bear++;else neut++;} }
        // ChaikinOsc
        { const cv=v(co310,i),cp=vPrev(co310,i)
          if(cv!=null&&cp!=null){cnt++;if(cv>0&&cv>cp)bull++;else if(cv<0&&cv<cp)bear++;else osci++;} }
        // ATR
        { const av4=v(atr14,i),ap4=vPrev(atr14,i)
          if(av4!=null&&ap4!=null){cnt++;if(av4>ap4)osci++;else neut++;} }
        // CHOP
        { const cv2=v(chop14,i);if(cv2!=null){cnt++;if(cv2>61.8)osci++;else neut++;} }
        // MassIndex
        { const mv2=v(miVal,i),mp=vPrev(miVal,i)
          if(mv2!=null&&mp!=null){cnt++;if(mp>27&&mv2<27)bull++;else neut++;} }
        // UlcerIndex
        { const uv=v(uiVal,i);if(uv!=null){cnt++;if(uv<5)bull++;else if(uv>15)bear++;else neut++;} }
        // TTM
        { const sq=v(ttmSq,i),mo=v(ttmMo,i)
          if(sq!=null&&mo!=null){cnt++;if(!sq&&mo>0)bull++;else if(!sq&&mo<0)bear++;else if(sq)osci++;else neut++;} }
        // ElderRay
        { const bp=v(erBull,i),brp=v(erBear,i)
          if(bp!=null&&brp!=null){cnt++;if(bp>0&&bp>brp)bull++;else if(brp<0&&brp<bp)bear++;else osci++;} }
        // 汇总
        if (cnt > 0) {
          bullishArr[i] = (bull / cnt) * 100
          bearishArr[i] = (bear / cnt) * 100
          netArr[i] = ((bull - bear) / cnt) * 100
        }
      }
      ind.signalRatioBullish = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: 'rgba(239, 68, 68, 0.7)',
          lineWidth: 1,
          title: '看多%',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioBearish = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: 'rgba(34, 197, 94, 0.7)',
          lineWidth: 1,
          title: '看空%',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioNet = chart.addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          lineWidth: 2,
          title: '净信号',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioBullish.setData(toLineData(times, bullishArr))
      ind.signalRatioBearish.setData(toLineData(times, bearishArr))
      ind.signalRatioNet.setData(toLineData(times, netArr))
    }
    paneIdx++
  }

  for (let i = 1; i < chart.panes().length; i++) {
    chart.panes()[i].setStretchFactor(1)
  }
}

function syncIndicators() {
  if (!chart || !candleSeries) return

  const { times, opens, closes, highs, lows, vols } = extractOHLCV(mergedRawRows)
  if (!times.length) {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
    ind.vwap = removeSeriesSafe(ind.vwap)
    ind.kama = removeSeriesSafe(ind.kama)
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.sar = removeSeriesSafe(ind.sar)
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
    ind.dema = removeSeriesSafe(ind.dema)
    ind.zigzag = removeSeriesSafe(ind.zigzag)
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
    ind.alligatorJaw = removeSeriesSafe(ind.alligatorJaw)
    ind.alligatorTeeth = removeSeriesSafe(ind.alligatorTeeth)
    ind.alligatorLips = removeSeriesSafe(ind.alligatorLips)
    ind.hullMA = removeSeriesSafe(ind.hullMA)
    ind.fractalHigh = removeSeriesSafe(ind.fractalHigh)
    ind.fractalLow = removeSeriesSafe(ind.fractalLow)
    ind.vwapBandsU = removeSeriesSafe(ind.vwapBandsU)
    ind.vwapBandsM = removeSeriesSafe(ind.vwapBandsM)
    ind.vwapBandsL = removeSeriesSafe(ind.vwapBandsL)
    ind.temaLine = removeSeriesSafe(ind.temaLine)
    ind.smcSwingHigh = removeSeriesSafe(ind.smcSwingHigh)
    ind.smcSwingLow = removeSeriesSafe(ind.smcSwingLow)
    ind.smcIntHigh = removeSeriesSafe(ind.smcIntHigh)
    ind.smcIntLow = removeSeriesSafe(ind.smcIntLow)
    ind.smcBos = removeSeriesSafe(ind.smcBos)
    ind.smcChoch = removeSeriesSafe(ind.smcChoch)
    ind.smcSwBos = removeSeriesSafe(ind.smcSwBos)
    ind.smcSwChoch = removeSeriesSafe(ind.smcSwChoch)
    ind.smcFvgTop = removeSeriesSafe(ind.smcFvgTop)
    ind.smcFvgBot = removeSeriesSafe(ind.smcFvgBot)
    ind.smcObTop = removeSeriesSafe(ind.smcObTop)
    ind.smcObBot = removeSeriesSafe(ind.smcObBot)
    tearDownAllSubPanes()
    return
  }

  const lineCommon = {
    lineWidth: 1,
    lastValueVisible: false,
    priceLineVisible: false,
  }

  if (showMA.value) {
    const m5 = smaValues(closes, 5)
    const m10 = smaValues(closes, 10)
    const m20 = smaValues(closes, 20)
    const m60 = smaValues(closes, 60)
    if (!ind.ma5) {
      ind.ma5 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'MA5' },
        0,
      )
    }
    if (!ind.ma10) {
      ind.ma10 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'MA10' },
        0,
      )
    }
    if (!ind.ma20) {
      ind.ma20 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', title: 'MA20' },
        0,
      )
    }
    if (!ind.ma60) {
      ind.ma60 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#64748b', title: 'MA60' },
        0,
      )
    }
    ind.ma5.setData(toLineData(times, m5))
    ind.ma10.setData(toLineData(times, m10))
    ind.ma20.setData(toLineData(times, m20))
    ind.ma60.setData(toLineData(times, m60))
  } else {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
  }

  if (showBOLL.value) {
    const { upper, mid, lower } = bollingerBands(closes, 20, 2)
    if (!ind.bollU) {
      ind.bollU = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL上',
        },
        0,
      )
    }
    if (!ind.bollM) {
      ind.bollM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#0ea5e9', title: 'BOLL中' },
        0,
      )
    }
    if (!ind.bollL) {
      ind.bollL = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL下',
        },
        0,
      )
    }
    ind.bollU.setData(toLineData(times, upper))
    ind.bollM.setData(toLineData(times, mid))
    ind.bollL.setData(toLineData(times, lower))
  } else {
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
  }

  if (showVWAP.value) {
    const vwap = vwapValues(highs, lows, closes, vols, 20)
    if (!ind.vwap) {
      ind.vwap = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'VWAP20' },
        0,
      )
    }
    ind.vwap.setData(toLineData(times, vwap))
  } else {
    ind.vwap = removeSeriesSafe(ind.vwap)
  }

  if (showKAMA.value) {
    const kama = kamaValues(closes, 10, 2, 30)
    if (!ind.kama) {
      ind.kama = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#14b8a6', title: 'KAMA10' },
        0,
      )
    }
    ind.kama.setData(toLineData(times, kama))
  } else {
    ind.kama = removeSeriesSafe(ind.kama)
  }

  if (showKeltner.value) {
    const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
    if (!ind.keltnerU) {
      ind.keltnerU = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt上',
        },
        0,
      )
    }
    if (!ind.keltnerM) {
      ind.keltnerM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#8b5cf6', title: 'Kelt中' },
        0,
      )
    }
    if (!ind.keltnerL) {
      ind.keltnerL = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt下',
        },
        0,
      )
    }
    ind.keltnerU.setData(toLineData(times, kU))
    ind.keltnerM.setData(toLineData(times, kM))
    ind.keltnerL.setData(toLineData(times, kL))
  } else {
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
  }

  if (showSupertrend.value) {
    const { supertrend: stVal, direction } = supertrendValues(highs, lows, closes, 10, 3)
    if (!ind.supertrend) {
      ind.supertrend = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'ST(10,3)' },
        0,
      )
    }
    const stData = []
    for (let i = 0; i < times.length; i++) {
      if (stVal[i] != null) {
        stData.push({
          time: times[i],
          value: stVal[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.supertrend.setData(stData)
  } else {
    ind.supertrend = removeSeriesSafe(ind.supertrend)
  }

  if (showEMA.value) {
    const e12 = emaFinite(closes, 12)
    const e21 = emaFinite(closes, 21)
    if (!ind.ema12) {
      ind.ema12 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'EMA12' },
        0,
      )
    }
    if (!ind.ema21) {
      ind.ema21 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'EMA21' },
        0,
      )
    }
    ind.ema12.setData(toLineData(times, e12))
    ind.ema21.setData(toLineData(times, e21))
  } else {
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
  }

  if (showIchimoku.value) {
    const { tenkan, kijun, spanA, senkouB, chikou } = ichimokuValues(highs, lows, closes)
    if (!ind.ichTenkan) {
      ind.ichTenkan = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', title: '转换' },
        0,
      )
    }
    if (!ind.ichKijun) {
      ind.ichKijun = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: '基准' },
        0,
      )
    }
    if (!ind.ichSpanA) {
      ind.ichSpanA = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: '先行A' },
        0,
      )
    }
    if (!ind.ichSpanB) {
      ind.ichSpanB = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: '先行B' },
        0,
      )
    }
    if (!ind.ichChikou) {
      ind.ichChikou = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', lineWidth: 1, lineStyle: LineStyle.Dotted, title: '迟行' },
        0,
      )
    }
    ind.ichTenkan.setData(toLineData(times, tenkan))
    ind.ichKijun.setData(toLineData(times, kijun))
    ind.ichSpanA.setData(toLineData(times, spanA))
    ind.ichSpanB.setData(toLineData(times, senkouB))
    ind.ichChikou.setData(toLineData(times, chikou))
  } else {
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
  }

  if (showSAR.value) {
    const { sar, direction } = sarValues(highs, lows, closes, 0.02, 0.2)
    if (!ind.sar) {
      ind.sar = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 3,
          title: 'SAR',
        },
        0,
      )
    }
    const sarData = []
    for (let i = 0; i < times.length; i++) {
      if (sar[i] != null) {
        sarData.push({
          time: times[i],
          value: sar[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.sar.setData(sarData)
  } else {
    ind.sar = removeSeriesSafe(ind.sar)
  }

  if (showDonchian.value) {
    const { upper, mid, lower } = donchianChannelValues(highs, lows, 20)
    if (!ind.donchianU) {
      ind.donchianU = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC上' },
        0,
      )
    }
    if (!ind.donchianM) {
      ind.donchianM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#fb923c', lineStyle: LineStyle.Dotted, title: 'DC中' },
        0,
      )
    }
    if (!ind.donchianL) {
      ind.donchianL = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC下' },
        0,
      )
    }
    ind.donchianU.setData(toLineData(times, upper))
    ind.donchianM.setData(toLineData(times, mid))
    ind.donchianL.setData(toLineData(times, lower))
  } else {
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
  }

  if (showPivot.value) {
    const { pp, s1, s2, r1, r2 } = pivotPointsValues(highs, lows, closes)
    if (!ind.pivotPP) {
      ind.pivotPP = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#a3a3a3', lineStyle: LineStyle.Dotted, title: 'PP' },
        0,
      )
    }
    if (!ind.pivotS1) {
      ind.pivotS1 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: 'S1' },
        0,
      )
    }
    if (!ind.pivotS2) {
      ind.pivotS2 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#16a34a', lineStyle: LineStyle.Dashed, title: 'S2' },
        0,
      )
    }
    if (!ind.pivotR1) {
      ind.pivotR1 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: 'R1' },
        0,
      )
    }
    if (!ind.pivotR2) {
      ind.pivotR2 = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#dc2626', lineStyle: LineStyle.Dashed, title: 'R2' },
        0,
      )
    }
    ind.pivotPP.setData(toLineData(times, pp))
    ind.pivotS1.setData(toLineData(times, s1))
    ind.pivotS2.setData(toLineData(times, s2))
    ind.pivotR1.setData(toLineData(times, r1))
    ind.pivotR2.setData(toLineData(times, r2))
  } else {
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
  }

  if (showDEMA.value) {
    const d = demaValues(closes, 21)
    if (!ind.dema) {
      ind.dema = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'DEMA21' },
        0,
      )
    }
    ind.dema.setData(toLineData(times, d))
  } else {
    ind.dema = removeSeriesSafe(ind.dema)
  }

  if (showZigZag.value) {
    const { zigzag, directions } = zigzagValues(highs, lows, closes, 5)
    if (!ind.zigzag) {
      ind.zigzag = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          color: '#f59e0b',
          pointMarkersVisible: true,
          pointMarkersRadius: 4,
          title: 'ZigZag',
        },
        0,
      )
    }
    const zzData = []
    for (let i = 0; i < times.length; i++) {
      if (zigzag[i] != null) {
        zzData.push({
          time: times[i],
          value: zigzag[i],
          color: directions[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.zigzag.setData(zzData)
  } else {
    ind.zigzag = removeSeriesSafe(ind.zigzag)
  }

  if (showSATS.value) {
    const { stLine, upper, lower, direction, tqi } = satsValues(highs, lows, closes, vols)
    if (!ind.satsLine) {
      ind.satsLine = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'SATS' },
        0,
      )
    }
    const satsData = []
    for (let i = 0; i < times.length; i++) {
      if (stLine[i] != null) {
        satsData.push({
          time: times[i],
          value: stLine[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.satsLine.setData(satsData)
    if (!ind.satsUpper) {
      ind.satsUpper = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS上' },
        0,
      )
    }
    if (!ind.satsLower) {
      ind.satsLower = chart.addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS下' },
        0,
      )
    }
    ind.satsUpper.setData(toLineData(times, upper))
    ind.satsLower.setData(toLineData(times, lower))
  } else {
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
  }

  if (showAlligator.value) {
    const { jaw, teeth, lips } = alligatorValues(highs, lows, closes)
    if (!ind.alligatorJaw) {
      ind.alligatorJaw = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineWidth: 1, lineStyle: LineStyle.Dashed, title: '颚(13)' },
        0,
      )
    }
    if (!ind.alligatorTeeth) {
      ind.alligatorTeeth = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineWidth: 1, lineStyle: LineStyle.Dashed, title: '齿(8)' },
        0,
      )
    }
    if (!ind.alligatorLips) {
      ind.alligatorLips = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', lineWidth: 1, title: '唇(5)' },
        0,
      )
    }
    ind.alligatorJaw.setData(toLineData(times, jaw))
    ind.alligatorTeeth.setData(toLineData(times, teeth))
    ind.alligatorLips.setData(toLineData(times, lips))
  } else {
    ind.alligatorJaw = removeSeriesSafe(ind.alligatorJaw)
    ind.alligatorTeeth = removeSeriesSafe(ind.alligatorTeeth)
    ind.alligatorLips = removeSeriesSafe(ind.alligatorLips)
  }

  if (showHullMA.value) {
    const hull = hullMaValues(closes, 9)
    if (!ind.hullMA) {
      ind.hullMA = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', lineWidth: 2, title: 'Hull(9)' },
        0,
      )
    }
    ind.hullMA.setData(toLineData(times, hull))
  } else {
    ind.hullMA = removeSeriesSafe(ind.hullMA)
  }

  if (showFractal.value) {
    const { fractalHigh, fractalLow } = fractalValues(highs, lows)
    if (!ind.fractalHigh) {
      ind.fractalHigh = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 5,
          title: '▲Fractal',
          color: '#ef4444',
        },
        0,
      )
    }
    if (!ind.fractalLow) {
      ind.fractalLow = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 5,
          title: '▼Fractal',
          color: '#22c55e',
        },
        0,
      )
    }
    ind.fractalHigh.setData(toLineData(times, fractalHigh))
    ind.fractalLow.setData(toLineData(times, fractalLow))
  } else {
    ind.fractalHigh = removeSeriesSafe(ind.fractalHigh)
    ind.fractalLow = removeSeriesSafe(ind.fractalLow)
  }

  if (showVWAPBands.value) {
    const { vwap: vbM, upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
    if (!ind.vwapBandsU) {
      ind.vwapBandsU = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'VB上',
        },
        0,
      )
    }
    if (!ind.vwapBandsM) {
      ind.vwapBandsM = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#8b5cf6', title: 'VWAP' },
        0,
      )
    }
    if (!ind.vwapBandsL) {
      ind.vwapBandsL = chart.addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'VB下',
        },
        0,
      )
    }
    ind.vwapBandsU.setData(toLineData(times, vbU))
    ind.vwapBandsM.setData(toLineData(times, vbM))
    ind.vwapBandsL.setData(toLineData(times, vbL))
  } else {
    ind.vwapBandsU = removeSeriesSafe(ind.vwapBandsU)
    ind.vwapBandsM = removeSeriesSafe(ind.vwapBandsM)
    ind.vwapBandsL = removeSeriesSafe(ind.vwapBandsL)
  }

  if (showTEMA.value) {
    const tema = temaValues(closes, 21)
    if (!ind.temaLine) {
      ind.temaLine = chart.addSeries(
        LineSeries,
        { ...lineCommon, color: '#06b6d4', lineWidth: 2, title: 'TEMA(21)' },
        0,
      )
    }
    ind.temaLine.setData(toLineData(times, tema))
  } else {
    ind.temaLine = removeSeriesSafe(ind.temaLine)
  }

  if (showSMC.value) {
    const smc = smcValues(highs, lows, closes, opens)
    if (!ind.smcSwingHigh) {
      ind.smcSwingHigh = chart.addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 6, title: 'SwH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwingLow = chart.addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 6, title: 'SwL', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcIntHigh = chart.addSeries(LineSeries, { ...lineCommon, color: '#f87171', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'iH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcIntLow = chart.addSeries(LineSeries, { ...lineCommon, color: '#4ade80', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'iL', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcBos = chart.addSeries(LineSeries, { ...lineCommon, color: '#3b82f6', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 4, title: 'BOS', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcChoch = chart.addSeries(LineSeries, { ...lineCommon, color: '#f59e0b', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 4, title: 'CHoCH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwBos = chart.addSeries(LineSeries, { ...lineCommon, color: '#6366f1', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 5, title: 'SwBOS', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwChoch = chart.addSeries(LineSeries, { ...lineCommon, color: '#eab308', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 5, title: 'SwCHoCH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcFvgTop = chart.addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 2, title: 'FVG↑', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcFvgBot = chart.addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 2, title: 'FVG↓', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcObTop = chart.addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'OB↑', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcObBot = chart.addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'OB↓', lastValueVisible: false, priceLineVisible: false }, 0)
    }
    ind.smcSwingHigh.setData(smc.swingHighPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcSwingLow.setData(smc.swingLowPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcIntHigh.setData(smc.intHighPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcIntLow.setData(smc.intLowPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcBos.setData(smc.bosLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcChoch.setData(smc.chochLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcSwBos.setData(smc.swingBosLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcSwChoch.setData(smc.swingChochLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    const fvgTopData = smc.fvgZones.filter(z => !z.mitigated).map(z => ({ time: times[z.startIdx], value: z.top })).sort((a, b) => a.time - b.time)
    const fvgBotData = smc.fvgZones.filter(z => !z.mitigated).map(z => ({ time: times[z.startIdx], value: z.bot })).sort((a, b) => a.time - b.time)
    ind.smcFvgTop.setData(fvgTopData)
    ind.smcFvgBot.setData(fvgBotData)
    const obTopData = smc.orderBlocks.filter(o => !o.mitigated).map(o => ({ time: times[o.idx], value: o.top })).sort((a, b) => a.time - b.time)
    const obBotData = smc.orderBlocks.filter(o => !o.mitigated).map(o => ({ time: times[o.idx], value: o.bot })).sort((a, b) => a.time - b.time)
    ind.smcObTop.setData(obTopData)
    ind.smcObBot.setData(obBotData)
  } else {
    ind.smcSwingHigh = removeSeriesSafe(ind.smcSwingHigh)
    ind.smcSwingLow = removeSeriesSafe(ind.smcSwingLow)
    ind.smcIntHigh = removeSeriesSafe(ind.smcIntHigh)
    ind.smcIntLow = removeSeriesSafe(ind.smcIntLow)
    ind.smcBos = removeSeriesSafe(ind.smcBos)
    ind.smcChoch = removeSeriesSafe(ind.smcChoch)
    ind.smcSwBos = removeSeriesSafe(ind.smcSwBos)
    ind.smcSwChoch = removeSeriesSafe(ind.smcSwChoch)
    ind.smcFvgTop = removeSeriesSafe(ind.smcFvgTop)
    ind.smcFvgBot = removeSeriesSafe(ind.smcFvgBot)
    ind.smcObTop = removeSeriesSafe(ind.smcObTop)
    ind.smcObBot = removeSeriesSafe(ind.smcObBot)
  }

  syncSubPaneIndicators(times, closes, highs, lows, vols)
}

function formatCrosshairTime(time) {
  const ms = chartTimeToUtcMs(time)
  if (!Number.isFinite(ms)) return ''
  const d = new Date(ms)
  const loc = 'zh-CN'
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
  if (minuteLike) {
    return new Intl.DateTimeFormat(loc, {
      timeZone: CN_TZ,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(d)
  }
  return new Intl.DateTimeFormat(loc, {
    timeZone: CN_TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(d)
}

function findRawRowByChartTime(t) {
  if (t === undefined || t === null) return null
  const targetMs = chartTimeToUtcMs(t)
  if (Number.isFinite(targetMs)) {
    for (const r of mergedRawRows) {
      const ct = toChartTime(r.day)
      const ms = chartTimeToUtcMs(ct)
      if (Number.isFinite(ms) && ms === targetMs) return r
    }
  }
  for (const r of mergedRawRows) {
    const ct = toChartTime(r.day)
    if (ct === t) return r
  }
  return null
}

/** mergedRawRows 按时间升序，取最后一根作为面板默认展示 */
function syncDefaultLatestPanelRow() {
  const rows = mergedRawRows
  if (!rows.length) {
    defaultLatestRawRow.value = null
    return
  }
  defaultLatestRawRow.value = rows[rows.length - 1]
}

function formatPanelTitleDay(r) {
  const dailyLike = DAILY_LIKE_KLT.has(activeKlt.value)
  if (dailyLike) {
    const ymd = extractYmdDatePart(String(r.day || '').replace(/\//g, '-'))
    if (/^\d{4}-\d{2}-\d{2}$/.test(ymd)) return ymd
  }
  const t = toChartTime(r.day)
  if (t == null) return String(r.day || '').trim() || '--'
  return formatCrosshairTime(t)
}

const crosshairPanel = computed(() => {
  const r = hoverRawRow.value ?? defaultLatestRawRow.value
  if (!r) return null
  const chgPct = parseNumStr(r.changePercent)
  const sign = chgPct > 0 ? 1 : chgPct < 0 ? -1 : 0
  const neu = props.darkTheme ? '#94a3b8' : '#64748b'
  const ohlcC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
  const chgC = sign > 0 ? CLR_RISE : sign < 0 ? CLR_FALL : neu
  const showLatestTag = !hoverRawRow.value && defaultLatestRawRow.value
  const titleDay = formatPanelTitleDay(r)
  const curDay = String(r.day || '').replace(/\//g, '-')
  const curIdx = mergedRawRows.findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)
  const amps = []
  for (let i = 0; i <= curIdx; i++) {
    const row = mergedRawRows[i]
    const rawAmp = parseNumStr(row.amplitude)
    const o = Number(row.open), h = Number(row.high), l = Number(row.low)
    if (Number.isFinite(rawAmp)) {
      amps.push(rawAmp)
    } else if (Number.isFinite(o) && o > 0 && Number.isFinite(h) && Number.isFinite(l)) {
      amps.push((h - l) / o * 100)
    } else {
      amps.push(NaN)
    }
  }
  let amp5 = '--', amp10 = '--', amp20 = '--'
  if (curIdx >= 0) {
    const a5 = avgAmplitude(amps, 5)
    const a10 = avgAmplitude(amps, 10)
    const a20 = avgAmplitude(amps, 20)
    if (Number.isFinite(a5)) amp5 = a5.toFixed(2) + '%'
    if (Number.isFinite(a10)) amp10 = a10.toFixed(2) + '%'
    if (Number.isFinite(a20)) amp20 = a20.toFixed(2) + '%'
  }
  return {
    title: showLatestTag ? `${titleDay} · 最新` : titleDay,
    open: formatPrice2(r.open),
    close: formatPrice2(r.close),
    high: formatPrice2(r.high),
    low: formatPrice2(r.low),
    changePercent: formatPctField(r.changePercent),
    changeValue: formatSigned2(r.changeValue),
    volume: formatVolumeCn(r.volume),
    amount: formatAmountCn(r.amount),
    amplitude: formatPctField(r.amplitude),
    avgAmp5: amp5,
    avgAmp10: amp10,
    avgAmp20: amp20,
    turnoverRate: formatPctField(r.turnoverRate),
    volumeRatio: formatVolumeRatio(r.volumeRatio),
    cOpenClose: ohlcC,
    cHigh: CLR_RISE,
    cLow: CLR_FALL,
    cChg: chgC,
    cNeu: neu,
  }
})

function evaluateIndicatorSignals(endIdx) {
  const rows = mergedRawRows
  if (!rows || rows.length < 2) return []

  // 截取到 endIdx（含），使指标计算基于该 K 线位置的数据
  const sliced = endIdx != null && endIdx >= 0 && endIdx < rows.length - 1
    ? rows.slice(0, endIdx + 1)
    : rows
  const { times, opens, closes, highs, lows, vols } = extractOHLCV(sliced)
  const n = times.length
  if (n < 2) return []

  const last = (arr) => {
    for (let i = arr.length - 1; i >= 0; i--) {
      if (arr[i] != null && Number.isFinite(arr[i])) return arr[i]
    }
    return null
  }
  const prev = (arr) => {
    let cnt = 0
    for (let i = arr.length - 1; i >= 0; i--) {
      if (arr[i] != null && Number.isFinite(arr[i])) {
        cnt++
        if (cnt === 2) return arr[i]
      }
    }
    return null
  }
  const signals = []

  // ── Trend indicators ──

  // MA
  {
    const m5 = smaValues(closes, 5)
    const m10 = smaValues(closes, 10)
    const m20 = smaValues(closes, 20)
    const m60 = smaValues(closes, 60)
    const v5 = last(m5), v10 = last(m10), v20 = last(m20), v60 = last(m60)
    if (v5 != null && v10 != null && v20 != null && v60 != null) {
      if (v5 > v10 && v10 > v20 && v20 > v60) signals.push({ name: 'MA', signal: 'bullish' })
      else if (v5 < v10 && v10 < v20 && v20 < v60) signals.push({ name: 'MA', signal: 'bearish' })
      else if ((v5 > v20 && v10 < v60) || (v5 < v20 && v10 > v60)) signals.push({ name: 'MA', signal: 'oscillating' })
      else signals.push({ name: 'MA', signal: 'neutral' })
    }
  }

  // EMA
  {
    const e12 = emaFinite(closes, 12)
    const e21 = emaFinite(closes, 21)
    const v12 = last(e12), v21 = last(e21)
    if (v12 != null && v21 != null) {
      if (v12 > v21) signals.push({ name: 'EMA', signal: 'bullish' })
      else if (v12 < v21) signals.push({ name: 'EMA', signal: 'bearish' })
      else signals.push({ name: 'EMA', signal: 'neutral' })
    }
  }

  // BOLL
  {
    const { upper, mid, lower } = bollingerBands(closes, 20, 2)
    const vU = last(upper), vM = last(mid), vL = last(lower), c = closes[n - 1]
    if (vU != null && vM != null && vL != null) {
      if (c > vU) signals.push({ name: 'BOLL', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'BOLL', signal: 'bearish' })
      else if (c > vM) signals.push({ name: 'BOLL', signal: 'oscillating' })
      else signals.push({ name: 'BOLL', signal: 'neutral' })
    }
  }

  // VWAP
  {
    const vw = vwapValues(highs, lows, closes, vols, 20)
    const v = last(vw), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'VWAP', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'VWAP', signal: 'bearish' })
      else signals.push({ name: 'VWAP', signal: 'neutral' })
    }
  }

  // DEMA
  {
    const d = demaValues(closes, 21)
    const v = last(d), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'DEMA', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'DEMA', signal: 'bearish' })
      else signals.push({ name: 'DEMA', signal: 'neutral' })
    }
  }

  // TEMA
  {
    const t = temaValues(closes, 21)
    const v = last(t), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'TEMA', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'TEMA', signal: 'bearish' })
      else signals.push({ name: 'TEMA', signal: 'neutral' })
    }
  }

  // KAMA
  {
    const k = kamaValues(closes, 10, 2, 30)
    const v = last(k), c = closes[n - 1], pv = prev(k)
    if (v != null && pv != null) {
      if (c > v && v > pv) signals.push({ name: 'KAMA', signal: 'bullish' })
      else if (c < v && v < pv) signals.push({ name: 'KAMA', signal: 'bearish' })
      else signals.push({ name: 'KAMA', signal: 'neutral' })
    }
  }

  // HullMA
  {
    const h = hullMaValues(closes, 9)
    const v = last(h), pv = prev(h)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'HullMA', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'HullMA', signal: 'bearish' })
      else signals.push({ name: 'HullMA', signal: 'neutral' })
    }
  }

  // Keltner
  {
    const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
    const vU = last(kU), vM = last(kM), vL = last(kL), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c > vU) signals.push({ name: 'Keltner', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'Keltner', signal: 'bearish' })
      else signals.push({ name: 'Keltner', signal: 'oscillating' })
    }
  }

  // SuperTrend
  {
    const { supertrend: stVal, direction } = supertrendValues(highs, lows, closes, 10, 3)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SuperTrend', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SuperTrend', signal: 'bearish' })
      else signals.push({ name: 'SuperTrend', signal: 'neutral' })
    }
  }

  // Ichimoku
  {
    const { tenkan, kijun, spanA, senkouB } = ichimokuValues(highs, lows, closes)
    const vT = last(tenkan), vK = last(kijun), vSA = last(spanA), vSB = last(senkouB), c = closes[n - 1]
    if (vT != null && vK != null && vSA != null && vSB != null) {
      const cloudTop = Math.max(vSA, vSB)
      const cloudBot = Math.min(vSA, vSB)
      if (c > cloudTop && vT > vK) signals.push({ name: 'Ichimoku', signal: 'bullish' })
      else if (c < cloudBot && vT < vK) signals.push({ name: 'Ichimoku', signal: 'bearish' })
      else if (c >= cloudBot && c <= cloudTop) signals.push({ name: 'Ichimoku', signal: 'oscillating' })
      else signals.push({ name: 'Ichimoku', signal: 'neutral' })
    }
  }

  // SAR
  {
    const { sar, direction } = sarValues(highs, lows, closes, 0.02, 0.2)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SAR', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SAR', signal: 'bearish' })
      else signals.push({ name: 'SAR', signal: 'neutral' })
    }
  }

  // Donchian
  {
    const { upper, mid, lower } = donchianChannelValues(highs, lows, 20)
    const vU = last(upper), vL = last(lower), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c >= vU) signals.push({ name: 'Donchian', signal: 'bullish' })
      else if (c <= vL) signals.push({ name: 'Donchian', signal: 'bearish' })
      else signals.push({ name: 'Donchian', signal: 'oscillating' })
    }
  }

  // Alligator
  {
    const { jaw, teeth, lips } = alligatorValues(highs, lows, closes)
    const vJ = last(jaw), vT = last(teeth), vL = last(lips)
    if (vJ != null && vT != null && vL != null) {
      if (vL > vT && vT > vJ) signals.push({ name: 'Alligator', signal: 'bullish' })
      else if (vL < vT && vT < vJ) signals.push({ name: 'Alligator', signal: 'bearish' })
      else signals.push({ name: 'Alligator', signal: 'oscillating' })
    }
  }

  // ZigZag
  {
    const { directions } = zigzagValues(highs, lows, closes, 5)
    let zzDir = 0
    for (let i = directions.length - 1; i >= 0; i--) {
      if (directions[i] === 1 || directions[i] === -1) { zzDir = directions[i]; break }
    }
    if (zzDir === -1) signals.push({ name: 'ZigZag', signal: 'bullish' })
    else if (zzDir === 1) signals.push({ name: 'ZigZag', signal: 'bearish' })
    else signals.push({ name: 'ZigZag', signal: 'neutral' })
  }

  // SATS
  {
    const { direction } = satsValues(highs, lows, closes, vols)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SATS', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SATS', signal: 'bearish' })
      else signals.push({ name: 'SATS', signal: 'neutral' })
    }
  }

  // Pivot
  {
    const { pp, s1, r1 } = pivotPointsValues(highs, lows, closes)
    const vPP = last(pp), vS1 = last(s1), vR1 = last(r1), c = closes[n - 1]
    if (vPP != null && vR1 != null && vS1 != null) {
      if (c > vR1) signals.push({ name: 'Pivot', signal: 'bullish' })
      else if (c < vS1) signals.push({ name: 'Pivot', signal: 'bearish' })
      else if (c > vPP) signals.push({ name: 'Pivot', signal: 'oscillating' })
      else signals.push({ name: 'Pivot', signal: 'neutral' })
    }
  }

  // VWAPBands
  {
    const { vwap: vbM, upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
    const vU = last(vbU), vL = last(vbL), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c > vU) signals.push({ name: 'VWAPBands', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'VWAPBands', signal: 'bearish' })
      else signals.push({ name: 'VWAPBands', signal: 'oscillating' })
    }
  }

  // ── Momentum / oscillator indicators ──

  // MACD
  {
    const { dif, dea, hist } = macdBundle(closes)
    const vDif = last(dif), vDea = last(dea), vHist = last(hist), pHist = prev(hist)
    if (vDif != null && vDea != null && vHist != null) {
      if (vDif > vDea && vHist > 0) signals.push({ name: 'MACD', signal: 'bullish' })
      else if (vDif < vDea && vHist < 0) signals.push({ name: 'MACD', signal: 'bearish' })
      else if (vDif > 0 && vHist < 0) signals.push({ name: 'MACD', signal: 'oscillating' })
      else if (vDif < 0 && vHist > 0) signals.push({ name: 'MACD', signal: 'oscillating' })
      else signals.push({ name: 'MACD', signal: 'neutral' })
    }
  }

  // RSI
  {
    const rsi = rsiBundle(closes, 14)
    const v = last(rsi)
    if (v != null) {
      if (v > 70) signals.push({ name: 'RSI', signal: 'oscillating' })
      else if (v < 30) signals.push({ name: 'RSI', signal: 'oscillating' })
      else if (v > 50) signals.push({ name: 'RSI', signal: 'bullish' })
      else signals.push({ name: 'RSI', signal: 'bearish' })
    }
  }

  // KDJ
  {
    const { K, D, J } = kdjBundle(highs, lows, closes, 9)
    const vK = last(K), vD = last(D), vJ = last(J)
    if (vK != null && vD != null && vJ != null) {
      if (vJ > vK && vK > vD && vK < 80) signals.push({ name: 'KDJ', signal: 'bullish' })
      else if (vJ < vK && vK < vD && vK > 20) signals.push({ name: 'KDJ', signal: 'bearish' })
      else if (vK > 80) signals.push({ name: 'KDJ', signal: 'bearish' })
      else if (vK < 20) signals.push({ name: 'KDJ', signal: 'bullish' })
      else signals.push({ name: 'KDJ', signal: 'oscillating' })
    }
  }

  // CCI
  {
    const cci = cciValues(highs, lows, closes, 20)
    const v = last(cci)
    if (v != null) {
      if (v > 100) signals.push({ name: 'CCI', signal: 'bullish' })
      else if (v < -100) signals.push({ name: 'CCI', signal: 'bearish' })
      else signals.push({ name: 'CCI', signal: 'oscillating' })
    }
  }

  // Williams %R
  {
    const wr = williamsRValues(highs, lows, closes, 14)
    const v = last(wr)
    if (v != null) {
      if (v < -80) signals.push({ name: 'W%R', signal: 'bullish' })
      else if (v > -20) signals.push({ name: 'W%R', signal: 'bearish' })
      else signals.push({ name: 'W%R', signal: 'oscillating' })
    }
  }

  // StochRSI
  {
    const { k, d } = stochRsiValues(closes, 14, 14, 3, 3)
    const vK = last(k), vD = last(d)
    if (vK != null && vD != null) {
      if (vK < 20 && vD < 20 && vK > vD) signals.push({ name: 'StochRSI', signal: 'bullish' })
      else if (vK > 80 && vD > 80 && vK < vD) signals.push({ name: 'StochRSI', signal: 'bearish' })
      else signals.push({ name: 'StochRSI', signal: 'oscillating' })
    }
  }

  // ADX
  {
    const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
    const vAdx = last(adx), vP = last(diP), vM = last(diM)
    if (vAdx != null && vP != null && vM != null) {
      if (vAdx > 25 && vP > vM) signals.push({ name: 'ADX', signal: 'bullish' })
      else if (vAdx > 25 && vP < vM) signals.push({ name: 'ADX', signal: 'bearish' })
      else signals.push({ name: 'ADX', signal: 'oscillating' })
    }
  }

  // Aroon
  {
    const { up, down } = aroonValues(highs, lows, 25)
    const vU = last(up), vD = last(down)
    if (vU != null && vD != null) {
      if (vU > 70 && vD < 30) signals.push({ name: 'Aroon', signal: 'bullish' })
      else if (vD > 70 && vU < 30) signals.push({ name: 'Aroon', signal: 'bearish' })
      else signals.push({ name: 'Aroon', signal: 'oscillating' })
    }
  }

  // CMO
  {
    const cmo = cmoValues(closes, 14)
    const v = last(cmo)
    if (v != null) {
      if (v > 50) signals.push({ name: 'CMO', signal: 'bullish' })
      else if (v < -50) signals.push({ name: 'CMO', signal: 'bearish' })
      else signals.push({ name: 'CMO', signal: 'oscillating' })
    }
  }

  // TRIX
  {
    const trix = trixValues(closes, 15)
    const signal = emaLeadingNull(trix, 9)
    const vT = last(trix), vS = last(signal)
    if (vT != null && vS != null) {
      if (vT > vS) signals.push({ name: 'TRIX', signal: 'bullish' })
      else if (vT < vS) signals.push({ name: 'TRIX', signal: 'bearish' })
      else signals.push({ name: 'TRIX', signal: 'neutral' })
    }
  }

  // ROC
  {
    const roc = rocValues(closes, 12)
    const v = last(roc)
    if (v != null) {
      if (v > 0) signals.push({ name: 'ROC', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'ROC', signal: 'bearish' })
      else signals.push({ name: 'ROC', signal: 'neutral' })
    }
  }

  // Coppock
  {
    const cp = coppockValues(closes)
    const v = last(cp), pv = prev(cp)
    if (v != null && pv != null) {
      if (v > 0 && pv <= 0) signals.push({ name: 'Coppock', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'Coppock', signal: 'bearish' })
      else signals.push({ name: 'Coppock', signal: 'neutral' })
    }
  }

  // SMI
  {
    const { smi: smiData, signal: smiSig } = smiValues(highs, lows, closes)
    const vSmi = last(smiData), vSig = last(smiSig)
    if (vSmi != null && vSig != null) {
      if (vSmi > vSig && vSmi > 0) signals.push({ name: 'SMI', signal: 'bullish' })
      else if (vSmi < vSig && vSmi < 0) signals.push({ name: 'SMI', signal: 'bearish' })
      else signals.push({ name: 'SMI', signal: 'oscillating' })
    }
  }

  // AO
  {
    const ao = aoValues(highs, lows)
    const v = last(ao), pv = prev(ao)
    if (v != null && pv != null) {
      if (v > 0 && v > pv) signals.push({ name: 'AO', signal: 'bullish' })
      else if (v < 0 && v < pv) signals.push({ name: 'AO', signal: 'bearish' })
      else if (v > 0 && v < pv) signals.push({ name: 'AO', signal: 'oscillating' })
      else signals.push({ name: 'AO', signal: 'neutral' })
    }
  }

  // ── Volume indicators ──

  // OBV
  {
    const obv = obvValues(closes, vols)
    const v = last(obv), pv = prev(obv)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'OBV', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'OBV', signal: 'bearish' })
      else signals.push({ name: 'OBV', signal: 'neutral' })
    }
  }

  // MFI
  {
    const mfi = mfiValues(highs, lows, closes, vols, 14)
    const v = last(mfi)
    if (v != null) {
      if (v > 80) signals.push({ name: 'MFI', signal: 'oscillating' })
      else if (v < 20) signals.push({ name: 'MFI', signal: 'oscillating' })
      else if (v > 50) signals.push({ name: 'MFI', signal: 'bullish' })
      else signals.push({ name: 'MFI', signal: 'bearish' })
    }
  }

  // CMF
  {
    const cmf = cmfValues(highs, lows, closes, vols, 20)
    const v = last(cmf)
    if (v != null) {
      if (v > 0.05) signals.push({ name: 'CMF', signal: 'bullish' })
      else if (v < -0.05) signals.push({ name: 'CMF', signal: 'bearish' })
      else signals.push({ name: 'CMF', signal: 'oscillating' })
    }
  }

  // A/D
  {
    const ad = adValues(highs, lows, closes, vols)
    const v = last(ad), pv = prev(ad)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'A/D', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'A/D', signal: 'bearish' })
      else signals.push({ name: 'A/D', signal: 'neutral' })
    }
  }

  // ForceIndex
  {
    const fi = forceIndexValues(closes, vols, 13)
    const v = last(fi)
    if (v != null) {
      if (v > 0) signals.push({ name: 'FI', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'FI', signal: 'bearish' })
      else signals.push({ name: 'FI', signal: 'neutral' })
    }
  }

  // ChaikinOsc
  {
    const co = chaikinOscValues(highs, lows, closes, vols, 3, 10)
    const v = last(co), pv = prev(co)
    if (v != null && pv != null) {
      if (v > 0 && v > pv) signals.push({ name: 'ChaikinOsc', signal: 'bullish' })
      else if (v < 0 && v < pv) signals.push({ name: 'ChaikinOsc', signal: 'bearish' })
      else signals.push({ name: 'ChaikinOsc', signal: 'oscillating' })
    }
  }

  // ── Volatility indicators ──

  // ATR
  {
    const atr = atrValues(highs, lows, closes, 14)
    const v = last(atr), pv = prev(atr)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'ATR', signal: 'oscillating' })
      else signals.push({ name: 'ATR', signal: 'neutral' })
    }
  }

  // CHOP
  {
    const chop = chopValues(highs, lows, closes, 14)
    const v = last(chop)
    if (v != null) {
      if (v > 61.8) signals.push({ name: 'CHOP', signal: 'oscillating' })
      else signals.push({ name: 'CHOP', signal: 'neutral' })
    }
  }

  // MassIndex
  {
    const mi = massIndexValues(highs, lows)
    const v = last(mi), pv = prev(mi)
    if (v != null && pv != null) {
      if (pv > 27 && v < 27) signals.push({ name: 'MassIndex', signal: 'bullish' })
      else signals.push({ name: 'MassIndex', signal: 'neutral' })
    }
  }

  // UlcerIndex
  {
    const ui = ulcerIndexValues(closes)
    const v = last(ui)
    if (v != null) {
      if (v < 5) signals.push({ name: 'UlcerIndex', signal: 'bullish' })
      else if (v > 15) signals.push({ name: 'UlcerIndex', signal: 'bearish' })
      else signals.push({ name: 'UlcerIndex', signal: 'neutral' })
    }
  }

  // TTMSqueeze
  {
    const { squeeze, momentum } = ttmSqueezeValues(highs, lows, closes)
    const sq = last(squeeze), mo = last(momentum), pMo = prev(momentum)
    if (sq != null && mo != null) {
      if (!sq && mo > 0) signals.push({ name: 'TTM', signal: 'bullish' })
      else if (!sq && mo < 0) signals.push({ name: 'TTM', signal: 'bearish' })
      else if (sq) signals.push({ name: 'TTM', signal: 'oscillating' })
      else signals.push({ name: 'TTM', signal: 'neutral' })
    }
  }

  // ElderRay
  {
    const { bullPower, bearPower } = elderRayValues(highs, lows, closes, 13)
    const bP = last(bullPower), brP = last(bearPower)
    if (bP != null && brP != null) {
      if (bP > 0 && bP > brP) signals.push({ name: 'ElderRay', signal: 'bullish' })
      else if (brP < 0 && brP < bP) signals.push({ name: 'ElderRay', signal: 'bearish' })
      else signals.push({ name: 'ElderRay', signal: 'oscillating' })
    }
  }

  return signals
}

const indicatorSignalSummary = computed(() => {
  // 依赖 mergedRawRowsVersion 以感知 mergedRawRows 变更
  void mergedRawRowsVersion.value
  // 根据 hoverRawRow 计算当前光标对应的 K 线索引
  const hoveredRow = hoverRawRow.value
  let endIdx = null
  if (hoveredRow) {
    const curDay = String(hoveredRow.day || '').replace(/\//g, '-')
    const idx = mergedRawRows.findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)
    if (idx >= 0) endIdx = idx
  }
  const sigs = evaluateIndicatorSignals(endIdx)
  if (!sigs.length) return null
  const counts = { bullish: 0, bearish: 0, neutral: 0, oscillating: 0 }
  for (const s of sigs) {
    if (counts[s.signal] !== undefined) counts[s.signal]++
  }
  const total = sigs.length
  return {
    total,
    bullish: counts.bullish,
    bearish: counts.bearish,
    neutral: counts.neutral,
    oscillating: counts.oscillating,
    bullishPct: total > 0 ? Math.round((counts.bullish / total) * 100) : 0,
    bearishPct: total > 0 ? Math.round((counts.bearish / total) * 100) : 0,
    neutralPct: total > 0 ? Math.round((counts.neutral / total) * 100) : 0,
    oscillatingPct: total > 0 ? Math.round((counts.oscillating / total) * 100) : 0,
    signals: sigs,
  }
})

function clearLongPositionPriceLines() {
  longLineByKind = { entry: null, stop: null, takeProfit: null }
  if (!candleSeries) {
    longPositionPriceLines = []
    return
  }
  for (const pl of longPositionPriceLines) {
    try {
      candleSeries.removePriceLine(pl)
    } catch {
      /* ignore */
    }
  }
  longPositionPriceLines = []
}

function syncLongPositionPriceLines() {
  console.log('[DEBUG syncLongPositionPriceLines] called, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
  clearLongPositionPriceLines()
  if (!showLongPosition.value || !candleSeries) {
    console.log('[DEBUG syncLongPositionPriceLines] early return, showLongPosition:', showLongPosition.value, 'candleSeries:', !!candleSeries)
    return
  }
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  const cost = parseNumStr(longCostStr.value)
  console.log('[DEBUG syncLongPositionPriceLines] entry:', entry, 'stop:', stop, 'tp:', tp, 'cost:', cost)

  // 如果没有任何价格信息，直接返回
  if (!Number.isFinite(entry) && !Number.isFinite(cost)) {
    console.log('[DEBUG syncLongPositionPriceLines] no valid price, returning')
    return
  }

  const pushLine = (price, kind, partial) => {
    const pl = candleSeries.createPriceLine({
      price,
      lineWidth: 2,
      axisLabelVisible: true,
      ...partial,
    })
    longPositionPriceLines.push(pl)
    longLineByKind[kind] = pl
  }
  if (Number.isFinite(entry)) {
    pushLine(entry, 'entry', {
      color: '#3b82f6',
      lineStyle: LineStyle.Solid,
      title: '开仓',
    })
  }
  if (Number.isFinite(cost)) {
    pushLine(cost, 'cost', {
      color: '#f59e0b',
      lineStyle: LineStyle.Dashed,
      title: '成本',
    })
  }
  if (Number.isFinite(stop)) {
    pushLine(stop, 'stop', {
      color: CLR_FALL,
      lineStyle: LineStyle.Dashed,
      title: '止损',
    })
  }
  if (Number.isFinite(tp)) {
    pushLine(tp, 'takeProfit', {
      color: CLR_RISE,
      lineStyle: LineStyle.Dashed,
      title: '止盈',
    })
  }
  console.log('[DEBUG syncLongPositionPriceLines] done, created', longPositionPriceLines.length, 'lines')
}

function getLongDragPaneElement() {
  if (!chart) return chartContainerRef.value
  return chart.panes()[0]?.getHTMLElement() ?? chartContainerRef.value
}

function longPaneLocalYFromClient(clientY) {
  const el = getLongDragPaneElement()
  if (!el) return null
  const r = el.getBoundingClientRect()
  const y = clientY - r.top
  return Number.isFinite(y) ? y : null
}

function refreshLongPriceLineCursorFromCrosshair(param) {
  const paneEl = getLongDragPaneElement()
  if (!paneEl) return
  if (longPositionDragActive) {
    paneEl.style.cursor = 'grabbing'
    return
  }
  if (
    !showLongPosition.value ||
    !longLineByKind.entry ||
    param.point === undefined ||
    (param.paneIndex != null && param.paneIndex !== 0)
  ) {
    paneEl.style.cursor = ''
    return
  }
  const kind = hitTestLongPriceLineKind(param.point.y)
  paneEl.style.cursor = kind ? 'grab' : ''
}

function clearLongPriceLinePaneCursor() {
  if (longPositionDragActive) return
  const paneEl = getLongDragPaneElement()
  if (paneEl) paneEl.style.cursor = ''
}

/** @returns {'entry'|'stop'|'takeProfit'|null} */
function hitTestLongPriceLineKind(localY) {
  if (!candleSeries || !showLongPosition.value || localY == null) return null
  let best = null
  let bestDist = LONG_PRICE_LINE_HIT_PX + 1
  for (const kind of ['entry', 'stop', 'takeProfit']) {
    const line = longLineByKind[kind]
    if (!line) continue
    const p = Number(line.options().price)
    if (!Number.isFinite(p)) continue
    const ly = candleSeries.priceToCoordinate(p)
    if (ly == null) continue
    const cy = Number(ly)
    if (!Number.isFinite(cy)) continue
    const d = Math.abs(cy - localY)
    if (d <= LONG_PRICE_LINE_HIT_PX && d < bestDist) {
      bestDist = d
      best = kind
    }
  }
  return best
}

function detachLongDragWindowListeners() {
  if (!longDragWindowListenersOn) return
  longDragWindowListenersOn = false
  window.removeEventListener('pointermove', onLongDragWindowMove, true)
  window.removeEventListener('pointerup', onLongDragWindowUp, true)
  window.removeEventListener('pointercancel', onLongDragWindowUp, true)
}

function onLongDragWindowMove(e) {
  if (!longPositionDragActive || !longDragKind || !candleSeries) return
  longLastPointerClientY = e.clientY
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const raw = candleSeries.coordinateToPrice(y)
  const price = raw == null ? NaN : Number(raw)
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const line = longLineByKind[longDragKind]
  if (line) {
    try {
      line.applyOptions({ price })
    } catch {
      /* ignore */
    }
  }
  if (longDragKind === 'entry') longEntryStr.value = s
  else if (longDragKind === 'stop') longStopStr.value = s
  else longTakeProfitStr.value = s
}

function onLongDragWindowUp() {
  if (!longDragWindowListenersOn) return
  const was = longPositionDragActive
  detachLongDragWindowListeners()
  longPositionDragActive = false
  longDragKind = null
  if (was) syncLongPositionPriceLines()
  const paneEl = getLongDragPaneElement()
  if (paneEl) {
    if (
      longLastPointerClientY != null &&
      showLongPosition.value &&
      longLineByKind.entry
    ) {
      const ly = longPaneLocalYFromClient(longLastPointerClientY)
      const kind = ly != null ? hitTestLongPriceLineKind(ly) : null
      paneEl.style.cursor = kind ? 'grab' : ''
    } else {
      paneEl.style.cursor = ''
    }
  }
  longLastPointerClientY = null
  setTimeout(() => {
    longSuppressChartClick = false
  }, 0)
}

function onLongPriceLinePointerDownCapture(e) {
  if (!showLongPosition.value || !candleSeries) return
  if (e.pointerType === 'mouse' && e.button !== 0) return
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const kind = hitTestLongPriceLineKind(y)
  if (!kind) return
  longLastPointerClientY = e.clientY
  longSuppressChartClick = true
  longPositionDragActive = true
  longDragKind = kind
  const paneElGrab = getLongDragPaneElement()
  if (paneElGrab) paneElGrab.style.cursor = 'grabbing'
  try {
    e.preventDefault()
  } catch {
    /* ignore */
  }
  e.stopPropagation()
  detachLongDragWindowListeners()
  longDragWindowListenersOn = true
  window.addEventListener('pointermove', onLongDragWindowMove, true)
  window.addEventListener('pointerup', onLongDragWindowUp, true)
  window.addEventListener('pointercancel', onLongDragWindowUp, true)
}

function attachLongPriceLineDragListeners() {
  detachLongPriceLinePaneListener()
  longPaneDragEl = getLongDragPaneElement()
  if (longPaneDragEl) {
    longPaneDragEl.addEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
  }
}

function detachLongPriceLinePaneListener() {
  if (longPaneDragEl) {
    longPaneDragEl.removeEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
    longPaneDragEl = null
  }
}

const longPositionStats = computed(() => {
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  if (!Number.isFinite(entry)) return null
  const risk = Number.isFinite(stop) ? entry - stop : NaN
  const reward = Number.isFinite(tp) ? tp - entry : NaN
  const rr =
    Number.isFinite(risk) && risk > 0 && Number.isFinite(reward)
      ? reward / risk
      : NaN
  return {
    riskPts: risk,
    rewardPts: reward,
    riskRr: rr,
    riskPct: Number.isFinite(risk) && entry !== 0 ? (risk / entry) * 100 : NaN,
    rewardPct: Number.isFinite(reward) && entry !== 0 ? (reward / entry) * 100 : NaN,
  }
})

const longPositionHint = computed(() => {
  if (!showLongPosition.value) return ''
  if (!Number.isFinite(parseNumStr(longEntryStr.value))) {
    return '输入开仓价后显示线；止损低于开仓、止盈高于开仓为典型多单'
  }
  const s = longPositionStats.value
  if (!s) return ''
  const parts = []
  if (Number.isFinite(s.riskPts)) parts.push(`风险幅度 ${s.riskPts.toFixed(2)}`)
  if (Number.isFinite(s.rewardPts)) parts.push(`目标幅度 ${s.rewardPts.toFixed(2)}`)
  if (Number.isFinite(s.riskRr) && s.riskRr > 0) parts.push(`盈亏比 1 : ${s.riskRr.toFixed(2)}`)
  parts.push('单击线条可拖动改价')
  return parts.join(' · ')
})

function toggleLongPosition() {
  showLongPosition.value = !showLongPosition.value
  if (!showLongPosition.value) {
    longClickPickEnabled.value = false
    clearLongFocusedPriceField()
    clearLongPriceLinePaneCursor()
  }
  syncLongPositionPriceLines()
}

function fillLongEntryFromLatestClose() {
  const r = defaultLatestRawRow.value
  if (!r) return
  const c = parseNumStr(r.close)
  if (!Number.isFinite(c)) return
  longEntryStr.value = c.toFixed(2)
  showLongPosition.value = true
  syncLongPositionPriceLines()
}

const longClickNextLabel = computed(() => {
  const m = { entry: '开仓价', stop: '止损价', takeProfit: '止盈价' }
  return m[longClickNextField.value] || '开仓价'
})

const longFocusChartHint = computed(() => {
  const k = longFocusedPriceField.value
  if (!k) return ''
  const m = { entry: '开仓', stop: '止损', takeProfit: '止盈' }
  return `已选「${m[k] || ''}」：请在 K 线主图（非成交量）点击纵轴位置写入价格`
})

function cancelLongFocusBlurTimer() {
  if (longFocusBlurTimer != null) {
    clearTimeout(longFocusBlurTimer)
    longFocusBlurTimer = null
  }
}

function onLongPriceInputFocus(kind) {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = kind
}

function onLongPriceInputBlur() {
  cancelLongFocusBlurTimer()
  longFocusBlurTimer = setTimeout(() => {
    longFocusBlurTimer = null
    longFocusedPriceField.value = null
  }, LONG_FOCUS_BLUR_CLEAR_MS)
}

function clearLongFocusedPriceField() {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
}

function applyLongPriceFromChartByField(kind, price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  showLongPosition.value = true
  if (kind === 'entry') longEntryStr.value = s
  else if (kind === 'stop') longStopStr.value = s
  else if (kind === 'takeProfit') longTakeProfitStr.value = s
  clearLongFocusedPriceField()
  syncLongPositionPriceLines()
}

function toggleLongClickPick() {
  longClickPickEnabled.value = !longClickPickEnabled.value
  if (longClickPickEnabled.value) {
    showLongPosition.value = true
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function resetLongClickSequence() {
  longClickNextField.value = 'entry'
}

function applyLongClickPrice(price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const step = longClickNextField.value
  if (step === 'entry') {
    longEntryStr.value = s
    longClickNextField.value = 'stop'
  } else if (step === 'stop') {
    longStopStr.value = s
    longClickNextField.value = 'takeProfit'
  } else {
    longTakeProfitStr.value = s
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function chartThemeOptions(isDark) {
  const minuteLike = !DAILY_LIKE_KLT.has(activeKlt.value)
  return {
    layout: {
      background: { type: 'solid', color: isDark ? '#141414' : '#ffffff' },
      textColor: isDark ? '#cbd5e1' : '#334155',
    },
    grid: {
      vertLines: { color: isDark ? '#27272a' : '#f1f5f9' },
      horzLines: { color: isDark ? '#27272a' : '#f1f5f9' },
    },
    crosshair: { mode: 1 },
    rightPriceScale: { borderColor: isDark ? '#3f3f46' : '#e2e8f0' },
    localization: {
      locale: 'zh-CN',
      dateFormat: 'yyyy-MM-dd',
      timeFormatter: (t) => formatCrosshairTime(t),
    },
    timeScale: {
      borderColor: isDark ? '#3f3f46' : '#e2e8f0',
      timeVisible: minuteLike,
      secondsVisible: false,
      tickMarkFormatter: (t, tickMarkType) => formatTickTime(t, tickMarkType),
    },
  }
}

/** 根据当前最旧一根 K 线的 day 字段生成东财 end 参数 */
function formatEastMoneyEndFromOldest(oldestDayField, klt) {
  const s = String(oldestDayField || '').trim()
  const dailyOnly = DAILY_LIKE_KLT.has(klt)
  if (dailyOnly) {
    const dateStr = extractYmdDatePart(s.replace(/\//g, '-'))
    if (!/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) return ''
    const noon = Date.parse(`${dateStr}T12:00:00+08:00`)
    if (!Number.isFinite(noon)) return ''
    return formatYmdCompactShanghai(noon - 86400000)
  }
  const sec = eastMoneyKlineFieldToUnixSeconds(s)
  if (sec == null) return ''
  const back = barSecondsForMinuteKlt(klt)
  return formatYmdHmsCompactShanghai((sec - back) * 1000)
}

function applySeriesFromRaw() {
  if (!candleSeries || !volSeries) return
  const { candles, volumes } = toSeriesData(mergedRawRows)
  candleSeries.setData(candles)
  volSeries.setData(volumes)
  syncIndicators()
  syncLongPositionPriceLines()
}

function withProgrammaticTimeRange(fn) {
  programmaticRangeDepth++
  try {
    return fn()
  } finally {
    programmaticRangeDepth--
  }
}

/** 默认只展开最近若干根，避免 fitContent 全量挤在一起；仍可向左拖看更早 */
function applyDefaultVisibleRange() {
  if (!chart || !mergedRawRows.length) return
  const n = mergedRawRows.length
  const vis = Math.min(DEFAULT_VISIBLE_BARS, n)
  const from = Math.max(0, n - vis)
  const to = n - 1 + DEFAULT_RIGHT_LOGICAL_GAP
  chart.timeScale().setVisibleLogicalRange({ from, to })
}

function toSeriesData(rows) {
  const candles = []
  const volumes = []
  if (!rows?.length) return { candles, volumes }
  const sorted = [...rows].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    candles.push({ time: t, open: o, high: h, low: l, close: c })
    const up = c >= o
    volumes.push({
      time: t,
      value: Number.isFinite(v) ? v : 0,
      color: up ? 'rgba(239, 83, 80, 0.45)' : 'rgba(38, 166, 154, 0.45)',
    })
  }
  return { candles, volumes }
}

function clearPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function setupPoll() {
  clearPoll()
  if (props.realtimeIntervalMs > 0 && props.code) {
    pollTimer = setInterval(refreshLatestPoll, props.realtimeIntervalMs)
  }
}

function disposeChart() {
  clearPoll()
  if (loadOlderDebounceTimer) {
    clearTimeout(loadOlderDebounceTimer)
    loadOlderDebounceTimer = null
  }
  stopHistoryVisiblePoll()
  detachLongDragWindowListeners()
  detachLongPriceLinePaneListener()
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
  longPositionDragActive = false
  longDragKind = null
  longSuppressChartClick = false
  if (chart && logicalRangeHandler) {
    chart.timeScale().unsubscribeVisibleLogicalRangeChange(logicalRangeHandler)
    logicalRangeHandler = null
  }
  if (chart && visibleTimeRangeHandler) {
    chart.timeScale().unsubscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
    visibleTimeRangeHandler = null
  }
  if (chart && crosshairMoveHandler) {
    chart.unsubscribeCrosshairMove(crosshairMoveHandler)
    crosshairMoveHandler = null
  }
  if (chart && chartClickHandler) {
    chart.unsubscribeClick(chartClickHandler)
    chartClickHandler = null
  }
  hoverRawRow.value = null
  defaultLatestRawRow.value = null
  if (chart) {
    try {
      const pe = chart.panes()[0]?.getHTMLElement()
      if (pe?.style) pe.style.cursor = ''
    } catch {
      /* ignore */
    }
    clearLongPositionPriceLines()
    chart.remove()
    chart = null
    candleSeries = null
    volSeries = null
  }
  ind.ma5 = ind.ma10 = ind.ma20 = ind.ma60 = null
  ind.bollU = ind.bollM = ind.bollL = null
  ind.obv = null
  ind.macdHist = ind.macdDif = ind.macdDea = null
  ind.kdjK = ind.kdjD = ind.kdjJ = null
  ind.rsi = null
  ind.atr = null
  ind.vwap = null
  ind.mfi = null
  ind.kama = null
  ind.keltnerU = ind.keltnerM = ind.keltnerL = null
  ind.supertrend = null
  ind.ema12 = ind.ema21 = null
  ind.ichTenkan = ind.ichKijun = ind.ichSpanA = ind.ichSpanB = ind.ichChikou = null
  ind.cci = null
  ind.ttmHist = ind.ttmDots = null
  ind.sar = null
  ind.donchianU = ind.donchianM = ind.donchianL = null
  ind.adx = ind.adxDiP = ind.adxDiM = null
  ind.williamsR = null
  ind.stochRsi = null
  ind.stochRsiD = null
  ind.cmf = null
  ind.aroonUp = ind.aroonDown = null
  ind.cmo = null
  ind.forceIndex = null
  ind.avgAmp5 = null
  ind.avgAmp10 = null
  ind.avgAmp20 = null
  ind.pivotPP = ind.pivotS1 = ind.pivotS2 = ind.pivotR1 = ind.pivotR2 = null
  ind.dema = null
  ind.zigzag = null
  ind.satsLine = null
  ind.satsUpper = null
  ind.satsLower = null
  ind.smcSwingHigh = null
  ind.smcSwingLow = null
  ind.smcIntHigh = null
  ind.smcIntLow = null
  ind.smcBos = null
  ind.smcChoch = null
  ind.smcSwBos = null
  ind.smcSwChoch = null
  ind.smcFvgTop = null
  ind.smcFvgBot = null
  ind.smcObTop = null
  ind.smcObBot = null
}

function scheduleLoadOlderDebounced() {
  if (loadOlderDebounceTimer) clearTimeout(loadOlderDebounceTimer)
  loadOlderDebounceTimer = setTimeout(() => {
    loadOlderDebounceTimer = null
    loadOlderHistory()
  }, 280)
}

/** 根据当前可见逻辑区间判断是否需要加载更早 K 线（与 subscribe 共用 + 轮询兜底） */
function tryScheduleLoadOlderFromVisibleRange(range) {
  if (!chart || !candleSeries) return
  if (programmaticRangeDepth > 0 || loadingHistory.value || !hasMoreOlder.value) return
  const lr = range ?? chart?.timeScale().getVisibleLogicalRange()
  if (!lr) return
  const info = candleSeries.barsInLogicalRange(lr)
  if (!info || typeof info.barsBefore !== 'number') return
  if (info.barsBefore < BARS_BEFORE_LOAD_MORE) {
    scheduleLoadOlderDebounced()
  }
}

function onVisibleLogicalRangeChanged(range) {
  if (!range || !candleSeries) return
  if (programmaticRangeDepth > 0) return
  tryScheduleLoadOlderFromVisibleRange(range)
}

/** 分钟线等：部分 WebView 下逻辑区间回调稀疏，时间区间在拖动时更可靠 */
function onVisibleTimeRangeChanged() {
  if (programmaticRangeDepth > 0 || !candleSeries) return
  tryScheduleLoadOlderFromVisibleRange(null)
}

function startHistoryVisiblePoll() {
  stopHistoryVisiblePoll()
  historyVisiblePollTimer = setInterval(() => {
    tryScheduleLoadOlderFromVisibleRange(null)
  }, 400)
}

function stopHistoryVisiblePoll() {
  if (historyVisiblePollTimer) {
    clearInterval(historyVisiblePollTimer)
    historyVisiblePollTimer = null
  }
}

async function loadOlderHistory() {
  if (
    loadingHistory.value ||
    !hasMoreOlder.value ||
    !mergedRawRows.length ||
    !props.code ||
    !chart ||
    !candleSeries
  ) {
    return
  }
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  const oldest = mergedRawRows[0]
  const end = formatEastMoneyEndFromOldest(oldest.day, kltSnap)
  if (!end) {
    hasMoreOlder.value = false
    return
  }
  loadingHistory.value = true
  const logical = chart.timeScale().getVisibleLogicalRange()
  const beforeCount = mergedRawRows.length
  try {
    const result = await GetStockKLinePageWithFallback(
      codeSnap,
      props.stockName || '',
      kltSnap,
      HISTORY_PAGE_SIZE,
      end,
    )
    if (kltSnap !== activeKlt.value || codeSnap !== props.code) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const inc = Array.isArray(raw) ? raw : []
    if (!inc.length) {
      hasMoreOlder.value = false
      lastOlderHistoryEndTried = ''
      return
    }
    const merged = mergeKlineRows(mergedRawRows, inc)
    const added = merged.length - beforeCount
    if (added <= 0) {
      if (end === lastOlderHistoryEndTried) {
        hasMoreOlder.value = false
      } else {
        lastOlderHistoryEndTried = end
      }
      return
    }
    lastOlderHistoryEndTried = ''
    mergedRawRows = merged
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      if (logical) {
        chart.timeScale().setVisibleLogicalRange({
          from: logical.from + added,
          to: logical.to + added,
        })
      }
    })
  } catch {
    /* 网络/桥接异常：保留 hasMoreOlder，用户可继续拖动重试 */
  } finally {
    loadingHistory.value = false
  }
}

async function refreshLatestPoll() {
  if (!props.code || !candleSeries) return
  const kltSnap = activeKlt.value
  const codeSnap = props.code
  try {
    const meta = INTERVALS.find((x) => x.klt === kltSnap) || INTERVALS[0]
    const result = await GetStockKLineWithFallback(
      codeSnap,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    if (codeSnap !== props.code || activeKlt.value !== kltSnap) return
    const src = result?.source || ''
    if (src) activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    if (!list.length) return
    mergedRawRows = mergeRefreshWithLatest(mergedRawRows, list)
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    withProgrammaticTimeRange(() => applySeriesFromRaw())
  } catch {
    /* 静默，避免打断看盘 */
  }
}

function ensureChart() {
  if (!chartContainerRef.value || chart) return
  chart = createChart(chartContainerRef.value, {
    autoSize: true,
    height: props.chartHeight,
    ...chartThemeOptions(props.darkTheme),
  })
  candleSeries = chart.addSeries(CandlestickSeries, {
    upColor: '#ef5350',
    downColor: '#26a69a',
    borderVisible: false,
    wickUpColor: '#ef5350',
    wickDownColor: '#26a69a',
  })
  volSeries = chart.addSeries(
    HistogramSeries,
    {
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
      color: 'rgba(38, 166, 154, 0.35)',
    },
    0,
  )
  chart.priceScale('vol').applyOptions({
    scaleMargins: { top: 0.82, bottom: 0 },
  })
  candleSeries.priceScale().applyOptions({
    scaleMargins: { top: 0.06, bottom: 0.22 },
  })
  logicalRangeHandler = onVisibleLogicalRangeChanged
  chart.timeScale().subscribeVisibleLogicalRangeChange(logicalRangeHandler)
  visibleTimeRangeHandler = onVisibleTimeRangeChanged
  chart.timeScale().subscribeVisibleTimeRangeChange(visibleTimeRangeHandler)
  startHistoryVisiblePoll()
  crosshairMoveHandler = (param) => {
    if (param.point === undefined) {
      hoverRawRow.value = null
      clearLongPriceLinePaneCursor()
      if (showChip.value) updateChipFromHover()
      return
    }
    refreshLongPriceLineCursorFromCrosshair(param)
    if (param.time === undefined) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    const bar = param.seriesData.get(candleSeries)
    if (!bar) {
      hoverRawRow.value = null
      if (showChip.value) updateChipFromHover()
      return
    }
    hoverRawRow.value = findRawRowByChartTime(param.time)
    if (showChip.value) updateChipFromHover()
  }
  chart.subscribeCrosshairMove(crosshairMoveHandler)
  chartClickHandler = (param) => {
    if (longSuppressChartClick) return
    if (!candleSeries || !param.point) return
    if (param.paneIndex != null && param.paneIndex !== 0) return
    if (param.hoveredSeries && param.hoveredSeries !== candleSeries) return
    const y = param.point.y
    const raw = candleSeries.coordinateToPrice(y)
    const price = raw == null ? NaN : Number(raw)
    if (!Number.isFinite(price)) return
    const focusKind = longFocusedPriceField.value
    if (focusKind === 'entry' || focusKind === 'stop' || focusKind === 'takeProfit') {
      applyLongPriceFromChartByField(focusKind, price)
      return
    }
    if (!longClickPickEnabled.value || !showLongPosition.value) return
    applyLongClickPrice(price)
  }
  chart.subscribeClick(chartClickHandler)
  syncLongPositionPriceLines()
  nextTick(() => attachLongPriceLineDragListeners())
}

async function loadData() {
  if (!props.code) {
    errorText.value = '未设置股票代码'
    mergedRawRows = []
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    hasMoreOlder.value = true
    lastOlderHistoryEndTried = ''
    candleSeries?.setData([])
    volSeries?.setData([])
    syncLongPositionPriceLines()
    chipItems.value = []
    chipMeta.value = { avgCost: 0, profitRatio: 0, current: 0, hoverDate: '', minPrice: 0, maxPrice: 0 }
    return
  }
  loading.value = true
  errorText.value = ''
  mergedRawRows = []
  mergedRawRowsVersion.value++
  syncDefaultLatestPanelRow()
  hasMoreOlder.value = true
  lastOlderHistoryEndTried = ''
  try {
    const meta = INTERVALS.find((x) => x.klt === activeKlt.value) || INTERVALS[0]
    const result = await GetStockKLineWithFallback(
      props.code,
      props.stockName || '',
      meta.klt,
      meta.limit,
    )
    const src = result?.source || ''
    activeDataSource.value = src
    const raw = result?.data
    const list = Array.isArray(raw) ? raw : []
    ensureChart()
    mergedRawRows = mergeKlineRows([], list)
    mergedRawRowsVersion.value++
    syncDefaultLatestPanelRow()
    const { candles } = toSeriesData(mergedRawRows)
    if (!candles.length) {
      errorText.value =
        '暂无 K 线数据（如 600519.SH、000001.SZ、00700.HK、AAPL.US）'
      candleSeries?.setData([])
      volSeries?.setData([])
      syncIndicators()
      syncLongPositionPriceLines()
      return
    }
    withProgrammaticTimeRange(() => {
      applySeriesFromRaw()
      applyDefaultVisibleRange()
    })

    if (showChip.value) {
      updateChipFromHover()
    } else {
      chipItems.value = []
    }
  } catch (e) {
    errorText.value = String(e?.message || e)
  } finally {
    loading.value = false
  }
}

function onSelectKlt(klt) {
  activeKlt.value = klt
}

const toggleMA = makeToggle(showMA, syncIndicators)
const toggleBOLL = makeToggle(showBOLL, syncIndicators)
const toggleOBV = makeToggle(showOBV, syncIndicators)
const toggleMACD = makeToggle(showMACD, syncIndicators)
const toggleKDJ = makeToggle(showKDJ, syncIndicators)
const toggleRSI = makeToggle(showRSI, syncIndicators)
const toggleATR = makeToggle(showATR, syncIndicators)
const toggleVWAP = makeToggle(showVWAP, syncIndicators)
const toggleMFI = makeToggle(showMFI, syncIndicators)
const toggleKAMA = makeToggle(showKAMA, syncIndicators)
const toggleKeltner = makeToggle(showKeltner, syncIndicators)
const toggleSupertrend = makeToggle(showSupertrend, syncIndicators)
const toggleEMA = makeToggle(showEMA, syncIndicators)
const toggleIchimoku = makeToggle(showIchimoku, syncIndicators)
const toggleCCI = makeToggle(showCCI, syncIndicators)
const toggleTTMSqueeze = makeToggle(showTTMSqueeze, syncIndicators)
const toggleSAR = makeToggle(showSAR, syncIndicators)
const toggleDonchian = makeToggle(showDonchian, syncIndicators)
const toggleADX = makeToggle(showADX, syncIndicators)
const toggleWilliamsR = makeToggle(showWilliamsR, syncIndicators)
const toggleStochRSI = makeToggle(showStochRSI, syncIndicators)
const toggleCMF = makeToggle(showCMF, syncIndicators)
const toggleAroon = makeToggle(showAroon, syncIndicators)
const toggleCMO = makeToggle(showCMO, syncIndicators)
const toggleForceIndex = makeToggle(showForceIndex, syncIndicators)
const togglePivot = makeToggle(showPivot, syncIndicators)
const toggleDEMA = makeToggle(showDEMA, syncIndicators)
const toggleZigZag = makeToggle(showZigZag, syncIndicators)
const toggleSATS = makeToggle(showSATS, syncIndicators)
const toggleAvgAmp = makeToggle(showAvgAmp, syncIndicators)
const toggleAlligator = makeToggle(showAlligator, syncIndicators)
const toggleAO = makeToggle(showAO, syncIndicators)
const toggleHullMA = makeToggle(showHullMA, syncIndicators)
const toggleAD = makeToggle(showAD, syncIndicators)
const toggleTRIX = makeToggle(showTRIX, syncIndicators)
const toggleTRIXSlope = makeToggle(showTRIXSlope, syncIndicators)
const toggleROC = makeToggle(showROC, syncIndicators)
const toggleFractal = makeToggle(showFractal, syncIndicators)
const toggleCHOP = makeToggle(showCHOP, syncIndicators)
const toggleElderRay = makeToggle(showElderRay, syncIndicators)
const toggleChaikinOsc = makeToggle(showChaikinOsc, syncIndicators)
const toggleVWAPBands = makeToggle(showVWAPBands, syncIndicators)
const toggleMassIndex = makeToggle(showMassIndex, syncIndicators)
const toggleUlcerIndex = makeToggle(showUlcerIndex, syncIndicators)
const toggleCoppock = makeToggle(showCoppock, syncIndicators)
const toggleTEMA = makeToggle(showTEMA, syncIndicators)
const toggleSMI = makeToggle(showSMI, syncIndicators)
const toggleSignalRatio = makeToggle(showSignalRatio, syncIndicators)
const toggleSMC = makeToggle(showSMC, syncIndicators)
let chipUpdateTimer = null

function toggleChip() {
  showChip.value = !showChip.value
  if (showChip.value) {
    updateChipFromHover()
  }
}

function updateChipFromHover() {
  if (!showChip.value || !mergedRawRows.length) {
    chipItems.value = []
    return
  }
  if (chipUpdateTimer) return
  chipUpdateTimer = setTimeout(() => {
    chipUpdateTimer = null
    doUpdateChip()
  }, 30)
}

function doUpdateChip() {
  if (!showChip.value || !mergedRawRows.length) {
    chipItems.value = []
    return
  }
  const r = hoverRawRow.value ?? defaultLatestRawRow.value
  let rows = mergedRawRows
  if (r) {
    const sk = sortKey(r.day)
    let hi = mergedRawRows.length
    for (let i = 0; i < mergedRawRows.length; i++) {
      if (sortKey(mergedRawRows[i].day) > sk) { hi = i; break }
    }
    rows = hi > 0 ? mergedRawRows.slice(0, hi) : mergedRawRows
  }
  if (!rows.length) {
    chipItems.value = []
    return
  }
  const result = calcChipDistribution(rows, chipBins.value)
  chipItems.value = result.items
  chipMeta.value = {
    avgCost: result.avgCost,
    profitRatio: result.profitRatio,
    current: result.current,
    hoverDate: r ? extractYmdDatePart(String(r.day || '').replace(/\//g, '-')) : '',
    minPrice: result.minPrice || 0,
    maxPrice: result.maxPrice || 0,
  }
  nextTick(() => drawChipCanvas())
}

function pricePropToInputStr(v) {
  if (v == null) return ''
  return String(v).trim()
}

watch(
  () => [props.longEntryPrice, props.longStopLossPrice, props.longTakeProfitPrice, props.costPrice],
  ([e, s, t, c]) => {
    console.log('[DEBUG props watch] triggered, e:', e, 's:', s, 't:', t, 'c:', c, 'candleSeries:', !!candleSeries)
    let needReleaseSuppress = false
    if (e !== undefined) {
      const se = pricePropToInputStr(e)
      console.log('[DEBUG props watch] entry: propVal=', e, 'converted=', se, 'current longEntryStr=', longEntryStr.value)
      if (se !== longEntryStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longEntryStr.value = se
      }
    }
    if (s !== undefined) {
      const ss = pricePropToInputStr(s)
      if (ss !== longStopStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longStopStr.value = ss
      }
    }
    if (t !== undefined) {
      const st = pricePropToInputStr(t)
      if (st !== longTakeProfitStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longTakeProfitStr.value = st
      }
    }
    if (c !== undefined) {
      const sc = pricePropToInputStr(c)
      if (sc !== longCostStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longCostStr.value = sc
      }
    }
    if (needReleaseSuppress) {
      nextTick(() => {
        suppressLongPriceEmit.value = false
      })
    }
    const hasPrice = Number.isFinite(parseNumStr(longEntryStr.value)) || Number.isFinite(parseNumStr(longStopStr.value)) || Number.isFinite(parseNumStr(longTakeProfitStr.value)) || Number.isFinite(parseNumStr(longCostStr.value))
    console.log('[DEBUG props watch] hasPrice:', hasPrice, 'showLongPosition:', showLongPosition.value)
    if (hasPrice) {
      showLongPosition.value = true
      nextTick(() => {
        console.log('[DEBUG props watch] nextTick: calling syncLongPositionPriceLines, candleSeries:', !!candleSeries)
        syncLongPositionPriceLines()
      })
    }
  },
  { immediate: true },
)

watch(longEntryStr, (v) => {
  console.log('[DEBUG longEntryStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longEntryPrice:', props.longEntryPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longEntryPrice', v)
})
watch(longStopStr, (v) => {
  console.log('[DEBUG longStopStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longStopLossPrice:', props.longStopLossPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longStopLossPrice', v)
})
watch(longTakeProfitStr, (v) => {
  console.log('[DEBUG longTakeProfitStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.longTakeProfitPrice:', props.longTakeProfitPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:longTakeProfitPrice', v)
})
watch(longCostStr, (v) => {
  console.log('[DEBUG longCostStr watch] v:', v, 'suppress:', suppressLongPriceEmit.value, 'props.costPrice:', props.costPrice)
  if (suppressLongPriceEmit.value) return
  emit('update:costPrice', v)
})

onMounted(() => {
  console.log('[DEBUG onMounted] starting')
  nextTick(() => {
    console.log('[DEBUG onMounted] nextTick callback')
    console.log('[DEBUG onMounted] current longEntryStr:', longEntryStr.value, 'showLongPosition:', showLongPosition.value)
    ensureChart()
    console.log('[DEBUG onMounted] after ensureChart, candleSeries:', !!candleSeries)
    loadData()
    console.log('[DEBUG onMounted] after loadData call')
    setupPoll()
  })
})

onBeforeUnmount(() => {
  disposeChart()
})

watch(
  () => props.code,
  () => {
    hoverRawRow.value = null
    loadData()
    setupPoll()
  },
)

watch(activeKlt, () => {
  hoverRawRow.value = null
  chart?.applyOptions(chartThemeOptions(props.darkTheme))
  loadData()
  setupPoll()
})

watch(
  () => props.darkTheme,
  (d) => {
    chart?.applyOptions(chartThemeOptions(d))
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.chartHeight,
  (h) => {
    chart?.applyOptions({ height: h })
    if (showChip.value) nextTick(() => drawChipCanvas())
  },
)

watch(
  () => props.realtimeIntervalMs,
  () => setupPoll(),
)

watch(
  [showLongPosition, longEntryStr, longStopStr, longTakeProfitStr],
  () => {
    console.log('[DEBUG priceLines watch] triggered, showLongPosition:', showLongPosition.value, 'longEntryStr:', longEntryStr.value)
    if (longPositionDragActive) return
    syncLongPositionPriceLines()
  },
)

watch(showLongPosition, (newVal) => {
  console.log('[DEBUG showLongPosition watch] changed to:', newVal)
  if (newVal && candleSeries) {
    nextTick(() => syncLongPositionPriceLines())
  }
})

</script>

<template>
  <div class="lw-kline-root" :class="{ 'lw-kline--dark': darkTheme }">
    <div class="lw-kline-body">
      <div class="lw-kline-sidebar">
        <div class="lw-kline-sidebar__inner">
          <NFlex vertical :size="6">
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(239,68,68,0.08); border-radius: 4px; border-left: 3px solid #ef4444; color: #ef4444">📈趋势</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMA ? 'primary' : 'default'" :secondary="!showMA" @click="toggleMA">MA</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ma }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showEMA ? 'primary' : 'default'" :secondary="!showEMA" @click="toggleEMA">EMA</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ema }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKAMA ? 'primary' : 'default'" :secondary="!showKAMA" @click="toggleKAMA">KAMA</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.kama }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSupertrend ? 'primary' : 'default'" :secondary="!showSupertrend" @click="toggleSupertrend">STrend</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.supertrend }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSAR ? 'primary' : 'default'" :secondary="!showSAR" @click="toggleSAR">SAR</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.sar }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showIchimoku ? 'primary' : 'default'" :secondary="!showIchimoku" @click="toggleIchimoku">Ichi</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ichimoku }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAroon ? 'primary' : 'default'" :secondary="!showAroon" @click="toggleAroon">Aroon</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.aroon }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showDEMA ? 'primary' : 'default'" :secondary="!showDEMA" @click="toggleDEMA">DEMA</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.dema }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSATS ? 'primary' : 'default'" :secondary="!showSATS" @click="toggleSATS">SATS</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.sats }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAlligator ? 'primary' : 'default'" :secondary="!showAlligator" @click="toggleAlligator">Gator</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.alligator }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showHullMA ? 'primary' : 'default'" :secondary="!showHullMA" @click="toggleHullMA">Hull</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.hullMA }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showTEMA ? 'primary' : 'default'" :secondary="!showTEMA" @click="toggleTEMA">TEMA</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.tema }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(245,158,11,0.08); border-radius: 4px; border-left: 3px solid #f59e0b; color: #d97706">🎢波动</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showBOLL ? 'primary' : 'default'" :secondary="!showBOLL" @click="toggleBOLL">BOLL</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.boll }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKeltner ? 'primary' : 'default'" :secondary="!showKeltner" @click="toggleKeltner">Kelt</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.keltner }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showDonchian ? 'primary' : 'default'" :secondary="!showDonchian" @click="toggleDonchian">Donch</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.donchian }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showATR ? 'primary' : 'default'" :secondary="!showATR" @click="toggleATR">ATR</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.atr }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAvgAmp ? 'primary' : 'default'" :secondary="!showAvgAmp" @click="toggleAvgAmp">均幅</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.avgAmp }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showTTMSqueeze ? 'primary' : 'default'" :secondary="!showTTMSqueeze" @click="toggleTTMSqueeze">TTM</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ttmSqueeze }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showZigZag ? 'primary' : 'default'" :secondary="!showZigZag" @click="toggleZigZag">ZigZag</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.zigzag }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showFractal ? 'primary' : 'default'" :secondary="!showFractal" @click="toggleFractal">Fractal</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.fractal }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMassIndex ? 'primary' : 'default'" :secondary="!showMassIndex" @click="toggleMassIndex">Mass</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.massIndex }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSMC ? 'primary' : 'default'" :secondary="!showSMC" @click="toggleSMC">SMC</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.smc }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(59,130,246,0.08); border-radius: 4px; border-left: 3px solid #3b82f6; color: #2563eb">💫动量</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMACD ? 'primary' : 'default'" :secondary="!showMACD" @click="toggleMACD">MACD</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.macd }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showKDJ ? 'primary' : 'default'" :secondary="!showKDJ" @click="toggleKDJ">KDJ</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.kdj }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showRSI ? 'primary' : 'default'" :secondary="!showRSI" @click="toggleRSI">RSI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.rsi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCCI ? 'primary' : 'default'" :secondary="!showCCI" @click="toggleCCI">CCI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.cci }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showWilliamsR ? 'primary' : 'default'" :secondary="!showWilliamsR" @click="toggleWilliamsR">W%R</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.williamsR }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showStochRSI ? 'primary' : 'default'" :secondary="!showStochRSI" @click="toggleStochRSI">SRSI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.stochRsi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCMO ? 'primary' : 'default'" :secondary="!showCMO" @click="toggleCMO">CMO</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.cmo }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAO ? 'primary' : 'default'" :secondary="!showAO" @click="toggleAO">AO</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ao }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showTRIX ? 'primary' : 'default'" :secondary="!showTRIX" @click="toggleTRIX">TRIX</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.trix }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showTRIXSlope ? 'primary' : 'default'" :secondary="!showTRIXSlope" @click="toggleTRIXSlope">TRIX斜率</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.trixSlope }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showROC ? 'primary' : 'default'" :secondary="!showROC" @click="toggleROC">ROC</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.roc }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSMI ? 'primary' : 'default'" :secondary="!showSMI" @click="toggleSMI">SMI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.smi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCoppock ? 'primary' : 'default'" :secondary="!showCoppock" @click="toggleCoppock">Coppck</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.coppock }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(16,185,129,0.08); border-radius: 4px; border-left: 3px solid #10b981; color: #059669">📊量价</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showOBV ? 'primary' : 'default'" :secondary="!showOBV" @click="toggleOBV">OBV</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.obv }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showVWAP ? 'primary' : 'default'" :secondary="!showVWAP" @click="toggleVWAP">VWAP</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.vwap }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showMFI ? 'primary' : 'default'" :secondary="!showMFI" @click="toggleMFI">MFI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.mfi }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCMF ? 'primary' : 'default'" :secondary="!showCMF" @click="toggleCMF">CMF</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.cmf }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showForceIndex ? 'primary' : 'default'" :secondary="!showForceIndex" @click="toggleForceIndex">FI</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.forceIndex }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showAD ? 'primary' : 'default'" :secondary="!showAD" @click="toggleAD">A/D</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ad }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showChaikinOsc ? 'primary' : 'default'" :secondary="!showChaikinOsc" @click="toggleChaikinOsc">ChkOsc</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.chaikinOsc }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showVWAPBands ? 'primary' : 'default'" :secondary="!showVWAPBands" @click="toggleVWAPBands">VWBnd</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.vwapBands }}</span>
                </NTooltip>
              </NFlex>
            </div>
            <div class="lw-kline-sidebar__section">
              <NText depth="3" style="font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(139,92,246,0.08); border-radius: 4px; border-left: 3px solid #8b5cf6; color: #7c3aed">📏强度</NText>
              <NFlex :size="4" wrap style="row-gap: 4px">
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showADX ? 'primary' : 'default'" :secondary="!showADX" @click="toggleADX">ADX</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.adx }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showPivot ? 'primary' : 'default'" :secondary="!showPivot" @click="togglePivot">Pivot</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.pivot }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showCHOP ? 'primary' : 'default'" :secondary="!showCHOP" @click="toggleCHOP">CHOP</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.chop }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showElderRay ? 'primary' : 'default'" :secondary="!showElderRay" @click="toggleElderRay">Elder</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.elderRay }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showUlcerIndex ? 'primary' : 'default'" :secondary="!showUlcerIndex" @click="toggleUlcerIndex">Ulcer</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.ulcerIndex }}</span>
                </NTooltip>
                <NTooltip :delay="500" placement="right-start">
                  <template #trigger>
                    <NButton size="tiny" :type="showSignalRatio ? 'primary' : 'default'" :secondary="!showSignalRatio" @click="toggleSignalRatio">信号比</NButton>
                  </template>
                  <span style="white-space: pre-line; text-align: left">{{ indicatorTips.signalRatio }}</span>
                </NTooltip>
                <NButton
                  v-if="SHOW_CHIP_TOOLBAR_BUTTON"
                  size="tiny"
                  :type="showChip ? 'primary' : 'default'"
                  :secondary="!showChip"
                  @click="toggleChip"
                >
                  筹码
                </NButton>
              </NFlex>
            </div>
          </NFlex>
        </div>
      </div>
      <div class="lw-kline-main">
        <NFlex :size="6" wrap style="row-gap: 4px; align-items: center">
          <NText depth="3" style="font-size: 12px; margin-right: 2px">周期</NText>
          <NButton
            v-for="it in INTERVALS"
            :key="it.klt"
            size="tiny"
            :type="activeKlt === it.klt ? 'primary' : 'default'"
            :secondary="activeKlt !== it.klt"
            @click="onSelectKlt(it.klt)"
          >
            {{ it.label }}
          </NButton>
          <span style="width: 12px" />
          <NText depth="3" style="font-size: 12px; margin-right: 2px">多单</NText>
          <NButton
            size="tiny"
            :type="showLongPosition ? 'primary' : 'default'"
            :secondary="!showLongPosition"
            @click="toggleLongPosition"
          >
            价位线
          </NButton>
          <NInput
            v-model:value="longEntryStr"
            size="tiny"
            placeholder="开仓"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('entry')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longStopStr"
            size="tiny"
            placeholder="止损"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('stop')"
            @blur="onLongPriceInputBlur"
          />
          <NInput
            v-model:value="longTakeProfitStr"
            size="tiny"
            placeholder="止盈"
            style="width: 80px"
            clearable
            @focus="onLongPriceInputFocus('takeProfit')"
            @blur="onLongPriceInputBlur"
          />
          <NButton size="tiny" secondary @click="fillLongEntryFromLatestClose">
            最新收盘
          </NButton>
          <NButton
            size="tiny"
            :type="longClickPickEnabled ? 'primary' : 'default'"
            :secondary="!longClickPickEnabled"
            @click="toggleLongClickPick"
          >
            设置价位线
          </NButton>
          <NButton
            v-if="longClickPickEnabled"
            size="tiny"
            quaternary
            @click="resetLongClickSequence"
          >
            重置
          </NButton>
          <NText
            v-if="longFocusChartHint"
            depth="3"
            class="lw-kline-longpos-focus-hint"
          >
            {{ longFocusChartHint }}
          </NText>
          <NText
            v-if="longClickPickEnabled && showLongPosition"
            depth="3"
            class="lw-kline-longpos-click-hint"
          >
            点击K线设置{{ longClickNextLabel }}
          </NText>
          <NText v-if="longPositionHint" depth="3" class="lw-kline-longpos-hint">
            {{ longPositionHint }}
          </NText>
        </NFlex>
        <div
          class="lw-kline-crosshair-strip"
          :class="{ 'lw-kline-crosshair-strip--dark': darkTheme }"
        >
          <template v-if="crosshairPanel">
            <div class="lw-kline-crosshair-strip__head">
              <span class="lw-kline-crosshair-strip__date">{{ crosshairPanel.title }}</span>
            </div>
            <div class="lw-kline-crosshair-strip__grid">
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">开盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.open
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">收盘</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cOpenClose }">{{
                  crosshairPanel.close
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最高</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cHigh }">{{
                  crosshairPanel.high
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">最低</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cLow }">{{
                  crosshairPanel.low
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changePercent
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">涨跌额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.changeValue
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交量</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.volume
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">成交额</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amount
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">振幅</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.amplitude
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅5</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp5
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅10</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp10
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">均幅20</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.avgAmp20
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">换手率</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cNeu }">{{
                  crosshairPanel.turnoverRate
                }}</span>
              </span>
              <span class="lw-kline-kv">
                <span class="lw-kline-crosshair-strip__k">量比</span>
                <span class="lw-kline-crosshair-strip__v" :style="{ color: crosshairPanel.cChg }">{{
                  crosshairPanel.volumeRatio
                }}</span>
              </span>
            </div>
          </template>
          <NText v-else depth="3" style="font-size: 11px; line-height: 1.5">
            {{ loading ? '加载中…' : '暂无 K 线数据' }}
          </NText>
        </div>
        <div
          v-if="indicatorSignalSummary"
          class="lw-kline-signal-summary"
          :class="{ 'lw-kline-signal-summary--dark': darkTheme }"
        >
          <div class="lw-kline-signal-summary__head">
            <span class="lw-kline-signal-summary__title">指标信号汇总</span>
            <span class="lw-kline-signal-summary__total">共 {{ indicatorSignalSummary.total }} 项</span>
          </div>
          <div class="lw-kline-signal-summary__bar">
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--bullish" :style="{ width: indicatorSignalSummary.bullishPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--bearish" :style="{ width: indicatorSignalSummary.bearishPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--oscillating" :style="{ width: indicatorSignalSummary.oscillatingPct + '%' }"></div>
            <div class="lw-kline-signal-summary__bar-seg lw-kline-signal-summary__bar-seg--neutral" :style="{ width: indicatorSignalSummary.neutralPct + '%' }"></div>
          </div>
          <div class="lw-kline-signal-summary__legend">
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--bullish"></span>
              看多 {{ indicatorSignalSummary.bullish }} ({{ indicatorSignalSummary.bullishPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--bearish"></span>
              看空 {{ indicatorSignalSummary.bearish }} ({{ indicatorSignalSummary.bearishPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--oscillating"></span>
              震荡 {{ indicatorSignalSummary.oscillating }} ({{ indicatorSignalSummary.oscillatingPct }}%)
            </span>
            <span class="lw-kline-signal-summary__legend-item">
              <span class="lw-kline-signal-summary__dot lw-kline-signal-summary__dot--neutral"></span>
              中性 {{ indicatorSignalSummary.neutral }} ({{ indicatorSignalSummary.neutralPct }}%)
            </span>
          </div>
          <div class="lw-kline-signal-summary__tags">
            <span
              v-for="s in indicatorSignalSummary.signals"
              :key="s.name"
              class="lw-kline-signal-summary__tag"
              :class="'lw-kline-signal-summary__tag--' + s.signal"
            >{{ s.name }}</span>
          </div>
        </div>
        <NText v-if="errorText" type="error" style="font-size: 12px">{{ errorText }}</NText>
        <div class="lw-kline-chart-wrap">
          <div
            ref="chartContainerRef"
            class="lw-kline-chart"
            :style="{ height: chartHeight-110 + 'px', minHeight: chartHeight-110 + 'px' }"
          />
          <div
            v-if="showChip"
            class="lw-chip"
            :class="{ 'lw-chip--dark': darkTheme }"
            :style="{ height: chartHeight-110 + 'px', minHeight: chartHeight-110 + 'px' }"
          >
            <div class="lw-chip__head">
              <span class="lw-chip__title">筹码分布</span>
              <span v-if="chipMeta.hoverDate" class="lw-chip__meta">
                {{ chipMeta.hoverDate }}
              </span>
              <span v-if="chipItems.length" class="lw-chip__meta">
                均成本 {{ chipMeta.avgCost.toFixed(2) }} · 获利
                {{ (chipMeta.profitRatio * 100).toFixed(1) }}%
              </span>
            </div>
            <div v-if="!chipItems.length" class="lw-chip__empty">
              {{ mergedRawRows.length ? '移动鼠标到K线查看' : '暂无K线数据' }}
            </div>
            <canvas
              v-show="chipItems.length"
              ref="chipCanvasRef"
              class="lw-chip__canvas"
            />
          </div>
        </div>
        <NFlex align="center" :size="8" class="lw-kline-hint-row">
          <NText depth="3" class="lw-kline-hint-text">
            {{ stockName || code }} ·
            {{ 
              realtimeIntervalMs > 0
                ? `每 ${Math.round(realtimeIntervalMs / 1000)} 秒刷新`
                : '切换周期后加载'
            }}
            · 按住拖动查看左侧历史时会自动加载更早 K 线
            <span v-if="activeDataSource" class="lw-kline-source-tag" :class="{ 'lw-kline-source-tag--fallback': activeDataSource !== 'eastmoney' && activeDataSource !== 'tdx-mac' && activeDataSource !== 'tdx-mac-ex' }">
              {{ activeDataSource === 'eastmoney' ? '东方财富' : activeDataSource === 'tdx-mac' ? '通达信MAC' : activeDataSource === 'tdx-mac-ex' ? '通达信MAC扩展' : activeDataSource === 'sina' ? '新浪财经' : activeDataSource === 'tencent' ? '腾讯财经' : activeDataSource === 'tdx' ? '通达信' : activeDataSource }}
            </span>
          </NText>
          <NSpin v-if="loading || loadingHistory" size="small" />
        </NFlex>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lw-kline-root {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  --wails-draggable: no-drag;
}
.lw-kline-body {
  display: flex;
  width: 100%;
  gap: 8px;
  align-items: stretch;
}
.lw-kline-sidebar {
  flex: 0 0 auto;
  width: 140px;
  min-width: 120px;
}
.lw-kline--dark .lw-kline-sidebar {
  border-color: #3f3f46;
}
.lw-kline-sidebar__inner {
  min-width: 0;
  position: sticky;
  top: 0;
}
.lw-kline-sidebar__section {
  margin-bottom: 6px;
}
.lw-kline-main {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lw-kline-hint-row {
  min-width: 0;
  max-width: 100%;
}
.lw-kline-hint-text {
  font-size: 12px;
  min-width: 0;
  flex: 1 1 auto;
  overflow-wrap: anywhere;
  word-break: break-word;
}
.lw-kline-longpos-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 200px;
  overflow-wrap: anywhere;
}
.lw-kline-longpos-click-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #0ea5e9;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-click-hint {
  color: #38bdf8;
}
.lw-kline-longpos-focus-hint {
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
  flex: 1 1 220px;
  color: #d97706;
  overflow-wrap: anywhere;
}
.lw-kline--dark .lw-kline-longpos-focus-hint {
  color: #fbbf24;
}
.lw-kline-crosshair-strip {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  overflow-x: auto;
  overflow-y: hidden;
}
.lw-kline-crosshair-strip--dark {
  border-color: #3f3f46;
  background: #18181b;
}
.lw-kline-crosshair-strip__head {
  margin-bottom: 4px;
}
.lw-kline-crosshair-strip__date {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__date {
  color: #f1f5f9;
}
.lw-kline-kv {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
  flex-shrink: 0;
}
.lw-kline-crosshair-strip__grid {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  column-gap: 14px;
  row-gap: 4px;
  font-size: 11px;
  min-width: 0;
}
.lw-kline-crosshair-strip__k {
  color: #64748b;
  white-space: nowrap;
}
.lw-kline-crosshair-strip--dark .lw-kline-crosshair-strip__k {
  color: #94a3b8;
}
.lw-kline-crosshair-strip__v {
  font-variant-numeric: tabular-nums;
  min-width: 0;
}
.lw-kline-chart {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  position: relative;
  touch-action: none;
  box-sizing: border-box;
}

.lw-kline-chart-wrap {
  width: 100%;
  display: flex;
  gap: 10px;
  align-items: stretch;
  min-width: 0;
}
.lw-kline--dark .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #27272a;
}
.lw-kline-root:not(.lw-kline--dark) .lw-kline-chart {
  border-radius: 4px;
  border: 1px solid #e2e8f0;
}

.lw-chip {
  width: 160px;
  flex: 0 0 160px;
  border-radius: 4px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  box-sizing: border-box;
  padding: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.lw-chip--dark {
  border-color: #27272a;
  background: #141414;
}
.lw-chip__head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  margin-bottom: 6px;
  flex-wrap: wrap;
}
.lw-chip__title {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__title {
  color: #f1f5f9;
}
.lw-chip__meta {
  font-size: 11px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lw-chip--dark .lw-chip__meta {
  color: #94a3b8;
}
.lw-chip__empty {
  font-size: 11px;
  color: #64748b;
}
.lw-chip--dark .lw-chip__empty {
  color: #94a3b8;
}
.lw-chip__canvas {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
  display: block;
}
.lw-kline-source-tag {
  display: inline-block;
  font-size: 10px;
  line-height: 1;
  padding: 2px 5px;
  border-radius: 3px;
  background: #e0f2fe;
  color: #0369a1;
  vertical-align: middle;
  margin-left: 4px;
}
.lw-kline--dark .lw-kline-source-tag {
  background: #1e3a5f;
  color: #7dd3fc;
}
.lw-kline-source-tag--fallback {
  background: #fef3c7;
  color: #b45309;
}
.lw-kline--dark .lw-kline-source-tag--fallback {
  background: #422006;
  color: #fbbf24;
}
.lw-kline-signal-summary {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 6px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}
.lw-kline-signal-summary--dark {
  border-color: #3f3f46;
  background: #18181b;
}
.lw-kline-signal-summary__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.lw-kline-signal-summary__title {
  font-weight: 700;
  font-size: 12px;
  color: #0f172a;
  white-space: nowrap;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__title {
  color: #f1f5f9;
}
.lw-kline-signal-summary__total {
  font-size: 11px;
  color: #64748b;
  white-space: nowrap;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__total {
  color: #94a3b8;
}
.lw-kline-signal-summary__bar {
  display: flex;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  background: #e2e8f0;
  margin-bottom: 6px;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__bar {
  background: #3f3f46;
}
.lw-kline-signal-summary__bar-seg {
  height: 100%;
  min-width: 0;
  transition: width 0.3s ease;
}
.lw-kline-signal-summary__bar-seg--bullish { background: #ef4444; }
.lw-kline-signal-summary__bar-seg--bearish { background: #22c55e; }
.lw-kline-signal-summary__bar-seg--oscillating { background: #f59e0b; }
.lw-kline-signal-summary__bar-seg--neutral { background: #94a3b8; }
.lw-kline-signal-summary__legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  margin-bottom: 6px;
  font-size: 11px;
  color: #334155;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__legend {
  color: #cbd5e1;
}
.lw-kline-signal-summary__legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
.lw-kline-signal-summary__dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.lw-kline-signal-summary__dot--bullish { background: #ef4444; }
.lw-kline-signal-summary__dot--bearish { background: #22c55e; }
.lw-kline-signal-summary__dot--oscillating { background: #f59e0b; }
.lw-kline-signal-summary__dot--neutral { background: #94a3b8; }
.lw-kline-signal-summary__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.lw-kline-signal-summary__tag {
  display: inline-block;
  font-size: 10px;
  line-height: 1;
  padding: 3px 6px;
  border-radius: 3px;
  white-space: nowrap;
}
.lw-kline-signal-summary__tag--bullish {
  background: #fef2f2;
  color: #dc2626;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--bullish {
  background: #450a0a;
  color: #fca5a5;
}
.lw-kline-signal-summary__tag--bearish {
  background: #f0fdf4;
  color: #16a34a;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--bearish {
  background: #052e16;
  color: #86efac;
}
.lw-kline-signal-summary__tag--oscillating {
  background: #fffbeb;
  color: #d97706;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--oscillating {
  background: #451a03;
  color: #fcd34d;
}
.lw-kline-signal-summary__tag--neutral {
  background: #f1f5f9;
  color: #475569;
}
.lw-kline-signal-summary--dark .lw-kline-signal-summary__tag--neutral {
  background: #1e293b;
  color: #94a3b8;
}
</style>
