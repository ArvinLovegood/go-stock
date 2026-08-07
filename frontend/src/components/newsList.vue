<script setup>
import {ReFleshTelegraphList} from "../../wailsjs/go/main/App";
import {RefreshCircle, RefreshCircleSharp, RefreshOutline} from "@vicons/ionicons5";
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref} from 'vue'

const { headerTitle,newsList } = defineProps({
  headerTitle: {
    type: String,
    default: '市场资讯'
  },
  newsList: {
    type: Array,
    default: () => []
  },
})

const emits = defineEmits(['update:message'])

const updateMessage = () => {
  emits('update:message', headerTitle)
}
</script>

<template>
  <n-list bordered>
    <template #header>
      <n-flex justify="space-between" align="center">
        <n-text strong>{{ headerTitle }}</n-text>
        <n-button  :bordered="false" @click="updateMessage"><n-icon color="#409EFF" size="25" :component="RefreshCircleSharp"/></n-button>
      </n-flex>
    </template>
    <n-list-item v-for="(item,idx) in newsList" :key="item.ID">
      <n-space justify="center" v-if="idx!==0 && item.dataTime.substring(0,10) !== newsList[idx-1].dataTime.substring(0,10)">
        <n-divider>
          {{ item.dataTime.substring(0,10) }}
        </n-divider>
      </n-space>
      <n-space justify="start" >
        <n-collapse v-if="item.title" arrow-placement="right" >
          <n-collapse-item :name="item.title">
            <template #header>
              <n-tag size="small" :type="item.isRed?'error':'warning'" :bordered="false"> {{ item.time }}</n-tag>
              <n-text size="small" :type="item.isRed?'error':'info'" :bordered="false">{{ item.title }}</n-text>
            </template>
            <n-text justify="start" :bordered="false" :type="item.isRed?'error':'info'">
              {{ item.content }}
            </n-text>
          </n-collapse-item>
        </n-collapse>
        <n-text  v-if="!item.title" justify="start" :bordered="false" :type="item.isRed?'error':'info'">
          <n-tag size="small" :type="item.isRed?'error':'warning'" :bordered="false"> {{ item.time }}</n-tag>
          {{ item.content }}
        </n-text>
      </n-space>
      <n-space v-if="item.subjects" style="margin-top: 2px">
        <n-tag :bordered="false" type="success" size="small" v-for="sub in item.subjects">
          {{ sub }}
        </n-tag>
        <n-space v-if="item.stocks">
          <n-tag :bordered="false" type="warning" size="small" v-for="sub in item.stocks">
            {{ sub }}
          </n-tag>
        </n-space>
        <n-tag v-if="item.url" :bordered="false" type="warning" size="small">
          <a :href="item.url" target="_blank">
            <n-text type="warning">查看原文</n-text>
          </a>
        </n-tag>
        <n-tag v-if="item.sentimentResult" :bordered="false" :type="item.sentimentResult==='看涨'?'error':item.sentimentResult==='看跌'?'success':'info'" size="small">
          {{ item.sentimentResult }}
        </n-tag>
      </n-space>
    </n-list-item>
  </n-list>
</template>
<style scoped>

</style>