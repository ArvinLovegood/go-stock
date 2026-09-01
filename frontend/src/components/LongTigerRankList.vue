<script setup lang="ts">
import {onBeforeMount, ref} from 'vue'
import {LongTigerRank, GetLhbSeatDetail} from "../../wailsjs/go/main/App";
import {BrowserOpenURL} from "../../wailsjs/runtime";
import {ArrowDownOutline} from "@vicons/ionicons5";
import _ from "lodash";
import KLineChart from "./KLineChart.vue";
import MoneyTrend from "./moneyTrend.vue";
import HotMoneySeatsManager from "./HotMoneySeatsManager.vue";
import {NButton, NText, useMessage} from "naive-ui";
const message = useMessage()

const lhbList=  ref([])
const EXPLANATIONs = ref([])

// 游资名录维护抽屉
const hotMoneyManager = ref()
function openHotMoneyManager() {
  if (hotMoneyManager.value) {
    hotMoneyManager.value.show = true
  }
}

// 龙虎榜席位明细弹窗（买5卖5，游资/机构识别）
const seatDetail = ref<{
  show: boolean;
  loading: boolean;
  stockCode: string;
  stockName: string;
  date: string;
  data: any;
}>({
  show: false,
  loading: false,
  stockCode: '',
  stockName: '',
  date: '',
  data: null,
})

function showSeatDetail(item) {
  const code = item.SECUCODE.split('.')[0]
  const date = item.TRADE_DATE ? item.TRADE_DATE.substring(0, 10) : SearchForm.value.dateValue
  seatDetail.value = {
    show: true,
    loading: true,
    stockCode: code,
    stockName: item.SECURITY_NAME_ABBR,
    date: date,
    data: null,
  }
  GetLhbSeatDetail(code, date).then(res => {
    seatDetail.value.data = res
    seatDetail.value.loading = false
    if (!res || (!res.buySeats?.length && !res.sellSeats?.length)) {
      message.info("该股当日无席位明细数据")
    }
  }).catch(err => {
    seatDetail.value.loading = false
    message.error("获取席位明细失败")
    console.error(err)
  })
}

function seatTypeTagType(t: string) {
  return ({
    '机构': 'info',
    '北向资金': 'purple',
    '游资': 'error',
    '散户': 'warning',
    '量化': 'success',
    '营业部': 'default',
  } as any)[t] || 'default'
}

function seatTooltip(seat: any) {
  if (seat.tier || seat.style) {
    return [seat.tier, seat.style, seat.riskLevel && `风险:${seat.riskLevel}`].filter(Boolean).join('｜')
  }
  return ''
}

// 金额自适应单位：≥1亿 用"亿"，否则用"万"
function fmtAmount(v: number) {
  if (Math.abs(v) >= 1e8) return (v / 1e8).toFixed(2) + '亿'
  return (v / 1e4).toFixed(2) + '万'
}

function fmtPct(v: number) {
  return (v * 100).toFixed(2) + '%'
}

const today = new Date();
const year = today.getFullYear();
const month = String(today.getMonth() + 1).padStart(2, '0'); // 月份从0开始，需要+1
const day = String(today.getDate()).padStart(2, '0');

// 常见格式：YYYY-MM-DD
const formattedDate = `${year}-${month}-${day}`;

const SearchForm=  ref({
  dateValue:  formattedDate,
  EXPLANATION:null,
})

onBeforeMount(() => {
  longTiger(formattedDate);
})
function longTiger_old(date) {
  if(date) {
    SearchForm.value.dateValue = date
  }
  let loading1=message.loading("正在获取龙虎榜数据...",{
    duration: 0,
  })
  LongTigerRank(date).then(res => {
    lhbList.value = res
    loading1.destroy()
    if (res.length === 0) {
      message.info("暂无数据,请切换日期")
    }
    EXPLANATIONs.value=_.uniqBy(_.map(lhbList.value,function (item){
      return {
        label: item['EXPLANATION'],
        value: item['EXPLANATION'],
      }
    }),'label');
  })
}

function longTiger(date) {
  if (date) {
    SearchForm.value.dateValue = date;
  }

  let loading1 = message.loading("正在获取龙虎榜数据...", {
    duration: 0,
  });

  const fetchDate = (currentDate, retryCount = 0) => {
    if (retryCount > 7) { // 防止无限循环，最多尝试7次
      lhbList.value = [];
      EXPLANATIONs.value = [];
      loading1.destroy();
      message.info("暂无历史数据");
      return;
    }

    LongTigerRank(currentDate).then(res => {
      if (res.length === 0) {
        const previousDate = new Date(currentDate);
        previousDate.setDate(previousDate.getDate() - 1);

        const year = previousDate.getFullYear();
        const month = String(previousDate.getMonth() + 1).padStart(2, '0');
        const day = String(previousDate.getDate()).padStart(2, '0');
        const prevFormattedDate = `${year}-${month}-${day}`;

        message.info(`当前日期 ${currentDate} 暂无数据，尝试查询前一日：${prevFormattedDate}`);

        SearchForm.value.dateValue = prevFormattedDate;
        fetchDate(prevFormattedDate, retryCount + 1); // 递归调用
      } else {
        lhbList.value = res;
        loading1.destroy();
        EXPLANATIONs.value = _.uniqBy(_.map(lhbList.value, function (item) {
          return {
            label: item['EXPLANATION'],
            value: item['EXPLANATION'],
          };
        }), 'label');
      }
    }).catch(err => {
      loading1.destroy();
      message.error("获取数据失败，请重试");
      console.error(err);
    });
  };

  fetchDate(date || formattedDate);
}

function handleEXPLANATION(value, option){
  SearchForm.value.EXPLANATION = value
  if(value){
    LongTigerRank(SearchForm.value.dateValue).then(res => {
      lhbList.value=_.filter(res, function(o) { return o['EXPLANATION']===value; });
      if (res.length === 0) {
        message.info("暂无数据,请切换日期")
      }
    })
  }else{
    longTiger(SearchForm.value.dateValue)
  }
}
</script>

<template>
  <n-form :model="SearchForm" >
    <n-grid :cols="24" :x-gap="24">
      <n-form-item-gi  :span="4" label="日期" path="dateValue" label-placement="left">
        <n-date-picker   v-model:formatted-value="SearchForm.dateValue"
                         value-format="yyyy-MM-dd"  type="date"  :on-update:value="(v,v2)=>longTiger(v2)"/>

      </n-form-item-gi>
      <n-form-item-gi :span="8" label="上榜原因" path="EXPLANATION" label-placement="left">
        <n-select  clearable placeholder="上榜原因过滤" v-model:value="SearchForm.EXPLANATION" :options="EXPLANATIONs" :on-update:value="handleEXPLANATION"/>
      </n-form-item-gi>
      <n-form-item-gi :span="8" label=""  label-placement="left">
        <n-text type="error">*当天的龙虎榜数据通常在收盘结束后一小时左右更新</n-text>
      </n-form-item-gi>
      <n-form-item-gi :span="2" label="" label-placement="left">
        <n-button size="small" @click="openHotMoneyManager">游资名录</n-button>
      </n-form-item-gi>
    </n-grid>
  </n-form>

  <HotMoneySeatsManager ref="hotMoneyManager"/>
  <n-table :single-line="false" striped>
    <n-thead>
      <n-tr>
        <n-th>代码</n-th>
        <!--                <n-th width="90px">日期</n-th>-->
        <n-th width="60px">名称</n-th>
        <n-th>收盘价</n-th>
        <n-th width="60px">涨跌幅</n-th>
        <n-th>龙虎榜净买额</n-th>
        <n-th>龙虎榜买入额</n-th>
        <n-th>龙虎榜卖出额</n-th>
        <n-th>龙虎榜成交额</n-th>
        <!--                <n-th>市场总成交额(万)</n-th>-->
        <!--                <n-th>净买额占总成交比</n-th>-->
        <!--                <n-th>成交额占总成交比</n-th>-->
        <n-th width="60px"  data-field="TURNOVERRATE">换手率<n-icon :component="ArrowDownOutline" /></n-th>
        <n-th>流通市值(亿)</n-th>
        <n-th>上榜原因</n-th>
        <!--                <n-th>解读</n-th>-->
      </n-tr>
    </n-thead>
    <n-tbody>
      <n-tr v-for="(item, index) in lhbList" :key="index">
        <n-td>
          <n-tag :bordered=false type="info" style="cursor: pointer" title="点击查看席位明细" @click="showSeatDetail(item)">{{ item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0] }}</n-tag>
        </n-td>
        <!--                <n-td>
                          {{item.TRADE_DATE.substring(0,10)}}
                        </n-td>-->
        <n-td>
          <!--                  <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.SECURITY_NAME_ABBR }}</n-text>-->
          <n-popover trigger="hover" placement="right">
            <template #trigger>
              <n-button tag="a"  text :type="item.CHANGE_RATE>0?'error':'success'" :bordered=false >{{ item.SECURITY_NAME_ABBR }}</n-button>
            </template>
            <k-line-chart style="width: 800px" :code="item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0]" :chart-height="500" :stockName="item.SECURITY_NAME_ABBR" :k-days="20" :dark-theme="true"></k-line-chart>
          </n-popover>
        </n-td>
        <n-td>
          <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.CLOSE_PRICE }}</n-text>
        </n-td>
        <n-td>
          <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ (item.CHANGE_RATE).toFixed(2) }}%</n-text>
        </n-td>
        <n-td>
          <!--                  <n-text :type="item.BILLBOARD_NET_AMT>0?'error':'success'">{{ (item.BILLBOARD_NET_AMT/10000).toFixed(2) }}</n-text>-->


          <n-popover trigger="hover" placement="right">
            <template #trigger>
              <n-button tag="a"  text :type="item.BILLBOARD_NET_AMT>0?'error':'success'" :bordered=false >{{ fmtAmount(item.BILLBOARD_NET_AMT) }}</n-button>
            </template>
            <money-trend :code="item.SECUCODE.split('.')[1].toLowerCase()+item.SECUCODE.split('.')[0]" :name="item.SECURITY_NAME_ABBR" :days="360" :dark-theme="true" :chart-height="500" style="width: 800px"></money-trend>
          </n-popover>

        </n-td>
        <n-td>
          <n-text :type="'error'">{{ fmtAmount(item.BILLBOARD_BUY_AMT) }}</n-text>
        </n-td>
        <n-td>
          <n-text :type="'success'">{{ fmtAmount(item.BILLBOARD_SELL_AMT) }}</n-text>
        </n-td>
        <n-td>
          <n-text :type="'info'">{{ fmtAmount(item.BILLBOARD_DEAL_AMT) }}</n-text>
        </n-td>
        <!--                <n-td>-->
        <!--                  <n-text :type="'info'">{{ (item.ACCUM_AMOUNT/10000).toFixed(2) }}</n-text>-->
        <!--                </n-td>-->
        <!--                <n-td>-->
        <!--                  <n-text :type="item.DEAL_NET_RATIO>0?'error':'success'">{{ (item.DEAL_NET_RATIO).toFixed(2) }}%</n-text>-->
        <!--                </n-td>-->
        <!--                <n-td>-->
        <!--                  <n-text :type="'info'">{{ (item.DEAL_AMOUNT_RATIO).toFixed(2) }}%</n-text>-->
        <!--                </n-td>-->
        <n-td>
          <n-text :type="'info'">{{ (item.TURNOVERRATE).toFixed(2) }}%</n-text>
        </n-td>
        <n-td>
          <n-text :type="'info'">{{ (item.FREE_MARKET_CAP/100000000).toFixed(2) }}</n-text>
        </n-td>
        <n-td>
          <n-text :type="'info'">{{ item.EXPLANATION }}</n-text>
        </n-td>
        <!--                <n-td>
                          <n-text :type="item.CHANGE_RATE>0?'error':'success'">{{ item.EXPLAIN }}</n-text>
                        </n-td>-->
      </n-tr>
    </n-tbody>
  </n-table>

  <!-- 龙虎榜席位明细弹窗（买5卖5，游资/机构识别） -->
  <n-modal v-model:show="seatDetail.show" preset="card"
           :title="seatDetail.stockName + '（' + seatDetail.stockCode + '）' + seatDetail.date + ' 席位明细'"
           style="width: 960px; max-width: 95vw">
    <n-spin :show="seatDetail.loading">
      <template v-if="seatDetail.data">
        <n-text type="info" depth="2" style="display: block; margin-bottom: 12px">
          上榜原因：{{ seatDetail.data.explanation || '-' }}
        </n-text>
        <n-grid :cols="2" :x-gap="16">
          <n-gi>
            <n-h6 style="margin: 0 0 8px">买入席位 TOP{{ seatDetail.data.buySeats?.length || 0 }}</n-h6>
            <n-table :single-line="false" striped size="small">
              <n-thead>
                <n-tr>
                  <n-th>席位</n-th>
                  <n-th width="90px">类型</n-th>
                  <n-th width="100px">买入额</n-th>
                  <n-th width="70px">占比</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="(seat, i) in seatDetail.data.buySeats" :key="'b'+i">
                  <n-td>
                    <div :title="seatTooltip(seat)">{{ seat.operateDeptName }}</div>
                    <n-tag v-if="seat.hotMoneyName" :bordered="false" type="warning" size="small" style="margin-top: 2px">{{ seat.hotMoneyName }}</n-tag>
                    <n-text v-if="seat.tier" depth="3" style="font-size: 11px; margin-left: 4px">{{ seat.tier }}</n-text>
                  </n-td>
                  <n-td>
                    <n-tag :bordered="false" :type="seatTypeTagType(seat.seatType)" size="small">{{ seat.seatType }}</n-tag>
                  </n-td>
                  <n-td><n-text type="error">{{ fmtAmount(seat.buy) }}</n-text></n-td>
                  <n-td>{{ fmtPct(seat.buyRatio) }}</n-td>
                </n-tr>
                <n-tr v-if="!seatDetail.data.buySeats?.length">
                  <n-td colspan="4">无数据</n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-gi>
          <n-gi>
            <n-h6 style="margin: 0 0 8px">卖出席位 TOP{{ seatDetail.data.sellSeats?.length || 0 }}</n-h6>
            <n-table :single-line="false" striped size="small">
              <n-thead>
                <n-tr>
                  <n-th>席位</n-th>
                  <n-th width="90px">类型</n-th>
                  <n-th width="100px">卖出额</n-th>
                  <n-th width="70px">占比</n-th>
                </n-tr>
              </n-thead>
              <n-tbody>
                <n-tr v-for="(seat, i) in seatDetail.data.sellSeats" :key="'s'+i">
                  <n-td>
                    <div :title="seatTooltip(seat)">{{ seat.operateDeptName }}</div>
                    <n-tag v-if="seat.hotMoneyName" :bordered="false" type="warning" size="small" style="margin-top: 2px">{{ seat.hotMoneyName }}</n-tag>
                    <n-text v-if="seat.tier" depth="3" style="font-size: 11px; margin-left: 4px">{{ seat.tier }}</n-text>
                  </n-td>
                  <n-td>
                    <n-tag :bordered="false" :type="seatTypeTagType(seat.seatType)" size="small">{{ seat.seatType }}</n-tag>
                  </n-td>
                  <n-td><n-text type="success">{{ fmtAmount(seat.sell) }}</n-text></n-td>
                  <n-td>{{ fmtPct(seat.sellRatio) }}</n-td>
                </n-tr>
                <n-tr v-if="!seatDetail.data.sellSeats?.length">
                  <n-td colspan="4">无数据</n-td>
                </n-tr>
              </n-tbody>
            </n-table>
          </n-gi>
        </n-grid>
        <n-text type="error" depth="2" style="display: block; margin-top: 12px; font-size: 12px">
          *游资花名与营业部映射为社区推断性共识（非官方认定），存在一人多席位/席位迁移/借马甲情况；未收录不代表非游资。数据来源于东方财富
        </n-text>
      </template>
    </n-spin>
  </n-modal>
</template>

<style scoped>

</style>