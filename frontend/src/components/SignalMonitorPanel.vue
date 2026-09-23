<script setup>
/**
 * 后台买卖点信号监控面板：监控池维护、灵敏度/周期设置、提醒渠道、信号流水。
 *
 * 引擎（kline/signalMonitor.ts）跟随应用生命周期常驻；本组件只负责交互与展示，
 * 抽屉常驻渲染（与 AI 助手抽屉同款做法），关闭态仅隐藏不销毁，避免重开时重建 DOM。
 */
import { computed, nextTick, onBeforeMount, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NButton, NCard, NCheckbox, NCheckboxGroup, NDatePicker, NEmpty, NFlex, NIcon, NInput, NModal,
  NPagination, NPopconfirm, NScrollbar, NSelect, NSwitch, NTag, NText, NTooltip, useMessage,
} from 'naive-ui'
import { CloseOutline, PulseOutline, StatsChartOutline } from '@vicons/ionicons5'
import { GetConfig, GetFollowList } from '../../wailsjs/go/main/App'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { BUY_SELL_SCORE_OPTIONS } from './kline/constants'
import { alertSpeechAvailable, primeAlertSpeech, speakAlertText } from './kline/alertSound'
import {
  SIGNAL_CHANNEL_OPTIONS, SIGNAL_FAMILY_OPTIONS, SIGNAL_INTERVAL_OPTIONS, SIGNAL_POOL_LIMIT, SIGNAL_PAGE_SIZE_OPTIONS,
  SIGNAL_STATS_PRESETS, addPoolEntry, canUseSignalMonitor, clearPool, clearSignals, entryKlts, formatSignalTime,
  loadSignalStats, persistMonitorSettings, querySignals, removePoolEntry, runSignalTick, setEntryKlts, setMonitorEnabled,
  setSignalFilter, setSignalPage, setStatsCustomRange, setStatsPreset, signalMonitorState, startSignalMonitor,
} from './kline/signalMonitor'

const message = useMessage()
const visible = ref(false)
const manualCode = ref('')
const pickCode = ref(null)
const followOptions = ref([])
/** 流水筛选的输入态：回车或点「查询」后才提交给后端，避免每敲一个字就查一次库 */
const keywordInput = ref('')
const rangeInput = ref(null)
/** 收益统计的自定义区间输入态（毫秒），与快捷区间互斥 */
const statsRangeInput = ref(null)
/** K 线弹窗：点击信号行的「K线」按钮时打开，代码已转为东方财富格式 */
const klineShow = ref(false)
const klineCode = ref('')
const klineName = ref('')
const darkTheme = ref(false)

/** 应用窗口高度：K 线弹窗的图表高度按它自适应 */
const winHeight = ref(typeof window !== 'undefined' ? window.innerHeight : 800)

/**
 * K 线图表高度。组件内除图表外还有工具条/图例/提示行等固定开销（且会随数据加载变化），
 * 用固定值估算必然对不上——算小了留白、算大了弹窗出现滚动条。
 * 这里按「实测卡片高度」反推：卡片比预算高就缩、比预算矮就长，一次收敛到刚好铺满。
 */
const KLINE_CARD_BUDGET_RATIO = 0.94
const KLINE_MIN_CHART_PX = 320
const klineChartHeight = ref(Math.max(420, winHeight.value - 230))
const klineWrapRef = ref(null)
let klineResizeObserver = null

function fitKlineChart() {
  const el = klineWrapRef.value
  const card = el && el.closest ? el.closest('.n-card') : null
  if (!card) return
  const budget = Math.round(winHeight.value * KLINE_CARD_BUDGET_RATIO)
  const next = Math.max(KLINE_MIN_CHART_PX, klineChartHeight.value + (budget - card.offsetHeight))
  // 4px 阈值：避免与 ResizeObserver 互相触发形成抖动
  if (Math.abs(next - klineChartHeight.value) > 4) klineChartHeight.value = next
}

function onWinResize() {
  winHeight.value = window.innerHeight
  fitKlineChart()
}

// 弹窗打开后（内容已渲染）量一次并对后续尺寸变化保持跟随
async function attachKlineFit(attempt = 0) {
  await nextTick()
  const el = klineWrapRef.value
  // 模态内容是懒渲染的，偶发一帧内还拿不到元素，重试几帧即可
  if (!el) {
    if (attempt < 5) requestAnimationFrame(() => attachKlineFit(attempt + 1))
    return
  }
  fitKlineChart()
  if (typeof ResizeObserver === 'undefined') return
  if (klineResizeObserver) klineResizeObserver.disconnect()
  // 组件内的工具条/信号汇总会随数据加载变高，观测后再校正一次
  klineResizeObserver = new ResizeObserver(() => fitKlineChart())
  klineResizeObserver.observe(el)
}

watch(klineShow, (v) => {
  if (v) {
    attachKlineFit()
    return
  }
  if (klineResizeObserver) klineResizeObserver.disconnect()
  klineResizeObserver = null
})

const state = signalMonitorState

const kltOptions = SIGNAL_INTERVAL_OPTIONS.map((it) => ({ label: it.label, value: it.klt }))
const scoreOptions = BUY_SELL_SCORE_OPTIONS.map((o) => ({ label: o.label, value: o.value }))

const unread = computed(() => state.newCount)
const activeCount = computed(() => (state.enabled ? state.pool.length : 0))

const sourceText = computed(() => {
  if (!state.pool.length) return '监控池为空，请先添加股票'
  if (!state.enabled) return `已配置 ${state.pool.length} 只，监控未开启`
  return `监控中 · ${state.pool.length} 只`
})

const lastTickText = computed(() => {
  if (!state.lastTickAt) return '尚未扫描'
  const d = new Date(state.lastTickAt)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  const ss = String(d.getSeconds()).padStart(2, '0')
  return `${hh}:${mm}:${ss}${state.lastTickCount ? ` · 命中 ${state.lastTickCount} 条` : ''}`
})

function labelOf(s) {
  if (s.family === 'tema') return s.kind === 'buy' ? 'T确买' : 'T确卖'
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
  const a = dayLabel(state.statsFrom)
  if (!a) return '—'
  const b = dayLabel(state.statsTo)
  return a === b ? a : `${a} ~ ${b}`
})

/** 切到快捷区间：清掉自定义输入态，避免两处状态同时亮着 */
function onStatsPreset(days) {
  statsRangeInput.value = null
  setStatsPreset(days)
}

/** 自定义区间：清空时回到「今日」 */
function onStatsDateRange(range) {
  statsRangeInput.value = Array.isArray(range) ? range : null
  if (Array.isArray(range) && range[0] && range[1]) {
    setStatsCustomRange(Number(range[0]), Number(range[1]))
    return
  }
  setStatsPreset(1)
}

/**
 * 打开/关闭面板。打开前实时校验 VIP（后端同步本地解密，微秒级，不影响打开速度），
 * 权限不足时提示并保持关闭，避免用户进了面板才发现用不了。
 */
async function togglePanel() {
  if (!visible.value && !(await ensureVip())) return
  visible.value = !visible.value
}

/** VIP 门槛校验：权限不足时给出统一的引导提示 */
async function ensureVip() {
  if (await canUseSignalMonitor()) return true
  message.warning('后台买卖点信号监控仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
  return false
}

/** 总开关：开启前先校验权限，拒绝时不改状态（开关受控于 state.enabled，不会误亮） */
async function onToggleMonitor(v) {
  if (v && !(await ensureVip())) return
  setMonitorEnabled(v)
}

/**
 * 语音报名称开关。开启时借这次点击的手势预热语音合成器（自动播放策略要求），
 * 并在语音列表异步加载完成后再确认一次，避免「开了却没声」。
 */
function onToggleVoice(v) {
  state.voice = v
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
async function applyFilter(range) {
  const r = Array.isArray(range) || range === null ? range : rangeInput.value
  const list = Array.isArray(r) ? r : []
  await setSignalFilter({ keyword: keywordInput.value, start: list[0] || null, end: list[1] || null })
}

function resetFilter() {
  keywordInput.value = ''
  rangeInput.value = null
  setSignalFilter({ keyword: '', start: null, end: null })
}

function onPageChange(p) {
  setSignalPage(p)
}

function onPageSizeChange(ps) {
  setSignalPage(1, ps)
}

// 打开时清未读并补一次收益统计；关闭时刷新回第 1 页，避免下次打开还停在上次的翻页结果
watch(visible, (v) => {
  if (v) {
    state.newCount = 0
    loadSignalStats()
    return
  }
  querySignals({ resetPage: true })
})

// 面板开着时新信号已直接并入列表，不必再累计未读角标
watch(
  () => state.newCount,
  (n) => {
    if (n && visible.value) state.newCount = 0
  },
)

function closePanel() {
  visible.value = false
}

async function scanNow() {
  if (!(await ensureVip())) return
  if (!state.pool.length) {
    message.warning('监控池为空，请先添加股票')
    return
  }
  if (!state.enabled) {
    message.warning('请先开启后台监控')
    return
  }
  await runSignalTick('manual')
  if (state.lastError) message.error(`扫描异常：${state.lastError}`)
  else if (!state.lastTickCount) message.info('本轮未发现新信号（非交易时段不会扫描）')
}

/**
 * 信号里的代码可能是 sh600519 / 600519（手输）或 600519.SH（自选股），
 * K 线组件的 code 需要东方财富格式（600519.SH），这里统一归一化。
 */
function toChartCode(code) {
  const c = String(code || '').trim()
  if (!c) return ''
  if (/\.(SH|SZ|BJ|HK|US|SS|CSI)$/i.test(c)) return c.toUpperCase()
  if (/^100\.[A-Za-z]+$/.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
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
  const r = addPoolEntry(code, name)
  if (!r.ok) message.warning(r.msg)
  return r.ok
}

function onAddManual() {
  const c = manualCode.value.trim()
  if (!c) return
  if (onAddCode(c)) manualCode.value = ''
}

function onPickFollow(code) {
  if (!code) return
  const opt = followOptions.value.find((o) => o.value === code)
  onAddCode(code, opt ? opt.name : '')
  // 选择器只当「添加」用，选完即复位，避免与监控池状态耦合
  pickCode.value = null
}

onBeforeMount(() => {
  // K 线弹窗跟随应用主题（与 AI 助手取法一致）
  GetConfig()
    .then((cfg) => { darkTheme.value = !!(cfg && cfg.darkTheme) })
    .catch(() => { /* 读取失败按浅色渲染 */ })
})

onMounted(() => {
  window.addEventListener('resize', onWinResize)
  startSignalMonitor()
  GetFollowList(0).then((list) => {
    followOptions.value = (list || [])
      .map((it) => {
        const code = it.StockCode || it.stockCode || ''
        const name = it.Name || it.StockName || it.name || ''
        return { value: code, name, label: `${name} ${code}`.trim() }
      })
      .filter((o) => o.value)
  }).catch(() => { /* 自选股拉取失败不影响手输代码 */ })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onWinResize)
  if (klineResizeObserver) klineResizeObserver.disconnect()
  klineResizeObserver = null
})

// 面板上的所有设置变更都落盘
watch(
  () => [state.enabled, state.klt, state.minScore, state.families, state.channels, state.sound, state.voice, state.pool.length],
  () => persistMonitorSettings(),
  { deep: true },
)
</script>

<template>
  <div
    class="edge-trigger"
    :class="{ 'edge-trigger-on': activeCount > 0 }"
    :title="activeCount > 0 ? `信号监控中（${state.pool.length} 只）` : '买卖点信号监控'"
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
              <NButton size="small" quaternary :loading="state.running" @click="scanNow">立即扫描</NButton>
              <NButton size="small" quaternary circle title="关闭" @click="closePanel">
                <template #icon>
                  <NIcon :component="CloseOutline" />
                </template>
              </NButton>
            </div>
          </div>
        </template>

        <NScrollbar class="panel-body">
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
                </NTooltip>
              </NFlex>
              <NSwitch :value="state.enabled" @update:value="onToggleMonitor" />
            </NFlex>
            <NFlex :size="6" align="center" style="margin-top: 6px;">
              <NText depth="3" style="font-size: 12px;">{{ sourceText }}</NText>
              <NText depth="3" style="font-size: 12px;">· {{ lastTickText }}</NText>
            </NFlex>
            <NText v-if="state.lastError" type="error" style="font-size: 12px;">{{ state.lastError }}</NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NText strong>监控池</NText>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">{{ state.pool.length }}/{{ SIGNAL_POOL_LIMIT }}</NText>
                <NButton v-if="state.pool.length" size="tiny" quaternary @click="clearPool">清空</NButton>
              </NFlex>
            </NFlex>
            <NFlex :size="6" align="center" style="margin-top: 6px;" :wrap="false">
              <NSelect
                v-model:value="pickCode"
                :options="followOptions"
                filterable
                clearable
                placeholder="从自选股选择"
                :z-index="10002"
                style="width: 220px;"
                @update:value="onPickFollow"
              />
              <NInput
                v-model:value="manualCode"
                placeholder="或输入代码，如 sh600519"
                style="flex: 1; min-width: 160px;"
                @keyup.enter="onAddManual"
              />
              <NButton size="small" @click="onAddManual">添加</NButton>
            </NFlex>
            <div v-if="state.pool.length" class="pool-list">
              <div v-for="e in state.pool" :key="e.code" class="pool-row">
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
                  :value="entryKlts(e)"
                  :options="kltOptions"
                  :max-tag-count="2"
                  :z-index="10002"
                  placeholder="继承默认周期"
                  style="flex: 1; min-width: 120px;"
                  @update:value="(v) => setEntryKlts(e.code, v)"
                />
                <NButton size="tiny" quaternary circle title="移出监控池" @click="removePoolEntry(e.code)">
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
            <NText v-else depth="3" style="font-size: 12px;">与自选股解耦的独立列表</NText>
          </div>

          <div class="section">
            <NText strong>检测参数</NText>
            <NFlex :size="8" align="center" style="margin-top: 6px;" :wrap="false">
              <div class="field">
                <NText depth="3" style="font-size: 12px;">默认周期</NText>
                <NSelect v-model:value="state.klt" :options="kltOptions" :z-index="10002" style="width: 100%;" />
              </div>
              <div class="field">
                <NText depth="3" style="font-size: 12px;">灵敏度</NText>
                <NSelect v-model:value="state.minScore" :options="scoreOptions" :z-index="10002" style="width: 100%;" />
              </div>
            </NFlex>
            <NCheckboxGroup v-model:value="state.families" style="margin-top: 8px;">
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
                <NSwitch v-model:value="state.sound" size="small" />
                <NTooltip trigger="hover" :z-index="10002" style="max-width: 340px">
                  <template #trigger>
                    <NText depth="3" style="font-size: 12px; cursor: help;">语音报名称</NText>
                  </template>
                  短音放完再念一句「股票名称 + 方向」，多只票同时命中时不用回到屏幕也能分辨是哪一只。
                  依赖系统中文语音包（Windows 可在「时间和语言 → 语言」中添加中文语音）；
                  开启后连播间隔会放宽。关闭提示音时语音不单独启用；打开开关时会试听一句示例。
                </NTooltip>
                <NSwitch :value="state.voice" :disabled="!state.sound" size="small" @update:value="onToggleVoice" />
              </NFlex>
            </NFlex>
            <NCheckboxGroup v-model:value="state.channels" style="margin-top: 6px;">
              <NFlex :size="14">
                <NCheckbox v-for="o in SIGNAL_CHANNEL_OPTIONS" :key="o.value" :value="o.value" :label="o.label" />
              </NFlex>
            </NCheckboxGroup>
            <NText depth="3" style="font-size: 12px;">
              同一轮命中的多只票会合并成一条消息推送；提示音按信号依次连播{{ state.sound && state.voice ? '，每段短音后念出股票名称' : '' }}
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
                  每笔按信号价满仓计算，累计收益为各笔收益率之和；未配到卖点的买点记为「持仓中」，不计入胜率。
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
                :type="state.statsRange.days === p.value ? 'primary' : 'default'"
                :quaternary="state.statsRange.days !== p.value"
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
                <NText strong :style="{ color: returnColor(state.stats.overall.return) }">
                  {{ pct(state.stats.overall.return) }}
                </NText>
              </div>
              <div class="stat-block">
                <NText depth="3" style="font-size: 12px;">胜率</NText>
                <NText strong>{{ winRateText(state.stats.overall) }}</NText>
              </div>
              <div class="stat-block">
                <NText depth="3" style="font-size: 12px;">笔数</NText>
                <NText strong>{{ state.stats.overall.trades }}</NText>
              </div>
              <div v-if="state.stats.overall.open" class="stat-block">
                <NText depth="3" style="font-size: 12px;">持仓中</NText>
                <NText strong>{{ state.stats.overall.open }}</NText>
              </div>
            </NFlex>

            <div v-if="state.stats.stocks.length" class="stat-list">
              <!-- 与信号流水同款：名称靠左，数据靠右，行内 space-between -->
              <div v-for="it in state.stats.stocks" :key="it.code" class="stat-row">
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
            <NText v-else depth="3" style="font-size: 12px;">该区间内暂无买卖点配对成交</NText>
          </div>

          <div class="section">
            <NFlex justify="space-between" align="center">
              <NText strong>信号流水</NText>
              <NFlex :size="6" align="center">
                <NText depth="3" style="font-size: 12px;">共 {{ state.total }} 条</NText>
                <NPopconfirm v-if="state.total" :z-index="10002" @positive-click="clearSignals">
                  <template #trigger>
                    <NButton size="tiny" quaternary>清空</NButton>
                  </template>
                  确认清空全部信号流水？不限于当前筛选结果，且不可恢复
                </NPopconfirm>
              </NFlex>
            </NFlex>

            <!-- 查询条件：关键词匹配代码/名称，时间区间按命中时刻筛，两者同排 -->
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
                <NButton size="small" type="primary" :loading="state.loading" style="flex: none;" @click="applyFilter">查询</NButton>
                <NButton size="small" quaternary style="flex: none;" @click="resetFilter">重置</NButton>
              </NFlex>
            </div>

            <NEmpty
              v-if="!state.signals.length"
              size="small"
              :description="state.loading ? '查询中…' : '暂无信号'"
              style="margin-top: 8px;"
            />
            <div v-else class="signal-list">
              <div v-for="s in state.signals" :key="s.id" class="signal-row">
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
                    <span v-if="s.score != null"> · {{ Math.round((s.score / 9) * 100) }}%</span>
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
            <div v-if="state.total > 0" class="pager">
              <NPagination
                :page="state.page"
                :page-size="state.pageSize"
                :item-count="state.total"
                :page-sizes="SIGNAL_PAGE_SIZE_OPTIONS"
                show-size-picker
                size="small"
                @update:page="onPageChange"
                @update:page-size="onPageSizeChange"
              />
            </div>
          </div>
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
    style="width: 92vw; max-width: 92vw; box-sizing: border-box"
    :content-style="{
      overflow: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
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