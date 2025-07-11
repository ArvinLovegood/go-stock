<script setup lang="ts">
import {h, onBeforeMount, onMounted, onUnmounted, ref} from 'vue'
import {SearchStock, GetHotStrategy, OpenURL} from "../../wailsjs/go/main/App";
import {useMessage, NText, NTag, NButton} from 'naive-ui'
import {Environment} from "../../wailsjs/runtime"
import {RefreshCircleSharp} from "@vicons/ionicons5";

const message = useMessage()
const search = ref('')
const columns = ref([])
const dataList = ref([])
const hotStrategy = ref([])
const traceInfo = ref('')

function Search() {
  if (!search.value) {
    message.warning('请输入选股指标或者要求')
    return
  }

  const loading = message.loading("正在获取选股数据...", {duration: 0});
  SearchStock(search.value).then(res => {
    loading.destroy()
    // console.log(res)
    if (res.code == 100) {
      traceInfo.value = res.data.traceInfo.showText
      // message.success(res.msg)
      columns.value = res.data.result.columns.filter(item => !item.hiddenNeed && (item.title != "市场码" && item.title != "市场简称")).map(item => {
        if (item.children) {
          return {
            title: item.title + (item.unit ? '[' + item.unit + ']' : ''),
            key: item.key,
            resizable: true,
            minWidth: 200,
            ellipsis: {
              tooltip: true
            },
            children: item.children.filter(item => !item.hiddenNeed).map(item => {
              return {
                title: item.dateMsg,
                key: item.key,
                minWidth: 100,
                resizable: true,
                ellipsis: {
                  tooltip: true
                },
                sorter: (row1, row2) => {
                  if (isNumeric(row1[item.key]) && isNumeric(row2[item.key])) {
                    return row1[item.key] - row2[item.key];
                  } else {
                    return 'default'
                  }
                },
              }
            })
          }
        } else {
          return {
            title: item.title + (item.unit ? '[' + item.unit + ']' : ''),
            key: item.key,
            resizable: true,
            minWidth: 120,
            ellipsis: {
              tooltip: true
            },
            sorter: (row1, row2) => {
              if (isNumeric(row1[item.key]) && isNumeric(row2[item.key])) {
                return row1[item.key] - row2[item.key];
              } else {
                return 'default'
              }
            },
          }


        }
      })
      dataList.value = res.data.result.dataList
    } else {
      message.error(res.msg)
    }
  }).catch(err => {
    message.error(err)
  })
}

function isNumeric(value) {
  return !isNaN(parseFloat(value)) && isFinite(value);
}

onBeforeMount(() => {
  GetHotStrategy().then(res => {
    console.log(res)
    if (res.code == 1) {
      hotStrategy.value = res.data
      search.value = hotStrategy.value[0].question
      Search()
    }
  }).catch(err => {
    message.error(err)
  })

})

function DoSearch(question) {
  search.value = question
  Search()
}

function openCenteredWindow(url, width, height) {
  const left = (window.screen.width - width) / 2;
  const top = (window.screen.height - height) / 2;

  Environment().then(env => {
    switch (env.platform) {
      case 'windows':
        window.open(
            url,
            'centeredWindow',
            `width=${width},height=${height},left=${left},top=${top},location=no,menubar=no,toolbar=no,display=standalone`
        )
        break
      default:
        OpenURL(url)
    }
  })
}
</script>

<template>
  <n-grid :cols="24" style="max-height: calc(100vh - 170px)">
    <n-gi :span="4">
      <n-list bordered style="text-align: left;" hoverable clickable>
        <n-scrollbar style="max-height: calc(100vh - 170px);">
          <n-list-item v-for="item in hotStrategy" :key="item.rank" @click="DoSearch(item.question)">
            <n-ellipsis line-clamp="1" :tooltip="true">
              <n-tag size="small" :bordered="false" type="info">#{{ item.rank }}</n-tag>
              <n-text type="warning">{{ item.question }}</n-text>
              <template #tooltip>
                <div style="text-align: center;max-width: 180px">
                  <n-text type="warning">{{ item.question }}</n-text>
                </div>
              </template>
            </n-ellipsis>
          </n-list-item>
        </n-scrollbar>
      </n-list>

      <!--        <n-virtual-list :items="hotStrategy" :item-size="hotStrategy.length">-->
      <!--          <template #default="{ item, index }">-->
      <!--                      <n-card :title="''" size="small">-->
      <!--                        <template #header-extra>-->
      <!--                          {{item.rank}}-->
      <!--                        </template>-->
      <!--                        <n-ellipsis expand-trigger="click" line-clamp="3" :tooltip="false" >-->
      <!--                          <n-text type="warning">{{item.question	}}</n-text>-->
      <!--                        </n-ellipsis>-->
      <!--                      </n-card>-->

      <!--          </template>-->
      <!--      </n-virtual-list>-->
    </n-gi>
    <n-gi :span="20">
      <n-flex>
        <n-input-group style="text-align: left">
          <n-input :rows="1" clearable v-model:value="search" placeholder="请输入选股指标或者要求"/>
          <n-button type="primary" @click="Search">搜索A股</n-button>
        </n-input-group>
      </n-flex>
      <n-flex justify="start" v-if="traceInfo" style="margin: 5px 0">

        <n-ellipsis line-clamp="1" :tooltip="true">
          <n-text type="info" :bordered="false">选股条件：</n-text>
          <n-text type="warning" :bordered="true">{{ traceInfo }}</n-text>
          <template #tooltip>
            <div style="text-align: center;max-width: 580px">
              <n-text type="warning">{{ traceInfo }}</n-text>
            </div>
          </template>
        </n-ellipsis>

        <!--    <n-button type="primary" size="small">保存策略</n-button>-->
      </n-flex>
      <n-data-table
          :striped="true"
          :max-height="'calc(100vh - 250px)'"
          size="medium"
          :columns="columns"
          :data="dataList"
          :pagination="{pageSize: 9}"
          :scroll-x="1800"
          :render-cell="(value, rowData, column) => {

        if(column.key=='SECURITY_CODE'||column.key=='SERIAL'){
          return h(NText, { type: 'info',border: false }, { default: () => `${value}` })
        }
        if (isNumeric(value)) {
          let type='info';
          if (Number(value)<0){
            type='success';
          }
          if(Number(value)>=0&&Number(value)<=5){
            type='warning';
          }
          if (Number(value)>5){
            type='error';
          }
            return h(NText, { type: type }, { default: () => `${value}` })
        }else{
            if(column.key=='SECURITY_SHORT_NAME'){
              return h(NButton, { type: 'info',bordered: false ,size:'small',onClick:()=>{
               //https://quote.eastmoney.com/sz300558.html#fullScreenChart
               openCenteredWindow(`https://quote.eastmoney.com/${rowData.MARKET_SHORT_NAME}${rowData.SECURITY_CODE}.html#fullScreenChart`,1240,700)
              }}, { default: () => `${value}` })
            }else{
              return h(NText, { type: 'info' }, { default: () => `${value}` })
            }
          }
      }"
      />
      <n-text>共找到
        <n-tag type="info" :bordered="false">{{ dataList.length }}</n-tag>
        只股
      </n-text>
    </n-gi>
  </n-grid>


</template>

<style scoped>

</style>