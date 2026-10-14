<script setup lang="ts">
// OKX USDT 永续行情榜单（独立顶层页签）：
// - 公共行情数据，无需交易密钥；后端统一缓存，模拟盘/实盘共用
// - 仅在页签激活时轮询（60 秒），切走即暂停
import {ref, watch, onBeforeUnmount} from 'vue'
import {GetOKXRankings} from '../../wailsjs/go/main/App'

const props = defineProps<{ darkTheme: boolean; active: boolean }>()

const RANK_TABS = [
  { key: 'gainers', label: '涨幅榜' },
  { key: 'losers', label: '跌幅榜' },
  { key: 'volume', label: '成交额榜' },
  { key: 'funding', label: '资金费率' },
  { key: 'longshort', label: '多空比' },
]
const rankTab = ref('gainers')
const rankItems = ref<any[]>([])
const rankLoading = ref(false)
let rankTimer: any = null

async function loadRankings() {
  rankLoading.value = true
  try {
    rankItems.value = (await GetOKXRankings(rankTab.value, 15)) || []
  } catch (e) {
    console.error('loadRankings error:', e)
  } finally {
    rankLoading.value = false
  }
}

function onRankTabChange(k: string) {
  if (rankTab.value === k) return
  rankTab.value = k
  loadRankings()
}

function fmtNum(v: any, digits = 4) {
  const n = Number(v)
  if (!Number.isFinite(n)) return '-'
  return n.toFixed(digits)
}

function fmtVolUsd(v: number): string {
  if (!Number.isFinite(v)) return '-'
  if (v >= 1e8) return (v / 1e8).toFixed(2) + '亿'
  if (v >= 1e4) return (v / 1e4).toFixed(1) + '万'
  return v.toFixed(0)
}

function fmtFundingTs(ts: number): string {
  if (!ts) return '-'
  const d = new Date(ts)
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${d.getMonth() + 1}/${d.getDate()} ${hh}:${mm}`
}

/** 榜单行辅助色：涨跌/费率正负用涨红跌绿；多空比偏离 1 越多越极端 */
function rankColor(v: number): string {
  return v > 0 ? '#e05252' : v < 0 ? '#2e9e6b' : 'inherit'
}

function startTimer() {
  if (rankTimer) return
  loadRankings()
  rankTimer = setInterval(loadRankings, 60000)
}

function stopTimer() {
  if (rankTimer) {
    clearInterval(rankTimer)
    rankTimer = null
  }
}

watch(() => props.active, (v) => {
  if (v) startTimer()
  else stopTimer()
}, {immediate: true})

onBeforeUnmount(stopTimer)
</script>

<template>
  <div class="rank-panel" :class="{ dark: props.darkTheme }">
    <div class="rank-tabs">
      <button
          v-for="t in RANK_TABS"
          :key="t.key"
          class="rank-tab-btn"
          :class="{ active: rankTab === t.key }"
          @click="onRankTabChange(t.key)"
      >{{ t.label }}</button>
      <n-spin :size="12" style="margin-left: auto; margin-right: 4px;" :show="rankLoading"/>
    </div>
    <div class="rank-scroll">
      <n-empty v-if="!rankItems.length && !rankLoading" size="small" description="暂无榜单数据"/>
      <table v-else class="rank-table">
        <thead>
          <tr>
            <th style="width: 32px;">#</th>
            <th>合约</th>
            <th style="text-align: right;">最新价</th>
            <th style="text-align: right;">24h</th>
            <th style="text-align: right;">{{ rankTab === 'funding' ? '费率' : rankTab === 'longshort' ? '多空比' : '成交额' }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in rankItems" :key="r.instId">
            <td class="idx">{{ i + 1 }}</td>
            <td class="inst" :title="r.instId">{{ r.base }}</td>
            <td class="num">{{ fmtNum(r.last, 4) }}</td>
            <td class="num" :style="{ color: rankColor(r.changePct) }">
              {{ r.changePct >= 0 ? '+' : '' }}{{ r.changePct.toFixed(2) }}%
            </td>
            <td class="num" :style="rankTab === 'funding' ? { color: rankColor(r.fundingRate) } : {}">
              <template v-if="rankTab === 'funding'">
                {{ r.fundingRate >= 0 ? '+' : '' }}{{ r.fundingRate.toFixed(4) }}%
              </template>
              <template v-else-if="rankTab === 'longshort'">
                {{ r.longShortRatio ? r.longShortRatio.toFixed(3) : '-' }}
              </template>
              <template v-else>{{ fmtVolUsd(r.volUsd) }}</template>
            </td>
            <td v-if="rankTab === 'funding'" class="num muted">{{ fmtFundingTs(r.nextFundingTs) }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="rankTab === 'funding'" class="rank-hint">按 24h 成交额前 60 合约的当期费率排序</div>
      <div v-else-if="rankTab === 'longshort'" class="rank-hint">多空持仓人数比，仅 OKX 支持的主流币种；&gt;1 散户偏多</div>
      <div v-else class="rank-hint">数据每 60 秒自动刷新</div>
    </div>
  </div>
</template>

<style scoped>
.rank-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.rank-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 0 6px;
}

.rank-tab-btn {
  border: 1px solid transparent;
  background: transparent;
  color: inherit;
  opacity: 0.65;
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 10px;
  cursor: pointer;
}

.rank-tab-btn.active {
  opacity: 1;
  border-color: currentColor;
  font-weight: 600;
}

.rank-scroll {
  flex: 1;
  overflow: auto;
}

.rank-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.rank-table th {
  position: sticky;
  top: 0;
  background: inherit;
  text-align: left;
  font-weight: 500;
  opacity: 0.6;
  padding: 3px 6px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.2);
  z-index: 1;
}

.rank-table td {
  padding: 3px 6px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.08);
}

.rank-table .idx {
  opacity: 0.5;
}

.rank-table .inst {
  font-weight: 600;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-table .num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.rank-table .muted {
  opacity: 0.55;
}

.rank-hint {
  font-size: 11px;
  opacity: 0.5;
  padding: 6px 4px 2px;
}

/* 暗色主题：表头 sticky 背景需要跟随容器底色，避免透出下层内容 */
.rank-panel.dark {
  background: #101014;
  color: rgba(255, 255, 255, 0.82);
}
</style>
