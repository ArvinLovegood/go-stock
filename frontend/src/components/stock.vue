<script setup>
import {onBeforeMount, onBeforeUnmount, onMounted, reactive, ref} from 'vue'
import {Greet, Follow, UnFollow, GetFollowList, GetStockList, SetCostPriceAndVolume} from '../../wailsjs/go/main/App'
import {NButton, NFlex, NForm, NFormItem, NInputNumber, NText, useMessage, useModal} from 'naive-ui'
import { WindowFullscreen,WindowUnfullscreen } from '../../wailsjs/runtime'


const message = useMessage()
const modal = useModal()

const stocks=ref([])
const results=ref({})
const ticker=ref({})
const stockList=ref([])
const followList=ref([])
const options=ref([])
const modalShow = ref(false)
const modalShow2 = ref(false)
const modalShow3 = ref(false)
const formModel = ref({
  name: "",
  code: "",
  costPrice: 0.000,
  volume: 0
})

const data = reactive({
  name: "",
  code: "",
  fenshiURL:"",
  kURL:"",
  resultText: "Please enter your name below 👇",
  fullscreen: false,
})


onBeforeMount(()=>{
  GetStockList("").then(result => {
    stockList.value = result
    options.value=result.map(item => {
      return {
        label: item.name+" "+item.ts_code,
        value: item.ts_code
      }
    })
  })
  GetFollowList().then(result => {
    followList.value = result
    for (const followedStock of result) {
      if (!stocks.value.includes(followedStock.StockCode)) {
        stocks.value.push(followedStock.StockCode)
      }
    }
    monitor()
    message.destroyAll
  })
})

onMounted(() => {
  message.loading("Loading...")
  console.log(`the component is now mounted.`)

    ticker.value=setInterval(() => {
      if(isTradingTime()){
        monitor()
      }
    }, 3000)

})

onBeforeUnmount(() => {
  console.log(`the component is now unmounted.`)
  clearInterval(ticker.value)
})


//判断是否是A股交易时间
function isTradingTime() {
  const now = new Date();
  const day = now.getDay(); // 获取星期几，0表示周日，1-6表示周一至周六
  if (day >= 1 && day <= 5) { // 周一至周五
    const hours = now.getHours();
    const minutes = now.getMinutes();
    const totalMinutes = hours * 60 + minutes;
    const startMorning = 9 * 60 + 15; // 上午9点15分换算成分钟数
    const endMorning = 11 * 60 + 30; // 上午11点30分换算成分钟数
    const startAfternoon = 13 * 60; // 下午13点换算成分钟数
    const endAfternoon = 15 * 60; // 下午15点换算成分钟数
    if ((totalMinutes >= startMorning && totalMinutes < endMorning) ||
        (totalMinutes >= startAfternoon && totalMinutes < endAfternoon)) {
      return true;
    }
  }
  return false;
}

function AddStock(){
  if (!stocks.value.includes(data.code)) {
      stocks.value.push(data.code)
      Follow(data.code).then(result => {
        message.success(result)
      })
  }
  monitor()
}



function removeMonitor(code,name) {
  console.log("removeMonitor",name,code)
  stocks.value.splice(stocks.value.indexOf(code),1)
  delete results.value[name]
  UnFollow(code).then(result => {
    message.success(result)
  })
}

function getStockList(){
  let result;
  result=stockList.value.filter(item => item.name.includes(data.name)||item.ts_code.includes(data.name))
  options.value=result.map(item => {
    return {
      label: item.name+" "+item.ts_code,
      value: item.ts_code
    }
  })
}

async function monitor() {
  for (let code of stocks.value) {
   // console.log(code)
    Greet(code).then(result => {
      let s=(result["当前价格"]-result["昨日收盘价"])*100/result["昨日收盘价"]
      let roundedNum = s.toFixed(2);  // 将数字转换为保留两位小数的字符串形式
      result.s=roundedNum+"%"

      result.highRate=((result["今日最高价"]-result["今日开盘价"])*100/result["今日开盘价"]).toFixed(2)+"%"
      result.lowRate=((result["今日最低价"]-result["今日开盘价"])*100/result["今日开盘价"]).toFixed(2)+"%"


      if (roundedNum>0) {
        result.type="error"
        result.color="#E88080"
      }else if (roundedNum<0) {
        result.type="success"
        result.color="#63E2B7"
      }else {
        result.type="default"
        result.color="#FFFFFF"
      }
      let res= followList.value.filter(item => item.StockCode===code)
      if (res.length>0) {
        result.costPrice=res[0].CostPrice
        result.volume=res[0].Volume
        result.profit=((result["当前价格"]-result.costPrice)*100/result.costPrice).toFixed(3)
        result.profitAmountToday=(result.volume*(result["当前价格"]-result["昨日收盘价"])).toFixed(2)
        result.profitAmount=(result.volume*(result["当前价格"]-result.costPrice)).toFixed(2)
        if(result.profitAmount>0){
          result.profitType="error"
        }else if(result.profitAmount<0){
          result.profitType="success"
        }
      }
      results.value[result["股票名称"]]=result
    })
  }
}
function onSelect(item) {
  data.code=item.split(".")[1].toLowerCase()+item.split(".")[0]
}

function search(code,name){
  setTimeout(() => {
    window.open("https://xueqiu.com/S/"+code)
  }, 500)
}
function setStock(code,name){
    let res=followList.value.filter(item => item.StockCode===code)
    console.log("res:",res)
    formModel.value.name=name
    formModel.value.code=code
    formModel.value.volume=res[0].Volume
    formModel.value.costPrice=res[0].CostPrice
    modalShow.value=true
}

function showFenshi(code,name){
  data.code=code
  data.name=name
  data.fenshiURL='http://image.sinajs.cn/newchart/min/n/'+data.code+'.gif'+"?t="+Date.now()
  modalShow2.value=true
}
function showK(code,name){
  data.code=code
  data.name=name
  data.kURL='http://image.sinajs.cn/newchart/daily/n/'+data.code+'.gif'+"?t="+Date.now()
  modalShow3.value=true
}


function updateCostPriceAndVolumeNew(code,price,volume){
  console.log(code,price,volume)
  SetCostPriceAndVolume(code,price,volume).then(result => {
    modalShow.value=false
    message.success(result)
    GetFollowList().then(result => {
      followList.value = result
      for (const followedStock of result) {
        if (!stocks.value.includes(followedStock.StockCode)) {
          stocks.value.push(followedStock.StockCode)
        }
      }
      monitor()
      message.destroyAll
    })
  })
}

function fullscreen(){
  if(data.fullscreen){
    WindowUnfullscreen()
  }else{
    WindowFullscreen()
  }
  data.fullscreen=!data.fullscreen
}
</script>

<template>
  <n-grid :x-gap="8" :cols="3"  :y-gap="8" >
      <n-gi v-for="result in results" >
         <n-card    :data-code="result['股票代码']" :bordered="false" :title="result['股票名称']"   :closable="true" @close="removeMonitor(result['股票代码'],result['股票名称'])">
           <n-grid :cols="1" :y-gap="6">
             <n-gi>
               <n-text :type="result.type" >{{result["当前价格"]}}</n-text><n-text style="padding-left: 10px;" :type="result.type">{{ result.s}}</n-text>&nbsp;
               <n-text  size="small" v-if="result.profitAmountToday>0" :type="result.type">{{result.profitAmountToday}}</n-text>
             </n-gi>
           </n-grid>
             <n-grid :cols="2" :y-gap="4" :x-gap="4"  >
               <n-gi>
                 <n-text :type="'info'">{{"最高 "+result["今日最高价"]+" "+result.highRate }}</n-text>
               </n-gi>
               <n-gi>
                 <n-text :type="'info'">{{"最低 "+result["今日最低价"]+" "+result.lowRate }}</n-text>
               </n-gi>
               <n-gi>
                 <n-text :type="'info'">{{"昨收 "+result["昨日收盘价"]}}</n-text>
               </n-gi>
               <n-gi>
                 <n-text :type="'info'">{{"今开 "+result["今日开盘价"]}}</n-text>
               </n-gi>
             </n-grid>
           <template #header-extra>
<!--             <n-tag size="small" v-if="result.volume>0" :type="result.profitType">{{result.volume+"股"}}</n-tag>-->
           </template>
           <template #footer>
             <n-flex justify="center">
               <n-tag size="small" v-if="result.volume>0" :type="result.profitType">{{result.volume+"股"}}</n-tag>
              <n-tag size="small" v-if="result.costPrice>0" :type="result.profitType">{{"成本:"+result.costPrice+"  "+result.profit+"%"+" ( "+result.profitAmount+" ¥ )"}}</n-tag>
             </n-flex>
           </template>
           <template #action>
             <n-flex justify="space-between">
               <n-text :type="'info'">{{result["日期"]+" "+result["时间"]}}</n-text>
               <n-button size="tiny" type="info" @click="setStock(result['股票代码'],result['股票名称'])"> 成本 </n-button>
               <n-button size="tiny" type="success" @click="showFenshi(result['股票代码'],result['股票名称'])"> 分时 </n-button>
               <n-button size="tiny" type="error" @click="showK(result['股票代码'],result['股票名称'])"> 日K </n-button>
               <n-button size="tiny" type="warning" @click="search(result['股票代码'],result['股票名称'])"> 详情 </n-button>

             </n-flex>
           </template>
         </n-card >
      </n-gi>
    </n-grid>

    <n-card :bordered="false"  :closable="false">
      <n-button-group>
        <n-auto-complete v-model:value="data.name" type="text"
                         :input-props="{
                                autocomplete: 'disabled',
                              }"
                         :options="options"
                         placeholder="输入股票名称或者代码"
                         clearable class="input" @input="getStockList" :on-select="onSelect"/>
        <n-button type="info" @click="AddStock">添加 </n-button>&nbsp;&nbsp;
        <n-button type="warning" @click="fullscreen"> {{data.fullscreen?'退出全屏':'全屏'}} </n-button>
      </n-button-group>
    </n-card>

      <n-modal transform-origin="center" size="small" v-model:show="modalShow" :title="formModel.name" style="width: 400px" :preset="'card'">
            <n-form :model="formModel" :rules="{ costPrice: { required: true, message: '请输入成本'}, volume: { required: true, message: '请输入数量'} }" label-placement="left" label-width="80px">
              <n-form-item label="成本(元)" path="costPrice">
                <n-input-number v-model:value="formModel.costPrice" min="0"  placeholder="请输入股票成本" />
              </n-form-item>
              <n-form-item label="数量(股)" path="volume">
                <n-input-number v-model:value="formModel.volume"  min="0" placeholder="请输入股票数量" />
              </n-form-item>
            </n-form>
            <template #footer>
              <n-button type="primary" @click="updateCostPriceAndVolumeNew(formModel.code,formModel.costPrice,formModel.volume)">保存</n-button>
            </template>
      </n-modal>

  <n-modal v-model:show="modalShow2" :title="data.name" style="width: 600px" :preset="'card'">
    <n-image :src="data.fenshiURL" />
  </n-modal>
  <n-modal v-model:show="modalShow3" :title="data.name" style="width: 600px" :preset="'card'">
    <n-image :src="data.kURL" />
  </n-modal>
</template>

<style scoped>
.result {
  height: 20px;
  line-height: 20px;
  margin: 1.5rem auto;
}
.input-box {
  text-align: center;
}
.input {
  width: 200px;
  margin-right: 10px;
}

.light-green {
  height: 108px;
  background-color: rgba(0, 128, 0, 0.12);
}
.green {
  height: 108px;
  background-color: rgba(0, 128, 0, 0.24);
}
</style>
