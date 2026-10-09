<script setup>
import {
  GetStockList,
  GetConfig,
  GetEffectiveSponsorVip,
  GetBinanceFuturesSymbols,
  GetBitgetFuturesSymbols,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import { pinyin } from 'pinyin-pro'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import { NAutoComplete, NButton, NFlex, NText, NInputGroup, NModal, NCard } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { nextTick, onBeforeMount, onMounted, onBeforeUnmount, ref } from 'vue'

const message = useMessage()
const searchQuery = ref('')
const selectedCode = ref('000001.SH')
const selectedName = ref('上证指数')
const stockList = ref([])
const options = ref([])
const darkTheme = ref(false)
const chartHeight = ref(window.innerHeight - 230)
const recentStocks = ref([])
const unsupportedCode = ref(false)
const vipLevel = ref(0)
/** 赞助码未生效原因（如"VIP 已到期/尚未生效/字段异常"），用于解释为什么等级显示为 0 */
const vipReason = ref('')
const showVipModal = ref(false)
let vipTimer = null
let stockChangeHandler = null
// 合约清单（币安 USDT-M 永续 bn: + Bitget 美股永续 bt:）：用于搜索联想与展示名解析
const contractList = ref([])
const contractNameMap = ref({})

// 合约代码识别：bn:=币安 USDT-M 永续，bt:=Bitget 美股永续
function isContractCode(code) {
  const l = String(code || '').toLowerCase()
  return l.startsWith('bn:') || l.startsWith('bt:')
}

// 由合约代码解析展示名（如 bt:AAPLUSDT → 苹果(AAPL)/USDT 美股永续）
function resolveContractName(emCode) {
  const sym = String(emCode || '').toUpperCase().replace(/^(BN|BT):/, '')
  return contractNameMap.value[sym] || ''
}

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  const lower = c.toLowerCase()
  // 合约直通：非东财体系，不做交易所后缀转换（交由行情组件按 bn:/bt: 前缀识别）
  if (lower.startsWith('bn:')) return 'bn:' + c.slice(3).toUpperCase()
  if (lower.startsWith('bt:')) return 'bt:' + c.slice(3).toUpperCase()
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6') return c + '.SH'
    if (d === '0' || d === '3') return c + '.SZ'
    if (d === '8' || d === '9') return c + '.BJ'
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

async function refreshEffectiveVip() {
  try {
    const r = await GetEffectiveSponsorVip()
    const active = !!r?.active
    const lvl = Number(r?.vipLevel ?? 0)
    vipLevel.value = active && !Number.isNaN(lvl) ? lvl : 0
    vipReason.value = active ? '' : String(r?.reason ?? '')
  } catch (_) {
    vipLevel.value = 0
    vipReason.value = ''
  }
}

function startVipCheck() {
  if (vipTimer) clearInterval(vipTimer)
  if (vipLevel.value < 2) {
    showVipModal.value = true
    vipTimer = setInterval(() => {
      showVipModal.value = true
    }, 60000)
  }
}

// 检索归一化：全角转半角，并去掉空格、连字符等分隔符，保证「连打」能命中
// 例：五 粮 液 → 五粮液、中巨芯-U → 中巨芯u、万  科Ａ → 万科a
function normalizeSearchKey(s) {
  return String(s || '')
    .replace(/[\uff01-\uff5e]/g, ch => String.fromCharCode(ch.charCodeAt(0) - 0xfee0))
    .replace(/[^\p{L}\p{N}]/gu, '')
    .toLowerCase()
}

// 繁→简（常用公司/港股名用字）：pinyin-pro 的多音词组词典只按简体建索引，
// 不转换会把「恒生銀行」的行读成 xíng（yx）而不是 háng（yh）
const TRAD_TO_SIMP = {
  銀: '银', 業: '业', 發: '发', 長: '长', 慶: '庆', 廈: '厦', 門: '门', 樂: '乐', 娛: '娱',
  東: '东', 鑰: '钥', 給: '给', 側: '侧', 祕: '秘', 魯: '鲁', 國: '国', 華: '华', 興: '兴',
  際: '际', 藥: '药', 證: '证', 險: '险', 產: '产', 團: '团', 島: '岛', 灣: '湾', 龍: '龙',
  萬: '万', 亞: '亚', 恆: '恒', 豐: '丰', 實: '实', 創: '创', 營: '营', 聯: '联', 匯: '汇',
  滙: '汇', 財: '财', 貿: '贸', 運: '运', 輸: '输', 電: '电', 鋼: '钢', 鐵: '铁', 銅: '铜',
  鋁: '铝', 輕: '轻', 車: '车', 馬: '马', 醫: '医', 學: '学', 農: '农', 縣: '县', 鎮: '镇',
  廣: '广', 寧: '宁', 寶: '宝', 麗: '丽', 潤: '润', 貴: '贵', 陽: '阳', 蘇: '苏', 閩: '闽',
  雲: '云', 煙: '烟', 臺: '台', 網: '网', 訊: '讯', 軟: '软', 數: '数', 據: '据', 機: '机',
  構: '构', 認: '认', 議: '议', 記: '记', 務: '务', 勢: '势', 億: '亿', 債: '债', 儲: '储',
  權: '权', 錢: '钱', 濟: '济', 眾: '众', 競: '竞', 總: '总', 會: '会',
}

function toSimplified(s) {
  let out = ''
  for (const ch of String(s || '')) out += TRAD_TO_SIMP[ch] || ch
  return out
}

// 股票名称拼音首字母（gzmt → 贵州茅台）：pinyin-pro 带词组词典，可正确处理多音字（银行→yh、重庆→cq）
const pinyinCache = new Map()

function initialsOf(name) {
  const key = String(name || '')
  if (!key) return ''
  if (pinyinCache.has(key)) return pinyinCache.get(key)
  let py = ''
  try {
    py = pinyin(toSimplified(key), { pattern: 'first', toneType: 'none', type: 'array' }).join('')
  } catch (_) {
    py = ''
  }
  const normalized = normalizeSearchKey(py)
  pinyinCache.set(key, normalized)
  return normalized
}

// 单条索引（归一化名称 + 拼音首字母），幂等：已建好就直接返回
function ensureItemIndex(item) {
  if (!item || item.nameKey !== undefined) return
  item.nameKey = normalizeSearchKey(item.name)
  item.pyInit = initialsOf(item.name)
}

let indexBuilt = false

// 分片建立检索索引，避免一次性转换上万条名称卡住界面
function buildPinyinIndex() {
  indexBuilt = false
  let i = 0
  const step = () => {
    const end = Math.min(i + 1500, stockList.value.length)
    for (; i < end; i++) ensureItemIndex(stockList.value[i])
    if (i < stockList.value.length) setTimeout(step, 0)
    else indexBuilt = true
  }
  step()
}

// 由关键字生成候选（股票 + 合约），供右上角搜索框与快捷搜索面板复用
function buildOptions(val) {
  if (!val || !val.trim()) return []
  // 分片建索引尚未完成时先补齐剩余条目，避免搜索落在这个时间窗口里漏检
  if (!indexBuilt) {
    for (const item of stockList.value) ensureItemIndex(item)
    indexBuilt = true
  }
  const q = val.trim().toLowerCase()
  const qn = normalizeSearchKey(val)
  const filtered = stockList.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.ts_code.toLowerCase().includes(q) ||
    (qn && ((item.nameKey && item.nameKey.includes(qn)) ||
      (item.pyInit && item.pyInit.includes(qn))))
  ).slice(0, 30)
  const opts = filtered.map(item => ({
    label: item.name + ' - ' + item.ts_code,
    value: item.ts_code,
  }))
  // 合约联想：支持 bn:/bt: 前缀或按 symbol/展示名匹配（如 aapl、btcusdt、苹果）
  const cq = q.replace(/^(bn:|bt:)/, '').toUpperCase()
  if (cq) {
    const upperQ = q.toUpperCase()
    for (const item of contractList.value) {
      if (item.sym.includes(cq) || item.name.toUpperCase().includes(upperQ)) {
        opts.push({label: item.name + ' - ' + item.code, value: item.code})
        if (opts.length >= 40) break
      }
    }
  }
  return opts
}

function findStockList(val) {
  options.value = buildOptions(val)
}

function handleSearch(value) {
  const emCode = toEastMoneyCode(value)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  // 合约：直接用合约展示名，跳过股票名称解析
  if (isContractCode(emCode)) {
    selectedName.value = resolveContractName(emCode) || value
    addToRecent(value, selectedName.value)
    return
  }
  // 名称解析：先按 ts_code 全等；手输纯数字代码时（如 600086）按「数字+交易所后缀」匹配
  // （ts_code 带后缀直接全等匹配不上，会导致 ST 等名称信号丢失、涨跌停档位退化为默认 10%）
  const digits = String(value || '').replace(/\D/g, '')
  const suffix = emCode.includes('.') ? emCode.split('.').pop() : ''
  const found = stockList.value.find(item => item.ts_code === value)
    || (digits.length === 6 && suffix
      ? stockList.value.find(item => {
        const tc = String(item.ts_code || '')
        return tc.endsWith('.' + suffix) && tc.replace(/\D/g, '') === digits
      })
      : undefined)
  selectedName.value = found ? found.name : ''
  addToRecent(value, selectedName.value)
}

// 回车/点放大镜：有候选优先取第一条，避免把拼音/名称文本当成美股代码（如 gzmt → GZMT.US）
function submitSearch() {
  if (options.value.length) {
    handleSearch(options.value[0].value)
    return
  }
  handleSearch(searchQuery.value)
}

function addToRecent(code, name) {
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name })
  if (list.length > 5) list.length = 5
  recentStocks.value = list
  try {
    localStorage.setItem('kline-recent-stocks', JSON.stringify(list))
  } catch {}
}

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) recentStocks.value = JSON.parse(raw).slice(0, 5)
  } catch {}
}

function selectRecent(code, name) {
  const emCode = toEastMoneyCode(code)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  selectedName.value = isContractCode(emCode) ? (resolveContractName(emCode) || name) : name
  addToRecent(code, selectedName.value)
}

// 快捷搜索面板：任意键盘输入直接唤起，↑↓ 选择、Enter 切换、Esc 关闭
const quickSearchVisible = ref(false)
const quickSearchQuery = ref('')
const quickOptions = ref([])
const quickSearchIndex = ref(0)
const quickInputRef = ref(null)

function refreshQuickOptions() {
  const q = quickSearchQuery.value
  quickOptions.value = q && q.trim() ? buildOptions(q).slice(0, 10) : []
  if (quickSearchIndex.value >= quickOptions.value.length) quickSearchIndex.value = 0
}

function openQuickSearch(initial) {
  quickSearchQuery.value = initial || ''
  quickSearchIndex.value = 0
  refreshQuickOptions()
  quickSearchVisible.value = true
  nextTick(() => {
    if (quickInputRef.value) quickInputRef.value.focus()
  })
}

function closeQuickSearch() {
  quickSearchVisible.value = false
  quickSearchQuery.value = ''
  quickOptions.value = []
  quickSearchIndex.value = 0
}

function onQuickInput(e) {
  quickSearchQuery.value = e.target.value
  refreshQuickOptions()
}

function moveQuick(step) {
  const n = quickOptions.value.length
  if (!n) return
  quickSearchIndex.value = (quickSearchIndex.value + step + n) % n
}

function pickQuick(opt) {
  handleSearch(opt.value)
  closeQuickSearch()
}

function onQuickKeydown(e) {
  if (e.key === 'Escape') {
    closeQuickSearch()
    e.preventDefault()
  } else if (e.key === 'ArrowDown') {
    moveQuick(1)
    e.preventDefault()
  } else if (e.key === 'ArrowUp') {
    moveQuick(-1)
    e.preventDefault()
  } else if (e.key === 'Enter') {
    const opt = quickOptions.value[quickSearchIndex.value]
    if (opt) pickQuick(opt)
    e.preventDefault()
  }
}

// 全局键盘监听：焦点不在输入框时，敲任意可见字符即唤起快捷搜索
function onGlobalKeydown(e) {
  if (quickSearchVisible.value || showVipModal.value) return
  if (e.ctrlKey || e.metaKey || e.altKey) return
  const t = e.target
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
  const k = e.key
  if (!k || k.length !== 1 || k === ' ') return
  e.preventDefault()
  openQuickSearch(k)
}

function updateChartHeight() {
  chartHeight.value = Math.max(400, window.innerHeight - 230)
}

onBeforeMount(() => {
  GetStockList('').then(result => {
    stockList.value = result || []
    buildPinyinIndex()
  }).catch(err => { console.error('GetStockList error:', err) })
  GetConfig().then(result => {
    darkTheme.value = !!result.darkTheme
  }).catch(err => { console.error('GetConfig error:', err) })
  // 合约清单：仅用于搜索联想与展示名，失败不影响股票K线
  Promise.all([GetBitgetFuturesSymbols(), GetBinanceFuturesSymbols()]).then(([bt, bn]) => {
    const list = []
    const nameMap = {}
    for (const s of bt || []) {
      const sym = String(s.symbol || '').toUpperCase()
      const name = s.displayName || sym
      list.push({sym, code: 'bt:' + sym, name})
      nameMap[sym] = name
    }
    for (const s of bn || []) {
      const sym = String(s.symbol || '').toUpperCase()
      const name = s.displayName || sym
      list.push({sym, code: 'bn:' + sym, name})
      nameMap[sym] = name
    }
    contractList.value = list
    contractNameMap.value = nameMap
  }).catch(err => { console.error('Get contract symbols error:', err) })
})

onMounted(async () => {
  loadRecentStocks()
  updateChartHeight()
  window.addEventListener('resize', updateChartHeight)
  window.addEventListener('keydown', onGlobalKeydown)

  await refreshEffectiveVip()
  startVipCheck()

  stockChangeHandler = (data) => {
    if (data && data.ts_code) {
      const emCode = toEastMoneyCode(data.ts_code)
      if (!emCode) {
        unsupportedCode.value = true
        return
      }
      unsupportedCode.value = false
      selectedCode.value = emCode
      selectedName.value = isContractCode(emCode)
        ? (resolveContractName(emCode) || data.name || '')
        : (data.name || '')
      addToRecent(data.ts_code, selectedName.value)
    }
  }
  EventsOn('klineSelectStock', stockChangeHandler)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateChartHeight)
  window.removeEventListener('keydown', onGlobalKeydown)
  if (vipTimer) {
    clearInterval(vipTimer)
    vipTimer = null
  }
})
</script>

<template>
  <div class="kline-analysis-page" :class="{ 'kline-analysis-page--dark': darkTheme }">
    <div class="kline-title-bar">
      <div v-if="recentStocks.length" class="kline-recent-bar">
        <NText depth="3" class="kline-recent-label">最近:</NText>
        <n-button
          v-for="s in recentStocks"
          :key="s.code"
          size="tiny"
          secondary
          class="kline-recent-tag"
          :title="s.name ? s.name + ' ' + s.code : s.code"
          @click="selectRecent(s.code, s.name)"
        >
          {{ s.name || s.code }}
        </n-button>
      </div>
      <div class="kline-title-text">
        <NText :depth="darkTheme ? 1 : 3" style="font-size: 15px; font-weight: 700">{{ selectedName }}&nbsp;</NText>
        <NText depth="3" style="font-size: 13px">{{ selectedCode }}</NText>
      </div>
      <div class="kline-search-bar">
        <n-input-group>
          <n-auto-complete
            v-model:value="searchQuery"
            :options="options"
            placeholder="名称/代码/拼音首字母(如 gzmt)，或合约 bn:btcusdt、bt:aaplusdt"
            clearable
            size="small"
            :on-select="handleSearch"
            @update:value="findStockList"
            @keyup.enter="submitSearch"
          />
          <n-button type="primary" size="small" @click="submitSearch">
            🔍
          </n-button>
        </n-input-group>
        <NFlex v-if="unsupportedCode" align="center" :size="6" style="margin-top: 4px">
          <NText type="warning" style="font-size: 12px">该代码暂不支持K线图</NText>
        </NFlex>
      </div>
    </div>
    <StockLightweightKlineChart
      :key="selectedCode"
      :code="selectedCode"
      :stockName="selectedName"
      :darkTheme="darkTheme"
      :chartHeight="chartHeight"
      :realtimeIntervalMs="60000"
    />

    <div v-if="quickSearchVisible" class="quick-search" :class="{ 'quick-search--dark': darkTheme }">
      <input
        ref="quickInputRef"
        class="quick-search-input"
        type="text"
        spellcheck="false"
        placeholder="输入名称/代码/拼音首字母快速切换"
        :value="quickSearchQuery"
        @input="onQuickInput"
        @keydown="onQuickKeydown"
      />
      <div class="quick-search-hint">↑↓ 选择 · Enter 切换 · Esc 关闭</div>
      <div v-if="quickOptions.length" class="quick-search-list">
        <div
          v-for="(o, i) in quickOptions"
          :key="o.value"
          class="quick-search-item"
          :class="{ 'quick-search-item--active': i === quickSearchIndex }"
          @mouseenter="quickSearchIndex = i"
          @click="pickQuick(o)"
        >
          {{ o.label }}
        </div>
      </div>
      <div v-else class="quick-search-empty">无匹配结果</div>
    </div>

    <n-modal v-model:show="showVipModal" :close-on-esc="true" :mask-closable="true" :z-index="9999">
      <n-card style="max-width: 440px; border-radius: 16px; padding: 24px" :theme-overrides="darkTheme ? { color: '#1e1e1e', textColor: '#e2e8f0' } : {}" role="dialog" aria-modal="true">
        <NFlex vertical align="center" :size="20">
          <NText style="font-size: 40px">🌟</NText>
          <NText :depth="darkTheme ? 1 : 3" style="font-size: 17px; font-weight: 700">K线技术分析 · VIP专属功能</NText>
          <NText depth="3" style="font-size: 13px; text-align: center; line-height: 2">
            K线技术分析为 <NText type="warning" style="font-weight:600">VIP2</NText> 及以上赞助用户专属功能<br/>
            当前等级：<NText type="warning" style="font-weight:600">VIP{{ vipLevel }}</NText>
          </NText>
          <NText v-if="vipReason" type="error" style="font-size: 13px; text-align: center; line-height: 2">
            赞助码状态：{{ vipReason }}
          </NText>
          <NText depth="3" style="font-size: 12px; text-align: center; line-height: 2; color: #888">
            开源不易，您的赞助是对作者最大的鼓励，也是项目持续迭代的动力 ❤️<br/>
            前往「关于」页面了解赞助详情，升级后即可解锁完整功能。
          </NText>
          <NButton type="primary" size="large" round style="width: 200px; margin-top: 4px" @click="showVipModal = false">
            我知道了
          </NButton>
        </NFlex>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.kline-analysis-page {
  width: 100%;
  padding: 4px 8px;
  box-sizing: border-box;
  --wails-draggable: no-drag;
  position: relative;
}
.kline-analysis-page--dark {
  background: #0a0a0a;
  color: #e2e8f0;
}
/* 左中右三栏：左右等宽，名称/代码标题恒定居中（flex 下会被较宽的一侧挤偏） */
.kline-title-bar {
  /* 上/右留白，避免搜索框贴边 */
  padding: 12px 12px 8px 0;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
}
/* 左上角最近访问：最多 5 个「名称 代码」小标签，过长整体省略 */
.kline-recent-bar {
  grid-column: 1;
  justify-self: start;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
}
.kline-recent-label {
  flex: 0 0 auto;
  font-size: 11px;
  white-space: nowrap;
}
.kline-recent-bar .kline-recent-tag {
  flex: 0 0 auto;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* 名称/代码标题居中，过长省略 */
.kline-title-text {
  grid-column: 2;
  min-width: 0;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.kline-search-bar {
  grid-column: 3;
  justify-self: end;
  position: relative;
  z-index: 10;
  width: 320px;
}
/* 快捷搜索面板：任意键盘输入唤起 */
.quick-search {
  position: fixed;
  top: 56px;
  left: 16px;
  width: 380px;
  max-width: calc(100vw - 32px);
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.14);
  z-index: 3000;
  overflow: hidden;
}
.quick-search--dark {
  background: #1a1a1a;
  border-color: #333;
}
.quick-search-input {
  display: block;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px 6px;
  border: none;
  outline: none;
  background: transparent;
  color: inherit;
  font-size: 13px;
}
.quick-search-input::placeholder {
  color: #aaa;
}
.quick-search--dark .quick-search-input::placeholder {
  color: #666;
}
.quick-search-hint {
  padding: 0 12px 8px;
  font-size: 11px;
  color: #999;
}
.quick-search-list {
  max-height: 300px;
  overflow-y: auto;
  border-top: 1px solid #f0f0f0;
}
.quick-search--dark .quick-search-list {
  border-top-color: #333;
}
.quick-search-item {
  padding: 8px 12px;
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.quick-search-item--active {
  background: #e8f2ff;
}
.quick-search--dark .quick-search-item--active {
  background: #14314f;
}
.quick-search-empty {
  padding: 12px;
  border-top: 1px solid #f0f0f0;
  font-size: 12px;
  color: #999;
  text-align: center;
}
.quick-search--dark .quick-search-empty {
  border-top-color: #333;
}
</style>
