<script setup>
/**
 * MASS 反转策略全市场回测面板（通达信数据源，日K）。
 *
 * 固定入场：日K 收盘后确认「MI 首次越过自适应进入线」（凸起首日），以信号日收盘价买入
 *（对应实盘尾盘确认、尾盘买入），同一凸起只入场一次；阈值 point-in-time（只用截至信号日的
 * MI 历史样本算 P90），无未来函数。9 种卖出策略对每次入场并行模拟：
 *   hold3/hold5/hold10/hold20（固定持有）、massExit（MI 跌破离开线）、
 *   ma10Break/ma20Break（均线自上向下穿越）、tp10sl8（止盈10%/止损8%）、
 *   combo（MI 离开线或破 MA20，先到先卖）。
 * 逐笔按「入场日 5 路趋势证据分」分桶统计，可直接对比趋势过滤的增益。
 * 事件流式推送：massBacktestProgress（进度）/ massBacktestEvent（逐笔入场事件）/
 * massBacktestResult（最终 策略×分桶 统计）。回测生命周期由 Go 侧单例管理。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NButton, NEmpty, NFlex, NModal, NProgress, NSelect, NTag, NText, NTooltip, useMessage,
} from 'naive-ui'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { ExportTableToXLSX, GetConfig, GetMassBacktestStatus, StartMassBacktest, StopMassBacktest } from '../../wailsjs/go/main/App'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { KLINE_MODAL_CONTENT_STYLE, KLINE_MODAL_STYLE, useKlineModalFit } from './kline/useKlineModalFit'

const message = useMessage()

// ---- 回测配置 ----
/** 市场范围（多选）：SH=沪市 SZ=深市 BJ=北交所；空数组 = 全部市场 */
const markets = ref([])
const marketOptions = [
  { label: '沪市', value: 'SH' },
  { label: '深市', value: 'SZ' },
  { label: '北交所', value: 'BJ' },
]
/** 板块细分（多选）：main=主板 gem=创业板 star=科创板 bj=北交所；空数组 = 全部板块 */
const boards = ref([])
const boardOptions = [
  { label: '主板', value: 'main' },
  { label: '创业板', value: 'gem' },
  { label: '科创板', value: 'star' },
  { label: '北交所', value: 'bj' },
]
/** 日K回看根数 */
const lookbackBars = ref(500)
const lookbackOptions = [
  { label: '300 根（约1.2年）', value: 300 },
  { label: '500 根（约2年）', value: 500 },
  { label: '800 根（约3.2年）', value: 800 },
]
/** 抽样只数（0=全部），全市场数千只需数分钟~十几分钟 */
const sampleLimit = ref(500)
const sampleOptions = [
  { label: '抽样 500 只（快）', value: 500 },
  { label: '抽样 1000 只', value: 1000 },
  { label: '抽样 2000 只', value: 2000 },
  { label: '全部股票（慢）', value: 0 },
]

// ---- 卖出策略 key → 展示 ----
const stgLabel = {
  hold3: '持有3日',
  hold5: '持有5日',
  hold10: '持有10日',
  hold20: '持有20日',
  massExit: 'MI离开线',
  ma10Break: '破MA10',
  ma20Break: '破MA20',
  tp10sl8: '盈10/损8',
  combo: '组合',
}
const stgOrder = ['hold3', 'hold5', 'hold10', 'hold20', 'massExit', 'ma10Break', 'ma20Break', 'tp10sl8', 'combo']
const bucketOrder = ['全部', '趋势≥3', '趋势≥4', '趋势=5', '趋势≤2']

// ---- 回测状态 ----
const progress = ref({ running: false, total: 0, scanned: 0, events: 0, current: '', error: '' })
/** 最终统计（策略 key → 分桶 → stat） */
const stats = ref(null)
const resultInfo = ref(null)
/** 入场事件明细（保留最近 1000 条） */
const events = ref([])
const MAX_EVENTS = 1000

const btPercent = computed(() => {
  const p = progress.value
  if (!p.total) return 0
  return Math.min(100, Math.round((p.scanned / p.total) * 100))
})
const btSummary = computed(() => {
  const p = progress.value
  if (p.running) {
    const cur = p.current ? ` · 当前 ${p.current}` : ''
    return `回测中 ${p.scanned}/${p.total} · 入场事件 ${p.events}${cur}`
  }
  if (p.finishedAt && p.error) return p.error
  if (p.finishedAt) return `已完成：回测 ${p.scanned} 只，入场事件 ${p.events} 次`
  return '尚未回测'
})

/** stats 矩阵：按策略分组的分桶统计 */
const statRows = computed(() => {
  if (!stats.value) return []
  return stgOrder.map((key) => {
    const byBucket = {}
    for (const st of stats.value) {
      if (st.strategy === key) byBucket[st.bucket] = st
    }
    return { key, byBucket }
  })
})
/** 每个分桶列中平均收益最高的策略（样本≥10 才参与「最佳」标记） */
function bestAvgRetByBucket() {
  const best = {}
  if (!stats.value) return best
  for (const bucket of bucketOrder) {
    let maxAvg = -Infinity
    let maxKey = ''
    for (const st of stats.value) {
      if (st.bucket !== bucket || st.trades < 10) continue
      if (st.avgRet > maxAvg) {
        maxAvg = st.avgRet
        maxKey = st.strategy
      }
    }
    if (maxKey) best[bucket] = maxKey
  }
  return best
}
const bestMap = computed(() => bestAvgRetByBucket())

async function startBt() {
  events.value = []
  stats.value = null
  resultInfo.value = null
  progress.value.error = ''
  try {
    await StartMassBacktest({
      markets: markets.value,
      boards: boards.value,
      lookbackBars: lookbackBars.value,
      sampleLimit: sampleLimit.value,
    })
    progress.value = { ...progress.value, running: true, total: 0, scanned: 0, events: 0, finishedAt: 0 }
  } catch (e) {
    message.error(`启动回测失败: ${e}`)
  }
}

function stopBt() {
  StopMassBacktest()
  message.info('正在停止回测…')
}

// ---- 事件订阅（组件常驻：订阅一次，跟随父面板生命周期） ----
function onProgress(p) {
  if (p && typeof p === 'object') progress.value = p
}
function onEvent(e) {
  if (!e || !e.code) return
  events.value.push(e)
  if (events.value.length > MAX_EVENTS) {
    events.value.splice(0, events.value.length - MAX_EVENTS)
  }
}
function onResult(r) {
  if (!r || typeof r !== 'object') return
  stats.value = r.stats || []
  resultInfo.value = { stockCount: r.stockCount, eventCount: r.eventCount, stopped: r.stopped, error: r.error }
}

onMounted(() => {
  EventsOn('massBacktestProgress', onProgress)
  EventsOn('massBacktestEvent', onEvent)
  EventsOn('massBacktestResult', onResult)
  // 应用重启/页面刷新后，若 Go 侧回测仍在进行，接上进度显示
  GetMassBacktestStatus().then(onProgress).catch(() => {})
  GetConfig().then((res) => { darkTheme.value = !!res.darkTheme }).catch(() => {})
})
onBeforeUnmount(() => {
  EventsOff('massBacktestProgress')
  EventsOff('massBacktestEvent')
  EventsOff('massBacktestResult')
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

function openKline(e) {
  const em = toChartCode(e.code)
  if (!em) {
    message.warning('该代码暂不支持K线图')
    return
  }
  klineCode.value = em
  klineName.value = e.name || ''
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
function fmtNum(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(2) : '—'
}
function winRateColor(v) {
  const n = Number(v) || 0
  if (n >= 55) return '#d03050'
  if (n >= 45) return undefined
  return '#18a058'
}

/** 导出回测入场事件明细为 Excel：弹系统保存框让用户选择保存位置，用户取消时后端返回空串 */
async function exportExcel() {
  const list = events.value
  if (!list.length) {
    message.warning('暂无可导出的入场事件')
    return
  }
  const d = new Date()
  const p2 = (x) => String(x).padStart(2, '0')
  const fname = `MASS策略回测明细_${d.getFullYear()}${p2(d.getMonth() + 1)}${p2(d.getDate())}-${p2(d.getHours())}${p2(d.getMinutes())}${p2(d.getSeconds())}.xlsx`
  const cols = [
    { title: '股票名称', key: 'name' }, { title: '代码', key: 'code' }, { title: '市场', key: 'market' },
    { title: '信号日(买入日)', key: 'signalDate' }, { title: '买入价', key: 'buyPrice' },
    { title: '趋势分', key: 'trendScore' }, { title: '趋势证据', key: 'trendInfo' },
    { title: 'MI值', key: 'mi' }, { title: '进入线', key: 'enter' }, { title: '阈值类型', key: 'adaptive' },
    ...stgOrder.map((k) => ({ title: `${stgLabel[k]} 收益%`, key: k })),
  ]
  const mk = { SH: '沪', SZ: '深', BJ: '京' }
  const out = list.map((e) => {
    const o = {
      name: e.name || '', code: e.code || '', market: mk[e.market] || e.market || '',
      signalDate: e.signalDate || '', buyPrice: Number(e.buyPrice) || 0,
      trendScore: Number(e.trendScore) || 0, trendInfo: e.trendInfo || '',
      mi: Number(e.mi) || 0, enter: Number(e.enter) || 0,
      adaptive: e.adaptive ? '自适应阈值' : '经典阈值',
    }
    // 无结果（未平仓/缺失）的策略收益留空，避免误写 0
    stgOrder.forEach((k) => { if (e.rets && e.rets[k] != null) o[k] = Number(e.rets[k]) })
    return o
  })
  try {
    const saved = await ExportTableToXLSX(fname, { sheetName: '回测事件明细', columns: cols, rows: out })
    if (saved) message.success(`已导出 ${out.length} 条：${saved}`)
    else message.info('已取消导出')
  } catch (err) {
    message.error(`导出失败: ${err}`)
  }
}
</script>

<template>
  <div class="scan-panel">
    <div class="section">
      <NFlex justify="space-between" align="center">
        <NText strong>MASS 反转策略回测（通达信 · 日K）</NText>
        <NFlex :size="8" align="center">
          <NButton v-if="!progress.running" size="small" type="primary" @click="startBt">开始回测</NButton>
          <NButton v-else size="small" type="warning" @click="stopBt">停止</NButton>
        </NFlex>
      </NFlex>
      <NFlex :size="8" align="center" style="margin-top: 8px;">
        <div class="field" style="flex: 1.2;">
          <NText depth="3" style="font-size: 12px;">市场范围（不选=全部）</NText>
          <NSelect v-model:value="markets" multiple clearable :options="marketOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1.2;">
          <NText depth="3" style="font-size: 12px;">板块细分（不选=全部）</NText>
          <NSelect v-model:value="boards" multiple clearable :options="boardOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1;">
          <NText depth="3" style="font-size: 12px;">回看根数</NText>
          <NSelect v-model:value="lookbackBars" :options="lookbackOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1;">
          <NText depth="3" style="font-size: 12px;">抽样只数</NText>
          <NSelect v-model:value="sampleLimit" :options="sampleOptions" :z-index="10002" style="width: 100%;" />
        </div>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 6px;">
        固定买点：凸起首日（MI 首次越过自适应进入线）收盘买入，同一凸起只入场一次，阈值 point-in-time 无未来函数。
        9 种卖出策略并行模拟对比胜率/收益/盈亏比，并按入场日趋势证据分（0~5）分桶，验证趋势过滤的增益。
        收益率按收盘价口径、未计滑点税费。全市场回测需数分钟，可随时停止。
      </NText>
    </div>

    <div class="section" v-if="progress.running || progress.finishedAt">
      <NFlex :size="8" align="center">
        <NProgress
          type="line"
          :percentage="btPercent"
          :height="8"
          :show-indicator="false"
          style="flex: 1;"
        />
        <NText depth="3" style="font-size: 12px; flex: none;">{{ btPercent }}%</NText>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 4px;">{{ btSummary }}</NText>
    </div>

    <div class="section" v-if="statRows.length">
      <NFlex justify="space-between" align="center">
        <NText strong>卖出策略 × 趋势分桶 统计矩阵</NText>
        <NText depth="3" style="font-size: 12px;">
          {{ resultInfo ? `回测 ${resultInfo.stockCount} 只 · ${resultInfo.eventCount} 次入场` : '' }}
        </NText>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 2px;">
        单元格：胜率 / 平均收益（期望）/ 笔数；悬停查看中位数、盈亏比、最大单笔亏损、平均持有天数。
        红框=该列平均收益最高（样本≥10）。「趋势≤2」列反映无趋势过滤的原始信号质量，与其余列对比即趋势过滤增益。
      </NText>
      <div class="bt-matrix" style="margin-top: 8px;">
        <div class="bm-row bm-head">
          <div class="bm-c stg">卖出策略</div>
          <div v-for="b in bucketOrder" :key="b" class="bm-c bucket">{{ b }}</div>
        </div>
        <div v-for="row in statRows" :key="row.key" class="bm-row">
          <div class="bm-c stg">{{ stgLabel[row.key] }}</div>
          <div v-for="b in bucketOrder" :key="b" class="bm-c bucket">
            <NTooltip v-if="row.byBucket[b]" trigger="hover" :z-index="10002" style="max-width: 320px;">
              <template #trigger>
                <div class="bm-cell" :class="{ best: bestMap[b] === row.key }">
                  <div class="bm-wr" :style="{ color: winRateColor(row.byBucket[b].winRate) }">
                    胜率 {{ row.byBucket[b].winRate.toFixed(1) }}%
                  </div>
                  <div class="bm-ret" :style="{ color: pctColor(row.byBucket[b].avgRet) }">
                    均 {{ pctText(row.byBucket[b].avgRet) }}
                  </div>
                  <div class="bm-n">{{ row.byBucket[b].trades }} 笔</div>
                </div>
              </template>
              <div>
                {{ stgLabel[row.key] }} × {{ b }}<br />
                笔数 {{ row.byBucket[b].trades }} · 盈利 {{ row.byBucket[b].wins }}<br />
                平均收益 {{ pctText(row.byBucket[b].avgRet) }} · 中位 {{ pctText(row.byBucket[b].medRet) }}<br />
                平均盈利 {{ pctText(row.byBucket[b].avgWin) }} · 平均亏损 {{ pctText(row.byBucket[b].avgLoss) }}<br />
                盈亏比 {{ row.byBucket[b].profitFactor >= 99 ? '∞' : row.byBucket[b].profitFactor.toFixed(2) }}
                · 最大单亏 {{ pctText(row.byBucket[b].maxLoss) }}<br />
                平均持有 {{ row.byBucket[b].avgHold.toFixed(1) }} 个交易日
              </div>
            </NTooltip>
            <div v-else class="bm-cell empty">—</div>
          </div>
        </div>
      </div>
    </div>

    <div class="section">
      <NFlex justify="space-between" align="center">
        <NText strong>入场事件明细</NText>
        <NFlex :size="8" align="center">
          <NButton size="tiny" type="primary" ghost :disabled="!events.length" @click="exportExcel">导出Excel</NButton>
          <NText depth="3" style="font-size: 12px;">{{ events.length }} 条（最多保留 {{ MAX_EVENTS }} 条）</NText>
        </NFlex>
      </NFlex>
      <NEmpty v-if="!events.length" description="尚无入场事件，设置条件后点击「开始回测」" style="margin-top: 20px;" />
      <div v-else class="result-table">
        <div class="rt-row rt-head">
          <div class="rt-c stock">股票</div>
          <div class="rt-c date">信号日</div>
          <div class="rt-c num">买入价</div>
          <div class="rt-c score">趋势分</div>
          <div class="rt-c mi">MI / 进入线</div>
          <div class="rt-c rets">各策略收益（红盈绿亏）</div>
          <div class="rt-c op">操作</div>
        </div>
        <div v-for="(e, i) in events" :key="e.stockCode + e.signalDate + i" class="rt-row">
          <div class="rt-c stock">
            <NText type="info" style="cursor: pointer;" @click="openKline(e)">{{ e.name }}</NText>
            <span class="rt-sub">{{ e.code }}</span>
            <NTag size="tiny" :bordered="false" :type="marketTagType[e.market]">{{ marketLabel[e.market] || e.market }}</NTag>
          </div>
          <div class="rt-c date">
            <div>{{ e.signalDate }}</div>
            <span class="rt-sub">{{ e.adaptive ? '自适应阈值' : '经典阈值' }}</span>
          </div>
          <div class="rt-c num">{{ fmtNum(e.buyPrice) }}</div>
          <div class="rt-c score">
            <NTooltip trigger="hover" :z-index="10002" style="max-width: 340px;">
              <template #trigger>
                <NTag size="small" :bordered="false" :type="e.trendScore >= 5 ? 'error' : e.trendScore >= 4 ? 'warning' : 'default'">
                  {{ e.trendScore }}/5
                </NTag>
              </template>
              趋势证据：{{ e.trendInfo || '无' }}
            </NTooltip>
          </div>
          <div class="rt-c mi">{{ fmtNum(e.mi) }} / {{ fmtNum(e.enter) }}</div>
          <div class="rt-c rets">
            <span
              v-for="key in stgOrder"
              :key="key"
              class="ret-chip"
              :style="{ color: pctColor(e.rets && e.rets[key]) }"
              :title="stgLabel[key] + '：' + pctText(e.rets && e.rets[key]) + '，持有 ' + ((e.holds && e.holds[key]) ?? '—') + ' 日'"
            >{{ stgLabel[key] }} {{ pctText(e.rets && e.rets[key]) }}</span>
          </div>
          <div class="rt-c op">
            <NButton size="tiny" quaternary type="info" @click="openKline(e)">K线</NButton>
          </div>
        </div>
      </div>
    </div>

    <!-- 入场标的 K 线弹窗：宽度按窗口宽、图表高度按窗口高自适应 -->
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
          :key="'mass-bt-kline-' + klineCode"
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

/* ---- 统计矩阵 ---- */
.bt-matrix {
  overflow-x: auto;
}

.bm-row {
  display: grid;
  grid-template-columns: 96px repeat(5, minmax(108px, 1fr));
  gap: 4px;
  padding: 4px 0;
  border-bottom: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.2));
}

.bm-row.bm-head {
  font-size: 12px;
  color: var(--n-text-color-3, #909399);
  border-bottom: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.3));
}

.bm-c.stg {
  font-size: 12px;
  display: flex;
  align-items: center;
}

.bm-cell {
  text-align: center;
  padding: 4px 2px;
  border-radius: 6px;
  border: 1px solid transparent;
}

.bm-cell.best {
  border-color: #d03050;
}

.bm-cell.empty {
  color: var(--n-text-color-3, #909399);
}

.bm-wr {
  font-size: 12px;
  font-weight: 600;
}

.bm-ret {
  font-size: 12px;
}

.bm-n {
  font-size: 11px;
  color: var(--n-text-color-3, #909399);
}

/* ---- 事件明细表 ---- */
.result-table {
  margin-top: 8px;
}

.rt-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 4px;
  border-bottom: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.2));
}

.rt-row.rt-head {
  font-size: 12px;
  color: var(--n-text-color-3, #909399);
  border-bottom: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.3));
}

.rt-c {
  flex: none;
}

.rt-c.stock {
  width: 150px;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.rt-sub {
  font-size: 11px;
  color: var(--n-text-color-3, #909399);
}

.rt-c.date {
  width: 88px;
  font-size: 12px;
}

.rt-c.num {
  width: 70px;
  text-align: right;
  font-size: 12px;
}

.rt-c.mi {
  width: 92px;
  text-align: right;
  font-size: 12px;
}

.rt-c.score {
  width: 56px;
  text-align: center;
}

.rt-c.rets {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  gap: 2px 8px;
  font-size: 11px;
}

.ret-chip {
  white-space: nowrap;
}

.rt-c.op {
  width: 52px;
  text-align: right;
}
</style>
