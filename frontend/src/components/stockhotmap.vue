<script setup lang="ts">
import {h} from 'vue'
import {NTag,NImage} from 'naive-ui'
import EmbeddedUrl from "./EmbeddedUrl.vue";
</script>

<template>
  <n-tabs class="stock-hotmap-tabs" type="line" animated>
    <n-tab-pane name="选股通" tab="选股通">
      <embedded-url url="https://xuangutong.com.cn" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="百度股市通" tab="百度股市通">
      <embedded-url url="https://gushitong.baidu.com" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="东财大盘星图" tab="东财大盘星图">
      <embedded-url url="https://quote.eastmoney.com/stockhotmap/" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="TopHub" tab="TopHub(今日热榜)">
      <embedded-url url="https://tophub.today/c/finance" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="摸鱼" tab="摸鱼">
      <embedded-url url="https://996.ninja/" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="财联社-行情数据" tab="财联社-行情数据">
      <embedded-url url="https://www.cls.cn/quotation" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="消息墙" tab="消息墙">
      <embedded-url url="https://go-stock.sparkmemory.top:16667/go-stock" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="现货黄金" tab="现货黄金走势">
      <embedded-url url="https://www.tradinghero.com/chart?symbol=XAUUSD.GOODS" height="100%"/>
    </n-tab-pane>

    <n-tab-pane name="金十期货" tab="金十期货">
      <embedded-url url="https://qihuo.jin10.com/" height="100%"/>
    </n-tab-pane>
    <n-tab-pane name="金十数据" tab="金十数据">
      <embedded-url url="https://datas.jin10.com/#/category/52005" height="100%"/>
    </n-tab-pane>


    <n-tab-pane name="欢迎推荐更多有趣的财经网页" tab="欢迎推荐更多有趣的财经网页">
    </n-tab-pane>
<!--    <n-tab-pane name="自在量化" tab="自在量化">-->
<!--      <embedded-url url="https://quant.zizizaizai.com/home"/>-->
<!--    </n-tab-pane>-->
  </n-tabs>
</template>

<style scoped>
.stock-hotmap-tabs {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.stock-hotmap-tabs :deep(.n-tabs-nav) {
  flex: 0 0 auto;
}
.stock-hotmap-tabs :deep(.n-tabs-pane-wrapper) {
  flex: 1 1 0;
  min-height: 0;
}
.stock-hotmap-tabs :deep(.n-tab-pane) {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}
</style>