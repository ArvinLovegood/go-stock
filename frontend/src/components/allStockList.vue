<script setup>
import {h, onBeforeMount, onMounted, ref, reactive, watch, computed} from 'vue'
import {
  GetAllStockInfoList,
  GetAllStocks,
  GetConfig, GetSponsorInfo, GetEffectiveSponsorVip, Follow, GetGroupList, AddStockGroup, AddGroup
} from "../../wailsjs/go/main/App";
import {NButton, NInput, NTag, NText, useMessage, useNotification, NDataTable, NSpace, NPagination, NDropdown, NIcon} from "naive-ui";
import sparkLine from "./stockSparkLine.vue"
import klineChart from "./KLineChart.vue"
import KLineChart from "./KLineChart.vue";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import {FolderOpenOutline, AddOutline} from "@vicons/ionicons5";
import {format} from "date-fns";

const notify = useNotification()
const message = useMessage()

const editorDataRef = reactive({
  darkTheme: false
})

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })

  GetSponsorInfo().then((res) => {
    // console.log(res)
    vipLevel.value = res.vipLevel;
    vipStartTime.value = res.vipStartTime;
    vipEndTime.value = res.vipEndTime;
    //判断时间是否到期
    if (res.vipLevel) {
      if (res.vipEndTime < format(new Date(), 'yyyy-MM-dd HH:mm:ss')) {
        //notify.warning({content: 'VIP已到期'})
        expired.value = true;
      }
    }else{
      //notify.success({content: '未开通VIP'})
    }
    isValidVip.value = !(vipLevel.value === "" || Number(vipLevel.value) <= 0);

  })

})

onMounted(() => {
  console.log('stock-list mounted')
  loadStocks(1, paginationReactive.pageSize)
  loadGroupList()
})

const dataRef = ref([])
const loadingRef = ref(false)
const vipLevel=ref("");
const vipStartTime=ref("");
const vipEndTime=ref("");
const expired=ref(false)
const isValidVip=ref(false) // 是否是会员

// 多周期 K 线（VIP2）
const effectiveVipLevel = ref(0)
const multiKlineModalShow = ref(false)
const multiKlineCode = ref('')
const multiKlineName = ref('')

async function refreshEffectiveVip() {
  try {
    const r = await GetEffectiveSponsorVip()
    const active = !!r?.active
    const lvl = Number(r?.vipLevel ?? 0)
    effectiveVipLevel.value = active && !Number.isNaN(lvl) ? lvl : 0
  } catch (_) {
    effectiveVipLevel.value = 0
  }
}

function toEastMoneyCodeFromSecucode(secucode) {
  if (!secucode) return ''
  const c = String(secucode).trim()
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  return c.toUpperCase()
}

async function showMultiKline(row) {
  const em = toEastMoneyCodeFromSecucode(row.SECUCODE)
  if (!em) {
    message.warning('当前代码暂不支持多周期K线图')
    return
  }
  await refreshEffectiveVip()
  if (effectiveVipLevel.value < 2) {
    message.warning('多周期 K 线仅限 VIP2 及以上用户使用，您当前权限不足')
    return
  }
  multiKlineCode.value = em
  multiKlineName.value = row.SECURITY_NAME_ABBR || ''
  multiKlineModalShow.value = true
}

// 关注分组选择
const groupList = ref([])
const showFollowGroupModal = ref(false)
const newGroupName = ref('')
const pendingFollowRow = ref(null)

const followGroupOptions = computed(() => {
  const opts = [{label: '默认（不分组）', key: 0}]
  groupList.value.forEach(g => opts.push({label: g.name, key: g.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建分组', key: 'new', icon: () => h(NIcon, null, {default: () => h(AddOutline)})})
  return opts
})

function loadGroupList() {
  GetGroupList().then(res => {
    groupList.value = res || []
  }).catch(() => {})
}

function handleFollowSelect(key, row) {
  if (key === 'new') {
    pendingFollowRow.value = row
    newGroupName.value = ''
    showFollowGroupModal.value = true
    return
  }
  doFollow(row, Number(key))
}

function doFollow(row, groupId) {
  let code = getStockCode(row.SECUCODE)
  Follow(code).then(result => {
    if (result === '关注成功') {
      message.success(groupId > 0 ? `已关注，并加入分组「${groupNameById(groupId)}」` : '关注成功')
    } else {
      message.info(result)
    }
    if (groupId > 0) {
      AddStockGroup(groupId, code).then(() => {
        loadGroupList()
      }).catch(() => {})
    }
  }).catch(err => {
    message.error('关注失败: ' + err)
  })
}

function groupNameById(id) {
  const g = groupList.value.find(item => item.ID === id)
  return g ? g.name : ''
}

function handleCreateGroupAndFollow() {
  const name = newGroupName.value.trim()
  if (!name) {
    message.warning('请输入分组名称')
    return
  }
  const maxSort = groupList.value.reduce((max, g) => Math.max(max, g.sort || 0), 0)
  AddGroup({name, sort: maxSort + 1}).then(res => {
    if (res === '添加成功') {
      GetGroupList().then(list => {
        groupList.value = list || []
        showFollowGroupModal.value = false
        const created = groupList.value.find(g => g.name === name)
        if (created && pendingFollowRow.value) {
          doFollow(pendingFollowRow.value, created.ID)
        }
        pendingFollowRow.value = null
      })
    } else {
      message.error(res)
    }
  }).catch(err => {
    message.error('新建分组失败: ' + err)
  })
}

const columnsRef = ref([
  // {
  //   title: '数据时间',
  //   key: 'MAX_TRADE_DATE',
  //   width: 120,
  // },
  {
    title: '股票代码',
    key: 'SECUCODE',
    width: 100,
    render(row) {
      return h(NText, { type: "info" }, { default: () => row.SECUCODE })
    }
  },
  {
    title: '股票名称',
    key: 'SECURITY_NAME_ABBR',
    width: 100,
    render(row) {
      return h(NText, { type: "success" }, { default: () => row.SECURITY_NAME_ABBR })
    }
  },
  {
    title: '最新价',
    key: 'NEW_PRICE',
    width: 100,
    render(row) {
      const price = row.NEW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'CHANGE_RATE',
    width: 100,
    render(row) {
      const rate = toNumber(row.CHANGE_RATE, 0)
      const type = rate >= 0 ? 'error' : 'success'
      const sign = rate >= 0 ? '+' : ''
      return h(NText, { type: type }, { default: () => `${sign}${rate.toFixed(2)}%` })
    }
  },
  {
    title: '分时图',
    key: 'sparkline',
    width: 120,
    render(row) {
      return h(sparkLine, {
        idSuffix: row.SECUCODE,
        stockName: row.SECURITY_NAME_ABBR,
        stockCode: row.SECUCODE,
        lastPrice: row.NEW_PRICE,
        openPrice: row.PRE_CLOSE_PRICE,
        tooltip: true
      })
    }
  },
  {
    title: '最高价',
    key: 'HIGH_PRICE',
    width: 100,
    render(row) {
      const price = row.HIGH_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  {
    title: '最低价',
    key: 'LOW_PRICE',
    width: 100,
    render(row) {
      const price = row.LOW_PRICE
      return h(NText, { type: "info" }, { default: () => isNumeric(price) ? price : '-' })
    }
  },
  // {
  //   title: '前收价',
  //   key: 'PRE_CLOSE_PRICE',
  //   width: 100,
  //   render(row) {
  //     return h(NText, { type: "info" }, { default: () => row.PRE_CLOSE_PRICE.toFixed(2) })
  //   }
  // },
  {
    title: '成交量',
    key: 'VOLUME',
    width: 120,
    render(row) {
      const volume = toNumber(row.VOLUME, 0)
      let displayVolume = volume
      if (volume >= 100000000) {
        displayVolume = (volume / 100000000).toFixed(2) + '亿'
      } else if (volume >= 10000) {
        displayVolume = (volume / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayVolume })
    }
  },
  {
    title: '成交额',
    key: 'DEAL_AMOUNT',
    width: 120,
    render(row) {
      const amount = toNumber(row.DEAL_AMOUNT, 0)
      let displayAmount = amount
      if (amount >= 100000000) {
        displayAmount = (amount / 100000000).toFixed(2) + '亿'
      } else if (amount >= 10000) {
        displayAmount = (amount / 10000).toFixed(2) + '万'
      }
      return h(NText, { type: "info" }, { default: () => displayAmount })
    }
  },
  {
    title: '换手率 (%)',
    key: 'TURNOVERRATE',
    width: 80,
    render(row) {
      const rate = row.TURNOVERRATE
      return h(NText, { type: "info" }, { default: () => isNumeric(rate) ? rate : '-' })
    }
  },
  {
    title: '量比',
    key: 'VOLUME_RATIO',
    width: 80,
    render(row) {
      const ratio = row.VOLUME_RATIO
      return h(NText, { type: "info" }, { default: () => isNumeric(ratio) ? ratio : '-' })
    }
  },
  {
    title: '所属行业',
    key: 'INDUSTRY',
    width: 100,
    render(row) {
      return h(NTag, { type: "primary", size: "small" }, { default: () => row.INDUSTRY })
    }
  },
  {
    title: '所属概念',
    key: 'CONCEPT',
    width: 100,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      if(typeof row.CONCEPT === 'string'){
        return h(NTag, { type: "info", size: "small" ,style: "margin-right: 4px;" }, { default: () => row.CONCEPT })
      }else{
        if (!row.CONCEPT || row.CONCEPT.length === 0) {
          return h(NText, { type: "secondary" }, { default: () => '无' })
        }
        return row.CONCEPT.map(concept =>
            h(NTag, { type: "info", size: "small", style: "margin-right: 4px;" }, { default: () => concept })
        )
      }
    }
  },
  // {
  //   title: '交易所',
  //   key: 'MARKET',
  //   width: 100,
  //   render(row) {
  //     return h(NTag, { type: "warning", size: "small" }, { default: () => row.MARKET })
  //   }
  // },
  {
    title: '操作',
    render(row, index) {
      return [h(
          NButton,
          {
            secondary: true,
            size: 'small',
            type: 'warning', // 橙色按钮
            onClick: () => showKline(row)
          },
          { default: () => '日K' }
      ), h(
          NButton,
          {
            secondary: true,
            size: 'small',
            type: 'primary',
            style: 'margin-left: 4px;',
            onClick: () => showMultiKline(row)
          },
          { default: () => '多周期' }
      ), h(
          NDropdown,
          {
            trigger: 'click',
            options: followGroupOptions.value,
            placement: 'bottom-end',
            onSelect: (key) => handleFollowSelect(key, row)
          },
          {
            default: () => h(
                NButton,
                {
                  tertiary: true,
                  size: 'small',
                  type: 'success',
                  style: 'margin-left: 4px;',
                },
                {
                  default: () => '关注',
                  icon: () => h(NIcon, null, {default: () => h(FolderOpenOutline, {size: 14})})
                }
            )
          }
      ),]
    }
  },
])

const paginationReactive = reactive({
  keyword:"",
  page: 1,
  pageCount: 1,
  pageSize: 9,
  itemCount: 0,
  prefix({ itemCount }) {
    return `${itemCount} 只股票`
  }
})
const optionsReactive= reactive([
  {
    label: '全部',
    value: ''
  },
 ])

function loadStocks(page, pageSize) {
  if((vipLevel.value===""|| Number(vipLevel.value) <=0)){
    handleReset()
  }
  if (!loadingRef.value) {
    loadingRef.value = true
    GetAllStocks(page, pageSize, paginationReactive.keyword, technicalIndicatorReactive).then((res) => {
      console.log(res)
      if (res && res.result && res.result.data) {
        dataRef.value = res.result.data
        paginationReactive.page = page
        paginationReactive.pageCount = Math.ceil(res.result.count / pageSize)
        paginationReactive.itemCount = res.result.count
      } else {
        dataRef.value = []
        paginationReactive.page = 1
        paginationReactive.pageCount = 1
        paginationReactive.itemCount = 0
        message.error('获取股票数据失败')
      }
      loadingRef.value = false
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
      loadingRef.value = false
    })
  }
}
function handleCheckedChange(checked) {

  if(checked&&(vipLevel.value===""|| Number(vipLevel.value) <=0)){
    handleReset()
    message.warning('未开通VIP或者已经过期，无法使用技术面筛选')
  }
}
function handlePageChange(currentPage) {
  loadStocks(currentPage, paginationReactive.pageSize)
}

function handlePageSizeChange(pageSize) {
  paginationReactive.pageSize = pageSize
  loadStocks(1, pageSize)
}
function handleSearch() {
  loadStocks(1, paginationReactive.pageSize)
}
function handleUpdateVal(value) {
  console.log('handleUpdateVal', value)
  if (value === '') {
    optionsReactive.splice(1, optionsReactive.length - 1)
  } else {
    GetAllStockInfoList({
      searchKeyWord: value
    }).then((res) => {
      console.log('GetAllStockInfoList result:', res)
      if (res  && res.list) {
        optionsReactive.splice(1, optionsReactive.length - 1)
        optionsReactive.push(...res.list.map(item => {
          return {
            label: item.SECURITY_NAME_ABBR,
            value: item.SECURITY_NAME_ABBR,
            obj: item,
          }
        }))
      }
    }).catch(err => {
      message.error('获取股票数据失败: ' + err.message)
    })
  }
}
const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
})
function showKline(row) {
  console.log('showKline', row)
  modalDataRef.title = row.SECURITY_NAME_ABBR
  modalDataRef.stockCode = getStockCode(row.SECUCODE)
  modalDataRef.stockName = row.SECURITY_NAME_ABBR
  modalDataRef.visible = true
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}
const technicalIndicatorReactive = reactive({
  MACD_GOLDEN_FORK: false,
  KDJ_GOLDEN_FORK: false,
  BREAK_THROUGH: false,
  LOW_FUNDS_INFLOW: false,
  HIGH_FUNDS_OUTFLOW: false,
  BREAKUP_MA_5DAYS: false,
  LONG_AVG_ARRAY: false,
  SHORT_AVG_ARRAY: false,
  UPPER_LARGE_VOLUME: false,
  DOWN_NARROW_VOLUME: false,
  ONE_DAYANG_LINE: false,
  TWO_DAYANG_LINES: false,
  RISE_SUN: false,
  POWER_FULGUN: false,
  RESTORE_JUSTICE: false,
  DOWN_7DAYS: false,
  UPPER_8DAYS:false,
  UPPER_9DAYS:false,
  UPPER_4DAYS:false,
  HEAVEN_RULE:false,
  UPSIDE_VOLUME: false,
  BEARISH_ENGULFING: false,
  REVERSING_HAMMER: false,
  SHOOTING_STAR: false,
  EVENING_STAR: false,
  FIRST_DAWN: false,
  PREGNANT: false,
  BLACK_CLOUD_TOPS: false,
  MORNING_STAR: false,
  NARROW_FINISH: false,
  UPP_DAYS:0,
  CONCERN_RANK_7DAYS:0,
  UPNDAY:0,
  DOWNNDAY :0,
})

function handleReset(){
  technicalIndicatorReactive.MACD_GOLDEN_FORK = false
  technicalIndicatorReactive.KDJ_GOLDEN_FORK = false
  technicalIndicatorReactive.BREAK_THROUGH = false
  technicalIndicatorReactive.LOW_FUNDS_INFLOW = false
  technicalIndicatorReactive.HIGH_FUNDS_OUTFLOW = false
  technicalIndicatorReactive.BREAKUP_MA_5DAYS = false
  technicalIndicatorReactive.LONG_AVG_ARRAY = false
  technicalIndicatorReactive.SHORT_AVG_ARRAY = false
  technicalIndicatorReactive.UPPER_LARGE_VOLUME = false
  technicalIndicatorReactive.DOWN_NARROW_VOLUME = false
  technicalIndicatorReactive.ONE_DAYANG_LINE = false
  technicalIndicatorReactive.TWO_DAYANG_LINES = false
  technicalIndicatorReactive.RISE_SUN = false
  technicalIndicatorReactive.POWER_FULGUN = false
  technicalIndicatorReactive.RESTORE_JUSTICE = false
  technicalIndicatorReactive.DOWN_7DAYS = false
  technicalIndicatorReactive.UPPER_8DAYS=false
  technicalIndicatorReactive.UPPER_9DAYS=false
  technicalIndicatorReactive.UPPER_4DAYS=false
  technicalIndicatorReactive.HEAVEN_RULE=false
  technicalIndicatorReactive.ONE_DAYANG_LINE=false
  technicalIndicatorReactive.TWO_DAYANG_LINES= false
  technicalIndicatorReactive.RISE_SUN=false
  technicalIndicatorReactive.POWER_FULGUN=false
  technicalIndicatorReactive.RESTORE_JUSTICE=false
  technicalIndicatorReactive.DOWN_7DAYS=false
  technicalIndicatorReactive.UPPER_8DAYS=false
  technicalIndicatorReactive.UPPER_9DAYS=false
  technicalIndicatorReactive.UPPER_4DAYS=false
  technicalIndicatorReactive.HEAVEN_RULE=false
  technicalIndicatorReactive.UPSIDE_VOLUME=false
  technicalIndicatorReactive.BEARISH_ENGULFING=false
  technicalIndicatorReactive.REVERSING_HAMMER=false
  technicalIndicatorReactive.SHOOTING_STAR=false
  technicalIndicatorReactive.EVENING_STAR=false
  technicalIndicatorReactive.FIRST_DAWN=false
  technicalIndicatorReactive.PREGNANT=false
  technicalIndicatorReactive.BLACK_CLOUD_TOPS=false
  technicalIndicatorReactive.MORNING_STAR=false
  technicalIndicatorReactive.NARROW_FINISH=false
  technicalIndicatorReactive.UPP_DAYS=0
  technicalIndicatorReactive.CONCERN_RANK_7DAYS=0
  technicalIndicatorReactive.UPNDAY=0
  technicalIndicatorReactive.DOWNNDAY=0
}

// 判断是否是数字
const isNumeric = (value) => {
  if (value === null || value === undefined || value === '') {
    return false
  }
  return !isNaN(Number(value))
}

// 安全转换数字
const toNumber = (value, defaultValue = 0) => {
  const num = Number(value)
  return isNaN(num) ? defaultValue : num
}

</script>

<template>
    <n-space justify="start">
      <n-card size="small" :bordered="false"  style="text-align: left">
        <n-checkbox   @update:checked="handleCheckedChange" v-model:checked="technicalIndicatorReactive.MACD_GOLDEN_FORK">
        MACD金叉
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.KDJ_GOLDEN_FORK">
        KDJ金叉
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BREAK_THROUGH">
        放量突破
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.LOW_FUNDS_INFLOW">
        低位资金净流入
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.HIGH_FUNDS_OUTFLOW">
        高位资金净流出
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BREAKUP_MA_5DAYS">
        向上突破5日均线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.LONG_AVG_ARRAY">
        均线多头排列
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.SHORT_AVG_ARRAY">
        均线空头排列
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_LARGE_VOLUME">
        连涨放量
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.DOWN_NARROW_VOLUME">
        下跌无量
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.ONE_DAYANG_LINE">
        一根大阳线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.TWO_DAYANG_LINES">
        两根大阳线
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"     v-model:checked="technicalIndicatorReactive.RISE_SUN">
        旭日东升
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.POWER_FULGUN">
        强势多方炮
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.RESTORE_JUSTICE">
        拨云见日
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"     v-model:checked="technicalIndicatorReactive.DOWN_7DAYS">
        七仙女下凡(七连阴)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_8DAYS">
        八仙过海(八连阳)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_9DAYS">
        九阳神功(九连阳)
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPPER_4DAYS">
        四串阳
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.HEAVEN_RULE">
        天量法则
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.UPSIDE_VOLUME">
        放量上攻
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BEARISH_ENGULFING">
        穿头破脚
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.REVERSING_HAMMER">
        倒转锤头
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.SHOOTING_STAR">
        射击之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.EVENING_STAR">
        黄昏之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.FIRST_DAWN">
        曙光初现
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.PREGNANT">
        身怀六甲
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.BLACK_CLOUD_TOPS">
        乌云盖顶
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.MORNING_STAR">
        早晨之星
      </n-checkbox>
      <n-checkbox  @update:checked="handleCheckedChange"    v-model:checked="technicalIndicatorReactive.NARROW_FINISH">
        窄幅整理
      </n-checkbox>
      </n-card>
      <n-card size="small" :bordered="false"  style="text-align: left">
        <n-radio-group size="small"  @update:checked="handleCheckedChange" name="UPP_DAYS"   v-model:value="technicalIndicatorReactive.UPP_DAYS">
          <n-radio :value="3">人气排名连涨:3天及以上</n-radio>
          <n-radio :value="5">人气排名连涨:5天及以上</n-radio>
          <n-radio :value="7">人气排名连涨:7天及以上</n-radio>
        </n-radio-group>
        <n-divider vertical/>
        <n-radio-group  size="small" @update:checked="handleCheckedChange"  name="CONCERN_RANK_7DAYS"  v-model:value="technicalIndicatorReactive.CONCERN_RANK_7DAYS">
          <n-radio :value="10"> 7日关注排名:前10名</n-radio>
          <n-radio :value="50"> 7日关注排名:前50名</n-radio>
          <n-radio :value="100"> 7日关注排名:前100名</n-radio>
        </n-radio-group>

        <n-radio-group  size="small" @update:checked="handleCheckedChange" name="UPNDAY"  v-model:value="technicalIndicatorReactive.UPNDAY">
          <n-radio :value="3"> 连涨天数:3天及以上</n-radio>
          <n-radio :value="5"> 连涨天数:5天及以上</n-radio>
          <n-radio :value="8"> 连涨天数:8天及以上</n-radio>
        </n-radio-group>
        <n-divider vertical/>
        <n-radio-group  size="small" @update:checked="handleCheckedChange" name="DOWNNDAY"  v-model:value="technicalIndicatorReactive.DOWNNDAY">
          <n-radio :value="3"> 连跌天数:3天及以上</n-radio>
          <n-radio :value="5"> 连跌天数:5天及以上</n-radio>
          <n-radio :value="8"> 连跌天数:8天及以上</n-radio>
          <n-radio :value="10"> 连跌天数:10天及以上</n-radio>
          <n-radio :value="14"> 连跌天数:14天及以上</n-radio>
        </n-radio-group>
      </n-card>

    </n-space>
    <n-input-group>
<!--    <n-input clearable placeholder="输入股票名称" v-model:value="paginationReactive.keyword"/>-->
      <n-auto-complete
          v-model:value="paginationReactive.keyword"
          :input-props="{
            autocomplete: 'disabled',
          }"
          :options="optionsReactive"
          placeholder="输入搜索关键词"
          clearable
          @input="handleUpdateVal"
          @select="(value) => {
            paginationReactive.keyword = value
            handleSearch()
          }"
      />
    <n-button type="primary" ghost @click="handleSearch"  @input="handleSearch">
      搜索
    </n-button>
      <n-button @click="handleReset">重置</n-button>

    </n-input-group>
    <!-- 数据表格 -->
    <n-data-table
      remote
      size="small"
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="paginationReactive"
      :row-key="(rowData) => rowData.SECUCODE"
      flex-height
      style="height: calc(100vh - 380px);margin-top: 10px"
      @update:page="handlePageChange"
    />
    
    <!-- 分页控件 -->
<!--    <div style="margin-top: 16px; display: flex; justify-content: center;">-->
<!--      <n-pagination-->
<!--        v-model:page="paginationReactive.page"-->
<!--        v-model:page-size="paginationReactive.pageSize"-->
<!--        :page-count="paginationReactive.pageCount"-->
<!--        :item-count="paginationReactive.itemCount"-->
<!--        :page-sizes="[10, 20, 50, 100]"-->
<!--        show-size-picker-->
<!--        show-quick-jumper-->
<!--        @update:page="handlePageChange"-->
<!--        @update:page-size="handlePageSizeChange"-->
<!--      />-->
<!--    </div>-->

  <n-modal v-model:show="modalDataRef.visible" :title="modalDataRef.title" preset="card" style="width: 850px;max-width: calc(100vw - 32px);">
    <n-card size="small">
      <KLineChart style="width: 100%;max-width: 800px;" :code="getStockCode(modalDataRef.stockCode)" :chart-height="500" :stock-name="modalDataRef.stockName" :k-days="30" :dark-theme="editorDataRef.darkTheme"></KLineChart>
    </n-card>
  </n-modal>

  <n-modal
    v-model:show="multiKlineModalShow"
    :title="(multiKlineName || '') + ' — 多周期K线'"
    preset="card"
    style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
    :content-style="{
      maxHeight: 'min(85vh, 820px)',
      overflowY: 'auto',
      overflowX: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
  >
    <StockLightweightKlineChart
      v-if="multiKlineModalShow && multiKlineCode"
      :key="'allstock-' + multiKlineCode"
      :code="multiKlineCode"
      :stock-name="multiKlineName"
      :dark-theme="editorDataRef.darkTheme"
      :chart-height="500"
    />
  </n-modal>

  <n-modal v-model:show="showFollowGroupModal" preset="dialog" title="新建分组" positive-text="创建并关注" negative-text="取消"
           @positive-click="handleCreateGroupAndFollow" style="width: 420px;">
    <n-form label-placement="left" label-width="80">
      <n-form-item label="分组名称">
        <n-input v-model:value="newGroupName" placeholder="请输入分组名称" @keyup.enter="handleCreateGroupAndFollow"/>
      </n-form-item>
    </n-form>
  </n-modal>
</template>

<style scoped>
</style>
