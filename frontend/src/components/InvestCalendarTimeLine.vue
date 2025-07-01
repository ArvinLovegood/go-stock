<script setup lang="ts">
import {nextTick, onBeforeMount, onMounted, onUnmounted, ref} from 'vue'
import {InvestCalendarTimeLine} from "../../wailsjs/go/main/App";
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
  InvestCalendarTimeLine(formattedYM).then(res => {
    list.value = res
    goBackToday();
  })
})
onMounted(()=>{

})
function loadMore(){
  if (list.value.length>0){
    let day=parse(list.value[list.value.length-1].date, 'yyyy-MM-dd', new Date())
    let nextMonth=addMonths(day,1)
    let ym = format(nextMonth, 'yyyy-MM');
    console.log(ym)
    InvestCalendarTimeLine(ym).then(res => {
      if (res.length==0){
        message.warning("没有更多数据了")
        return
      }
      list.value.push( ...res)
    })
  }
}
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
      <n-list-item v-for="(item, index) in list" :id="item.date" :key="item.date">
          <n-thing :title="item.date+' '+getweekday(item.date)">
            <n-list :bordered="false" hoverable>
              <n-list-item v-for="(l,i ) in item.list" :key="l.article_id	">
                <n-flex justify="space-between">
                <n-text :type="item.date===formattedDate?'warning':'info'">{{i+1}}# {{l.title}}</n-text>
                <n-rate v-if="l.like_count>0" readonly :default-value="l.like_count" :count="l.like_count" >
                  <n-icon :component="Star48Filled"/>
                </n-rate>
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
            <n-button  strong secondary type="info" @click="loadMore">加载更多</n-button>
            <n-button  strong secondary  type="warning" @click="goBackToday">回到今天</n-button>
          </n-button-group>
        </n-list-item>
      </n-scrollbar>
    </n-list>
</template>

<style scoped>

</style>