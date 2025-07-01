<script setup lang="ts">
import {nextTick, onBeforeMount, onMounted, onUnmounted, ref} from 'vue'
import {ClsCalendar} from "../../wailsjs/go/main/App";
import { addMonths, format ,parse} from 'date-fns';
import { zhCN } from 'date-fns/locale';

import {useMessage} from 'naive-ui'
import {Star48Filled} from "@vicons/fluent";
const today = new Date();
const year = today.getFullYear();
const month = String(today.getMonth() + 1).padStart(2, '0'); // 月份从0开始，需要+1
const day = String(today.getDate()).padStart(2, '0');

// 常见格式：YYYY-MM-DD
const formattedDate = `${year}-${month}-${day}`;
const formattedYM = `${year}-${month}`;
const list  = ref([])
const message=useMessage()

function goBackToday() {
  setTimeout(() => {
    nextTick(
        () => {
          const elementById = document.getElementById(formattedDate);
          if (elementById) {
            elementById.scrollIntoView({
              behavior: 'auto',
              block: 'start'
            })
          }
        }
    )
  }, 500)
}

onBeforeMount(() => {
  ClsCalendar().then(res => {
    list.value = res
    goBackToday();
  })
})

function getweekday(date){
  let day=parse(date, 'yyyy-MM-dd', new Date())
  return format(day, 'EEEE', {locale: zhCN})
}
</script>

<template>
<!--    <n-timeline size="large"  style="text-align: left">-->
<!--      <n-timeline-item v-for="item in list" :key="item.date" :title="item.date"  type="info" >-->
<!--        <n-list>-->
<!--          <n-list-item v-for="l in item.list" :key="l.article_id	">-->
<!--            <n-text>{{l.title}}</n-text>-->
<!--          </n-list-item>-->
<!--        </n-list>-->
<!--      </n-timeline-item>-->
<!--    </n-timeline>-->

    <n-list bordered   style="max-height: calc(100vh - 230px);text-align: left;">
      <n-scrollbar style="max-height: calc(100vh - 230px);" >
      <n-list-item v-for="(item, index) in list" :id="item.calendar_day" :key="item.calendar_day">
          <n-thing :title="item.calendar_day	+' '+item.week">
            <n-list :bordered="false" hoverable>
              <n-list-item v-for="(l,i ) in item.items" :key="l.id	">
                <n-flex justify="space-between">
                  <n-text :type="item.calendar_day===formattedDate?'warning':'info'">{{i+1}}# {{l.title}}
                    <n-tag  v-if="l.event" size="small" round  type="success">事件</n-tag>
                    <n-tag  v-if="l.economic" size="small" round  type="error">数据</n-tag>
                  </n-text>
                  <n-rate v-if="l.event&&(l.event.star>0)" readonly :default-value="l.event.star">
                    <n-icon :component="Star48Filled"/>
                  </n-rate>
                  <n-rate v-if="l.economic&&(l.economic.star>0)" readonly :default-value="l.economic.star" >
                    <n-icon :component="Star48Filled"/>
                  </n-rate>
                </n-flex>

                <n-flex  v-if="l.economic">
                  <n-tag type="warning" :bordered="false" :size="'small'">公布：{{l.economic.actual	}}</n-tag>
                  <n-tag type="warning"  :bordered="false" :size="'small'">预测：{{l.economic.consensus}}</n-tag>
                  <n-tag type="warning"  :bordered="false" :size="'small'">前值：{{l.economic.front}}</n-tag>
                </n-flex>
              </n-list-item>
            </n-list>
          </n-thing>
      </n-list-item>
        <n-list-item v-if="list.length==0">
          <n-text type="info">没有数据</n-text>
        </n-list-item>
        <n-list-item v-else style="text-align: center;">
          <n-button-group>
            <n-button  strong secondary  type="warning" @click="goBackToday">回到今天</n-button>
          </n-button-group>
        </n-list-item>
      </n-scrollbar>
    </n-list>
</template>

<style scoped>

</style>