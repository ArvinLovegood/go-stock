<script setup>
import {onBeforeMount, ref} from 'vue'
import {GetConfig} from "../../wailsjs/go/main/App";
import AnalyzeMartket from "./AnalyzeMartket.vue";
import ConceptEventList from "./ConceptEventList.vue";
import RzrqRank from "./RzrqRank.vue";

const darkTheme = ref(false)

onBeforeMount(() => {
  GetConfig().then(res => {
    darkTheme.value = res.darkTheme
  }).catch(err => {
    console.error('Home GetConfig error:', err)
  })
})
</script>

<template>
  <n-card size="small" style="--wails-draggable:no-drag">
    <template #header>
      <n-flex align="center" :wrap="false">
        <n-text strong>首页</n-text>
        <n-text depth="3" style="font-size: 12px;">市场总览 · 炒作题材</n-text>
      </n-flex>
    </template>

    <n-flex vertical :size="12">
      <!-- 大盘分析：全球股指跑马灯 + 市场情绪 + 涨跌停/分时/融资融券走势 -->
      <AnalyzeMartket :dark-theme="darkTheme" :chart-height="280"/>

      <!-- 每日炒作题材 + 融资融券 并列 -->
      <n-flex :size="12" :wrap="false" style="align-items: flex-start;">
        <div style="flex: 1; min-width: 0;">
          <ConceptEventList/>
        </div>
        <div style="flex: 1; min-width: 0;">
          <RzrqRank :dark-theme="darkTheme"/>
        </div>
      </n-flex>
    </n-flex>
  </n-card>
</template>

<style scoped>
</style>
