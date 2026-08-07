<script setup>
import {onBeforeMount, onBeforeUnmount, onMounted, ref} from 'vue'
import {GetConfig, GetTelegraphList, ReFleshTelegraphList} from "../../wailsjs/go/main/App";
import {EventsOff, EventsOn} from "../../wailsjs/runtime";
import AnalyzeMartket from "./AnalyzeMartket.vue";
import ConceptEventList from "./ConceptEventList.vue";
import RzrqRank from "./RzrqRank.vue";
import NewsList from "./newsList.vue";

const darkTheme = ref(false)
const telegraphList = ref([])

onBeforeMount(() => {
  GetConfig().then(res => {
    darkTheme.value = res.darkTheme
  }).catch(err => {
    console.error('Home GetConfig error:', err)
  })
})

onMounted(() => {
  GetTelegraphList("财联社电报").then(res => {
    telegraphList.value = res || []
  }).catch(err => {
    console.error('GetTelegraphList error:', err)
  })
  EventsOn("newTelegraph", (data) => {
    if (data != null) {
      for (let i = 0; i < data.length; i++) {
        telegraphList.value.pop()
      }
      telegraphList.value.unshift(...data)
    }
  })
})

onBeforeUnmount(() => {
  EventsOff("newTelegraph")
})

function refreshTelegraph() {
  ReFleshTelegraphList("财联社电报").then(res => {
    telegraphList.value = res || []
  })
}
</script>

<template>
  <n-card size="small" style="--wails-draggable:no-drag">
    <template #header>
      <n-flex align="center" :wrap="false">
        <n-text strong>首页</n-text>
        <n-text depth="3" style="font-size: 12px;">市场总览 · 炒作题材 · 财联社电报</n-text>
      </n-flex>
    </template>

    <n-flex vertical :size="12">
      <!-- 大盘分析：全球股指跑马灯 + 市场情绪 + 涨跌停/分时/融资融券走势 -->
      <AnalyzeMartket :dark-theme="darkTheme" :chart-height="280"/>

      <!-- 每日炒作题材 + 融资融券 + 财联社电报 三列等高 -->
      <n-flex :size="12" :wrap="false" style="align-items: stretch;">
        <div class="thin-scroll" style="flex: 1; min-width: 0; max-height: 600px; overflow-y: auto;">
          <ConceptEventList/>
        </div>
        <div class="thin-scroll" style="flex: 1; min-width: 0; max-height: 600px; overflow-y: auto;">
          <RzrqRank :dark-theme="darkTheme"/>
        </div>
        <div class="thin-scroll" style="flex: 1; min-width: 0; max-height: 600px; overflow-y: auto;">
          <NewsList :newsList="telegraphList" :headerTitle="'财联社电报'" @update:message="refreshTelegraph"/>
        </div>
      </n-flex>
    </n-flex>
  </n-card>
</template>

<style scoped>
:deep(.thin-scroll) {
  scrollbar-width: none;
}
:deep(.thin-scroll::-webkit-scrollbar) {
  display: none;
}
</style>
