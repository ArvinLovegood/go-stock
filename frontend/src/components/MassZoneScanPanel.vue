<script setup>
/**
 * 全市场 MASS 反转预警扫描面板（通达信数据源，支持日K/分钟/周K周期）。
 *
 * 筛出「当前正处于 Mass Index 反转预警窗口（凸起已形成、尚未回落确认）」
 * 且「处于多头/上升趋势」的标的。MASS 口径与主图琥珀色预警竖带一致
 * （自适应进入线=自身 MI 历史 P90 钳制 24~31，离开线=进入线-0.5）；
 * 趋势口径复刻前端 okxUptrendScan 的 5 路证据等权计分（指标可自选，默认全部 5 路 ≥3）。
 * 命中与进度经事件流式推送：
 *   - massZoneScanProgress：进度快照（Go 侧 300ms 节流）
 *   - massZoneScanHit：单条命中
 * 本组件只做交互与展示，扫描生命周期由 Go 侧单例管理（面板销毁不影响扫描继续）。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NButton, NEmpty, NFlex, NModal, NProgress, NSelect, NTag, NText, NTooltip, useMessage,
} from 'naive-ui'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { ExportTableToXLSX, GetConfig, GetMassZoneScanStatus, StartMassZoneScan, StopMassZoneScan } from '../../wailsjs/go/main/App'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
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
/** 板块细分（多选）：main=主板 gem=创业板 star=科创板 bj=北交所；空数组 = 全部板块 */
const boards = ref([])
const boardOptions = [
  { label: '主板', value: 'main' },
  { label: '创业板', value: 'gem' },
  { label: '科创板', value: 'star' },
  { label: '北交所', value: 'bj' },
]
/** K线周期：101=日K 1/5/15/30/60=分钟 102=周K */
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
const kltLabel = { '1': '1分', '5': '5分', '15': '15分', '30': '30分', '60': '60分', '101': '日K', '102': '周K' }
/** 自选趋势指标（多选）：空 = 全部 5 个都参与计分 */
const trendChecks = ref([])
const trendCheckOptions = [
  { label: '多头排列', value: 'ma_align' },
  { label: 'MA20上行', value: 'ma20_up' },
  { label: 'Supertrend翻多', value: 'supertrend' },
  { label: 'ADX强多头', value: 'adx' },
  { label: '通道上半区', value: 'donchian' },
]
/** 趋势过滤：在自选指标范围内按满足个数过滤（后端钳制到自选个数） */
const minTrendScore = ref(3)
const trendScoreOptions = [
  { label: '满足 ≥1（宽松）', value: 1 },
  { label: '满足 ≥2', value: 2 },
  { label: '满足 ≥3（标准）', value: 3 },
  { label: '满足 ≥4', value: 4 },
  { label: '全部满足', value: 5 },
]
/** 进入预警根数筛选（一根K线=一个周期，日K下即天数；从越过进入线那根起算；min/max=0 表示对应侧不限） */
const zoneDayPresets = [
  { label: '不限', min: 0, max: 0 },
  { label: '刚进入 1~3 天', min: 1, max: 3 },
  { label: '4~7 天', min: 4, max: 7 },
  { label: '8~15 天（持续预警）', min: 8, max: 15 },
  { label: '15 天以上（长期预警）', min: 15, max: 0 },
]
const zoneDayIdx = ref(0)
/** 预警窗口计数单位（跟随当前周期下拉）：日K=天 周K=周 分钟=根 */
const zoneUnit = computed(() => (klt.value === '101' ? '天' : klt.value === '102' ? '周' : '根'))
const zoneColLabel = computed(() => `进入预警${zoneUnit.value}数`)
const zoneDayOptions = computed(() => zoneDayPresets.map((p, i) => ({ label: p.label.replace(/天/g, zoneUnit.value), value: i })))
/** 命中行内的计数单位（跟随命中数据自身的周期，旧数据无 klt 时按日K） */
function zoneUnitOf(k) {
  if (k === '102') return '周'
  if (k === '101' || !k) return '天'
  return '根'
}

// ---- 扫描状态 ----
const progress = ref({ running: false, total: 0, scanned: 0, hits: 0, current: '', error: '' })
/** 命中结果（追加序，展示时按趋势分+MI 超线幅度降序） */
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

/** 展示结果：趋势分降序 → MI 超出进入线幅度降序 */
const visibleRows = computed(() => {
  return [...rows.value].sort((a, b) =>
    (b.trendScore - a.trendScore) || ((b.mi - b.enter) - (a.mi - a.enter)))
})

async function startScan() {
  rows.value = []
  progress.value.error = ''
  try {
    const preset = zoneDayPresets[zoneDayIdx.value] || zoneDayPresets[0]
    await StartMassZoneScan({
      markets: markets.value,
      boards: boards.value,
      klt: klt.value,
      trendChecks: trendChecks.value,
      minTrendScore: minTrendScore.value,
      minZoneBars: preset.min,
      maxZoneBars: preset.max,
    })
    progress.value = { ...progress.value, running: true, scanned: 0, hits: 0, finishedAt: 0 }
  } catch (e) {
    message.error(`启动扫描失败: ${e}`)
  }
}

function stopScan() {
  StopMassZoneScan()
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
  EventsOn('massZoneScanProgress', onProgress)
  EventsOn('massZoneScanHit', onHit)
  // 应用重启/页面刷新后，若 Go 侧扫描仍在进行，接上进度显示
  GetMassZoneScanStatus().then(onProgress).catch(() => {})
  GetConfig().then((res) => { darkTheme.value = !!res.darkTheme }).catch(() => {})
})
onBeforeUnmount(() => {
  EventsOff('massZoneScanProgress')
  EventsOff('massZoneScanHit')
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
function fmtNum(v) {
  const n = Number(v)
  return Number.isFinite(n) ? n.toFixed(2) : '—'
}

/** 导出命中标的为 Excel：弹系统保存框让用户选择保存位置，用户取消时后端返回空串 */
async function exportExcel() {
  const list = visibleRows.value
  if (!list.length) {
    message.warning('暂无可导出的命中标的')
    return
  }
  const d = new Date()
  const p2 = (x) => String(x).padStart(2, '0')
  const fname = `MASS预警扫描_${d.getFullYear()}${p2(d.getMonth() + 1)}${p2(d.getDate())}-${p2(d.getHours())}${p2(d.getMinutes())}${p2(d.getSeconds())}.xlsx`
  const cols = [
    { title: '股票名称', key: 'name' }, { title: '代码', key: 'code' }, { title: '市场', key: 'market' }, { title: '周期', key: 'kltLabel' },
    { title: '最新价', key: 'close' }, { title: '最新涨跌%', key: 'pct' },
    { title: 'MI值', key: 'mi' }, { title: '进入线', key: 'enter' }, { title: '阈值类型', key: 'adaptive' },
    { title: zoneColLabel.value, key: 'zoneBars' }, { title: '预警起始', key: 'zoneStart' },
    { title: '趋势分（满足/参与）', key: 'trendScore' }, { title: '趋势证据', key: 'trendInfo' },
    { title: 'MA20', key: 'ma20' }, { title: 'MA60', key: 'ma60' },
  ]
  const mk = { SH: '沪', SZ: '深', BJ: '京' }
  const out = list.map((r) => ({
    name: r.name || '', code: r.code || '', market: mk[r.market] || r.market || '',
    kltLabel: kltLabel[r.klt] || '日K',
    close: Number(r.close) || 0, pct: Number(r.pct) || 0,
    mi: Number(r.mi) || 0, enter: Number(r.enter) || 0,
    adaptive: r.adaptive ? '自适应阈值' : '经典阈值',
    zoneBars: Number(r.zoneBars) || 0, zoneStart: r.zoneStart || '',
    trendScore: `${r.trendScore ?? 0}/${r.trendTotal || 5}`, trendInfo: r.trendInfo || '',
    ma20: Number(r.ma20) || 0, ma60: Number(r.ma60) || 0,
  }))
  try {
    const saved = await ExportTableToXLSX(fname, { sheetName: 'MASS预警扫描', columns: cols, rows: out })
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
        <NText strong>MASS 反转预警扫描（通达信 · 多周期）</NText>
        <NFlex :size="8" align="center">
          <NButton v-if="!progress.running" size="small" type="primary" @click="startScan">开始扫描</NButton>
          <NButton v-else size="small" type="warning" @click="stopScan">停止</NButton>
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
        <div class="field" style="flex: 0.8;">
          <NText depth="3" style="font-size: 12px;">K线周期</NText>
          <NSelect v-model:value="klt" :options="kltOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1;">
          <NText depth="3" style="font-size: 12px;">趋势过滤</NText>
          <NSelect v-model:value="minTrendScore" :options="trendScoreOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1.2;">
          <NText depth="3" style="font-size: 12px;">趋势指标（不选=全部）</NText>
          <NSelect v-model:value="trendChecks" multiple clearable :options="trendCheckOptions" :z-index="10002" style="width: 100%;" />
        </div>
        <div class="field" style="flex: 1;">
          <NText depth="3" style="font-size: 12px;">进入预警{{ zoneUnit }}数</NText>
          <NSelect v-model:value="zoneDayIdx" :options="zoneDayOptions" :z-index="10002" style="width: 100%;" />
        </div>
      </NFlex>
      <NText depth="3" style="font-size: 12px; display: block; margin-top: 6px;">
        筛出正处于 MASS 反转预警窗口（MI 越过自适应进入线、尚未回落确认，与主图琥珀色预警带同口径）
        且处于多头/上升趋势（自选指标：多头排列/MA20上行/Supertrend翻多/ADX强/通道上半区，不选=全部 5 个，按满足个数过滤）的标的，
        可按K线周期（日K/分钟/周K）扫描，并按进入预警根数（日K下即天数）筛选刚进入（1~3）或长期预警（15 以上）的标的。
        MI 只提示「大波动临近、趋势可能反转」，不指方向——上升趋势中的预警宜视为止盈/减仓预警而非做空信号。
        全市场数千只，扫描需数分钟，可随时停止。
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
        <NText strong>命中标的</NText>
        <NFlex :size="8" align="center">
          <NButton size="tiny" type="primary" ghost :disabled="!visibleRows.length" @click="exportExcel">导出Excel</NButton>
          <NText depth="3" style="font-size: 12px;">{{ visibleRows.length }} 条</NText>
        </NFlex>
      </NFlex>
      <NEmpty v-if="!visibleRows.length" description="尚无命中标的，设置条件后点击「开始扫描」" style="margin-top: 20px;" />
      <div v-else class="result-table">
        <div class="rt-row rt-head">
          <div class="rt-c stock">股票</div>
          <div class="rt-c num">最新价</div>
          <div class="rt-c mi">MI / 进入线</div>
          <div class="rt-c zone">{{ zoneColLabel }}</div>
          <div class="rt-c score">趋势分</div>
          <div class="rt-c num">MA20 / MA60</div>
          <div class="rt-c op">操作</div>
        </div>
        <div v-for="r in visibleRows" :key="r.code" class="rt-row">
          <div class="rt-c stock">
            <NText type="info" style="cursor: pointer;" @click="openKline(r)">{{ r.name }}</NText>
            <span class="rt-sub">{{ r.code }}</span>
            <NTag size="tiny" :bordered="false" :type="marketTagType[r.market]">{{ marketLabel[r.market] || r.market }}</NTag>
          </div>
          <div class="rt-c num">
            <div>{{ fmtNum(r.close) }}</div>
            <span class="rt-sub" :style="{ color: pctColor(r.pct) }">{{ (r.klt === '101' || !r.klt) ? '当日' : '当前' }} {{ pctText(r.pct) }}</span>
          </div>
          <div class="rt-c mi">
            <div>{{ fmtNum(r.mi) }} / {{ fmtNum(r.enter) }}</div>
            <span class="rt-sub">{{ r.adaptive ? '自适应阈值' : '经典阈值' }}</span>
          </div>
          <div class="rt-c zone">
            <NTag size="small" :bordered="false" :type="r.zoneBars >= 15 ? 'error' : r.zoneBars >= 8 ? 'warning' : 'success'">
              进入第 {{ r.zoneBars }} {{ zoneUnitOf(r.klt) }}
            </NTag>
            <span class="rt-sub">{{ r.zoneStart }} 起</span>
          </div>
          <div class="rt-c score">
            <NTooltip trigger="hover" :z-index="10002" style="max-width: 340px;">
              <template #trigger>
                <NTag size="small" :bordered="false" :type="r.trendScore >= (r.trendTotal || 5) ? 'error' : r.trendScore >= 4 ? 'warning' : 'default'">
                  {{ r.trendScore }}/{{ r.trendTotal || 5 }}
                </NTag>
              </template>
              趋势证据：{{ r.trendInfo || '无' }}
            </NTooltip>
          </div>
          <div class="rt-c num">
            <span class="rt-sub">{{ fmtNum(r.ma20) }} / {{ fmtNum(r.ma60) }}</span>
          </div>
          <div class="rt-c op">
            <NButton size="tiny" quaternary type="info" @click="openKline(r)">K线</NButton>
          </div>
        </div>
      </div>
    </div>

    <!-- 命中标的 K 线弹窗：宽度按窗口宽、图表高度按窗口高自适应 -->
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
          :key="'mass-scan-kline-' + klineCode"
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

.rt-c.num {
  width: 96px;
}

.rt-c.mi {
  width: 118px;
}

.rt-c.zone {
  width: 150px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.rt-c.score {
  width: 56px;
}

.rt-c.op {
  width: 56px;
  text-align: right;
}

.rt-sub {
  color: rgba(128, 128, 128, 0.9);
  font-size: 11px;
}
</style>
