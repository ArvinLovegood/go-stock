<script setup lang="ts">
import {h, onBeforeMount, onMounted, onUnmounted, ref, reactive, computed} from 'vue'
import {SearchStock, GetHotStrategy, OpenURL, Follow, GetFollowList, GetAllCustomStrategies, SaveCustomStrategy, DeleteCustomStrategy, GetEffectiveSponsorVip, GetConfig, GetGroupList, AddStockGroup, AddGroup} from "../../wailsjs/go/main/App";
import {useMessage, NText, NTag, NButton, NPopconfirm, NDropdown, NIcon} from 'naive-ui'
import {Environment} from "../../wailsjs/runtime"
import {BookmarkOutline, TrashOutline, CreateOutline, AddOutline, FolderOpenOutline} from "@vicons/ionicons5";
import {EventsEmit} from "../../wailsjs/runtime";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";

const message = useMessage()
const search = ref('')
const columns = ref([])
const dataList = ref([])
const hotStrategy = ref([])
const customStrategies = ref([])
const traceInfo = ref('')
const tableScrollX = ref(2800)
const leftTab = ref('hot')
const showSaveModal = ref(false)
const vipLevel = ref(0)
const darkTheme = ref(false)
const klineModalShow = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
let klineAutoCloseTimer = null
const saveForm = reactive({
  id: 0,
  name: '',
  query: '',
  description: '',
  sortOrder: 0,
})

// 关注分组选择
const groupList = ref<any[]>([])
const showFollowGroupModal = ref(false)
const newGroupName = ref('')
const pendingFollowRow = ref<any>(null)

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
  let code = row.MARKET_SHORT_NAME.toLowerCase() + row.SECURITY_CODE
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
  // 排序值取当前最大 sort + 1，追加到末尾
  const maxSort = groupList.value.reduce((max, g) => Math.max(max, g.sort || 0), 0)
  AddGroup({name, sort: maxSort + 1}).then(res => {
    if (res === '添加成功') {
      GetGroupList().then(list => {
        groupList.value = list || []
        showFollowGroupModal.value = false
        // 通过名称找到新建分组 ID
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

const paginationProps = computed(() => ({
  pageSize: 10,
  prefix: ({itemCount}) => h('span', {style: 'margin-right: 8px'}, [
    '共找到 ',
    h(NTag, {type: 'info', bordered: false, size: 'small'}, {default: () => itemCount}),
    ' 只股',
  ]),
}))

function calculateTableWidth(cols) {
  let totalWidth = 0;
  cols.forEach(col => {
    if (col.children && col.children.length > 0) {
      let childrenWidth = 0;
      col.children.forEach(child => {
        childrenWidth += child.width || child.minWidth || 100;
      });
      totalWidth += Math.max(col.width || col.minWidth || 200, childrenWidth);
    } else {
      totalWidth += col.width || col.minWidth || 120;
    }
  });
  totalWidth += 100;
  return Math.max(totalWidth, 1200);
}

function Search() {
  if (!search.value) {
    message.warning('请输入选股指标或者要求')
    return
  }
  const loading = message.loading("正在获取选股数据...", {duration: 0});
  SearchStock(search.value).then(res => {
    loading.destroy()
    if (res.code == 100) {
      traceInfo.value = res.data.traceInfo.showText
      columns.value = res.data.result.columns.filter(item => !item.hiddenNeed && (item.title != "市场码" && item.title != "市场简称")).map(item => {
        if (item.children) {
          return {
            title: item.title + (item.unit ? '[' + item.unit + ']' : ''),
            key: item.key,
            resizable: true,
            minWidth: 200,
            ellipsis: {tooltip: true},
            children: item.children.filter(item => !item.hiddenNeed).map(item => {
              return {
                title: item.dateMsg,
                key: item.key,
                minWidth: 100,
                resizable: true,
                ellipsis: {tooltip: true},
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
            ellipsis: {tooltip: true},
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
      columns.value.push({
        title: '操作',
        key: 'actions',
        width: 170,
        fixed: 'right',
        render: (row) => {
          return h('div', {style: 'display:flex;gap:4px;align-items:center;'}, [
            h(
              NButton,
              {
                size: 'tiny',
                type: 'info',
                onClick: () => showStockKline(row)
              },
              {default: () => 'K线'}
            ),
            h(
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
                    strong: true,
                    tertiary: true,
                    size: 'small',
                    type: 'warning',
                    style: 'font-size: 14px; padding: 0 10px;',
                  },
                  {
                    default: () => '关注',
                    icon: () => h(NIcon, null, {default: () => h(FolderOpenOutline, {size: 14})})
                  }
                )
              }
            )
          ])
        }
      });
      dataList.value = res.data.result.dataList
      tableScrollX.value = calculateTableWidth(columns.value);
    } else {
      if (res.msg) {
        message.error(res.msg)
      }
      if (res.message) {
        message.error(res.message)
      }
    }
  }).catch(err => {
    message.error(err)
  })
}

function refreshEffectiveVip() {
  return GetEffectiveSponsorVip().then(res => {
    if (res) {
      vipLevel.value = res.vipLevel || 0
    }
  }).catch(() => {})
}

function toEastMoneyCode(stockCode, marketShortName) {
  const m = (marketShortName || '').toUpperCase()
  if (m === 'SH' || m === 'SZ' || m === 'BJ') return stockCode + '.' + m
  if (m === 'HK') return stockCode + '.HK'
  if (m === 'US') return stockCode + '.US'
  if (/^(6|5)/.test(stockCode)) return stockCode + '.SH'
  if (/^(8|4)/.test(stockCode)) return stockCode + '.BJ'
  // 纯字母代码视为美股（如 AAPL）
  if (/^[a-zA-Z]+$/.test(stockCode)) return stockCode.toUpperCase() + '.US'
  return stockCode + '.SZ'
}

function showStockKline(row) {
  const stockCode = row.SECURITY_CODE
  const stockName = row.SECURITY_SHORT_NAME
  const em = toEastMoneyCode(stockCode, row.MARKET_SHORT_NAME)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  refreshEffectiveVip().then(() => {
    klineStockCode.value = em
    klineStockName.value = stockName || ''
    if (vipLevel.value < 2) {
      message.warning('K线图仅限 VIP2 及以上用户使用，您当前权限不足，将在 10 秒后自动关闭')
      klineModalShow.value = true
      if (klineAutoCloseTimer) clearTimeout(klineAutoCloseTimer)
      klineAutoCloseTimer = setTimeout(() => {
        klineModalShow.value = false
      }, 10000)
      return
    }
    klineModalShow.value = true
    if (klineAutoCloseTimer) clearTimeout(klineAutoCloseTimer)
  })
}

function isNumeric(value) {
  return !isNaN(parseFloat(value)) && isFinite(value);
}

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result.darkTheme) darkTheme.value = true
  })
  GetHotStrategy().then(res => {
    if (res.code == 1) {
      hotStrategy.value = res.data
      search.value = hotStrategy.value[0].question
      Search()
    }
  }).catch(err => {
    message.error(err)
  })
  loadCustomStrategies()
  loadGroupList()
})

function loadCustomStrategies() {
  GetAllCustomStrategies().then(res => {
    customStrategies.value = res || []
  }).catch(err => {
    message.error(err)
  })
}

function DoSearch(question) {
  search.value = question
  Search()
}

function openSaveModal(isEdit = false, strategy = null) {
  if (isEdit && strategy) {
    saveForm.id = strategy.id
    saveForm.name = strategy.name
    saveForm.query = strategy.query
    saveForm.description = strategy.description || ''
    saveForm.sortOrder = strategy.sortOrder || 0
  } else {
    saveForm.id = 0
    saveForm.name = ''
    saveForm.query = search.value
    saveForm.description = ''
    saveForm.sortOrder = 0
  }
  showSaveModal.value = true
}

function handleSaveStrategy() {
  if (!saveForm.name.trim()) {
    message.warning('请输入策略名称')
    return
  }
  if (!saveForm.query.trim()) {
    message.warning('请输入选股条件')
    return
  }
  SaveCustomStrategy({
    id: saveForm.id || 0,
    name: saveForm.name,
    query: saveForm.query,
    description: saveForm.description,
    sortOrder: saveForm.sortOrder,
  }).then(res => {
    message.success(res)
    showSaveModal.value = false
    loadCustomStrategies()
  }).catch(err => {
    message.error(err)
  })
}

function handleDeleteStrategy(id) {
  DeleteCustomStrategy(id).then(res => {
    message.success(res)
    loadCustomStrategies()
  }).catch(err => {
    message.error(err)
  })
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
  <n-grid :cols="24" style="max-height: calc(100vh - 165px)">
    <n-gi :span="4">
      <n-tabs v-model:value="leftTab" type="segment" size="small" style="margin-bottom: 4px;">
        <n-tab name="hot">热门策略</n-tab>
        <n-tab name="custom">我的策略</n-tab>
      </n-tabs>

      <n-list bordered style="text-align: left;" hoverable clickable v-show="leftTab==='hot'">
        <n-scrollbar style="max-height: calc(100vh - 210px);">
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

      <div v-show="leftTab==='custom'">
        <n-scrollbar style="max-height: calc(100vh - 250px);">
          <n-list bordered hoverable clickable v-if="customStrategies.length > 0">
            <n-list-item v-for="item in customStrategies" :key="item.id">
              <template #suffix>
                <n-flex :size="2" align="center">
                  <n-button text type="info" size="small" @click.stop="openSaveModal(true, item)">
                    <template #icon><n-icon :component="CreateOutline"/></template>
                  </n-button>
                  <n-popconfirm @positive-click="handleDeleteStrategy(item.id)">
                    <template #trigger>
                      <n-button text type="error" size="small" @click.stop>
                        <template #icon><n-icon :component="TrashOutline"/></template>
                      </n-button>
                    </template>
                    确定删除策略「{{ item.name }}」吗？
                  </n-popconfirm>
                </n-flex>
              </template>
              <div @click="DoSearch(item.query)" style="cursor: pointer;">
                <n-ellipsis line-clamp="1" :tooltip="true">
                  <n-tag size="small" :bordered="false" type="success">
                    <template #icon><n-icon :component="BookmarkOutline" size="12"/></template>
                  </n-tag>
                  <n-text strong>{{ item.name }}</n-text>
                  <template #tooltip>
                    <div style="max-width: 200px">
                      <div><n-text strong>{{ item.name }}</n-text></div>
                      <div v-if="item.description" style="margin-top:2px"><n-text depth="3">{{ item.description }}</n-text></div>
                      <div style="margin-top:2px"><n-text type="warning">{{ item.query }}</n-text></div>
                    </div>
                  </template>
                </n-ellipsis>
                <n-ellipsis line-clamp="1" style="margin-top: 2px;">
                  <n-text depth="3" style="font-size: 12px;">{{ item.query }}</n-text>
                </n-ellipsis>
              </div>
            </n-list-item>
          </n-list>
          <n-empty v-else description="暂无自定义策略" style="margin-top: 40px;"/>
        </n-scrollbar>
        <n-button block dashed type="primary" size="small" @click="openSaveModal(false)" style="margin-top: 4px;">
          <template #icon><n-icon :component="AddOutline"/></template>
          添加策略
        </n-button>
      </div>
    </n-gi>
    <n-gi :span="20">
      <div style="--wails-draggable:no-drag">
        <n-input-group style="text-align: left">
          <n-input :rows="1" clearable v-model:value="search" placeholder="请输入选股指标或者要求" @keyup.enter="Search"/>
          <n-button type="primary" @click="Search">搜索A股</n-button>
          <n-button type="warning" @click="openSaveModal(false)" :disabled="!search">
            <template #icon><n-icon :component="BookmarkOutline" size="16"/></template>
            保存策略
          </n-button>
        </n-input-group>
      </div>
      <div v-if="traceInfo" style="margin: 5px 0; --wails-draggable:no-drag">
        <n-ellipsis line-clamp="1" :tooltip="true">
          <n-text type="info" :bordered="false">选股条件：</n-text>
          <n-text type="warning" :bordered="true">{{ traceInfo }}</n-text>
          <template #tooltip>
            <div style="text-align: center;max-width: 580px">
              <n-text type="warning">{{ traceInfo }}</n-text>
            </div>
          </template>
        </n-ellipsis>
      </div>
      <n-data-table
          :striped="true"
          flex-height
          size="small"
          :columns="columns"
          :data="dataList"
          :pagination="paginationProps"
          :scroll-x="tableScrollX"
          style="height: calc(100vh - 240px)"
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
              return h(NText, { type: 'info',bordered: false ,size:'small',onClick:()=>{
               openCenteredWindow(`https://quote.eastmoney.com/${rowData.MARKET_SHORT_NAME}${rowData.SECURITY_CODE}.html#fullScreenChart`,1240,700)
              }}, { default: () => `${value}` })
            }else{
              return h(NText, { type: 'info' }, { default: () => `${value}` })
            }
          }
      }"
      />
    </n-gi>
  </n-grid>

  <n-modal v-model:show="showSaveModal" preset="dialog" :title="saveForm.id ? '编辑策略' : '保存策略'" positive-text="保存" negative-text="取消"
           @positive-click="handleSaveStrategy" style="width: 500px;">
    <n-form label-placement="left" label-width="80">
      <n-form-item label="策略名称">
        <n-input v-model:value="saveForm.name" placeholder="请输入策略名称"/>
      </n-form-item>
      <n-form-item label="选股条件">
        <n-input v-model:value="saveForm.query" type="textarea" :rows="3" placeholder="请输入选股条件"/>
      </n-form-item>
      <n-form-item label="策略描述">
        <n-input v-model:value="saveForm.description" type="textarea" :rows="2" placeholder="可选，对策略的简要说明"/>
      </n-form-item>
    </n-form>
  </n-modal>

  <n-modal
    v-model:show="klineModalShow"
    :title="(klineStockName || '') + ' - ' + klineStockCode + ' K线图'"
    preset="card"
    style="width: 1400px;max-width: calc(100vw - 32px);"
    :mask-closable="true"
  >
    <StockLightweightKlineChart
      v-if="klineModalShow && klineStockCode"
      :key="klineStockCode"
      :code="klineStockCode"
      :stock-name="klineStockName"
      :dark-theme="darkTheme"
      :chart-height="460"
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
