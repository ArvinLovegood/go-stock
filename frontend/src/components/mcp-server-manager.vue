<template>
  <n-space vertical style="margin-bottom: 12px">
    <n-space>
      <n-input
        v-model:value="searchKeyword"
        placeholder="搜索服务器名称..."
        style="width: 200px"
        clearable
        @keyup.enter="handleSearch"
      >
        <template #prefix>
          <n-icon :component="SearchOutline" />
        </template>
      </n-input>

      <n-select
        v-model:value="filterStatus"
        :options="statusOptions"
        placeholder="服务器状态"
        style="width: 120px"
        clearable
      />

      <n-button type="primary" @click="handleSearch">
        搜索
      </n-button>

      <n-button type="warning" @click="handleCreate">
        <template #icon>
          <n-icon :component="AddOutline" />
        </template>
        新建服务器
      </n-button>
    </n-space>
  </n-space>

  <n-data-table
    remote
    size="small"
    :columns="columns"
    :data="serverList"
    :loading="loading"
    :pagination="pagination"
    :row-key="(rowData) => rowData.id"
    :expanded-row-keys="expandedKeys"
    :row-props="rowProps"
    @update:expanded-row-keys="handleExpand"
    @update:page="handlePageChange"
    flex-height
    style="height: calc(100vh - 210px); margin-top: 10px"
  />

  <n-modal
    v-model:show="showCreateModal"
    :title="editingServer ? '修改服务器' : '创建新服务器'"
    preset="dialog"
    :style="{ width: '750px' }"
    @close="resetForm"
    :z-index="2000"
    to="body"
  >
    <n-form
      ref="formRef"
      :model="formData"
      :rules="formRules"
      label-placement="left"
      label-width="130px"
      require-mark-placement="right-hanging"
    >
      <n-form-item label="服务器名称" path="name">
        <n-input v-model:value="formData.name" placeholder="请输入服务器名称" clearable />
      </n-form-item>

      <n-form-item label="描述" path="description">
        <n-input
          v-model:value="formData.description"
          type="textarea"
          :rows="2"
          placeholder="请输入服务器描述（可选）"
          show-count
          maxlength="500"
        />
      </n-form-item>

      <n-form-item label="URL" path="url">
        <n-input v-model:value="formData.url" placeholder="例如：http://localhost:8080 或 SSE 端点地址" clearable />
      </n-form-item>

      <n-form-item label="HTTP Headers">
        <div style="width: 100%">
          <div
            v-for="(item, index) in headerList"
            :key="index"
            style="display: flex; gap: 8px; margin-bottom: 8px; align-items: center"
          >
            <n-input
              v-model:value="item.key"
              placeholder="Header 名称（如 X-api-key）"
              style="flex: 1"
            />
            <n-input
              v-model:value="item.value"
              placeholder="Header 值"
              style="flex: 1.5"
            />
            <n-button quaternary circle @click="removeHeader(index)">
              <template #icon>
                <n-icon :component="TrashOutline" />
              </template>
            </n-button>
          </div>
          <n-button dashed block size="small" @click="addHeader">
            <template #icon>
              <n-icon :component="AddOutline" />
            </template>
            添加 Header
          </n-button>
        </div>
      </n-form-item>

      <n-form-item label="启用状态" path="enable">
        <n-switch v-model:value="formData.enable" size="large">
          <template #checked>
            <n-icon :component="PlayCircleOutline" />
            启用
          </template>
          <template #unchecked>
            <n-icon :component="StopCircleOutline" />
            禁用
          </template>
        </n-switch>
      </n-form-item>
    </n-form>

    <template #action>
      <n-button @click="showCreateModal = false">取消</n-button>
      <n-button type="primary" @click="handleSubmit" :loading="submitting">
        <template #icon>
          <n-icon :component="CheckmarkCircleOutline" />
        </template>
        {{ editingServer ? '修改服务器' : '创建新服务器' }}
      </n-button>
    </template>
  </n-modal>

  <n-modal
    v-model:show="showToolDetailModal"
    title="工具参数详情"
    preset="card"
    style="width: 850px"
    :z-index="2000"
  >
    <template v-if="currentTool">
      <n-descriptions bordered label-placement="top" :column="1" style="margin-bottom: 16px" content-style="text-align: left">
        <n-descriptions-item label="工具名称">
          <n-text code>{{ currentTool.toolName }}</n-text>
        </n-descriptions-item>
        <n-descriptions-item label="描述">{{ currentTool.description || '无描述' }}</n-descriptions-item>
      </n-descriptions>

      <template v-if="parsedParams.length > 0">
        <n-text strong style="margin-bottom: 8px; display: block">参数列表</n-text>
        <n-data-table
          :columns="paramDetailColumns"
          :data="parsedParams"
          size="small"
          bordered
          :pagination="false"
        />
      </template>
      <n-text v-else depth="3">此工具无需参数</n-text>

      <n-collapse style="margin-top: 12px" v-if="currentTool.paramsSchema">
        <n-collapse-item title="原始 JSON Schema" name="raw">
          <VueJsonPretty
            :data="parseJSON(currentTool.paramsSchema)"
            :deep="3"
            show-length
            show-line
            collapsed-on-click-bracket
          />
        </n-collapse-item>
      </n-collapse>
    </template>
  </n-modal>
</template>

<script setup>
import { ref, reactive, onMounted, computed, h } from 'vue'
import {
  NButton,
  NIcon,
  NTag,
  NSpace,
  NPopconfirm,
  NDescriptions,
  NDescriptionsItem,
  NText,
  NDataTable,
  NCollapse,
  NCollapseItem,
  useMessage
} from 'naive-ui'
import VueJsonPretty from 'vue-json-pretty'
import 'vue-json-pretty/lib/styles.css'
import {
  SearchOutline,
  AddOutline,
  PlayOutline,
  PauseOutline,
  TrashOutline,
  CreateOutline,
  PlayCircleOutline,
  StopCircleOutline,
  CheckmarkCircleOutline,
  FlashOutline,
  EyeOutline
} from '@vicons/ionicons5'
import {
  CreateMCPServer,
  UpdateMCPServer,
  DeleteMCPServer,
  GetMCPServerByID,
  GetMCPServerList,
  EnableMCPServer,
  TestMCPServer,
  GetMCPToolsByServerID,
  GetAllMCPTools
} from '../../wailsjs/go/main/App'

const message = useMessage()

const formRef = ref(null)

const loading = ref(false)
const submitting = ref(false)
const showCreateModal = ref(false)
const showToolDetailModal = ref(false)
const editingServer = ref(false)
const searchKeyword = ref('')
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const expandedKeys = ref([])
const currentTool = ref(null)

const formData = reactive({
  id: null,
  name: '',
  description: '',
  url: '',
  command: '',
  args: '',
  headers: '',
  enable: true,
  status: 'untested'
})

// HTTP Headers 表单式编辑：避免用户直接写 JSON
const headerList = reactive([])

const addHeader = () => {
  headerList.push({ key: '', value: '' })
}

const removeHeader = (index) => {
  headerList.splice(index, 1)
}

const parseHeadersToList = (jsonStr) => {
  headerList.splice(0, headerList.length)
  if (!jsonStr) {
    headerList.push({ key: '', value: '' })
    return
  }
  try {
    const obj = JSON.parse(jsonStr)
    Object.entries(obj).forEach(([k, v]) => {
      headerList.push({ key: k, value: String(v) })
    })
  } catch (e) {
    // JSON 解析失败时留空，让用户重新填写
  }
  if (headerList.length === 0) {
    headerList.push({ key: '', value: '' })
  }
}

const serializeHeaders = () => {
  const obj = {}
  headerList.forEach((item) => {
    const key = item.key.trim()
    if (key) {
      obj[key] = item.value
    }
  })
  return Object.keys(obj).length === 0 ? '' : JSON.stringify(obj)
}

const formRules = {
  name: { required: true, message: '请输入服务器名称', trigger: ['input', 'blur'] },
  command: { required: true, message: '请输入命令', trigger: ['input', 'blur'] }
}

const statusOptions = [
  { label: '可用', value: 'available' },
  { label: '未测试', value: 'untested' },
  { label: '不可用', value: 'unavailable' }
]

const getStatusLabel = (status) => {
  switch (status) {
    case 'available':
      return '可用'
    case 'untested':
      return '未测试'
    case 'unavailable':
      return '不可用'
    default:
      return status
  }
}

const formatJSON = (str) => {
  try {
    return JSON.stringify(JSON.parse(str), null, 2)
  } catch {
    return str
  }
}

const parseJSON = (str) => {
  try {
    return JSON.parse(str)
  } catch {
    return str
  }
}

const handleExpand = (keys) => {
  expandedKeys.value = keys
}

const rowProps = (row) => {
  return {
    style: 'cursor: pointer',
    onClick: (e) => {
      if (e.target.closest('.n-button, .n-popconfirm, .n-switch, a')) return
      const key = row.id
      const idx = expandedKeys.value.indexOf(key)
      if (idx === -1) {
        expandedKeys.value = [...expandedKeys.value, key]
      } else {
        expandedKeys.value = expandedKeys.value.filter(k => k !== key)
      }
    }
  }
}

const handleViewToolDetail = (tool) => {
  currentTool.value = tool
  showToolDetailModal.value = true
}

const parsedParams = computed(() => {
  if (!currentTool.value || !currentTool.value.paramsSchema) return []
  try {
    const schema = JSON.parse(currentTool.value.paramsSchema)
    const props = schema.properties || {}
    const required = schema.required || []
    return Object.entries(props).map(([name, prop]) => ({
      name,
      type: prop.type || '-',
      required: required.includes(name),
      description: prop.description || prop.desc || '-',
      enum: prop.enum ? prop.enum.join(', ') : '',
      default: prop.default !== undefined ? String(prop.default) : ''
    }))
  } catch {
    return []
  }
})

const paramDetailColumns = [
  {
    title: '参数名',
    key: 'name',
    width: 180,
    ellipsis: { tooltip: { style: { maxWidth: '300px' } } }
  },
  {
    title: '类型',
    key: 'type',
    width: 80
  },
  {
    title: '必填',
    key: 'required',
    width: 60,
    render(row) {
      return h(NTag, { type: row.required ? 'error' : 'default', size: 'small' }, {
        default: () => row.required ? '是' : '否'
      })
    }
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: { style: { maxWidth: '400px', wordBreak: 'break-all' } } }
  },
  {
    title: '枚举值',
    key: 'enum',
    width: 120,
    ellipsis: { tooltip: { style: { maxWidth: '300px' } } }
  },
  {
    title: '默认值',
    key: 'default',
    width: 80,
    ellipsis: { tooltip: true }
  }
]

const columns = [
  {
    type: 'expand',
    renderExpand: (row) => {
      const tools = row.tools
      if (!tools || tools.length === 0) {
        return h(NText, { depth: 3, style: 'padding: 8px 16px' }, { default: () => '暂无工具信息，请先测试连接以获取工具列表' })
      }

      const toolColumns = [
        {
          title: '工具名称',
          key: 'toolName',
          width: 200,
          ellipsis: { tooltip: true }
        },
        {
          title: '描述',
          key: 'description',
          ellipsis: { tooltip: { style: { maxWidth: '400px', wordBreak: 'break-all' } } }
        },
        {
          title: '参数',
          key: 'paramsSchema',
          width: 260,
          render(toolRow) {
            if (!toolRow.paramsSchema) {
              return h(NTag, { type: 'default', size: 'small' }, { default: () => '无参数' })
            }
            try {
              const schema = JSON.parse(toolRow.paramsSchema)
              const props = schema.properties || {}
              const required = schema.required || []
              const paramNames = Object.keys(props)
              if (paramNames.length === 0) {
                return h(NTag, { type: 'default', size: 'small' }, { default: () => '无参数' })
              }
              const tags = paramNames.map(name => {
                const prop = props[name]
                const isReq = required.includes(name)
                return h(NTag, {
                  size: 'small',
                  type: isReq ? 'info' : 'default',
                  style: 'margin: 2px'
                }, {
                  default: () => name + (prop.type ? ':' + prop.type : '') + (isReq ? '*' : '')
                })
              })
              return h('div', { style: 'display: flex; flex-wrap: wrap; align-items: center; gap: 0' }, [
                ...tags,
                h(NButton, {
                  size: 'tiny',
                  type: 'info',
                  quaternary: true,
                  style: 'margin-left: 4px',
                  onClick: () => handleViewToolDetail(toolRow)
                }, {
                  icon: () => h(NIcon, { component: EyeOutline })
                })
              ])
            } catch {
              return h(NButton, {
                size: 'tiny',
                type: 'info',
                quaternary: true,
                onClick: () => handleViewToolDetail(toolRow)
              }, {
                icon: () => h(NIcon, { component: EyeOutline }),
                default: () => '查看参数'
              })
            }
          }
        }
      ]

      return h('div', { style: 'padding: 8px 16px' }, [
        h(NDataTable, {
          columns: toolColumns,
          data: tools,
          size: 'small',
          bordered: false,
          rowKey: (t) => t.id,
          pagination: false
        })
      ])
    }
  },
  {
    title: 'ID',
    key: 'id',
    width: 60,
    ellipsis: { tooltip: true }
  },
  {
    title: '服务器名称',
    key: 'name',
    width: 180,
    ellipsis: { tooltip: true }
  },
  {
    title: '描述',
    key: 'description',
    width: 200,
    ellipsis: { tooltip: { style: { maxWidth: '400px', wordBreak: 'break-all' } } }
  },
  {
    title: 'URL',
    key: 'url',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      if (!row.url) return h(NText, { depth: 3 }, { default: () => '-' })
      return h(NText, { code: true, depth: 2 }, { default: () => row.url })
    }
  },
  {
    title: '启用',
    key: 'enable',
    width: 70,
    render(row) {
      return h(NTag, { type: row.enable ? 'success' : 'error' }, {
        default: () => (row.enable ? '是' : '否')
      })
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 80,
    render(row) {
      const typeMap = {
        available: 'success',
        untested: 'default',
        unavailable: 'error'
      }
      return h(NTag, { type: typeMap[row.status] || 'default' }, {
        default: () => getStatusLabel(row.status)
      })
    }
  },
  {
    title: '测试结果',
    key: 'testResult',
    width: 200,
    ellipsis: { tooltip: { style: { maxWidth: '400px', wordBreak: 'break-all' } } },
    render(row) {
      if (!row.testResult) return h(NText, { depth: 3 }, { default: () => '-' })
      const isSuccess = row.status === 'available'
      return h(NText, { type: isSuccess ? 'success' : 'error' }, { default: () => row.testResult })
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 280,
    fixed: 'right',
    render(row) {
      return h(NSpace, {}, {
        default: () => [
          h(
            NButton,
            {
              size: 'tiny',
              type: 'info',
              onClick: () => handleTest(row)
            },
            {
              icon: () => h(NIcon, { component: FlashOutline }),
              default: () => '测试'
            }
          ),
          h(
            NButton,
            {
              size: 'tiny',
              type: row.enable ? 'warning' : 'info',
              onClick: () => handleToggleEnable(row)
            },
            {
              icon: () => h(NIcon, { component: row.enable ? PauseOutline : PlayOutline }),
              default: () => (row.enable ? '禁用' : '启用')
            }
          ),
          h(
            NButton,
            {
              size: 'tiny',
              type: 'primary',
              onClick: () => handleEdit(row)
            },
            {
              icon: () => h(NIcon, { component: CreateOutline }),
              default: () => '编辑'
            }
          ),
          h(
            NPopconfirm,
            {
              onPositiveClick: () => handleDelete(row.id)
            },
            {
              trigger: () =>
                h(
                  NButton,
                  {
                    size: 'tiny',
                    type: 'error'
                  },
                  {
                    icon: () => h(NIcon, { component: TrashOutline }),
                    default: () => '删除'
                  }
                ),
              default: () => `确定要删除服务器 "${row.name}" 吗？`
            }
          )
        ]
      })
    }
  }
]

const pagination = computed(() => ({
  page: currentPage.value,
  pageSize: pageSize.value,
  itemCount: total.value,
  pageCount: Math.ceil(total.value / pageSize.value) || 1,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: ({ itemCount }) => `共 ${itemCount} 条`,
  onChange: handlePageChange,
  onUpdatePageSize: handlePageSizeChange
}))

const loadServerList = async () => {
  loading.value = true
  try {
    const query = {
      page: currentPage.value,
      pageSize: pageSize.value,
      name: searchKeyword.value,
      status: filterStatus.value
    }

    const result = await GetMCPServerList(query)
    if (result) {
      const servers = result.data || []
      let allTools = []
      try {
        allTools = await GetAllMCPTools() || []
      } catch (error) {
        console.error('加载工具列表失败:', error)
      }
      const toolsMap = {}
      for (const t of allTools) {
        if (!toolsMap[t.mcpServerId]) toolsMap[t.mcpServerId] = []
        toolsMap[t.mcpServerId].push(t)
      }
      for (const server of servers) {
        server.tools = toolsMap[server.id] || []
      }
      serverList.value = servers
      total.value = result.total || 0
    }
  } catch (error) {
    console.error('加载服务器列表失败:', error)
    message.error('加载服务器列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  currentPage.value = 1
  await loadServerList()
}

const handlePageChange = (page) => {
  currentPage.value = page
  loadServerList()
}

const handlePageSizeChange = (size) => {
  pageSize.value = size
  currentPage.value = 1
  loadServerList()
}

const handleTest = async (row) => {
  try {
    const result = await TestMCPServer(row.id)
    message.success(result)
    await loadServerList()
  } catch (error) {
    message.error('测试失败：' + error.message)
  }
}

const handleToggleEnable = async (row) => {
  try {
    const newEnable = !row.enable
    const result = await EnableMCPServer(row.id, newEnable)
    message.success(result)
    await loadServerList()
  } catch (error) {
    message.error('操作失败：' + error.message)
  }
}

const handleCreate = () => {
  editingServer.value = false
  resetForm()
  showCreateModal.value = true
}

const handleEdit = async (row) => {
  editingServer.value = true
  try {
    const server = await GetMCPServerByID(row.id)
    if (server) {
      resetForm()
      formData.id = server.id
      formData.name = server.name
      formData.description = server.description
      formData.url = server.url
      formData.command = server.command
      formData.args = server.args
      formData.headers = server.headers
      parseHeadersToList(server.headers)
      formData.enable = server.enable
      formData.status = server.status
      showCreateModal.value = true
    }
  } catch (error) {
    message.error('获取服务器详情失败：' + error.message)
  }
}

const handleDelete = async (id) => {
  try {
    const result = await DeleteMCPServer(id)
    message.success(result)
    await loadServerList()
  } catch (error) {
    message.error('删除失败：' + error.message)
  }
}

const handleSubmit = async () => {
  try {
    if (formRef.value) {
      try {
        await formRef.value.validate()
      } catch {
        return
      }
    }

    submitting.value = true
    const submitData = { ...formData }
    submitData.headers = serializeHeaders()

    let result
    if (formData.id) {
      result = await UpdateMCPServer(submitData)
    } else {
      result = await CreateMCPServer(submitData)
    }

    if (result.includes('成功')) {
      message.success(result)
      showCreateModal.value = false
      await loadServerList()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('操作失败：' + error.message)
  } finally {
    submitting.value = false
  }
}

const resetForm = () => {
  Object.assign(formData, {
    id: null,
    name: '',
    description: '',
    url: '',
    command: '',
    args: '',
    headers: '',
    enable: true,
    status: 'stopped'
  })
  headerList.splice(0, headerList.length)
  headerList.push({ key: '', value: '' })
  if (formRef.value) {
    formRef.value.restoreValidation()
  }
}

const serverList = ref([])

onMounted(async () => {
  await loadServerList()
})
</script>

<style scoped>
</style>
