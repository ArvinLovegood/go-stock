<template>
  <n-drawer v-model:show="show" :width="760" placement="right">
    <n-drawer-content :title="'游资席位名录维护（' + (file?.meta?.version || '') + '）'" closable>
      <template #footer>
        <n-space>
          <n-button type="primary" :loading="saving" @click="save">保存（即时生效）</n-button>
          <n-popconfirm @positive-click="reset">
            <template #trigger>
              <n-button type="warning">恢复内置数据</n-button>
            </template>
            将覆盖当前全部自定义修改，确定？
          </n-popconfirm>
        </n-space>
      </template>

      <div class="hms-manager">
      <n-spin :show="loading">
        <n-text type="error" depth="3" style="display:block;margin-bottom:8px;font-size:12px">
          *游资花名与营业部映射为社区推断性共识（非官方认定），存在一人多席位/席位迁移/借马甲情况。
          席位名称支持简写（可省略"股份有限公司"），匹配时自动归一化。
        </n-text>

        <n-space vertical size="large">
          <!-- 远程同步 -->
          <n-card size="small" title="远程同步（可选）">
            <n-space>
              <n-input v-model:value="file.remoteUrl" placeholder="名录 JSON 的 URL（自托管/社区维护）" style="width: 480px" clearable/>
              <n-button :loading="refreshing" @click="refreshFromRemote">拉取</n-button>
            </n-space>
            <n-text depth="3" style="display:block;margin-top:4px;font-size:12px">
              配置后每次启动自动拉取一次；格式与本地 data/hot_money_seats.json 一致
            </n-text>
          </n-card>

          <!-- 游资列表 -->
          <n-card size="small">
            <template #header>
              <n-space>
                <span>游资列表（{{ file.hot_money_list.length }}）</span>
                <n-button size="small" @click="addHotMoney">+ 添加游资</n-button>
              </n-space>
            </template>
            <n-space vertical>
              <n-card v-for="(hm, i) in file.hot_money_list" :key="i" size="small" embedded>
                <template #header>
                  <n-input v-model:value="hm.name" placeholder="花名（必填）" size="small" style="width: 160px"/>
                </template>
                <template #header-extra>
                  <n-popconfirm @positive-click="file.hot_money_list.splice(i, 1)">
                    <template #trigger>
                      <n-button size="tiny" type="error" quaternary>删除</n-button>
                    </template>
                    确定删除该游资？
                  </n-popconfirm>
                </template>
                <n-grid :cols="24" :x-gap="12">
                  <n-form-item-gi :span="6" label="真名" label-placement="left" :show-feedback="false">
                    <n-input v-model:value="hm.real_name" placeholder="真名" size="small"/>
                  </n-form-item-gi>
                  <n-form-item-gi :span="6" label="梯队" label-placement="left" :show-feedback="false">
                    <n-input v-model:value="hm.tier" placeholder="如 一线/顶级" size="small"/>
                  </n-form-item-gi>
                  <n-form-item-gi :span="6" label="风险" label-placement="left" :show-feedback="false">
                    <n-input v-model:value="hm.risk_level" placeholder="如 低/中/高" size="small"/>
                  </n-form-item-gi>
                </n-grid>
                <n-input v-model:value="hm.style" placeholder="操作风格描述" size="small" style="margin-top: 6px"/>
                <n-text depth="3" style="display:block;margin-top:6px;font-size:12px">
                  席位（每行一个，行首加 * 号标记核心席位）：
                </n-text>
                <n-input
                  v-model:value="hm._seatsText" type="textarea" :autosize="{minRows: 2, maxRows: 10}"
                  placeholder="国泰君安证券上海江苏路证券营业部&#10;*中信证券杭州延安路证券营业部"/>
              </n-card>
            </n-space>
          </n-card>

          <!-- 特殊席位 -->
          <n-card size="small" title="散户集中营席位（每行一个）">
            <n-input v-model:value="file.special_seats.retail_cluster._seatsText" type="textarea"
                     :autosize="{minRows: 2, maxRows: 6}" placeholder="东方财富证券拉萨团结路第一证券营业部"/>
          </n-card>
          <n-card size="small" title="量化/机构通道席位（每行一个，格式：席位名称|类型）">
            <n-input v-model:value="file.special_seats.quant_seats._seatsText" type="textarea"
                     :autosize="{minRows: 2, maxRows: 6}" placeholder="华鑫证券上海分公司|量化明星席位"/>
          </n-card>
        </n-space>
      </n-spin>
      </div>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.hms-manager {
  text-align: left;
}

/* 表单项标签与内容全部左对齐 */
.hms-manager :deep(.n-form-item-label) {
  text-align: left;
}

.hms-manager :deep(.n-card-header__main),
.hms-manager :deep(.n-card__content),
.hms-manager :deep(.n-form-item-blank) {
  text-align: left;
}

/* 卡片头部右侧按钮区保持右侧，仅文字左对齐 */
.hms-manager :deep(.n-card-header__extra) {
  text-align: right;
}
</style>

<script setup lang="ts">
import {ref, watch} from 'vue'
import {GetHotMoneySeats, SaveHotMoneySeats, ResetHotMoneySeats, RefreshHotMoneySeats} from '../../wailsjs/go/main/App'
import {useMessage} from 'naive-ui'

const message = useMessage()
const show = ref(false)
const loading = ref(false)
const saving = ref(false)
const refreshing = ref(false)

const emptyFile = () => ({
  meta: {title: '', version: ''},
  remoteUrl: '',
  hot_money_list: [],
  special_seats: {
    retail_cluster: {name: '拉萨天团(散户集中营)', note: '', seats: [], _seatsText: ''},
    quant_seats: {name: '量化/机构通道席位', note: '', seats: [], _seatsText: ''},
  },
})
const file = ref<any>(emptyFile())

// 席位对象 <-> 文本互转（* 前缀 = primary）
const seatsToText = (seats: any[]) => (seats || []).map((s: any) => (s.primary ? '*' : '') + s.branch).join('\n')
const textToSeats = (text: string) => text.split('\n').map(l => l.trim()).filter(Boolean)
  .map(l => ({branch: l.replace(/^\*/, '').trim(), primary: l.startsWith('*')}))

const quantToText = (seats: any[]) => (seats || []).map((s: any) => `${s.branch}|${s.type || ''}`).join('\n')
const textToQuant = (text: string) => text.split('\n').map(l => l.trim()).filter(Boolean).map(l => {
  const [branch, ...rest] = l.split('|')
  return {branch: branch.trim(), type: rest.join('|').trim()}
})

// 加载名录并填充编辑态文本
function load() {
  loading.value = true
  GetHotMoneySeats().then((res: any) => {
    const f = res || emptyFile()
    f.hot_money_list = f.hot_money_list || []
    f.hot_money_list.forEach((hm: any) => {
      hm._seatsText = seatsToText(hm.seats)
    })
    f.special_seats = f.special_seats || {}
    f.special_seats.retail_cluster = f.special_seats.retail_cluster || {name: '', note: '', seats: []}
    f.special_seats.retail_cluster._seatsText = (f.special_seats.retail_cluster.seats || []).join('\n')
    f.special_seats.quant_seats = f.special_seats.quant_seats || {name: '', note: '', seats: []}
    f.special_seats.quant_seats._seatsText = quantToText(f.special_seats.quant_seats.seats)
    file.value = f
    loading.value = false
  }).catch((err: any) => {
    loading.value = false
    message.error('加载游资名录失败')
    console.error(err)
  })
}

watch(show, v => {
  if (v) load()
})

function addHotMoney() {
  file.value.hot_money_list.push({
    name: '', real_name: '', aliases: [], tier: '', style: '', risk_level: '',
    seats: [], _seatsText: '',
  })
}

// 组装提交结构（剥离编辑态字段，还原 seats 数组）
function buildPayload() {
  const f = file.value
  return {
    meta: f.meta,
    remoteUrl: (f.remoteUrl || '').trim(),
    hot_money_list: f.hot_money_list.map((hm: any) => ({
      name: hm.name, real_name: hm.real_name || '', aliases: hm.aliases || [],
      tier: hm.tier || '', style: hm.style || '', risk_level: hm.risk_level || '',
      seats: textToSeats(hm._seatsText || ''),
    })),
    special_seats: {
      retail_cluster: {
        name: f.special_seats.retail_cluster.name || '散户集中营',
        note: f.special_seats.retail_cluster.note || '',
        seats: (f.special_seats.retail_cluster._seatsText || '').split('\n').map((l: string) => l.trim()).filter(Boolean),
      },
      quant_seats: {
        name: f.special_seats.quant_seats.name || '量化/机构通道席位',
        note: f.special_seats.quant_seats.note || '',
        seats: textToQuant(f.special_seats.quant_seats._seatsText || ''),
      },
    },
  }
}

function save() {
  saving.value = true
  SaveHotMoneySeats(buildPayload()).then(() => {
    saving.value = false
    message.success('游资名录已保存并即时生效')
  }).catch((err: any) => {
    saving.value = false
    message.error(typeof err === 'string' ? err : '保存失败')
    console.error(err)
  })
}

function reset() {
  loading.value = true
  ResetHotMoneySeats().then(() => {
    message.success('已恢复内置数据')
    load()
  }).catch((err: any) => {
    loading.value = false
    message.error(typeof err === 'string' ? err : '重置失败')
    console.error(err)
  })
}

function refreshFromRemote() {
  const url = (file.value.remoteUrl || '').trim()
  if (!url) {
    message.warning('请先填写远程名录 URL')
    return
  }
  refreshing.value = true
  RefreshHotMoneySeats(url).then(() => {
    refreshing.value = false
    message.success('已从远程拉取')
    load()
  }).catch((err: any) => {
    refreshing.value = false
    message.error(typeof err === 'string' ? err : '拉取失败')
    console.error(err)
  })
}

defineExpose({show})
</script>
