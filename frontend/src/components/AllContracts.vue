<script setup lang="ts">
// 合约行情独立页：集中展示全部合约
// - 美股永续（TRADIFI_PERPETUAL）：bn: 前缀，复用 BinanceFuturesList（market=tradfi）
// - USDT-M 永续（加密）：bn: 前缀，复用 BinanceFuturesList（market=crypto）
// 显隐由设置页「启用合约行情」开关控制（enableContracts）。
import {onBeforeMount, ref} from 'vue'
import {GetConfig} from '../../wailsjs/go/main/App'
import BinanceFuturesList from './BinanceFuturesList.vue'

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
    </n-tabs>
  </div>
</template>

<style scoped>
.all-contracts-page {
  padding: 12px 16px;
  height: 100%;
  overflow: hidden;
}
</style>