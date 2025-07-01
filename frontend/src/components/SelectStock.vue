<script setup lang="ts">
import {h, onBeforeMount, onMounted, onUnmounted, ref} from 'vue'
import {SearchStock} from "../../wailsjs/go/main/App";
import {useMessage, NText, NTag} from 'naive-ui'
const message = useMessage()
const search = ref('科技股;换手率连续3日大于2')
const columns = ref([])
const dataList = ref([])

function Search() {
  const loading = message.loading("正在获取选股数据...", {duration: 0});
  SearchStock(search.value).then(res => {
    loading.destroy()
    //console.log(res)
    if(res.code==100){
      message.success(res.msg)
      columns.value=res.data.result.columns.filter(item=>!item.hiddenNeed&&(item.title!="市场码"&&item.title!="市场简称")).map(item=>{

        if(item.children){
          return {
            title:item.title+(item.unit?'['+item.unit+']':''),
            key:item.key,
            resizable: true,
            minWidth:200,
            ellipsis: {
              tooltip: true
            },
            children:item.children.filter(item=>!item.hiddenNeed).map(item=>{
              return {
                title:item.dateMsg,
                key:item.key,
                minWidth:100,
                resizable: true,
                ellipsis: {
                  tooltip: true
                }
              }
            })
          }
        }else{
          return {
            title:item.title+(item.unit?'['+item.unit+']':''),
            key:item.key,
            resizable: true,
            minWidth:100,
            ellipsis: {
              tooltip: true
            }
          }
        }
      })
     dataList.value=res.data.result.dataList
   }else {
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
  Search()
})

</script>

<template>
  <n-flex>
    <n-input-group>
      <n-input v-model:value="search" placeholder="请输入选股指标或者要求" />
      <n-button type="success" @click="Search">搜索A股</n-button>
    </n-input-group>
  </n-flex>
<!--  <n-table striped size="small">-->
<!--    <n-thead>-->
<!--      <n-tr>-->
<!--        <n-th v-for="item in columns">{{item.title}}</n-th>-->
<!--      </n-tr>-->
<!--    </n-thead>-->
<!--    <n-tbody>-->
<!--      <n-tr v-for="(item,index) in dataList">-->
<!--        <n-td v-for="d in columns">{{item[d.key]}}</n-td>-->
<!--      </n-tr>-->
<!--    </n-tbody>-->
<!--  </n-table>-->
  <n-data-table
      :max-height="'calc(100vh - 285px)'"
      size="small"
      :columns="columns"
      :data="dataList"
      :pagination="false"
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
              return h(NTag, { type: 'info',bordered: false }, { default: () => `${value}` })
            }else{
              return h(NText, { type: 'info' }, { default: () => `${value}` })
            }
          }
      }"
  />
</template>

<style scoped>

</style>