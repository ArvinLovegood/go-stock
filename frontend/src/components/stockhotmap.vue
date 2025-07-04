<script setup lang="ts">

import EmbeddedUrl from "./EmbeddedUrl.vue";
</script>

<template>
  <n-tabs  type="line" animated>
    <n-tab-pane name="百度股市通" tab="百度股市通">
      <embedded-url url="https://gushitong.baidu.com"  :height="'calc(100vh - 252px)'"/>
    </n-tab-pane>
    <n-tab-pane name="东财大盘星图" tab="东财大盘星图">
      <embedded-url url="https://quote.eastmoney.com/stockhotmap/"  :height="'calc(100vh - 252px)'"/>
    </n-tab-pane>
    <n-tab-pane name="TopHub" tab="TopHub(今日热榜)">
      <embedded-url url="https://tophub.today/c/finance"  :height="'calc(100vh - 252px)'"/>
    </n-tab-pane>
    <n-tab-pane name="摸鱼" tab="摸鱼">
      <embedded-url url="https://996.ninja/"  :height="'calc(100vh - 252px)'"/>
    </n-tab-pane>


    <n-tab-pane name="欢迎推荐更多有趣的财经网页" tab="欢迎推荐更多有趣的财经网页">
    </n-tab-pane>
<!--    <n-tab-pane name="自在量化" tab="自在量化">-->
<!--      <embedded-url url="https://quant.zizizaizai.com/home"/>-->
<!--    </n-tab-pane>-->
  </n-tabs>
</template>

<style scoped>

</style>