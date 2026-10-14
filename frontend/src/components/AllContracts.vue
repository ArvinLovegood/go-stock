<script setup lang="ts">
// 合约行情独立页：集中展示全部合约
// - 美股永续（TRADIFI_PERPETUAL）：bn: 前缀，复用 BinanceFuturesList（market=tradfi）
// - USDT-M 永续（加密）：bn: 前缀，复用 BinanceFuturesList（market=crypto）
// - 模拟盘 / 实盘：同一 OKXTradingPanel 渲染两份实例，分别固定走 OKX 模拟环境 / 实盘环境
//   （各用各的密钥）；active 只在对应页签激活时为 true，避免双实例同时轮询。
// 显隐由设置页「启用合约行情」开关控制（enableContracts）。
import {onBeforeMount, ref} from 'vue'
import {GetConfig} from '../../wailsjs/go/main/App'
import BinanceFuturesList from './BinanceFuturesList.vue'
import OKXTradingPanel from './OKXTradingPanel.vue'
import OKXRankingsPanel from './OKXRankingsPanel.vue'
import ExchangeNewsPanel from './ExchangeNewsPanel.vue'

const darkTheme = ref(false)
const activeTab = ref('us')

onBeforeMount(() => {
  GetConfig().then((res) => {
    darkTheme.value = !!res.darkTheme
  }).catch((err) => {
    console.error('GetConfig error:', err)
  })
})
</script>

<template>
  <div class="all-contracts-page">
    <n-tabs v-model:value="activeTab" type="line" animated>
      <n-tab-pane name="us" tab="美股永续">
        <BinanceFuturesList :dark-theme="darkTheme" market="tradfi"/>
      </n-tab-pane>
      <n-tab-pane name="crypto" tab="加密永续">
        <BinanceFuturesList :dark-theme="darkTheme" market="crypto"/>
      </n-tab-pane>
      <n-tab-pane name="rankings" tab="行情榜单">
        <OKXRankingsPanel :dark-theme="darkTheme" :active="activeTab === 'rankings'"/>
      </n-tab-pane>
      <n-tab-pane name="okxSim" tab="模拟盘">
        <OKXTradingPanel :dark-theme="darkTheme" :simulated="true" :active="activeTab === 'okxSim'"/>
      </n-tab-pane>
      <n-tab-pane name="news" tab="公告资讯">
        <ExchangeNewsPanel :dark-theme="darkTheme"/>
      </n-tab-pane>
      <n-tab-pane name="okxLive">
        <template #tab>
          <span class="live-tab-label">实盘</span>
          <n-tag type="error" size="tiny" :bordered="false">真实资金</n-tag>
        </template>
        <OKXTradingPanel :dark-theme="darkTheme" :simulated="false" :active="activeTab === 'okxLive'"/>
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<style scoped>
.all-contracts-page {
  padding: 12px 16px;
  height: 100%;
  overflow: hidden;
}

/* 实盘页签：名称 + 红色「真实资金」小标 */
.live-tab-label {
  margin-right: 4px;
}
</style>