<template>
  <n-space vertical style="margin-bottom: 12px">
    <n-space>
      <n-button type="info" :loading="importing" @click="handleImportSkill">
        <template #icon>
          <n-icon :component="CloudDownloadOutline" />
        </template>
        导入技能包
      </n-button>

      <n-button @click="loadFsSkills">
        <template #icon>
          <n-icon :component="RefreshOutline" />
        </template>
        刷新
      </n-button>
    </n-space>

    <n-data-table
      :columns="fsColumns"
      :data="fsSkills"
      :row-key="row => row.dirName"
      :loading="loading"
      size="small"
    />
  </n-space>

  <n-modal
    v-model:show="showFileEditor"
    preset="card"
    :title="'编辑技能文件 - ' + editingSkillDir"
    style="width: 90%; max-width: 1200px; max-height: 85vh"
    :mask-closable="false"
  >
    <n-layout has-sider style="height: calc(85vh - 120px)">
      <n-layout-sider
        bordered
        :width="220"
        :collapsed-width="0"
        show-trigger="arrow-circle"
        collapse-mode="width"
      >
        <div style="padding: 8px">
          <n-button
            block
            type="primary"
            size="small"
            :loading="creatingFile"
            @click="showNewFileInput = !showNewFileInput"
          >
            <template #icon>
              <n-icon :component="AddOutline" />
            </template>
            新建文件
          </n-button>
          <n-input
            v-if="showNewFileInput"
            v-model:value="newFileName"
            size="small"
            placeholder="如 scripts/run.sh 或 config.yaml"
            style="margin-top: 6px"
            @keyup.enter="handleCreateFile"
          >
            <template #suffix>
              <n-button text type="primary" size="tiny" @click="handleCreateFile">确定</n-button>
            </template>
          </n-input>
        </div>
        <n-divider style="margin: 4px 0" />
        <n-scrollbar style="max-height: calc(85vh - 200px)">
          <n-tree
            :data="fileTreeData"
            key-field="path"
            label-field="name"
            :selectable="true"
            :default-expand-all="true"
            @update:selected-keys="handleSelectFile"
          />
        </n-scrollbar>
      </n-layout-sider>
      <n-layout>
        <div style="padding: 8px 12px; height: 100%; display: flex; flex-direction: column">
          <n-space justify="space-between" align="center" style="margin-bottom: 6px">
            <n-tag v-if="currentFilePath" size="small" type="info">
              {{ currentFilePath }}
            </n-tag>
            <n-text v-else depth="3">请从左侧选择文件</n-text>
            <n-space>
              <n-button
                v-if="currentFilePath"
                size="small"
                type="warning"
                quaternary
                :loading="savingFile"
                @click="handleSaveFile"
              >
                <template #icon>
                  <n-icon :component="SaveOutline" />
                </template>
                保存
              </n-button>
              <n-popconfirm
                v-if="currentFilePath && currentFilePath !== 'SKILL.md'"
                @positive-click="handleDeleteFile"
              >
                <template #trigger>
                  <n-button size="small" type="error" quaternary>
                    <template #icon>
                      <n-icon :component="TrashOutline" />
                    </template>
                    删除文件
                  </n-button>
                </template>
                确定删除 "{{ currentFilePath }}"？
              </n-popconfirm>
            </n-space>
          </n-space>
          <div style="flex: 1; overflow: hidden">
            <MdEditor
              v-if="currentFilePath && currentFilePath.endsWith('.md')"
              v-model="currentFileContent"
              style="height: 100%"
              :theme="editorTheme"
              :preview="true"
              :toolbarsExclude="['github', 'htmlPreview', 'catalog', 'save']"
            />
            <n-input
              v-else-if="currentFilePath"
              v-model:value="currentFileContent"
              type="textarea"
              style="height: 100%"
              :autosize="false"
              placeholder="文件内容"
              spellcheck="false"
            />
            <n-empty v-else description="选择一个文件开始编辑" style="margin-top: 100px" />
          </div>
        </div>
      </n-layout>
    </n-layout>
  </n-modal>

</template>

<script setup>
import { ref, h, onMounted, computed } from 'vue'
import {
  NButton, NSpace, NInput, NDataTable, NModal, NTag, NIcon, NPopconfirm, NScrollbar,
  NLayout, NLayoutSider, NTree, NDivider, NText, NEmpty, useMessage
} from 'naive-ui'
import { AddOutline, TrashOutline, CloudDownloadOutline, FolderOpenOutline, SaveOutline, RefreshOutline, CreateOutline } from '@vicons/ionicons5'
import { MdEditor } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import {
  GetConfig, ImportSkillPackage, ListFilesystemSkills, DeleteFilesystemSkill,
  ListSkillFiles, ReadSkillFile, WriteSkillFile, DeleteSkillFile
} from '../../wailsjs/go/main/App.js'

const message = useMessage()
const loading = ref(false)
const importing = ref(false)
const fsSkills = ref([])
const editorTheme = ref('light')

// ==================== 文件系统技能列表 ====================
const fsColumns = [
  {
    title: '技能名称',
    key: 'name',
    width: 160,
    render(row) {
      return row.name || row.dirName || '(未命名)'
    }
  },
  {
    title: '目录',
    key: 'dirName',
    width: 160,
    render(row) {
      return h(NTag, { type: 'info', size: 'small' }, {
        default: () => h('span', null, [
          h(NIcon, { component: FolderOpenOutline, size: 14, style: 'margin-right:4px;vertical-align:-2px' }),
          row.dirName
        ])
      })
    }
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: { style: { maxWidth: '400px', wordBreak: 'break-all' } } }
  },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    render(row) {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(NButton, {
            size: 'small', type: 'info', quaternary: true,
            onClick: () => openFileEditor(row)
          }, {
            icon: () => h(NIcon, null, { default: () => h(CreateOutline) }),
            default: () => '编辑文件'
          }),
          h(NPopconfirm, {
            onPositiveClick: () => handleDeleteFsSkill(row)
          }, {
            trigger: () => h(NButton, {
              size: 'small', type: 'error', quaternary: true
            }, {
              icon: () => h(NIcon, null, { default: () => h(TrashOutline) }),
              default: () => '删除'
            }),
            default: () => `确定删除文件系统技能 "${row.dirName}"？\n此操作将删除整个技能目录。`
          })
        ]
      })
    }
  }
]

const loadFsSkills = async () => {
  loading.value = true
  try {
    const result = await ListFilesystemSkills()
    fsSkills.value = result || []
  } catch (error) {
    message.error('加载技能列表失败: ' + error)
  } finally {
    loading.value = false
  }
}

const handleImportSkill = async () => {
  importing.value = true
  try {
    const result = await ImportSkillPackage()
    if (result && result.includes('成功')) {
      message.success(result)
      loadFsSkills()
    } else if (result && result !== '未选择文件') {
      message.warning(result)
    }
  } catch (error) {
    message.error('导入失败: ' + error)
  } finally {
    importing.value = false
  }
}

const handleDeleteFsSkill = async (row) => {
  try {
    const result = await DeleteFilesystemSkill(row.dirName)
    if (result && result.includes('已删除')) {
      message.success(result)
      loadFsSkills()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('删除失败: ' + error)
  }
}

// ==================== 文件编辑器 ====================
const showFileEditor = ref(false)
const editingSkillDir = ref('')
const skillFiles = ref([])
const currentFilePath = ref('')
const currentFileContent = ref('')
const savingFile = ref(false)
const showNewFileInput = ref(false)
const newFileName = ref('')
const creatingFile = ref(false)

// 将扁平文件列表转为树形结构
const fileTreeData = computed(() => buildFileTree(skillFiles.value))

function buildFileTree(files) {
  const root = []
  const dirMap = new Map()
  const sorted = [...files].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    return a.name.localeCompare(b.name)
  })

  for (const f of sorted) {
    if (f.isDir) continue
    const parts = f.path.split('/')
    let currentLevel = root
    let currentPath = ''

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      currentPath = currentPath ? currentPath + '/' + part : part
      const isLast = i === parts.length - 1

      if (isLast) {
        currentLevel.push({ name: part, path: f.path, isLeaf: true })
      } else {
        let dirNode = dirMap.get(currentPath)
        if (!dirNode) {
          dirNode = { name: part, path: currentPath, children: [] }
          dirMap.set(currentPath, dirNode)
          currentLevel.push(dirNode)
        }
        currentLevel = dirNode.children
      }
    }
  }
  return root
}

const openFileEditor = async (row) => {
  editingSkillDir.value = row.dirName
  currentFilePath.value = ''
  currentFileContent.value = ''
  showFileEditor.value = true
  await loadSkillFiles(row.dirName)
  // 默认选中 SKILL.md
  if (skillFiles.value.some(f => f.path === 'SKILL.md')) {
    await loadFileContent('SKILL.md')
  }
}

const loadSkillFiles = async (dirName) => {
  try {
    const result = await ListSkillFiles(dirName)
    skillFiles.value = result || []
  } catch (error) {
    message.error('加载文件列表失败: ' + error)
    skillFiles.value = []
  }
}

const loadFileContent = async (filePath) => {
  try {
    const content = await ReadSkillFile(editingSkillDir.value, filePath)
    currentFilePath.value = filePath
    currentFileContent.value = content
  } catch (error) {
    message.error('读取文件失败: ' + error)
  }
}

const handleSelectFile = (keys) => {
  if (keys && keys.length > 0) {
    const selectedPath = keys[0]
    const file = skillFiles.value.find(f => f.path === selectedPath && !f.isDir)
    if (file) {
      loadFileContent(selectedPath)
    }
  }
}

const handleSaveFile = async () => {
  if (!currentFilePath.value) return
  savingFile.value = true
  try {
    const result = await WriteSkillFile(editingSkillDir.value, currentFilePath.value, currentFileContent.value)
    if (result === '保存成功') {
      message.success(result)
      loadFsSkills()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('保存失败: ' + error)
  } finally {
    savingFile.value = false
  }
}

const handleDeleteFile = async () => {
  if (!currentFilePath.value) return
  try {
    const result = await DeleteSkillFile(editingSkillDir.value, currentFilePath.value)
    if (result === '删除成功') {
      message.success(result)
      currentFilePath.value = ''
      currentFileContent.value = ''
      await loadSkillFiles(editingSkillDir.value)
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('删除失败: ' + error)
  }
}

const handleCreateFile = async () => {
  if (!newFileName.value) {
    showNewFileInput.value = false
    return
  }
  creatingFile.value = true
  try {
    const result = await WriteSkillFile(editingSkillDir.value, newFileName.value, '')
    if (result === '保存成功') {
      message.success('文件已创建')
      await loadSkillFiles(editingSkillDir.value)
      await loadFileContent(newFileName.value)
      newFileName.value = ''
      showNewFileInput.value = false
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('创建文件失败: ' + error)
  } finally {
    creatingFile.value = false
  }
}

onMounted(() => {
  loadFsSkills()
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorTheme.value = 'dark'
    }
  })
})
</script>

<style scoped>
:deep(.md-editor) {
  text-align: left;
}
</style>
