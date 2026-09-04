<template>
  <div style="text-align: left">
    <n-grid :cols="24" :x-gap="12">
      <!-- 左侧：部门选择 -->
      <n-gi :span="4">
        <n-card size="small" title="部门">
          <n-flex vertical>
            <n-tag :type="currentDept === '全部部门' ? 'primary' : 'default'"
                   :bordered="currentDept === '全部部门'"
                   size="medium"
                   style="cursor: pointer; justify-content: flex-start"
                   @click="selectDept('全部部门')">
              全部部门
            </n-tag>
          </n-flex>
          <n-divider style="margin: 8px 0"/>
          <n-flex vertical>
            <n-tag v-for="dept in keyDeptList" :key="dept"
                   :type="currentDept === dept ? 'primary' : 'default'"
                   :bordered="currentDept === dept"
                   size="medium"
                   style="cursor: pointer; justify-content: flex-start"
                   closable
                   @click="selectDept(dept)"
                   @close="removeKeyDept(dept)">
              {{ shortDeptName(dept) }}
            </n-tag>
          </n-flex>
          <n-divider style="margin: 8px 0"/>
          <n-button size="tiny" dashed block @click="openKeyDeptManage">
            自定义重点部门
          </n-button>
          <template #header-extra>
            <n-text depth="3" style="font-size: 12px">{{ departments.length }} 个</n-text>
          </template>
        </n-card>
        <n-card size="small" title="选择部门" style="margin-top: 8px">
          <n-select v-model:value="selectedDept" filterable placeholder="搜索全部部门"
                    :options="deptOptions" size="small" @update:value="onDeptSelect"/>
        </n-card>
      </n-gi>

      <!-- 右侧：政策新闻列表 -->
      <n-gi :span="20">
        <n-card size="small">
          <template #header>
            <n-flex align="center" :wrap="false">
              <n-text strong>{{ headerTitle }}</n-text>
              <n-tag size="small" :bordered="false" type="info">{{ newsList.length }} 条</n-tag>
              <n-tag v-if="currentDept !== '全部部门' && currentDeptUrl" size="small" type="success" :bordered="false"
                     style="cursor: pointer" @click="openDeptSite">
                官网 ↗
              </n-tag>
              <n-tag v-if="currentDept !== '全部部门'" size="small" :bordered="false"
                     :type="isKeyDept ? 'warning' : 'default'"
                     style="cursor: pointer" :title="isKeyDept ? '从重点部门移除' : '加入重点部门'"
                     @click="toggleKeyDept">
                {{ isKeyDept ? '★ 已重点' : '☆ 加重点' }}
              </n-tag>
              <n-text depth="3" style="font-size: 12px">{{ headerHint }}</n-text>
            </n-flex>
          </template>
          <template #header-extra>
            <n-flex align="center" :wrap="false" :size="6">
              <n-input-group>
                <n-input v-model:value="keyword" placeholder="关键词搜索历史政策"
                         size="small" clearable style="width: 200px"
                         @keyup.enter="search" @clear="clearSearch"/>
                <n-button size="small" tertiary type="primary" :loading="loading" @click="search">
                  搜索
                </n-button>
              </n-input-group>
              <n-button v-if="searchMode" size="small" tertiary type="error" @click="clearSearch">
                退出搜索
              </n-button>
              <n-button v-else size="small" tertiary type="primary" :loading="loading" @click="refresh">
                <template #icon>
                  <n-icon><RefreshCircleSharp/></n-icon>
                </template>
                刷新
              </n-button>
            </n-flex>
          </template>
          <n-spin :show="loading">
            <n-list v-if="newsList.length" bordered hoverable clickable size="small">
              <n-list-item v-for="item in newsList" :key="item.url">
                <n-flex align="center" :size="8">
                  <n-tag size="small" :bordered="false" type="warning">{{ item.date }}</n-tag>
                  <n-tag size="small" :bordered="false" type="info" style="cursor: pointer"
                         @click="selectDept(item.source)">
                    {{ shortDeptName(item.source) }}
                  </n-tag>
                  <n-text style="flex: 1">
                    <a :href="item.url" target="_blank" style="text-decoration: none; color: inherit">
                      {{ item.title }}
                    </a>
                  </n-text>
                  <n-tag size="small" :bordered="false">
                    <a :href="item.url" target="_blank" style="text-decoration: none">
                      <n-text type="warning" style="font-size: 12px">原文</n-text>
                    </a>
                  </n-tag>
                </n-flex>
              </n-list-item>
            </n-list>
            <n-empty v-else-if="!loading" :description="emptyText" style="padding: 40px 0"/>
          </n-spin>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- 自定义重点部门弹窗 -->
    <n-modal v-model:show="showKeyDeptModal" preset="card" title="自定义重点部门" style="width: 560px">
      <n-flex vertical :size="12">
        <n-select v-model:value="keyDeptDraft" multiple filterable placeholder="搜索并选择重点部门"
                  :options="deptOptions" :max-tag-count="8" size="small"/>
        <n-text depth="3" style="font-size: 12px">
          清空后保存将恢复默认列表；重点部门显示在左侧快捷入口，保存后立即生效
        </n-text>
        <n-flex justify="end" :size="8">
          <n-button size="small" @click="resetKeyDepts">恢复默认</n-button>
          <n-button size="small" @click="showKeyDeptModal = false">取消</n-button>
          <n-button size="small" type="primary" :loading="savingKeyDepts" @click="saveKeyDepts">保存</n-button>
        </n-flex>
      </n-flex>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {
  GetAllDeptPolicyNews,
  GetGovDepartments,
  GetKeyDepartments,
  GetPolicyNews,
  GetStoredPolicyNews,
  SaveKeyDepartments
} from '../../wailsjs/go/main/App'
import {RefreshCircleSharp} from '@vicons/ionicons5'
import {useMessage} from 'naive-ui'
import {BrowserOpenURL, EventsOn} from '../../wailsjs/runtime/runtime'

const message = useMessage()
const loading = ref(false)
const departments = ref<any[]>([])
const selectedDept = ref<string | null>(null)
const currentDept = ref('全部部门')
const newsList = ref<any[]>([])
const keyword = ref('')
const searchMode = ref(false)

// 重点部门快捷入口（用户自定义，后端持久化 data/key_departments.json）
const keyDeptList = ref<string[]>([])
const showKeyDeptModal = ref(false)
const keyDeptDraft = ref<string[]>([])
const savingKeyDepts = ref(false)

// 加载重点部门配置
function loadKeyDepts() {
  GetKeyDepartments().then(res => {
    keyDeptList.value = res || []
  }).catch(err => {
    console.error('获取重点部门失败', err)
  })
}

// 当前选中部门是否已加入重点
const isKeyDept = computed(() =>
    currentDept.value !== '全部部门' && keyDeptList.value.includes(currentDept.value)
)

// 星标快捷切换：加入/移除当前部门
function toggleKeyDept() {
  if (currentDept.value === '全部部门') return
  if (isKeyDept.value) {
    removeKeyDept(currentDept.value)
  } else {
    const list = [...keyDeptList.value, currentDept.value]
    persistKeyDepts(list, `"${currentDept.value}" 已加入重点部门`)
  }
}

// 左侧标签 close 移除
function removeKeyDept(dept: string) {
  persistKeyDepts(keyDeptList.value.filter(d => d !== dept), `"${dept}" 已从重点部门移除`)
}

// 统一保存（列表为空时后端恢复默认）
function persistKeyDepts(list: string[], okMsg: string) {
  savingKeyDepts.value = true
  SaveKeyDepartments(list).then(err => {
    savingKeyDepts.value = false
    if (err) {
      message.error(err)
      return
    }
    keyDeptList.value = list
    if (list.length) {
      message.success(okMsg)
    } else {
      message.info('已恢复默认重点部门列表')
      loadKeyDepts()
    }
  }).catch(err => {
    savingKeyDepts.value = false
    message.error('保存失败')
    console.error(err)
  })
}

// 打开自定义弹窗（草稿 = 当前列表）
function openKeyDeptManage() {
  keyDeptDraft.value = [...keyDeptList.value]
  showKeyDeptModal.value = true
}

// 弹窗保存
function saveKeyDepts() {
  persistKeyDepts([...keyDeptDraft.value], '重点部门已更新')
  showKeyDeptModal.value = false
}

// 弹窗恢复默认：清空草稿保存（后端空列表即恢复默认）
function resetKeyDepts() {
  keyDeptDraft.value = []
  persistKeyDepts([], '')
}

const deptOptions = computed(() =>
    departments.value.map(d => ({label: d.name, value: d.name}))
)

const headerTitle = computed(() =>
    searchMode.value ? `搜索：${keyword.value}` : currentDept.value
)

// 当前选中部门的官网链接（"全部部门"时为空）
const currentDeptUrl = computed(() => {
  if (currentDept.value === '全部部门') return ''
  const d = departments.value.find(d => d.name === currentDept.value)
  return d?.url || ''
})

// 系统默认浏览器打开部门官网
function openDeptSite() {
  if (currentDeptUrl.value) BrowserOpenURL(currentDeptUrl.value)
}

const headerHint = computed(() => {
  if (searchMode.value) return '搜索已入库的历史政策'
  return currentDept.value === '全部部门' ? '全部部门按日期倒序' : '只展示该部门官网发布的内容'
})

const emptyText = computed(() => {
  if (searchMode.value) return '没有匹配的政策新闻，换个关键词试试'
  return currentDept.value === '全部部门'
      ? '暂无数据，请点击刷新重试'
      : '该部门网站暂未抓取到政策新闻（可能为动态渲染站点）'
})

// 部门名缩短显示
function shortDeptName(name: string) {
  return name
      .replace('中华人民共和国', '')
      .replace('国家发展和改革委员会', '发改委')
      .replace('中国证券监督管理委员会', '证监会')
      .replace('中国人民银行', '央行')
      .replace('国家金融监督管理总局', '金监总局')
      .replace('国家外汇管理局', '外汇局')
      .replace('国家统计局', '统计局')
      .replace('国家卫生健康委员会', '卫健委')
      .replace('人力资源和社会保障部', '人社部')
      .replace('住房和城乡建设部', '住建部')
      .replace('国有资产监督管理委员会', '国资委')
      .replace('市场监督管理总局', '市场监管总局')
      .replace('国家数据局', '数据局')
      .replace('国家能源局', '能源局')
      .replace('国家国际发展合作署', '国合署')
      .replace('国家国防科技工业局', '国防科工局')
}

function onDeptSelect(name: string) {
  if (name) selectDept(name)
}

function selectDept(dept: string) {
  if (!dept) return
  currentDept.value = dept
  selectedDept.value = dept === '全部部门' ? null : dept
  if (searchMode.value) {
    // 搜索模式下切换部门：保持搜索，仅换过滤部门
    search()
  } else {
    fetchNews()
  }
}

// 默认加载：读库（秒开，后台定时任务持续入库），库为空时回退实时抓取
function fetchNews() {
  loading.value = true
  const dept = currentDept.value === '全部部门' ? '' : currentDept.value
  GetStoredPolicyNews(dept, '', 1, 100).then(res => {
    newsList.value = res || []
    if (newsList.value.length) {
      loading.value = false
      return
    }
    // 库中无数据（首次使用或该部门未入库）：实时抓取
    fetchLatest()
  }).catch(err => {
    loading.value = false
    message.error('获取政策新闻失败')
    console.error(err)
  })
}

// 实时抓取（刷新按钮 / 库为空回退）：抓部门网站并自动入库
function fetchLatest() {
  loading.value = true
  if (currentDept.value === '全部部门') {
    GetAllDeptPolicyNews(100).then(res => {
      newsList.value = res || []
      loading.value = false
      if (!newsList.value.length) message.info('暂未抓取到政策新闻')
    }).catch(err => {
      loading.value = false
      message.error('获取政策新闻失败')
      console.error(err)
    })
  } else {
    GetPolicyNews(currentDept.value, 30).then(res => {
      newsList.value = res || []
      loading.value = false
      if (!newsList.value.length) {
        message.info('该部门网站暂未抓取到政策新闻（可能为动态渲染站点）')
      }
    }).catch(err => {
      loading.value = false
      message.error('获取政策新闻失败')
      console.error(err)
    })
  }
}

// 关键词搜索：查询已入库的历史政策（部门跟随当前选择）
function search() {
  const kw = keyword.value.trim()
  if (!kw) {
    clearSearch()
    return
  }
  loading.value = true
  searchMode.value = true
  const dept = currentDept.value === '全部部门' ? '' : currentDept.value
  GetStoredPolicyNews(dept, kw, 1, 200).then(res => {
    newsList.value = res || []
    loading.value = false
    if (!newsList.value.length) {
      message.info('没有匹配的历史政策，可先刷新抓取最新政策入库')
    }
  }).catch(err => {
    loading.value = false
    message.error('搜索失败')
    console.error(err)
  })
}

function clearSearch() {
  keyword.value = ''
  searchMode.value = false
  fetchNews()
}

// 刷新按钮：实时抓取部门网站（结果自动入库并更新缓存）
function refresh() {
  if (searchMode.value) {
    clearSearch()
    return
  }
  fetchLatest()
}

// 后台定时抓取完成后自动刷新（数据已入库，直接读库，非搜索模式）
EventsOn("policyNewsUpdated", () => {
  if (!searchMode.value && !loading.value) {
    fetchNews()
  }
})

onMounted(() => {
  GetGovDepartments().then(res => {
    departments.value = res || []
  })
  loadKeyDepts()
  fetchNews()
})
</script>

<style scoped>

</style>
