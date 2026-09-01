<template>
  <div style="text-align: left">
    <n-form :model="searchForm">
      <n-grid :cols="24" :x-gap="24">
        <n-form-item-gi :span="6" label="日期" path="dateValue" label-placement="left">
          <n-date-picker v-model:formatted-value="searchForm.dateValue" value-format="yyyy-MM-dd" type="date"
                         :on-update:value="(v, v2) => fetch(v2)"/>
        </n-form-item-gi>
        <n-form-item-gi :span="12" label="" label-placement="left">
          <n-text type="error">*汇总当日龙虎榜全部上榜个股的席位明细，按游资/机构聚合；首次查询较慢（约几秒），结果缓存 10 分钟</n-text>
        </n-form-item-gi>
      </n-grid>
    </n-form>

    <n-spin :show="loading">
      <n-tabs type="segment" animated>
        <!-- 游资动向 -->
        <n-tab-pane name="hotMoney" :tab="`游资动向（${summary?.hotMoneyActivities?.length || 0}）`">
          <template v-if="summary?.hotMoneyActivities?.length">
            <n-table :single-line="false" striped size="small">
              <n-thead>
                <n-tr>
                  <n-th width="120px">游资</n-th>
                  <n-th width="150px">梯队</n-th>
                  <n-th>当日操作（股票 / 涨跌幅 / 买卖净额）</n-th>
                  <n-th width="110px">合计买入</n-th>
                  <n-th width="110px">合计卖出</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="act in summary.hotMoneyActivities" :key="act.hotMoneyName">
                  <n-td>
                    <n-tag :bordered="false" type="error" size="small">{{ act.hotMoneyName }}</n-tag>
                  </n-td>
                  <n-td>
                    <n-text depth="3" style="font-size: 12px" :title="act.style + (act.riskLevel ? `｜风险:${act.riskLevel}` : '')">
                      {{ act.tier || '-' }}
                    </n-text>
                  </n-td>
                  <n-td>
                    <div v-for="(s, i) in act.stocks" :key="i" style="margin: 2px 0">
                      <n-text :type="s.changeRate >= 0 ? 'error' : 'success'" style="font-weight: bold">{{ s.stockName }}</n-text>
                      <n-text depth="3" style="margin: 0 6px">{{ s.stockCode }}</n-text>
                      <n-tag :bordered="false" size="tiny" :type="s.changeRate >= 0 ? 'error' : 'success'">
                        {{ (s.changeRate >= 0 ? '+' : '') + s.changeRate.toFixed(2) }}%
                      </n-tag>
                      <n-tag v-if="s.buy > 0" :bordered="false" size="tiny" type="error" style="margin-left: 6px">买 {{ fmtAmount(s.buy) }}</n-tag>
                      <n-tag v-if="s.sell > 0" :bordered="false" size="tiny" type="success" style="margin-left: 4px">卖 {{ fmtAmount(s.sell) }}</n-tag>
                    </div>
                  </n-td>
                  <n-td><n-text type="error">{{ fmtAmount(act.totalBuy) }}</n-text></n-td>
                  <n-td><n-text type="success">{{ fmtAmount(act.totalSell) }}</n-text></n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </template>
          <n-empty v-else description="当日无游资上榜（或名录未收录）" style="padding: 40px 0"/>
        </n-tab-pane>

        <!-- 机构动向 -->
        <n-tab-pane name="institution" :tab="`机构动向（${summary?.institutionActions?.length || 0}）`">
          <template v-if="summary?.institutionActions?.length">
            <n-table :single-line="false" striped size="small">
              <n-thead>
                <n-tr>
                  <n-th width="100px">代码</n-th>
                  <n-th width="120px">名称</n-th>
                  <n-th width="90px">涨跌幅</n-th>
                  <n-th width="90px">买方机构数</n-th>
                  <n-th width="90px">卖方机构数</n-th>
                  <n-th width="110px">机构买入</n-th>
                  <n-th width="110px">机构卖出</n-th>
                  <n-th width="110px">机构净额</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="ia in summary.institutionActions" :key="ia.stockCode">
                  <n-td><n-tag :bordered="false" type="info" size="small">{{ ia.stockCode }}</n-tag></n-td>
                  <n-td>{{ ia.stockName }}</n-td>
                  <n-td>
                    <n-text :type="ia.changeRate >= 0 ? 'error' : 'success'">
                      {{ (ia.changeRate >= 0 ? '+' : '') + ia.changeRate.toFixed(2) }}%
                    </n-text>
                  </n-td>
                  <n-td>{{ ia.buyCount }}</n-td>
                  <n-td>{{ ia.sellCount }}</n-td>
                  <n-td><n-text type="error">{{ fmtAmount(ia.buy) }}</n-text></n-td>
                  <n-td><n-text type="success">{{ fmtAmount(ia.sell) }}</n-text></n-td>
                  <n-td>
                    <n-text :type="ia.net >= 0 ? 'error' : 'success'" style="font-weight: bold">{{ fmtAmount(ia.net) }}</n-text>
                  </n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </template>
          <n-empty v-else description="当日无机构席位上榜" style="padding: 40px 0"/>
        </n-tab-pane>
      </n-tabs>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import {onMounted, ref} from 'vue'
import {GetLhbDailySummary} from '../../wailsjs/go/main/App'
import {useMessage} from 'naive-ui'

const message = useMessage()
const loading = ref(false)
const summary = ref<any>(null)
const searchForm = ref({
  dateValue: new Date().toISOString().substring(0, 10),
})

// 金额自适应单位：≥1亿 用"亿"，否则用"万"
function fmtAmount(v: number) {
  if (Math.abs(v) >= 1e8) return (v / 1e8).toFixed(2) + '亿'
  return (v / 1e4).toFixed(2) + '万'
}

function fetch(date?: string) {
  const d = date || searchForm.value.dateValue
  if (!d) return
  loading.value = true
  GetLhbDailySummary(d).then(res => {
    summary.value = res
    loading.value = false
    if (!res || res.stockCount === 0) {
      message.info(`${d} 无龙虎榜数据（非交易日或数据未更新）`)
    }
  }).catch(err => {
    loading.value = false
    message.error('获取游资动向失败')
    console.error(err)
  })
}

onMounted(() => fetch())
</script>
