<script setup>
/**
 * 全市场买卖点扫描面板（通达信数据源）。
 *
 * 按市场范围（沪/深/京/全部）枚举全市场股票，Go 侧逐只拉 TDX K 线并用与
 * 前端 calc.ts 同口径的算法打分，命中与进度经事件流式推送：
 *   - marketSignalScanProgress：进度快照（Go 侧 300ms 节流）
 *   - marketSignalScanHit：单条命中信号
 * 本组件只做交互与展示，扫描生命周期由 Go 侧单例管理（面板销毁不影响扫描继续）。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NButton, NEmpty, NFlex, NModal, NProgress, NSelect, NTag, NText, NTooltip, useMessage,
} from 'naive-ui'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { GetConfig, GetMarketSignalScanStatus, StartMarketSignalScan, StopMarketSignalScan } from '../../wailsjs/go/main/App'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { BUY_SELL_SCORE_OPTIONS } from './kline/constants'
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from './kline/useKlineModalFit'

const message = useMessage()

// ---- 扫描条件 ----
/** 市场范围（多选）：SH=沪市 SZ=深市 BJ=北交所；空数组 = 全部市场 */
const markets = ref([])
const marketOptions = [
  { label: '沪市', value: 'SH' },
  { label: '深市', value: 'SZ' },
  { label: '北交所', value: 'BJ' },
]
const klt = ref('101')
const kltOptions = [
  { label: '日K', value: '101' },
  { label: '60分', value: '60' },
  { label: '30分', value: '30' },
  { label: '15分', value: '15' },
  { label: '5分', value: '5' },
  { label: '1分', value: '1' },
  { label: '周K', value: '102' },
]
const minScore = ref(6)
const scoreOptions = BUY_SELL_SCORE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))
/** 信号时间窗口：只保留最近 N 个交易日内命中的信号 */
const recentDays = ref(1)
const daysOptions = [1, 2, 3, 5, 10, 20].map((d) => ({ label: `近${d}个交易日`, value: d }))
/** 方向筛选（纯前端过滤） */
const kindFilter = ref('')
const kindOptions = [
  { label: '全部', value: '' },
  { label: '买点', value: 'buy' },
  { label: '卖点', value: 'sell' },
]

// ---- 扫描状态 ----
const progress = ref({ running: false, total: 0, scanned: 0, hits: 0, current: '', error: '' })
/** 命中结果（追加序，展示时排序 + 方向过滤） */
const rows = ref([])

const scanPercent = computed(() => {
  const p = progress.value
  if (!p.total) return 0
  return Math.min(100, Math.round((p.scanned / p.total) * 100))
})
const scanSummary = computed(() => {
  const p = progress.value
  if (p.running) {
    const cur = p.current ? ` · 当前 ${p.current}` : ''
    return `扫描中 ${p.scanned}/${p.total} · 命中 ${p.hits}${cur}`
  }
  if (p.finishedAt && p.error) return p.error
  if (p.finishedAt) return `已完成：扫描 ${p.scanned} 只，命中 ${p.hits} 条`
  return '尚未扫描'
})

/** 展示结果：方向过滤 + 强度降序、时间降序（默认按信号强度排序） */
const visibleRows = computed(() => {
  const arr = rows.value.filter((r) => !kindFilter.value || r.kind === kindFilter.value)
  return [...arr].sort((a, b) => (b.score - a.score) || (b.time - a.time))
})

async function startScan() {
  rows.value = []
  progress.value.error = ''
  try {
    await StartMarketSignalScan({
      markets: markets.value,
      klt: klt.value,
      minScore: minScore.value,
      recentDays: recentDays.value,
    })
    progress.value = { ...progress.value, running: true, scanned: 0, hits: 0, finishedAt: 0 }
  } catch (e) {
    message.error(`启动扫描失败: ${e}`)
  }
}

function stopScan() {
  StopMarketSignalScan()
  message.info('正在停止扫描…')
}

// ---- 事件订阅（组件常驻：订阅一次，跟随父面板生命周期） ----
function onProgress(p) {
  if (p && typeof p === 'object') progress.value = p
}
function onHit(h) {
  if (!h || !h.code) return
  rows.value.push(h)
}

onMounted(() => {
  EventsOn('marketSignalScanProgress', onProgress)
  EventsOn('marketSignalScanHit', onHit)
  // 应用重启/页面刷新后，若 Go 侧扫描仍在进行，接上进度显示
  GetMarketSignalScanStatus().then(onProgress).catch(() => {})
  GetConfig().then((res) => { darkTheme.value = !!res.darkTheme }).catch(() => {})
})
onBeforeUnmount(() => {
  EventsOff('marketSignalScanProgress')
  EventsOff('marketSignalScanHit')
})

// ---- K 线弹窗 ----
const klineShow = ref(false)
const klineCode = ref('')
const klineName = ref('')
const darkTheme = ref(false)
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

/** 代码归一化为东方财富格式（600519.SH），行情组件按此取数 */
function toChartCode(code) {
  const c = String(code || '').trim()
  if (!c) return ''
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return `${lower.slice(2)}.SH`
  if (lower.startsWith('sz')) return `${lower.slice(2)}.SZ`
  if (lower.startsWith('bj')) return `${lower.slice(2)}.BJ`
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return `${c}.SH`
    if (d === '4' || d === '8' || d === '9') return `${c}.BJ`
    return `${c}.SZ`
  }
  return ''
}

function openKline(r) {
  const em = toChartCode(r.code)
  if (!em) {
    message.warning('该代码暂不支持K线图')
    return
  }
  klineCode.value = em
  klineName.value = r.name || ''
  klineShow.value = true
}

// ---- 展示辅助 ----
const marketTagType = { SH: 'info', SZ: 'success', BJ: 'warning' }
const marketLabel = { SH: '沪', SZ: '深', BJ: '京' }

function pctText(v) {
  const n = Number(v) || 0
  return `${n >= 0 ? '+' : ''}${n.toFixed(2)}%`
}
function pctColor(v) {
  const n = Number(v) || 0
  if (n > 0) return '#d03050'
  if (n < 0) return '#18a058'
  return undefined
}
/** 相对信号点涨跌幅：(最新收盘 - 信号价) / 信号价 */
function pctVsSignal(r) {
  const p = Number(r.price) || 0
  const c = Number(r.latestClose) || 0
  if (!p || !c) return null
  return (c - p) / p * 100
}
</script>

<template>
  <div class="scan-panel">
    <div class="section">
      <NFlex justify="space-between" align="center">
        <NText strong>全市场扫描（通达信）</NText>
        <NFlex :size="8" align="center">
          <NButton v-if="!progress.running" size="small" type="primary" @click="startScan">开始扫描</NButton>
          <NButton v-else size="small" type="warning" @click="stopScan">停止</NButton>
        </NFlex>
      </NFlex>
      <NFlex :size="8" align="center" style="margin-top: 8px;" :wrap="false">
        <div class="field" style="flex: 1.2;">
          <NText depth="3" style="font-size: 12px;">市场范围（不选=全部）</NText>
          <NSelect v-model:value="markets" multiple clearable :options="marketOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field">
          <NText depth="3" style="font-size: 12px;">周期</NText>
          <NSelect v-model:value="klt" :options="kltOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field">
          <NText depth="3" style="font-size: 12px;">灵敏度</NText>
          <NSelect v-model:value="minScore" :options="scoreOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field">
          <NText depth="3" style="font-size: 12px;">时间范围</NText>
          <NSelect v-model:value="recentDays" :options="daysOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field">
          <NText depth="3" style="font-size: 12px;">方向</NText>
          <NSelect v-model:value="kindFilter" :options="kindOptions" :z-index="10002" style="width: 100%;" />
        </div>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 6px;">
        数据源为通达信行情（gotdx），逐只拉取 K 线并用与图表同口径的买卖点共振算法打分；
        全市场数千只，扫描需数分钟，可随时停止。时间范围=只保留最近 N 个交易日内确认的信号。
      </NText>
    </div>

    <div class="section" v-if="progress.running || progress.finishedAt">
      <NFlex :size="8" align="center">
        <NProgress
          type="line"
          :percentage="scanPercent"
          :height="8"
          :show-indicator="false"
          style="flex: 1;"
        />
        <NText depth="3" style="font-size: 12px; flex: none;">{{ scanPercent }}%</NText>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 4px;">{{ scanSummary }}</NText>
    </div>

    <div class="section">
      <NFlex justify="space-between" align="center">
        <NText strong>命中信号</NText>
        <NText depth="3" style="font-size: 12px;">{{ visibleRows.length }} 条</NText>
      </NFlex>
      <NEmpty v-if="!visibleRows.length" description="尚无命中信号，设置条件后点击「开始扫描」" style="margin-top: 20px;" />
      <div v-else class="result-table">
        <div class="rt-row rt-head">
          <div class="rt-c stock">股票</div>
          <div class="rt-c dir">方向</div>
          <div class="rt-c score">强度</div>
          <div class="rt-c time">信号时间</div>
          <div class="rt-c num">信号价</div>
          <div class="rt-c num">最新价</div>
          <div class="rt-c num">相对信号</div>
          <div class="rt-c reasons">命中信号</div>
          <div class="rt-c op">操作</div>
        </div>
        <div v-for="r in visibleRows" :key="`${r.code}|${r.kind}|${r.time}`" class="rt-row">
          <div class="rt-c stock">
            <NText type="info" style="cursor: pointer;" @click="openKline(r)">{{ r.name }}</NText>
            <span class="rt-sub">{{ r.code }}</span>
            <NTag size="tiny" :bordered="false" :type="marketTagType[r.market]">{{ marketLabel[r.market] || r.market }}</NTag>
          </div>
          <div class="rt-c dir">
            <NTag size="small" :bordered="false" :type="r.kind === 'buy' ? 'error' : 'success'">
              {{ r.kind === 'buy' ? '买点' : '卖点' }}
            </NTag>
          </div>
          <div class="rt-c score">
            <NTooltip trigger="hover" :z-index="10002">
              <template #trigger>
                <NTag size="small" :bordered="false" :type="r.score >= 7 ? 'error' : r.score >= 6 ? 'warning' : 'default'">
                  {{ r.score }}/14
                </NTag>
              </template>
              共振路数：{{ r.reasons }}
            </NTooltip>
          </div>
          <div class="rt-c time">
            <div>{{ r.timeText }}</div>
            <span class="rt-sub">距最新 {{ r.lag }} 根</span>
          </div>
          <div class="rt-c num">{{ r.price }}</div>
          <div class="rt-c num">
            <div>{{ r.latestClose }}</div>
            <span class="rt-sub" :style="{ color: pctColor(r.latestPct) }">当日 {{ pctText(r.latestPct) }}</span>
          </div>
          <div class="rt-c num">
            <span v-if="pctVsSignal(r) !== null" :style="{ color: pctColor(pctVsSignal(r)) }">{{ pctText(pctVsSignal(r)) }}</span>
            <span v-else class="rt-sub">—</span>
          </div>
          <div class="rt-c reasons">
            <NTooltip trigger="hover" :z-index="10002" style="max-width: 340px;">
              <template #trigger>
                <span class="reasons-ellipsis">{{ r.reasons }}</span>
              </template>
              {{ r.reasons }}
            </NTooltip>
          </div>
          <div class="rt-c op">
            <NButton size="tiny" quaternary type="info" @click="openKline(r)">K线</NButton>
          </div>
        </div>
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
          :key="'market-scan-kline-' + klineCode"
          :code="klineCode"
          :stock-name="klineName"
          :dark-theme="darkTheme"
          :chart-height="klineChartHeight"
        />
      </div>
    </NModal>
  </div>
</template>

<style scoped>
.scan-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section {
  padding: 10px 12px;
  border-radius: 8px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 96px;
}

.result-table {
  margin-top: 8px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid rgba(128, 128, 128, 0.18);
}

.rt-row {
  display: flex;
  align-items: center;
  padding: 6px 8px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.12);
  font-size: 12px;
  gap: 4px;
}

.rt-row:last-child {
  border-bottom: none;
}

.rt-head {
  background: rgba(128, 128, 128, 0.08);
  font-weight: 600;
  font-size: 12px;
}

.rt-c {
  padding: 0 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rt-c.stock {
  flex: 1.6;
  min-width: 130px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.rt-c.dir {
  width: 52px;
}

.rt-c.score {
  width: 62px;
}

.rt-c.time {
  width: 128px;
}

.rt-c.num {
  width: 92px;
}

.rt-c.reasons {
  flex: 1;
  min-width: 120px;
}

.rt-c.op {
  width: 56px;
  text-align: right;
}

.rt-sub {
  color: rgba(128, 128, 128, 0.9);
  font-size: 11px;
}

.reasons-ellipsis {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
</style>
