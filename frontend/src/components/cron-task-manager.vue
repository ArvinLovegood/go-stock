<template>
      <!-- 搜索和筛选区域 -->
   <n-space vertical style="margin-bottom: 12px">
      <n-space>
    <n-input
          v-model:value="searchKeyword"
          placeholder="搜索任务名称..."
          style="width: 200px"
          clearable
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <n-icon :component="SearchOutline" />
          </template>
        </n-input>
        
        <n-select
          v-model:value="filterTaskType"
          :options="taskTypeOptions"
          placeholder="任务类型"
          style="width: 140px"
          clearable
        />
        
        <n-select
          v-model:value="filterStatus"
          :options="statusOptions"
          placeholder="任务状态"
          style="width: 120px"
          clearable
        />
        
        <n-button type="primary"  @click="handleSearch">
          搜索
        </n-button>
        
        <n-button type="warning"  @click="handleCreate">
          <template #icon>
            <n-icon :component="AddOutline" />
          </template>
          新建任务
        </n-button>
 </n-space>
 </n-space> 

      <!-- 任务列表表格 -->
      <n-data-table
          remote
          size="small"
          :columns="columns"
          :data="taskList"
          :loading="loading"
          :pagination="pagination"
          :row-key="(rowData)=>rowData.id"
          flex-height
          style="height: calc(100vh - 210px);margin-top: 10px"
      />

    <!-- 创建/编辑任务弹窗 -->
    <n-modal
      v-model:show="showCreateModal"
      :title="editingTask ? '修改任务' : '创建新任务'"
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
        <n-form-item label="任务名称" path="name">
          <n-input v-model:value="formData.name" placeholder="请输入任务名称" clearable />
        </n-form-item>

        <n-form-item label="任务类型" path="taskType">
          <n-select
            v-model:value="formData.taskType"
            :options="taskTypeOptions"
            placeholder="请选择任务类型"
          />
        </n-form-item>

        <n-form-item label="Cron 表达式" path="cronExpr">
          <n-space :vertical="true" :size="8" style="width: 100%">
            <n-input
              v-model:value="formData.cronExpr"
              placeholder="通过下方选择器生成或直接输入"
              clearable
            >
              <template #suffix>
                <n-button size="small" @click="showCronBuilder = true">
                  <template #icon>
                    <n-icon :component="SettingsOutline" />
                  </template>
                  配置
                </n-button>
              </template>
            </n-input>
            <n-space :vertical="true" :size="4" style="width: 100%">
              <n-text depth="3" style="font-size: 12px">
                <n-icon :component="InformationCircleOutline" size="14" />
                点击"配置"按钮打开可视化配置器 | 当前值：{{ formData.cronExpr || '未设置' }}
              </n-text>
              <n-text v-if="calculateNextRunTime" depth="2" style="font-size: 12px; color: #18a058">
                <n-icon :component="TimeOutline" size="14" />
                下次执行时间：{{ calculateNextRunTime }}
              </n-text>
            </n-space>
          </n-space>
        </n-form-item>

<!--        <n-form-item label="目标任务" path="target">-->
<!--          <n-input-->
<!--            v-model:value="formData.target"-->
<!--            placeholder="股票代码或多个代码（逗号分隔），如：600519,000001"-->
<!--            clearable-->
<!--          />-->
<!--        </n-form-item>-->

        <n-form-item :label="'任务参数'" path="params">
          <!-- 股票分析任务的参数配置 UI -->
          <n-card v-if="formData.taskType === 'stock_analysis'" size="small" style="width: 100%">
            <n-space :vertical="true" :size="12">
              <!-- 第一行：提示词模板和 AI 配置 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="提示词模板:">
                    <n-select
                      v-model:value="stockAnalysisParamsData.promptId"
                      :options="promptTemplateOptions"
                      placeholder="请选择提示词模板"
                      filterable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item label-width="90px" label="AI 配置:">
                    <n-select
                      v-model:value="stockAnalysisParamsData.aiConfigId"
                      :options="aiConfigOptions"
                      placeholder="请选择 AI 配置"
                      filterable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>
              
              <!-- 第二行：系统提示词和启用思考 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="系统提示词:">
                    <n-select
                      v-model:value="stockAnalysisParamsData.sysPromptId"
                      :options="sysPromptOptions"
                      placeholder="请选择系统提示词（可选）"
                      filterable
                      clearable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item label-width="90px" label="启用思考:">
                    <n-switch v-model:value="stockAnalysisParamsData.thinking" size="large">
                      <template #checked>
                        开启
                      </template>
                      <template #unchecked>
                        关闭
                      </template>
                    </n-switch>
                  </n-form-item>
                </n-gi>
              </n-grid>

              <!-- 第三行：Agent模式和股票代码 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="Agent模式:">
                    <n-select
                      v-model:value="stockAnalysisParamsData.agentMode"
                      :options="agentModeOptions"
                      placeholder="请选择Agent模式"
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item label-width="90px" label="股票代码:">
                    <n-input
                      v-model:value="stockAnalysisParamsData.stockCode"
                      placeholder="请输入股票代码，如：600519"
                      clearable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>

              <!-- 第四行：股票名称 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="股票名称:">
                    <n-input
                      v-model:value="stockAnalysisParamsData.stockName"
                      placeholder="请输入股票名称，如：贵州茅台"
                      clearable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>
            </n-space>
          </n-card>
          
          <!-- 市场分析任务的参数配置 UI -->
          <n-card v-else-if="formData.taskType === 'market_analysis'" size="small" style="width: 100%">
            <n-space :vertical="true" :size="12">
              <!-- 第一行：提示词模板和 AI 配置 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="提示词模板:">
                    <n-select
                      v-model:value="marketAnalysisParamsData.promptId"
                      :options="promptTemplateOptions"
                      placeholder="请选择提示词模板"
                      filterable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item label-width="90px" label="AI 配置:">
                    <n-select
                      v-model:value="marketAnalysisParamsData.aiConfigId"
                      :options="aiConfigOptions"
                      placeholder="请选择 AI 配置"
                      filterable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>
              
              <!-- 第二行：系统提示词和启用思考 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="系统提示词:">
                    <n-select
                      v-model:value="marketAnalysisParamsData.sysPromptId"
                      :options="sysPromptOptions"
                      placeholder="请选择系统提示词（可选）"
                      filterable
                      clearable
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
                <n-gi>
                  <n-form-item label-width="90px" label="启用思考:">
                    <n-switch v-model:value="marketAnalysisParamsData.thinking" size="large">
                      <template #checked>
                        开启
                      </template>
                      <template #unchecked>
                        关闭
                      </template>
                    </n-switch>
                  </n-form-item>
                </n-gi>
              </n-grid>

              <!-- 第三行：Agent模式 -->
              <n-grid :cols="2" :x-gap="12">
                <n-gi>
                  <n-form-item label-width="90px" label="Agent模式:">
                    <n-select
                      v-model:value="marketAnalysisParamsData.agentMode"
                      :options="agentModeOptions"
                      placeholder="请选择Agent模式"
                      style="width: 100%"
                    />
                  </n-form-item>
                </n-gi>
              </n-grid>
            </n-space>
          </n-card>
          
          <!-- 其他任务类型仍使用文本输入框 -->
          <n-input
            v-else
            v-model:value="formData.params"
            type="textarea"
            :rows="5"
            placeholder='JSON 格式，如：{"stock_codes":["600519"],"ai_config_id":1}'
            show-count
          />
        </n-form-item>

        <n-form-item label="任务描述" path="description">
          <n-input
            v-model:value="formData.description"
            type="textarea"
            :rows="3"
            placeholder="请输入任务描述（可选）"
            show-count
            maxlength="500"
          />
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
          {{ editingTask ? '修改任务' : '创建新任务' }}
        </n-button>
      </template>
    </n-modal>

    <!-- Cron 表达式配置器 -->
    <n-modal
      v-model:show="showCronBuilder"
      title="Cron 表达式配置器"
      preset="dialog"
      :style="{ width: '850px' }"
      :z-index="11000"
      to="body"
    >
      <n-card size="small">
        <n-space :vertical="true" :size="12">
          <!-- 秒 -->
          <div class="cron-row">
            <span class="cron-label">秒:</span>
            <n-radio-group v-model:value="cronSecond.type" name="secondType">
              <n-space :size="8">
                <n-radio :value="'*'">每秒</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-input-number v-model:value="cronSecond.start" :min="0" :max="59" :disabled="cronSecond.type !== 'interval'" style="width: 80px" />-
                <n-input-number v-model:value="cronSecond.end" :min="0" :max="59" :disabled="cronSecond.type !== 'interval'" style="width: 80px" />
                <n-radio :value="'loop'">循环</n-radio>
                <n-input-number v-model:value="cronSecond.loopStart" :min="0" :max="59" :disabled="cronSecond.type !== 'loop'" style="width: 80px" />/
                <n-input-number v-model:value="cronSecond.loopStep" :min="1" :max="59" :disabled="cronSecond.type !== 'loop'" style="width: 80px" />
                <n-radio :value="'appoint'">指定</n-radio>
                <n-select v-model:value="cronSecond.appoint" multiple :options="secondOptions" :disabled="cronSecond.type !== 'appoint'" style="width: 400px" placeholder="选择具体的秒" />
              </n-space>
            </n-radio-group>
          </div>

          <!-- 分 -->
          <div class="cron-row">
            <span class="cron-label">分:</span>
            <n-radio-group v-model:value="cronMinute.type" name="minuteType">
              <n-space :size="8">
                <n-radio :value="'*'">每分</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-input-number v-model:value="cronMinute.start" :min="0" :max="59" :disabled="cronMinute.type !== 'interval'" style="width: 80px" />-
                <n-input-number v-model:value="cronMinute.end" :min="0" :max="59" :disabled="cronMinute.type !== 'interval'" style="width: 80px" />
                <n-radio :value="'loop'">循环</n-radio>
                <n-input-number v-model:value="cronMinute.loopStart" :min="0" :max="59" :disabled="cronMinute.type !== 'loop'" style="width: 80px" />/
                <n-input-number v-model:value="cronMinute.loopStep" :min="1" :max="59" :disabled="cronMinute.type !== 'loop'" style="width: 80px" />
                <n-radio :value="'appoint'">指定</n-radio>
                <n-select v-model:value="cronMinute.appoint" multiple :options="minuteOptions" :disabled="cronMinute.type !== 'appoint'" style="width: 400px" placeholder="选择具体的分" />
              </n-space>
            </n-radio-group>
          </div>

          <!-- 时 -->
          <div class="cron-row">
            <span class="cron-label">时:</span>
            <n-radio-group v-model:value="cronHour.type" name="hourType">
              <n-space :size="8">
                <n-radio :value="'*'">每小时</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-input-number v-model:value="cronHour.start" :min="0" :max="23" :disabled="cronHour.type !== 'interval'" style="width: 80px" />-
                <n-input-number v-model:value="cronHour.end" :min="0" :max="23" :disabled="cronHour.type !== 'interval'" style="width: 80px" />
                <n-radio :value="'loop'">循环</n-radio>
                <n-input-number v-model:value="cronHour.loopStart" :min="0" :max="23" :disabled="cronHour.type !== 'loop'" style="width: 80px" />/
                <n-input-number v-model:value="cronHour.loopStep" :min="1" :max="23" :disabled="cronHour.type !== 'loop'" style="width: 80px" />
                <n-radio :value="'appoint'">指定</n-radio>
                <n-select v-model:value="cronHour.appoint" multiple :options="hourOptions" :disabled="cronHour.type !== 'appoint'" style="width: 400px" placeholder="选择具体的时" />
              </n-space>
            </n-radio-group>
          </div>

          <!-- 日 -->
          <div class="cron-row">
            <span class="cron-label">日:</span>
            <n-radio-group v-model:value="cronDay.type" name="dayType">
              <n-space :size="8">
                <n-radio :value="'*'">每日</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-input-number v-model:value="cronDay.start" :min="1" :max="31" :disabled="cronDay.type !== 'interval'" style="width: 80px" />-
                <n-input-number v-model:value="cronDay.end" :min="1" :max="31" :disabled="cronDay.type !== 'interval'" style="width: 80px" />
                <n-radio :value="'?'">不指定</n-radio>
              </n-space>
            </n-radio-group>
          </div>

          <!-- 月 -->
          <div class="cron-row">
            <span class="cron-label">月:</span>
            <n-radio-group v-model:value="cronMonth.type" name="monthType">
              <n-space :size="8">
                <n-radio :value="'*'">每月</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-input-number v-model:value="cronMonth.start" :min="1" :max="12" :disabled="cronMonth.type !== 'interval'" style="width: 80px" />-
                <n-input-number v-model:value="cronMonth.end" :min="1" :max="12" :disabled="cronMonth.type !== 'interval'" style="width: 80px" />
              </n-space>
            </n-radio-group>
          </div>

          <!-- 周 -->
          <div class="cron-row">
            <span class="cron-label">周:</span>
            <n-radio-group v-model:value="cronWeek.type" name="weekType">
              <n-space :size="8">
                <n-radio :value="'*'">每周</n-radio>
                <n-radio :value="'interval'">周期</n-radio>
                <n-select v-model:value="cronWeek.days" multiple :options="weekOptions" :disabled="cronWeek.type !== 'interval'" style="width: 250px" />
                <n-radio :value="'?'">不指定</n-radio>
              </n-space>
            </n-radio-group>
          </div>
        </n-space>
      </n-card>

      <!-- 预览结果 -->
      <n-alert type="info" title="生成的 Cron 表达式" style="margin-top: 12px;">
        <n-space :vertical="true" :size="8">
          <n-space align="center">
            <n-text strong style="font-size: 14px; font-family: monospace;">{{ generatedCronExpr }}</n-text>
            <n-button size="small" @click="copyCronExpr">
              <template #icon>
                <n-icon :component="CreateOutline" />
              </template>
              复制
            </n-button>
          </n-space>
          <n-space :vertical="true" :size="4">
            <n-text strong style="font-size: 14px;">未来 5 次执行时间：</n-text>
            <n-text v-if="!nextRunTimes.length" depth="3" style="font-size: 12px;">
              暂无可用时间，请检查 Cron 表达式是否有效。
            </n-text>
            <n-text
              v-for="(time, index) in nextRunTimes"
              :key="index"
              strong
              style="font-size: 13px; font-family: monospace;"
            >
              {{ index + 1 }}. {{ time }}
            </n-text>
          </n-space>
        </n-space>
      </n-alert>

      <template #action>
        <n-button @click="showCronBuilder = false">取消</n-button>
        <n-button type="primary" @click="saveCronExpr">
          <template #icon>
            <n-icon :component="CheckmarkCircleOutline" />
          </template>
          确定
        </n-button>
      </template>
    </n-modal>
</template>

<script setup>
import { ref, reactive, onMounted, computed, h, watch } from 'vue'
import { NButton, NIcon, NTag, NSpace, NPopconfirm, useMessage, NText, NCard, NRadioGroup, NRadio, NInputNumber, NSelect, NAlert, NCode, NSwitch } from 'naive-ui'
import {
  SearchOutline,
  AddOutline,
  PlayOutline,
  PauseOutline,
  TrashOutline,
  CreateOutline,
  SettingsOutline,
  InformationCircleOutline,
  PlayCircleOutline,
  StopCircleOutline,
  CheckmarkCircleOutline,
  FlashOutline,
  TimeOutline
} from '@vicons/ionicons5'
import {
  CreateCronTask,
  UpdateCronTask,
  DeleteCronTask,
  GetCronTaskByID,
  GetCronTaskList,
  EnableCronTask,
  ExecuteCronTaskNow,
  GetCronTaskTypes,
  ValidateCronExpr,
  SearchCronTasks,
  GetAiConfigs,
  CalculateNextRunTime,
  CalculateNextRunTimes,
  GetPromptTemplates
} from '../../wailsjs/go/main/App'

const message = useMessage()

// 表单引用
const formRef = ref(null)

// 响应式数据
const loading = ref(false)
const submitting = ref(false)
const showCreateModal = ref(false)
const editingTask = ref(false)
const searchKeyword = ref('')
const filterTaskType = ref('')
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const runningCount = ref(0)
const pausedCount = ref(0)

// 表单数据
const formData = reactive({
  id: null,
  name: '',
  cronExpr: '',
  taskType: 'market_analysis',
  target: '',
  params: '',
  enable: true,
  status: 'active',
  description: ''
})

// 表单验证规则
const formRules = {
  name: { required: true, message: '请输入任务名称', trigger: ['input', 'blur'] },
  cronExpr: { required: true, message: '请输入 Cron 表达式', trigger: ['input', 'blur'] },
  taskType: { required: true, message: '请选择任务类型', trigger: [ 'input', 'blur'] }
}

// 选项数据
const taskTypeOptions = ref([])
const statusOptions = [
  { label: '活跃', value: 'active' },
  { label: '暂停', value: 'paused' },
  { label: '错误', value: 'error' }
]

// 生成参数 JSON 预览
const generatedParamsJson = computed(() => {

  if(formData.taskType==='stock_analysis'){
    return JSON.stringify({
      promptId: stockAnalysisParamsData.promptId ,
      aiConfigId: stockAnalysisParamsData.aiConfigId,
      sysPromptId: stockAnalysisParamsData.sysPromptId ,
      thinking: stockAnalysisParamsData.thinking ,
      stockCode: stockAnalysisParamsData.stockCode,
      stockName: stockAnalysisParamsData.stockName,
      agentMode: stockAnalysisParamsData.agentMode
    }, null, 2)
  }
  if(formData.taskType==='market_analysis'){
    return JSON.stringify({
      promptId: marketAnalysisParamsData.promptId ,
      aiConfigId: marketAnalysisParamsData.aiConfigId,
      sysPromptId: marketAnalysisParamsData.sysPromptId ,
      thinking: marketAnalysisParamsData.thinking,
      agentMode: marketAnalysisParamsData.agentMode
    }, null, 2)
  }

})

// 生成 Cron 表达式
const generateCronExpression = () => {
  // 解析秒
  let second = '*'
  if (cronSecond.type === 'interval') {
    second = `${cronSecond.start}-${cronSecond.end}`
  } else if (cronSecond.type === 'loop') {
    second = `${cronSecond.loopStart}/${cronSecond.loopStep}`
  } else if (cronSecond.type === 'appoint' && cronSecond.appoint.length > 0) {
    second = cronSecond.appoint.join(',')
  }
  
  // 解析分
  let minute = '*'
  if (cronMinute.type === 'interval') {
    minute = `${cronMinute.start}-${cronMinute.end}`
  } else if (cronMinute.type === 'loop') {
    minute = `${cronMinute.loopStart}/${cronMinute.loopStep}`
  } else if (cronMinute.type === 'appoint' && cronMinute.appoint.length > 0) {
    minute = cronMinute.appoint.join(',')
  }
  
  // 解析时
  let hour = '*'
  if (cronHour.type === 'interval') {
    hour = `${cronHour.start}-${cronHour.end}`
  } else if (cronHour.type === 'loop') {
    hour = `${cronHour.loopStart}/${cronHour.loopStep}`
  } else if (cronHour.type === 'appoint' && cronHour.appoint.length > 0) {
    hour = cronHour.appoint.join(',')
  }
  
  // 解析日
  let day = '*'
  if (cronDay.type === 'interval') {
    day = `${cronDay.start}-${cronDay.end}`
  } else if (cronDay.type === '?') {
    day = '?'
  }
  
  // 解析月
  let month = '*'
  if (cronMonth.type === 'interval') {
    month = `${cronMonth.start}-${cronMonth.end}`
  }
  
  // 解析周
  let week = '*'
  if (cronWeek.type === 'interval' && cronWeek.days.length > 0) {
    week = cronWeek.days.join(',')
  } else if (cronWeek.type === '?') {
    week = '?'
  }
  
  return `${second} ${minute} ${hour} ${day} ${month} ${week}`
}

// 保存 Cron 表达式
const saveCronExpr = () => {
  formData.cronExpr = generatedCronExpr.value
  showCronBuilder.value = false
  message.success('Cron 表达式已保存')
}

// 解析 Cron 表达式并回填到配置器（支持 6 段：秒 分 时 日 月 周；兼容 5 段时自动补秒为 0）
const parseCronExpression = (cronExpr) => {
  if (!cronExpr || typeof cronExpr !== 'string') return
  const raw = cronExpr.trim().replace(/\s+/g, ' ').split(' ')
  if (raw.length < 5) return
  // 5 段视为 分 时 日 月 周，前面补秒 0
  const parts = raw.length === 5 ? ['0', ...raw] : raw.slice(0, 6)
  const [second, minute, hour, day, month, week] = parts

  const applySecond = (val) => {
    if (val === '*') {
      cronSecond.type = '*'
    } else if (val.includes('-')) {
      const [start, end] = val.split('-').map(Number)
      cronSecond.type = 'interval'
      cronSecond.start = start
      cronSecond.end = end
    } else if (val.includes('/')) {
      const [start, step] = val.split('/').map(Number)
      cronSecond.type = 'loop'
      cronSecond.loopStart = start
      cronSecond.loopStep = step
    } else if (val.includes(',')) {
      cronSecond.type = 'appoint'
      cronSecond.appoint = val.split(',').map(s => String(s).trim()).filter(Boolean)
    } else {
      cronSecond.type = 'appoint'
      cronSecond.appoint = [String(val)]
    }
  }
  const applyMinute = (val) => {
    if (val === '*') cronMinute.type = '*'
    else if (val.includes('-')) {
      const [start, end] = val.split('-').map(Number)
      cronMinute.type = 'interval'
      cronMinute.start = start
      cronMinute.end = end
    } else if (val.includes('/')) {
      const [start, step] = val.split('/').map(Number)
      cronMinute.type = 'loop'
      cronMinute.loopStart = start
      cronMinute.loopStep = step
    } else if (val.includes(',')) {
      cronMinute.type = 'appoint'
      cronMinute.appoint = val.split(',').map(s => String(s).trim()).filter(Boolean)
    } else {
      cronMinute.type = 'appoint'
      cronMinute.appoint = [String(val)]
    }
  }
  const applyHour = (val) => {
    if (val === '*') cronHour.type = '*'
    else if (val.includes('-')) {
      const [start, end] = val.split('-').map(Number)
      cronHour.type = 'interval'
      cronHour.start = start
      cronHour.end = end
    } else if (val.includes('/')) {
      const [start, step] = val.split('/').map(Number)
      cronHour.type = 'loop'
      cronHour.loopStart = start
      cronHour.loopStep = step
    } else if (val.includes(',')) {
      cronHour.type = 'appoint'
      cronHour.appoint = val.split(',').map(s => String(s).trim()).filter(Boolean)
    } else {
      cronHour.type = 'appoint'
      cronHour.appoint = [String(val)]
    }
  }

  applySecond(second)
  applyMinute(minute)
  applyHour(hour)

  if (day === '*') cronDay.type = '*'
  else if (day === '?') cronDay.type = '?'
  else if (day.includes('-')) {
    const [start, end] = day.split('-').map(Number)
    cronDay.type = 'interval'
    cronDay.start = start
    cronDay.end = end
  } else {
    cronDay.type = 'interval'
    const n = Number(day)
    if (!Number.isNaN(n)) {
      cronDay.start = n
      cronDay.end = n
    }
  }

  if (month === '*') cronMonth.type = '*'
  else if (month.includes('-')) {
    const [start, end] = month.split('-').map(Number)
    cronMonth.type = 'interval'
    cronMonth.start = start
    cronMonth.end = end
  } else {
    cronMonth.type = 'interval'
    const n = Number(month)
    if (!Number.isNaN(n)) {
      cronMonth.start = n
      cronMonth.end = n
    }
  }

  if (week === '*') cronWeek.type = '*'
  else if (week === '?') cronWeek.type = '?'
  else if (week.includes(',')) {
    cronWeek.type = 'interval'
    cronWeek.days = week.split(',').map(s => String(s).trim()).filter(Boolean)
  } else {
    cronWeek.type = 'interval'
    const n = String(week).trim()
    if (n) cronWeek.days = [n]
  }
}

// 复制 Cron 表达式
const copyCronExpr = async () => {
  try {
    await navigator.clipboard.writeText(generatedCronExpr.value)
    message.success('已复制到剪贴板')
  } catch (err) {
    message.error('复制失败')
  }
}

// 股票分析参数
const showCronBuilder = ref(false)
const generatedCronExpr = ref('')
const calculateNextRunTime = ref('')
const nextRunTimes = ref([])

//任务参数
const agentModeOptions = [
  { label: '🤖 自动选择', value: '' },
  { label: '⚡ 快速模式', value: 'react' },
  { label: '🧠 规划模式', value: 'plan_execute' },
  { label: '🔬 DeepAgents', value: 'deepagents' }
]

const stockAnalysisParamsData = reactive({
  promptId: 0,
  aiConfigId: 0,
  sysPromptId: 0,
  thinking: true,
  stockCode: '',
  stockName: '',
  agentMode: ''
})
const marketAnalysisParamsData= reactive({
  promptId: 0,
  aiConfigId: 0,
  sysPromptId: 0,
  thinking: true,
  agentMode: ''
})


// Cron 配置器数据
const cronSecond = reactive({ type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
const cronMinute = reactive({ type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
const cronHour = reactive({ type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
const cronDay = reactive({ type: '*', start: 1, end: 31 })
const cronMonth = reactive({ type: '*', start: 1, end: 12 })
const cronWeek = reactive({ type: '*', days: [] })

// 生成选项数据
const secondOptions = Array.from({ length: 60 }, (_, i) => ({ label: String(i).padStart(2, '0'), value: String(i) }))
const minuteOptions = Array.from({ length: 60 }, (_, i) => ({ label: String(i).padStart(2, '0'), value: String(i) }))
const hourOptions = Array.from({ length: 24 }, (_, i) => ({ label: String(i).padStart(2, '0'), value: String(i) }))

const weekOptions = [
  { label: '周日', value: '0' },
  { label: '周一', value: '1' },
  { label: '周二', value: '2' },
  { label: '周三', value: '3' },
  { label: '周四', value: '4' },
  { label: '周五', value: '5' },
  { label: '周六', value: '6' }
]

// 监听 Cron 配置变化，自动生成表达式并预览未来执行时间
watch([
  () => cronSecond.type,
  () => cronSecond.start,
  () => cronSecond.end,
  () => cronSecond.loopStart,
  () => cronSecond.loopStep,
  () => cronSecond.appoint,
  () => cronMinute.type,
  () => cronMinute.start,
  () => cronMinute.end,
  () => cronMinute.loopStart,
  () => cronMinute.loopStep,
  () => cronMinute.appoint,
  () => cronHour.type,
  () => cronHour.start,
  () => cronHour.end,
  () => cronHour.loopStart,
  () => cronHour.loopStep,
  () => cronHour.appoint,
  () => cronDay.type,
  () => cronDay.start,
  () => cronDay.end,
  () => cronMonth.type,
  () => cronMonth.start,
  () => cronMonth.end,
  () => cronWeek.type,
  () => cronWeek.days
], () => {
  generatedCronExpr.value = generateCronExpression()

  if (!generatedCronExpr.value) {
    calculateNextRunTime.value = ''
    nextRunTimes.value = []
    return
  }

  // 预览未来最近 5 次执行时间
  CalculateNextRunTimes(generatedCronExpr.value, 5)
    .then(res => {
      nextRunTimes.value = Array.isArray(res) ? res : []
      calculateNextRunTime.value = nextRunTimes.value[0] || ''
    })
    .catch(() => {
      nextRunTimes.value = []
      calculateNextRunTime.value = ''
    })
}, { deep: true })

// 获取任务类型显示名称
const getTaskTypeLabel = (value) => {
  const option = taskTypeOptions.value.find(opt => opt.value === value)
  return option ? option.label : value
}

// 表格列定义
const columns = [
  {
    title: 'ID',
    key: 'id',
    width: 60,
    ellipsis: { tooltip: true }
  },
  {
    title: '任务名称',
    key: 'name',
    width: 180,
    ellipsis: { tooltip: true },
    render(row) {
      return h('div', { style: 'display: flex; align-items: center; gap: 8px;' }, [
        h(NTag, { type: 'info', size: 'small', bordered: false }, {
          default: () => getTaskTypeLabel(row.taskType)
        }),
        h('span', {}, { default: () => row.name })
      ])
    }
  },
  // {
  //   title: '任务类型',
  //   key: 'taskType',
  //   width: 120,
  //   render(row) {
  //     return h(NTag, { type: 'info' }, { default: () => row.taskType })
  //   }
  // },
  {
    title: 'Cron 表达式',
    key: 'cronExpr',
    width: 150,
    ellipsis: { tooltip: true },
    render(row) {
      return h(NText, { code: true, depth: 2 }, { default: () => row.cronExpr })
    }
  },
  {
    title: '目标',
    key: 'target',
    width: 150,
    ellipsis: { tooltip: true }
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
        active: 'success',
        paused: 'warning',
        error: 'error'
      }
      return h(NTag, { type: typeMap[row.status] || 'default' }, {
        default: () => row.status
      })
    }
  },
  {
    title: '运行次数',
    key: 'runCount',
    width: 90,
    render(row) {
      return h(NSpace, { align: 'center' }, {
        default: () => [
          h(NIcon, { component: FlashOutline, size: 16, color: '#f0a020' }),
          h(NText, {}, { default: () => row.runCount || 0 })
        ]
      })
    }
  },
  {
    title: '最近执行',
    key: 'lastRunAt',
    width: 200,
    render(row) {
      if (!row.lastRunAt) return h(NText, { depth: 3 }, { default: () => '未运行' })
      const date = new Date(row.lastRunAt)
      const resultType = row.lastRunResult && row.lastRunResult.startsWith('成功') ? 'success' : 'error'
      return h(NSpace, { vertical: true, size: 2 }, {
        default: () => [
          h(NText, {}, { default: () => date.toLocaleString('zh-CN') }),
          row.lastRunResult ? h(NTag, { type: resultType, size: 'small' }, { default: () => row.lastRunResult }) : null
        ]
      })
    }
  },
  // {
  //   title: '下次运行',
  //   key: 'nextRunAt',
  //   width: 160,
  //   render(row) {
  //     if (!row.nextRunAt) return h(NText, { depth: 3 }, { default: () => '-' })
  //     const date = new Date(row.nextRunAt)
  //     return h(NText, {}, {
  //       default: () => date.toLocaleString('zh-CN')
  //     })
  //   }
  // },
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
              type: 'success',
              onClick: () => handleExecute(row)
            },
            {
              icon: () => h(NIcon, { component: PlayOutline }),
              default: () => '执行'
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
              default: () => (row.enable ? '暂停' : '启用')
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
              default: () => `确定要删除任务 "${row.name}" 吗？`
            }
          )
        ]
      })
    }
  }
]

// 分页配置
const pagination = computed(() => ({
  page: currentPage.value,
  pageSize: pageSize.value,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  onChange: handlePageChange,
  onUpdatePageSize: handlePageSizeChange
}))

// 加载任务类型
const loadTaskTypes = async () => {
  try {
    const types = await GetCronTaskTypes()
    taskTypeOptions.value = types.map(t => ({
      label: t.B,
      value: t.A
    }))
  } catch (error) {
    console.error('加载任务类型失败:', error)
  }
}

// 加载 AI 配置
const aiConfigOptions=ref([])
const loadAiConfigs = async () => {
  try {
    const configs = await GetAiConfigs()
    console.log('aiConfigOptions', configs)
    aiConfigOptions.value = configs.map(c => ({
      label: c.name+"["+c.modelName+"]",
      value: c.ID
    }))
  } catch (error) {
    console.error('加载 AI 配置失败:', error)
  }
}
const promptTemplateOptions=ref([])
const sysPromptOptions=ref([])
// 加载提示词模板
const loadPromptTemplates = async () => {
  try {
    // 加载用户提示词模板
    const userTemplates = await GetPromptTemplates('', '模型用户Prompt')
    promptTemplateOptions.value = userTemplates.map(t => ({
      label: t.name,
      value: t.ID
    }))
    
    // 加载系统提示词模板（假设类型为 system）
    const sysTemplates = await GetPromptTemplates('', '模型系统Prompt')
    sysPromptOptions.value = sysTemplates.map(t => ({
      label: t.name,
      value: t.ID
    }))
  } catch (error) {
    console.error('加载提示词模板失败:', error)
  }
}

// 加载任务列表
const loadTaskList = async () => {
  loading.value = true
  try {
    const query = {
      page: currentPage.value,
      pageSize: pageSize.value,
      name: searchKeyword.value,
      taskType: filterTaskType.value,
      status: filterStatus.value
    }
    
    const result = await GetCronTaskList(query)
    if (result) {
      taskList.value = result.data || []
      total.value = result.total || 0
    }
  } catch (error) {
    console.error('加载任务列表失败:', error)
    message.error('加载任务列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索任务
const handleSearch = async () => {
  currentPage.value = 1
  await loadTaskList()
}

// 分页变化
const handlePageChange = (page) => {
  currentPage.value = page
  loadTaskList()
}

const handlePageSizeChange = (size) => {
  pageSize.value = size
  currentPage.value = 1
  loadTaskList()
}

// 执行任务
const handleExecute = async (row) => {
  try {
    const result = await ExecuteCronTaskNow(row.id)
    message.success(result)
  } catch (error) {
    message.error('执行任务失败：' + error.message)
  }
}

// 切换启用状态
const handleToggleEnable = async (row) => {
  try {
    const newEnable = !row.enable
    const result = await EnableCronTask(row.id, newEnable)
    if (result === '操作成功') {
      message.success(newEnable ? '任务已启用' : '任务已禁用')
      await loadTaskList()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('操作失败：' + error.message)
  }
}

// 创建任务
const handleCreate = () => {
  editingTask.value = false
  resetForm()
  showCreateModal.value = true
}

// 编辑任务
const handleEdit = async (row) => {
  editingTask.value = true
  try {
    const task = await GetCronTaskByID(row.id)
    console.log("task",task)
    if (task) {
      // 先重置表单和 Cron 配置器
      resetForm()
      
      // 然后填充表单数据
      formData.id = task.id
      formData.name = task.name
      // 兼容后端返回 cronExpr 或 CronExpr
      formData.cronExpr = (task.cronExpr ?? task.CronExpr ?? '').trim()
      formData.taskType = task.taskType
      formData.target = task.target
      formData.params = task.params
      formData.enable = task.enable
      formData.status = task.status
      formData.description = task.description
      
      // 解析 Cron 表达式并回填到配置器
      parseCronExpression(formData.cronExpr)

      console.log("task.params",task.params)
      // 如果是股票分析任务，解析参数到表单
      if (task.taskType === 'stock_analysis' && task.params) {
        try {
          const parsed = JSON.parse(task.params)
          stockAnalysisParamsData.promptId = parsed.promptId ?? null
          stockAnalysisParamsData.aiConfigId = parsed.aiConfigId ?? null
          stockAnalysisParamsData.sysPromptId = parsed.sysPromptId ?? null
          stockAnalysisParamsData.thinking = parsed.thinking || false
          stockAnalysisParamsData.stockCode = parsed.stockCode || ''
          stockAnalysisParamsData.stockName = parsed.stockName || ''
          stockAnalysisParamsData.agentMode = parsed.agentMode || ''
        } catch (e) {
          console.error('解析参数失败:', e)
        }
      }
      
      // 如果是市场分析任务，解析参数到表单
      if (task.taskType === 'market_analysis' && task.params) {
        try {
          const parsed = JSON.parse(task.params)
          marketAnalysisParamsData.promptId = parsed.promptId ?? null
          marketAnalysisParamsData.aiConfigId = parsed.aiConfigId ?? null
          marketAnalysisParamsData.sysPromptId = parsed.sysPromptId ?? null
          marketAnalysisParamsData.thinking = parsed.thinking || false
          marketAnalysisParamsData.agentMode = parsed.agentMode || ''
        } catch (e) {
          console.error('解析参数失败:', e)
        }
      }
      
      showCreateModal.value = true
    }
  } catch (error) {
    message.error('获取任务详情失败：' + error.message)
  }
}

// 删除任务
const handleDelete = async (id) => {
  try {
    const result = await DeleteCronTask(id)
    if (result === '删除成功') {
      message.success('任务已删除')
      await loadTaskList()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('删除失败：' + error.message)
  }
}

// 检查两次执行间隔是否至少 60 秒，返回 { ok: boolean, minIntervalSeconds?: number }
const checkCronInterval = async (cronExpr) => {
  if (!cronExpr) return { ok: true }
  try {
    const times = await CalculateNextRunTimes(cronExpr, 10)
    if (!Array.isArray(times) || times.length < 2) return { ok: true }
    let minSeconds = Infinity
    for (let i = 1; i < times.length; i++) {
      const prev = new Date(times[i - 1]).getTime()
      const curr = new Date(times[i]).getTime()
      if (!Number.isNaN(prev) && !Number.isNaN(curr)) {
        const sec = (curr - prev) / 1000
        if (sec < minSeconds) minSeconds = sec
      }
    }
    if (minSeconds !== Infinity && minSeconds < 60) {
      return { ok: false, minIntervalSeconds: Math.round(minSeconds) }
    }
    return { ok: true }
  } catch (_) {
    return { ok: true }
  }
}

// 提交表单
const handleSubmit = async () => {
  try {
    // 先整体校验表单（会同步更新所有校验状态）
    if (formRef.value) {
      try {
        await formRef.value.validate()
      } catch {
        // 表单校验未通过，直接返回
        return
      }
    }

    // 简单验证 Cron 表达式
    if (!await validateCronExpression()) {
      return
    }

    // 检查执行间隔不小于 60 秒
    const intervalCheck = await checkCronInterval(formData.cronExpr)
    if (!intervalCheck.ok) {
      message.warning(
        `两次执行间隔过短（约 ${intervalCheck.minIntervalSeconds} 秒），请将间隔设置为至少 60 秒后再保存。`
      )
      return
    }

    formData.params = generatedParamsJson.value

    submitting.value = true
    const submitData = { ...formData }
    
    let result
    if (formData.id) {
      result = await UpdateCronTask(submitData)
    } else {
      result = await CreateCronTask(submitData)
    }

    if (result.includes('成功')) {
      message.success(result)
      showCreateModal.value = false
      await loadTaskList()
    } else {
      message.error(result)
    }
  } catch (error) {
    message.error('操作失败：' + error.message)
  } finally {
    submitting.value = false
  }
}

// 验证 Cron 表达式
const validateCronExpression = async () => {
  if (!formData.cronExpr) return false
  
  try {
    const result = await ValidateCronExpr(formData.cronExpr)
    if (result.includes('有效')) {
      //message.success('Cron 表达式有效')
      return true
    } else {
      message.error(result)
      return false
    }
  } catch (error) {
    message.error('Cron 表达式无效：' + error.message)
    return false
  }
}

// 重置表单
const resetForm = () => {
  Object.assign(formData, {
    id: null,
    name: '',
    cronExpr: '',
    taskType: '',
    target: '',
    params: '',
    enable: true,
    status: 'active',
    description: ''
  })
  Object.assign(stockAnalysisParamsData, {
    promptId: null,
    aiConfigId: null,
    sysPromptId: null,
    thinking: true,
    stockCode: '',
    stockName: '',
    agentMode: ''
  })
  Object.assign(marketAnalysisParamsData, {
    promptId: null,
    aiConfigId: null,
    sysPromptId: null,
    thinking: true,
    agentMode: ''
  })
  // 重置 Cron 配置器
  Object.assign(cronSecond, { type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
  Object.assign(cronMinute, { type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
  Object.assign(cronHour, { type: '*', start: 0, end: 0, loopStart: 0, loopStep: 1, appoint: [] })
  Object.assign(cronDay, { type: '*', start: 1, end: 31 })
  Object.assign(cronMonth, { type: '*', start: 1, end: 12 })
  Object.assign(cronWeek, { type: '*', days: [] })
  // 重置表单校验状态
  if (formRef.value) {
    formRef.value.restoreValidation()
  }
}

// 任务列表数据
const taskList = ref([])

// 监听任务类型变化，重置参数
watch(() => formData.taskType, (newType) => {
  if (newType === 'stock_analysis') {
    // 如果是股票分析任务，尝试解析现有参数
    if (formData.params) {
      try {
        const parsed = JSON.parse(formData.params)
        stockAnalysisParamsData.promptId = parsed.promptId ?? null
        stockAnalysisParamsData.aiConfigId = parsed.aiConfigId ?? null
        stockAnalysisParamsData.sysPromptId = parsed.sysPromptId ?? null
        stockAnalysisParamsData.thinking = parsed.thinking || false
        stockAnalysisParamsData.stockCode = parsed.stockCode || ''
        stockAnalysisParamsData.stockName = parsed.stockName || ''
        stockAnalysisParamsData.agentMode = parsed.agentMode || ''
      } catch (e) {
        console.error('解析参数失败:', e)
      }
    }
  }
})

// 初始化
onMounted(async () => {
  await loadTaskTypes()
  await loadAiConfigs()
  await loadPromptTemplates()
  await loadTaskList()
})
</script>

<style scoped>
.cron-row {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.cron-row:last-child {
  margin-bottom: 0;
}

.cron-label {
  width: 30px;
  font-weight: 600;
  color: #333;
  flex-shrink: 0;
}
</style>
