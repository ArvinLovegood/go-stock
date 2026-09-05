<script setup lang="ts">
import {ref, watch} from "vue";
import {GetBKConstituentStocks} from "../../wailsjs/go/main/App";

// 板块/概念成分股弹窗：bkFundFlowChart 与 conceptFundFlowChart 共用
const props = defineProps({
  show: {
    type: Boolean,
    default: false
  },
  bkCode: {
    type: String,
    default: ''
  },
  bkName: {
    type: String,
    default: ''
  },
  darkTheme: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:show'])

const loading = ref(false)
const stocks = ref<any[]>([])
const sortKey = ref('mainNetInflow') // 排序字段：mainNetInflow / changePercent / dealAmount

// 停牌/无数据股票 code 兼容（接口字段可能为 "-"）
function num(v: any): number {
  const n = Number(v)
  return isNaN(n) ? 0 : n
}

// 金额自适应单位：|v|≥1亿 显示 "X.XX亿"，否则 "X.XX万"
function fmtAmount(v: number): string {
  const abs = Math.abs(v)
  if (abs >= 100000000) return (v / 100000000).toFixed(2) + '亿'
  if (abs >= 10000) return (v / 10000).toFixed(2) + '万'
  return v.toFixed(0)
}

// 成交量（手）自适应单位
function fmtVolume(v: number): string {
  const abs = Math.abs(v)
  if (abs >= 100000000) return (v / 100000000).toFixed(2) + '亿手'
  if (abs >= 10000) return (v / 10000).toFixed(2) + '万手'
  return v.toFixed(0) + '手'
}

// 涨跌颜色类型
function upDownType(v: number): 'error' | 'success' | 'default' {
  if (v > 0) return 'error'
  if (v < 0) return 'success'
  return 'default'
}

// 排序后的成分股列表
const sortedStocks = ref<any[]>([])
function applySort() {
  const key = sortKey.value
  const list = [...stocks.value]
  if (key === 'changePercent') {
    list.sort((a: any, b: any) => num(b.changePercent) - num(a.changePercent))
  } else if (key === 'dealAmount') {
    list.sort((a: any, b: any) => num(b.dealAmount) - num(a.dealAmount))
  } else {
    // 默认主力净流入降序
    list.sort((a: any, b: any) => num(b.mainNetInflow) - num(a.mainNetInflow))
  }
  sortedStocks.value = list
}

async function loadStocks() {
  if (!props.bkCode) return
  loading.value = true
  stocks.value = []
  applySort()
  try {
    const res = await GetBKConstituentStocks(props.bkCode)
    stocks.value = (res || []).map((s: any) => ({
      ...s,
      price: num(s.price),
      changePercent: num(s.changePercent),
      change: num(s.change),
      volume: num(s.volume),
      dealAmount: num(s.dealAmount),
      turnoverRate: num(s.turnoverRate),
      volumeRatio: num(s.volumeRatio),
      flowMarketCap: num(s.flowMarketCap),
      totalMarketCap: num(s.totalMarketCap),
      peRatio: num(s.peRatio),
      mainNetInflow: num(s.mainNetInflow),
      mainNetInflowPct: num(s.mainNetInflowPct)
    }))
  } catch (e) {
    console.error('loadStocks error:', e)
    stocks.value = []
  } finally {
    applySort()
    loading.value = false
  }
}

watch(() => props.show, (val) => {
  if (val && props.bkCode) {
    loadStocks()
  }
})

watch(() => props.bkCode, (val) => {
  if (props.show && val) {
    loadStocks()
  }
})

watch(sortKey, () => {
  applySort()
})

function onClose() {
  emit('update:show', false)
}
</script>

<template>
  <n-modal :show="props.show" preset="card"
           :title="props.bkName + '（' + props.bkCode + '）成分股'"
           style="width: 1100px; max-width: 95vw"
           @update:show="onClose">
    <n-spin :show="loading">
      <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap;">
        <n-text :depth="3" style="font-size: 12px;">
          共 {{ sortedStocks.length }} 只成分股，数据来源于东方财富
        </n-text>
        <n-radio-group v-model:value="sortKey" size="small">
          <n-radio-button value="mainNetInflow">主力净流入</n-radio-button>
          <n-radio-button value="changePercent">涨跌幅</n-radio-button>
          <n-radio-button value="dealAmount">成交额</n-radio-button>
        </n-radio-group>
        <n-button size="small" @click="loadStocks" :loading="loading">刷新</n-button>
      </div>
      <n-table :single-line="false" striped size="small" style="max-height: 60vh; overflow-y: auto;">
        <n-thead>
          <n-tr>
            <n-th>代码</n-th>
            <n-th>名称</n-th>
            <n-th>最新价</n-th>
            <n-th>涨跌幅</n-th>
            <n-th>涨跌额</n-th>
            <n-th>成交额</n-th>
            <n-th>换手率</n-th>
            <n-th>量比</n-th>
            <n-th>流通市值</n-th>
            <n-th>总市值</n-th>
            <n-th>市盈率</n-th>
            <n-th>主力净流入</n-th>
            <n-th>主力净流入占比</n-th>
          </n-tr>
        </n-thead>
        <n-tbody>
          <n-tr v-for="item in sortedStocks" :key="item.code">
            <n-td>
              <n-tag :bordered="false" type="info" size="small">{{ item.code }}</n-tag>
            </n-td>
            <n-td>{{ item.name }}</n-td>
            <n-td>
              <n-text :type="upDownType(item.changePercent)">{{ item.price > 0 ? item.price.toFixed(2) : '-' }}</n-text>
            </n-td>
            <n-td>
              <n-text :type="upDownType(item.changePercent)">
                {{ item.changePercent > 0 ? '+' : '' }}{{ item.changePercent.toFixed(2) }}%
              </n-text>
            </n-td>
            <n-td>
              <n-text :type="upDownType(item.change)">
                {{ item.change > 0 ? '+' : '' }}{{ item.change.toFixed(2) }}
              </n-text>
            </n-td>
            <n-td>{{ fmtAmount(item.dealAmount) }}</n-td>
            <n-td>{{ item.turnoverRate > 0 ? item.turnoverRate.toFixed(2) + '%' : '-' }}</n-td>
            <n-td>{{ item.volumeRatio > 0 ? item.volumeRatio.toFixed(2) : '-' }}</n-td>
            <n-td>{{ fmtAmount(item.flowMarketCap) }}</n-td>
            <n-td>{{ fmtAmount(item.totalMarketCap) }}</n-td>
            <n-td>{{ item.peRatio > 0 ? item.peRatio.toFixed(2) : '-' }}</n-td>
            <n-td>
              <n-text :type="upDownType(item.mainNetInflow)">{{ fmtAmount(item.mainNetInflow) }}</n-text>
            </n-td>
            <n-td>
              <n-text :type="upDownType(item.mainNetInflow)">
                {{ item.mainNetInflowPct > 0 ? '+' : '' }}{{ item.mainNetInflowPct.toFixed(2) }}%
              </n-text>
            </n-td>
          </n-tr>
          <n-tr v-if="sortedStocks.length === 0 && !loading">
            <n-td colspan="13" style="text-align: center; color: #999;">暂无数据</n-td>
          </n-tr>
        </n-tbody>
      </n-table>
    </n-spin>
  </n-modal>
</template>

<style scoped>
</style>
