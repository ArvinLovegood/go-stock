<template>
  <n-space vertical style="margin-bottom: 12px">
    <n-space>
      <n-button type="info" :loading="importing" @click="handleImportSkill">
        <template #icon>
          <n-icon :component="CloudDownloadOutline" />
        </template>
        导入技能包
      </n-button>

      <n-button type="success" @click="showPlaza = true">
        <template #icon>
          <n-icon :component="CloudUploadOutline" />
        </template>
        技能广场
      </n-button>

      <n-button @click="loadFsSkills">
        <template #icon>
          <n-icon :component="RefreshOutline" />
        </template>
        刷新
      </n-button>
    </n-space>

    <n-spin :show="loading">
      <n-empty
        v-if="!fsSkills.length && !loading"
        description="暂无技能，点击“导入技能包”或前往“技能广场”获取"
        style="margin: 48px 0"
      />
      <div v-else class="skill-card-grid">
        <n-card
          v-for="row in fsSkills"
          :key="row.dirName"
          size="small"
          class="skill-card"
          :class="{ 'skill-card-disabled': row.disabled }"
        >
          <template #header>
            <div class="skill-card-title">
              <span class="skill-card-name" :title="row.name || row.dirName">
                {{ row.name || row.dirName || '(未命名)' }}
              </span>
              <n-tag v-if="row.disabled" size="small" type="warning" :bordered="false">
                已停用
              </n-tag>
            </div>
          </template>
          <template #header-extra>
            <n-switch
              size="small"
              :value="!row.disabled"
              :loading="row._toggling"
              @update:value="val => handleToggleSkill(row, val)"
            >
              <template #checked>启用</template>
              <template #unchecked>停用</template>
            </n-switch>
          </template>
          <div class="skill-card-meta">
            <n-tag size="small" type="info">
              <n-icon :component="FolderOpenOutline" size="13" style="margin-right: 4px; vertical-align: -2px" />
              {{ row.dirName }}
            </n-tag>
          </div>
          <div class="skill-card-desc" title="点击查看完整描述" @click="showSkillDetail(row)">
            {{ row.description || '暂无描述' }}
          </div>
          <template #action>
            <n-space size="small" justify="end">
              <n-button size="tiny" quaternary type="primary" @click="showSkillDetail(row)">
                详情
              </n-button>
              <n-button size="tiny" quaternary type="info" @click="openFileEditor(row)">
                <template #icon>
                  <n-icon :component="CreateOutline" />
                </template>
                编辑文件
              </n-button>
              <n-popconfirm @positive-click="handleDeleteFsSkill(row)">
                <template #trigger>
                  <n-button size="tiny" quaternary type="error">
                    <template #icon>
                      <n-icon :component="TrashOutline" />
                    </template>
                    删除
                  </n-button>
                </template>
                确定删除文件系统技能 "{{ row.dirName }}"？
                此操作将删除整个技能目录。
              </n-popconfirm>
            </n-space>
          </template>
        </n-card>
      </div>
    </n-spin>

    <!-- 技能详情：查看完整描述 -->
    <n-modal
      v-model:show="showDetail"
      preset="card"
      :title="'技能详情 - ' + (detailSkill.name || detailSkill.dirName || '')"
      style="width: 70%; max-width: 760px"
    >
      <n-descriptions :column="1" size="small" bordered label-placement="left">
        <n-descriptions-item label="技能名称">
          {{ detailSkill.name || '(未命名)' }}
        </n-descriptions-item>
        <n-descriptions-item label="目录">
          {{ detailSkill.dirName }}
        </n-descriptions-item>
        <n-descriptions-item label="状态">
          <n-tag v-if="detailSkill.disabled" type="warning" size="small">已停用</n-tag>
          <n-tag v-else type="success" size="small">已启用</n-tag>
        </n-descriptions-item>
      </n-descriptions>
      <n-divider style="margin: 12px 0">描述详情</n-divider>
      <!-- 查看态：完整描述 + 编辑入口 -->
      <div v-if="!detailEditing" class="skill-detail-desc">{{ detailSkill.description || '暂无描述' }}</div>
      <!-- 编辑态：textarea + 保存/取消 -->
      <template v-else>
        <n-input
          v-model:value="detailEditText"
          type="textarea"
          :autosize="{ minRows: 6, maxRows: 16 }"
          placeholder="输入技能描述。描述是 Agent 自动路由技能的核心依据，建议包含：技能定位、覆盖能力、适用场景、与相近技能的边界。"
          :disabled="detailSaving"
        />
        <n-space size="small" justify="end" style="margin-top: 8px">
          <n-button size="small" :disabled="detailSaving" @click="cancelEditDescription">取消</n-button>
          <n-button size="small" type="primary" :loading="detailSaving" @click="saveDescription">
            保存
          </n-button>
        </n-space>
      </template>
      <template #footer>
        <n-space justify="end" v-if="!detailEditing">
          <n-button size="small" type="primary" ghost @click="startEditDescription">
            编辑描述
          </n-button>
        </n-space>
      </template>
    </n-modal>
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
        <div style="padding: 8px 12px; height: 100%; min-height: 0; display: flex; flex-direction: column; overflow: hidden">
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
          <div style="flex: 1; min-height: 0; overflow: hidden">
            <MdEditor
              v-if="currentFilePath && currentFilePath.endsWith('.md')"
              v-model="currentFileContent"
              style="height: 100%"
              :theme="editorTheme"
              :preview="true"
              :toolbarsExclude="['github', 'htmlPreview', 'catalog', 'save']"
            />
            <Codemirror
              v-else-if="currentFilePath"
              v-model="currentFileContent"
              :extensions="codeExtensions"
              :style="{ height: '100%' }"
              :indent-with-tab="true"
              :tab-size="4"
              placeholder="文件内容"
            />
            <n-empty v-else description="选择一个文件开始编辑" style="margin-top: 100px" />
          </div>
        </div>
      </n-layout>
    </n-layout>
  </n-modal>
  <!-- 技能广场：浏览/下载他人分享的技能，上传分享本地技能 -->
  <n-modal
    v-model:show="showPlaza"
    preset="card"
    title="技能广场"
    style="width: 90%; max-width: 1300px; max-height: 88vh"
  >
    <n-scrollbar style="max-height: calc(88vh - 120px)">
      <SkillPlaza @imported="loadFsSkills" />
    </n-scrollbar>
  </n-modal>

</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import {
  NButton, NSpace, NInput, NModal, NTag, NIcon, NPopconfirm, NScrollbar,
  NLayout, NLayoutSider, NTree, NDivider, NText, NEmpty, NSwitch, NCard, NSpin,
  NDescriptions, NDescriptionsItem, useMessage
} from 'naive-ui'
import { AddOutline, TrashOutline, CloudDownloadOutline, CloudUploadOutline, FolderOpenOutline, SaveOutline, RefreshOutline, CreateOutline } from '@vicons/ionicons5'
import SkillPlaza from './skillPlaza.vue'
import { MdEditor } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { Codemirror } from 'vue-codemirror'
import { python } from '@codemirror/lang-python'
import { yaml } from '@codemirror/lang-yaml'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'
import {
  GetConfig, ImportSkillPackage, ListFilesystemSkills, DeleteFilesystemSkill,
  ListSkillFiles, ReadSkillFile, WriteSkillFile, DeleteSkillFile,
  EnableFilesystemSkill, DisableFilesystemSkill, UpdateFilesystemSkillDescription
} from '../../wailsjs/go/main/App.js'

const message = useMessage()
const loading = ref(false)
const importing = ref(false)
const fsSkills = ref([])
const editorTheme = ref('light')
const showPlaza = ref(false)

// ==================== 文件系统技能列表（卡片展示） ====================
// 技能详情：查看完整描述（卡片内描述截断为最多 3 行，点击打开详情弹窗）
const showDetail = ref(false)
const detailSkill = ref({})

// 描述编辑态（在详情弹窗内切换查看/编辑）
const detailEditing = ref(false)
const detailEditText = ref('')
const detailSaving = ref(false)

function showSkillDetail(row) {
  detailSkill.value = row
  detailEditing.value = false
  showDetail.value = true
}

function startEditDescription() {
  detailEditText.value = detailSkill.value.description || ''
  detailEditing.value = true
}

function cancelEditDescription() {
  detailEditing.value = false
  detailEditText.value = ''
}

// 保存描述：后端仅改写 frontmatter 的 description 字段，其余内容不变。
// 描述是 Agent 自动路由技能的核心依据，下次对话即生效。
const saveDescription = async () => {
  if (detailSaving.value) return
  detailSaving.value = true
  try {
    const result = await UpdateFilesystemSkillDescription(detailSkill.value.dirName, detailEditText.value)
    if (result === '描述已更新') {
      message.success('描述已更新，下次对话生效')
      detailSkill.value.description = detailEditText.value
      detailEditing.value = false
      loadFsSkills()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('保存失败: ' + error)
  } finally {
    detailSaving.value = false
  }
}

// 停用/启用切换：后端通过重命名 SKILL.md ↔ SKILL.md.disabled 实现，
// Agent 每次对话重建并重新扫描技能目录，因此下一次对话即实时生效。
const handleToggleSkill = async (row, enable) => {
  if (row._toggling) return
  row._toggling = true
  try {
    const result = enable
      ? await EnableFilesystemSkill(row.dirName)
      : await DisableFilesystemSkill(row.dirName)
    if (result && (result.includes('已启用') || result.includes('已停用'))) {
      message.success(result + '，下次对话生效')
      loadFsSkills()
    } else {
      message.warning(result)
      loadFsSkills()
    }
  } catch (error) {
    message.error('操作失败: ' + error)
    loadFsSkills()
  }
}

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

// ==================== 代码编辑器语言映射 ====================
// 根据文件扩展名返回对应的 CodeMirror 语言扩展。
// 未匹配的扩展名返回空数组（仍可用 CodeMirror 编辑，仅无语法高亮）。
function getLanguageExtension(filePath) {
  if (!filePath) return []
  const parts = filePath.split('.')
  const ext = parts.length > 1 ? parts.pop().toLowerCase() : ''
  switch (ext) {
    case 'py':
    case 'pyw':
      return [python()]
    case 'yaml':
    case 'yml':
      return [yaml()]
    case 'json':
      return [json()]
    default:
      return []
  }
}

// CodeMirror 扩展集合：语言高亮 + 暗色主题（与 MdEditor 主题同步）。
const codeExtensions = computed(() => {
  const exts = getLanguageExtension(currentFilePath.value)
  if (editorTheme.value === 'dark') {
    exts.push(oneDark)
  }
  return exts
})

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
/* 技能卡片网格：自适应列数，窄屏单列、宽屏多列 */
.skill-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

/* 停用技能整体降透明度 */
.skill-card-disabled {
  opacity: 0.55;
}

.skill-card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

/* 卡片头技能名：单行截断，避免长名称撑破布局 */
.skill-card-name {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 180px;
}

.skill-card-meta {
  margin-bottom: 8px;
}

/* 卡片内描述：最多 3 行截断，点击打开详情弹窗 */
.skill-card-desc {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 58px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--n-text-color, rgba(0, 0, 0, 0.65));
  cursor: pointer;
  word-break: break-all;
}

/* 详情弹窗完整描述：保留换行 */
.skill-detail-desc {
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 13px;
  line-height: 1.7;
  max-height: 50vh;
  overflow: auto;
}

:deep(.md-editor) {
  text-align: left;
}

/* CodeMirror 编辑器内容强制左对齐。
   .cm-content 是 inline-block 元素，会继承父级 text-align，
   若父布局存在居中样式会导致代码内容偏移，故显式覆盖。 */
:deep(.cm-editor),
:deep(.cm-content),
:deep(.cm-line) {
  text-align: left;
}

/* 隐藏 n-layout 内置滚动容器的外层滚动条。
   n-layout-scroll-container 默认 overflow: auto，当子内容超出时会触发外层滚动；
   编辑器内部（CodeMirror .cm-scroller / MdEditor）已有自己的滚动，
   外层只需截断，避免出现双滚动条。 */
:deep(.n-layout-scroll-container) {
  overflow: hidden;
}
</style>
