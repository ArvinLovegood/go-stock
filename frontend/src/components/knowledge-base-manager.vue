<template>
  <n-tabs v-model:value="activeTab" type="line" animated size="medium">
    <n-tab-pane name="kb" tab="知识库管理">
  <!-- 顶部操作栏 -->
  <n-space vertical style="margin-bottom: 12px">
    <n-space>
      <n-input
        v-model:value="searchKeyword"
        placeholder="搜索知识库名称/描述..."
        style="width: 240px"
        clearable
        @keyup.enter="loadKBList"
      >
        <template #prefix>
          <n-icon :component="SearchOutline" />
        </template>
      </n-input>

      <n-button type="primary" @click="loadKBList">
        <template #icon>
          <n-icon :component="RefreshOutline" />
        </template>
        刷新
      </n-button>

      <n-button type="warning" @click="openCreateModal">
        <template #icon>
          <n-icon :component="AddOutline" />
        </template>
        新建知识库
      </n-button>

      <n-tag :bordered="false" type="info" round>
        共 {{ kbList.length }} 个知识库
      </n-tag>
    </n-space>
  </n-space>

  <!-- 长期记忆向量服务配置 -->
  <n-card size="small" :bordered="true" style="margin-bottom: 12px">
    <template #header>
      <n-space align="center" :size="6">
        <n-icon :component="CubeOutline" />
        <span>长期记忆向量服务</span>
        <n-tooltip placement="top">
          <template #trigger>
            <n-icon color="#0e7a0d" size="16" style="cursor: help">
              <HelpCircleOutline />
            </n-icon>
          </template>
          长期记忆将历史问答向量化存储，供 Agent 自我进化时按语义召回。
          指定向量服务可避免自动选错；切换服务后旧向量维度可能不匹配，建议清空 memory 目录重建。
      </n-tooltip>
      </n-space>
    </template>
    <n-space align="center" :size="8">
      <n-select
        v-model:value="ltmAiConfigId"
        :options="ltmServiceOptions"
        :loading="loadingLtmServices"
        placeholder="选择向量服务（0 或留空 = 自动模式）"
        style="width: 360px"
        filterable
        @update:value="onLtmServiceChange"
      />
      <n-tag :bordered="false" :type="ltmAiConfigId > 0 ? 'success' : 'default'" size="small">
        {{ ltmAiConfigId > 0 ? '已指定' : '自动模式' }}
      </n-tag>
      <n-text depth="3" style="font-size: 12px">
        选项来自设置中模型类型为"向量模型"的 AI 服务
      </n-text>
    </n-space>
  </n-card>

  <!-- 知识库列表 -->
  <n-data-table
    size="small"
    :columns="kbColumns"
    :data="filteredKBList"
    :loading="loading"
    :row-key="(row) => row.name"
    flex-height
    style="height: calc(100vh - 210px); margin-top: 10px"
  />
    </n-tab-pane>

    <n-tab-pane name="ltm" tab="长期记忆检索">
      <n-space vertical :size="12">
        <!-- 长期记忆信息卡片 -->
        <n-card size="small" :bordered="true">
          <n-descriptions :column="3" label-placement="left" size="small">
            <n-descriptions-item label="状态">
              <n-tag :bordered="false" :type="ltmInfo?.ready ? 'success' : 'error'" size="small">
                {{ ltmInfo?.ready ? '已就绪' : '未就绪' }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="文档切片数">
              <n-tag :bordered="false" type="info" size="small">
                {{ ltmInfo?.docCount ?? 0 }}
              </n-tag>
            </n-descriptions-item>
            <n-descriptions-item label="向量服务">
              <n-tag v-if="ltmInfo?.aiConfigId > 0" :bordered="false" type="success" size="small">
                {{ ltmInfo.aiConfigName || '已指定' }}
              </n-tag>
              <n-tag v-else :bordered="false" type="default" size="small">自动模式</n-tag>
            </n-descriptions-item>
            <n-descriptions-item v-if="ltmInfo?.error" label="错误" :span="3">
              <n-text type="error">{{ ltmInfo.error }}</n-text>
            </n-descriptions-item>
          </n-descriptions>
          <n-space :size="8" style="margin-top: 8px">
            <n-button size="small" @click="loadLTMInfo">
              <template #icon><n-icon :component="RefreshOutline" /></template>
              刷新
            </n-button>
            <n-text depth="3" style="font-size: 12px">
              长期记忆自动归档历史问答，Agent 自我进化时按语义召回
            </n-text>
          </n-space>
        </n-card>

        <!-- 检索测试 -->
        <n-card title="检索测试" size="small" :bordered="true">
          <n-space :size="8" style="width: 100%">
            <n-input
              v-model:value="ltmQuery"
              placeholder="输入查询语句，检索语义相关的历史问答..."
              style="flex: 1"
              clearable
              @keyup.enter="handleLTMSearch"
            />
            <n-input-number
              v-model:value="ltmTopK"
              :min="1"
              :max="20"
              style="width: 110px"
              placeholder="Top-K"
            />
            <n-button type="primary" @click="handleLTMSearch" :loading="ltmSearching">
              检索
            </n-button>
          </n-space>

          <div v-if="ltmResults.length > 0" style="margin-top: 12px">
            <n-divider style="margin: 8px 0" />
            <n-space vertical :size="8">
              <n-card
                v-for="(r, idx) in ltmPagedResults"
                :key="(ltmCurrentPage - 1) * ltmPageSize + idx"
                size="small"
                :bordered="true"
              >
                <n-space justify="space-between" align="center">
                  <n-space :size="8" align="center">
                    <n-tag :bordered="false" type="warning" size="small">#{{ (ltmCurrentPage - 1) * ltmPageSize + idx + 1 }}</n-tag>
                    <n-tag :bordered="false" type="info" size="small">
                      相似度 {{ (r.similarity * 100).toFixed(1) }}%
                    </n-tag>
                    <n-tag v-if="r.mode" :bordered="false" size="small">{{ r.mode }}</n-tag>
                    <n-text v-if="r.date" depth="3" style="font-size: 12px">{{ r.date }}</n-text>
                  </n-space>
                </n-space>
                <n-text strong style="display: block; margin-top: 6px">
                  Q: {{ r.question }}
                </n-text>
                <n-text style="white-space: pre-wrap; display: block; margin-top: 4px">
                  A: {{ r.reply }}
                </n-text>
              </n-card>
            </n-space>
            <n-space justify="end" style="margin-top: 12px">
              <n-pagination
                v-model:page="ltmCurrentPage"
                :page-count="Math.ceil(ltmResults.length / ltmPageSize)"
                :page-size="ltmPageSize"
                :item-count="ltmResults.length"
                show-size-picker
                :page-sizes="[10, 20, 50]"
                @update:page-size="(s) => { ltmPageSize = s; ltmCurrentPage = 1 }"
              />
            </n-space>
          </div>
          <n-empty
            v-else-if="ltmSearchedOnce"
            description="未找到相关历史问答"
            style="margin-top: 12px"
          />
        </n-card>
      </n-space>
    </n-tab-pane>

    <n-tab-pane name="kbqa" tab="知识库问答">
      <n-space vertical :size="12">
        <!-- 说明卡片 -->
        <n-card size="small" :bordered="true">
          <n-space align="center" :size="8">
            <n-icon :component="ChatbubblesOutline" />
            <n-text strong>跨所有知识库 + 历史经验统一问答</n-text>
            <n-text depth="3" style="font-size: 12px">
              输入问题 → 一次性检索所有自定义知识库与历史问答 → Agent 基于检索内容综合回答
            </n-text>
          </n-space>
        </n-card>

        <!-- 问答输入区 -->
        <n-card size="small" :bordered="true">
          <n-space vertical :size="10" style="width: 100%">
            <n-input
              v-model:value="kbqaQuestion"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 5 }"
              placeholder="输入你的问题，将跨所有知识库与历史经验检索后由 Agent 综合回答..."
              clearable
            />
            <n-space align="center" :size="8" wrap>
              <n-input-number
                v-model:value="kbqaTopK"
                :min="1"
                :max="20"
                style="width: 110px"
                placeholder="Top-K"
              >
                <template #prefix>TopK</template>
              </n-input-number>
              <n-select
                v-model:value="kbqaAiConfigId"
                :options="aiConfigOptions"
                placeholder="选择 AI 服务"
                style="width: 220px"
                filterable
              />
              <n-select
                v-model:value="kbqaAgentMode"
                :options="kbqaAgentModeOptions"
                style="width: 160px"
              />
              <n-button
                type="primary"
                ghost
                :loading="kbqaSearching"
                @click="handleKBQASearch"
              >
                <template #icon><n-icon :component="SearchOutline" /></template>
                统一检索
              </n-button>
              <n-button
                type="primary"
                :loading="kbqaAnswering"
                :disabled="kbqaSearching"
                @click="handleKBQAAsk"
              >
                <template #icon><n-icon :component="SendOutline" /></template>
                检索并回答
              </n-button>
              <n-button
                v-if="kbqaAnswering"
                type="error"
                ghost
                @click="handleKBQAAbort"
              >
                <template #icon><n-icon :component="StopCircleOutline" /></template>
                中止
              </n-button>
            </n-space>
          </n-space>
        </n-card>

        <!-- 检索结果 + AI 回答：左右布局，参考 Home 题材组件——内容区 max-height + overflow 滚动 -->
        <div :style="{ display: 'flex', gap: '8px', alignItems: 'flex-start', justifyContent: 'flex-start' }">
          <!-- 检索命中片段侧 -->
          <div
            v-if="kbqaHits.length > 0"
            :style="{ flex: hitsCollapsed ? '0 0 36px' : (answerCollapsed || !kbqaAnsweredOnce ? '1 1 100%' : '1 1 50%'), minWidth: hitsCollapsed ? '36px' : '320px' }"
          >
            <!-- 折叠窄条：竖排标题 + 展开按钮 -->
            <div v-if="hitsCollapsed" class="kbqa-collapse-bar" @click="hitsCollapsed = false">
              <n-icon :component="LayoutSidebarLeftExpand" size="18" />
              <span class="kbqa-vertical-text">检索结果（{{ kbqaHits.length }}）</span>
            </div>
            <!-- 展开态卡片 -->
            <n-card v-else size="small" :bordered="true">
              <template #header>
                <n-space :size="6" align="center" justify="space-between" style="width: 100%">
                  <n-space :size="6" align="center">
                    <span>检索命中片段（按相似度全局排序）</span>
                    <n-tag :bordered="false" type="info" size="small">共 {{ kbqaHits.length }} 条</n-tag>
                  </n-space>
                  <n-button text size="tiny" @click="hitsCollapsed = true" title="向左收起">
                    <n-icon :component="LayoutSidebarLeftCollapse" size="18" />
                  </n-button>
                </n-space>
              </template>
              <div class="thin-scroll" style="max-height: calc(92vh - 450px); overflow-y: auto;">
                <n-space vertical :size="8">
                  <n-card
                    v-for="(hit, idx) in kbqaPagedHits"
                    :key="(kbqaCurrentPage - 1) * kbqaPageSize + idx"
                    size="small"
                    :bordered="true"
                  >
                    <n-space :size="8" align="center" wrap>
                      <n-tag :bordered="false" type="warning" size="small">#{{ (kbqaCurrentPage - 1) * kbqaPageSize + idx + 1 }}</n-tag>
                      <n-tag :bordered="false" type="info" size="small">
                        相似度 {{ (hit.similarity * 100).toFixed(1) }}%
                      </n-tag>
                      <n-tag
                        :bordered="false"
                        :type="hit.sourceType === 'long_term_memory' ? 'success' : 'default'"
                        size="small"
                      >
                        {{ hit.sourceType === 'long_term_memory' ? '历史经验' : '知识库' }}
                      </n-tag>
                      <n-tag v-if="hit.kbName" :bordered="false" size="small">{{ hit.kbName }}</n-tag>
                      <n-text v-if="hit.source" depth="3" style="font-size: 12px">{{ hit.source }}</n-text>
                      <n-text v-if="hit.date" depth="3" style="font-size: 12px">{{ hit.date }}</n-text>
                    </n-space>
                    <n-text v-if="hit.question" strong style="display: block; margin-top: 6px; text-align: left">
                      Q: {{ hit.question }}
                    </n-text>
                    <n-text style="white-space: pre-wrap; display: block; margin-top: 4px; text-align: left">
                      {{ hit.content }}
                    </n-text>
                  </n-card>
                </n-space>
                <n-space justify="end" style="margin-top: 12px">
                  <n-pagination
                    v-model:page="kbqaCurrentPage"
                    :page-count="Math.ceil(kbqaHits.length / kbqaPageSize)"
                    :page-size="kbqaPageSize"
                    :item-count="kbqaHits.length"
                    show-size-picker
                    :page-sizes="[10, 20, 50]"
                    @update:page-size="(s) => { kbqaPageSize = s; kbqaCurrentPage = 1 }"
                  />
                </n-space>
              </div>
            </n-card>
          </div>
          <n-empty
            v-else-if="kbqaSearchedOnce && !kbqaAnsweredOnce"
            description="未检索到相关内容"
            :style="{ flex: '1 1 100%', marginTop: '12px' }"
          />

          <!-- Agent 综合回答侧 -->
          <div
            v-if="kbqaAnsweredOnce"
            :style="{ flex: answerCollapsed ? '0 0 36px' : (hitsCollapsed || kbqaHits.length === 0 ? '1 1 100%' : '1 1 50%'), minWidth: answerCollapsed ? '36px' : '320px' }"
          >
            <!-- 折叠窄条 -->
            <div v-if="answerCollapsed" class="kbqa-collapse-bar" @click="answerCollapsed = false">
              <n-icon :component="LayoutSidebarRightExpand" size="18" />
              <span class="kbqa-vertical-text">
                Agent 回答
                <n-tag v-if="kbqaAnswering" :bordered="false" type="warning" size="small" style="margin-left: 4px">中</n-tag>
              </span>
            </div>
            <!-- 展开态卡片 -->
            <n-card v-else size="small" :bordered="true">
              <template #header>
                <n-space :size="6" align="center" justify="space-between" style="width: 100%">
                  <n-space :size="6" align="center">
                    <span>Agent 综合回答</span>
                    <n-tag v-if="kbqaAnswering" :bordered="false" type="warning" size="small">回答中...</n-tag>
                    <n-tag v-else :bordered="false" type="success" size="small">已完成</n-tag>
                  </n-space>
                  <n-space :size="2" align="center">
                    <n-button text size="tiny" :disabled="!kbqaAnswer" @click="copyKbqaAnswer" title="复制回答">
                      <n-icon :component="CopyOutline" size="18" />
                    </n-button>
                    <n-button text size="tiny" :loading="kbqaExportLoading" :disabled="!kbqaAnswer" @click="exportKbqaAnswerImage" title="导出为图片">
                      <n-icon :component="ImageOutline" size="18" />
                    </n-button>
                    <n-button text size="tiny" :loading="kbqaShareLoading" :disabled="!kbqaAnswer" @click="shareKbqaAnswerToCommunity" title="分享到社区">
                      <n-icon :component="ShareSocialOutline" size="18" />
                    </n-button>
                    <n-button text size="tiny" @click="answerCollapsed = true" title="向右收起">
                      <n-icon :component="LayoutSidebarRightCollapse" size="18" />
                    </n-button>
                  </n-space>
                </n-space>
              </template>
              <div class="thin-scroll" style="max-height: calc(92vh - 450px); overflow-y: auto;">
                <div v-if="kbqaReasoning" style="margin-bottom: 8px; text-align: left">
                  <n-text depth="3" style="font-size: 12px">思考过程：</n-text>
                  <MdPreview
                    :modelValue="kbqaReasoning"
                    :theme="mdTheme"
                    style="font-size: 0.92em; opacity: 0.75"
                  />
                </div>
                <div ref="kbqaAnswerWrapRef">
                  <MdPreview
                    :modelValue="kbqaAnswer || (kbqaAnswering ? '正在生成...' : '')"
                    :theme="mdTheme"
                  />
                </div>
                <div ref="kbqaAnswerEndRef" style="height: 0; overflow: hidden"></div>
              </div>
            </n-card>
          </div>
        </div>
      </n-space>
    </n-tab-pane>
  </n-tabs>

  <!-- 创建知识库弹窗 -->
  <n-modal
    v-model:show="showCreateModal"
    title="新建知识库"
    preset="dialog"
    :style="{ width: '520px' }"
    @close="resetCreateForm"
    :z-index="2000"
    to="body"
  >
    <n-form
      ref="createFormRef"
      :model="createForm"
      :rules="createRules"
      label-placement="left"
      label-width="100px"
      require-mark-placement="right-hanging"
    >
      <n-form-item label="名称" path="name">
        <n-input
          v-model:value="createForm.name"
          placeholder="如：财报知识、行业研究、投资策略"
          clearable
          maxlength="50"
          show-count
        />
      </n-form-item>

      <n-form-item label="描述" path="description">
        <n-input
          v-model:value="createForm.description"
          type="textarea"
          :rows="3"
          placeholder="知识库用途说明（可选）"
          maxlength="200"
          show-count
        />
      </n-form-item>

      <n-form-item label="向量模型" path="aiConfigID" required>
        <n-select
          v-model:value="createForm.aiConfigID"
          :options="embeddingServiceOptions"
          :loading="loadingAIServices"
          placeholder="选择向量模型（来自设置中配置了向量模型的 AI 服务）"
          filterable
          @update:value="onEmbeddingServiceChange"
        />
        <n-text depth="3" style="font-size: 12px; margin-top: 4px; display: block">
          选项来自"设置 → AI 配置"中模型类型为"向量模型"的 AI 服务
        </n-text>
      </n-form-item>

      <n-alert
        v-if="!loadingAIServices && embeddingServiceOptions.length === 0"
        type="warning"
        :bordered="false"
        style="margin-top: 8px"
      >
        未找到向量模型类型的 AI 服务。请先到"设置 → AI 配置"中新增一个 AI 服务，将"模型类型"选为"向量模型"，并填写向量模型名（如 text-embedding-3-small / text-embedding-v3 / BAAI/bge-m3）。
      </n-alert>

      <n-alert type="info" :bordered="false" style="margin-top: 8px">
        向量化由后台处理：上传文档时自动调用所选 AI 服务的 embedding 接口切片入库。
        创建后可在"管理文档"中上传文件，Agent 会自动检索该知识库。
      </n-alert>
    </n-form>

    <template #action>
      <n-button @click="showCreateModal = false">取消</n-button>
      <n-button type="primary" @click="handleCreate" :loading="submitting">
        <template #icon>
          <n-icon :component="CheckmarkCircleOutline" />
        </template>
        创建
      </n-button>
    </template>
  </n-modal>

  <!-- 文档管理抽屉 -->
  <n-drawer
    v-model:show="showDocDrawer"
    :width="drawerWidth"
    placement="right"
    :z-index="1500"
    to="body"
  >
    <n-drawer-content
      :title="`知识库：${currentKB?.name || ''}`"
      closable
    >
      <template v-if="currentKB">
        <n-space vertical :size="12">
          <!-- KB 概览 -->
          <n-card size="small" :bordered="true">
            <n-descriptions :column="3" label-placement="left" size="small">
              <n-descriptions-item label="名称">
                <n-text strong>{{ currentKB.name }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item label="文档数">
                <n-space :size="4" align="center" :wrap="false">
                  <n-tag :bordered="false" type="success" size="small">
                    {{ currentKB.documentCount }}
                  </n-tag>
                  <n-tag
                    v-if="currentKBVectorizingState && currentKBVectorizingState.isVectorizing"
                    :bordered="false" type="warning" size="small" round
                  >
                    向量化中 {{ currentKBVectorizingState.processedFiles }}/{{ currentKBVectorizingState.totalFiles }}
                  </n-tag>
                </n-space>
              </n-descriptions-item>
              <n-descriptions-item label="创建时间">
                {{ formatTime(currentKB.createdAt) }}
              </n-descriptions-item>
              <n-descriptions-item label="AI 服务">
                <n-tag :bordered="false" type="info" size="small">
                  {{ currentKB.aiConfigName || '默认' }}
                </n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="向量模型">
                <n-text code>{{ currentKB.embeddingModel || '默认' }}</n-text>
              </n-descriptions-item>
              <n-descriptions-item label="描述" :span="3">
                {{ currentKB.description || '未填写' }}
              </n-descriptions-item>
            </n-descriptions>
          </n-card>

          <!-- 检索测试 -->
          <n-card title="检索测试" size="small" :bordered="true">
            <n-space :size="8" style="width: 100%">
              <n-input
                v-model:value="searchQuery"
                placeholder="输入查询语句测试语义检索..."
                style="flex: 1"
                clearable
                @keyup.enter="handleSearch"
              />
              <n-input-number
                v-model:value="searchTopK"
                :min="1"
                :max="20"
                style="width: 110px"
                placeholder="Top-K"
              />
              <n-button type="primary" @click="handleSearch" :loading="searching">
                检索
              </n-button>
            </n-space>

            <div v-if="searchResults.length > 0" style="margin-top: 12px">
              <n-divider style="margin: 8px 0" />
              <n-space vertical :size="8">
                <n-card
                  v-for="(r, idx) in searchResults"
                  :key="r.documentId"
                  size="small"
                  :bordered="true"
                >
                  <n-space justify="space-between" align="center">
                    <n-space :size="8" align="center">
                      <n-tag :bordered="false" type="warning" size="small">
                        #{{ idx + 1 }}
                      </n-tag>
                      <n-tag :bordered="false" type="info" size="small">
                        相似度 {{ (r.similarity * 100).toFixed(1) }}%
                      </n-tag>
                      <n-text depth="3" style="font-size: 12px">
                        来源: {{ r.source || '未知' }}
                      </n-text>
                    </n-space>
                  </n-space>
                  <n-text style="white-space: pre-wrap; display: block; margin-top: 6px">
                    {{ r.content }}
                  </n-text>
                </n-card>
              </n-space>
            </div>
            <n-empty
              v-else-if="searchedOnce"
              description="未找到相关文档"
              style="margin-top: 12px"
            />
          </n-card>

          <!-- 添加文档 -->
          <n-card title="添加文档" size="small" :bordered="true">
            <n-tabs type="line" size="small" animated>
              <n-tab-pane name="file" tab="上传文件（支持多选）">
                <n-space vertical :size="8" style="width: 100%">
                  <n-space :size="8" style="width: 100%" align="center">
                    <n-button @click="handlePickFiles" :loading="picking">
                      <template #icon>
                        <n-icon :component="FolderOpenOutline" />
                      </template>
                      选择文件（可多选）
                    </n-button>
                    <n-text depth="3" style="font-size: 12px">
                      支持 .txt/.md，单文件 ≤10MB
                    </n-text>
                    <n-button
                      type="primary"
                      @click="handleUploadFiles"
                      :loading="uploading"
                      :disabled="uploadFileList.length === 0"
                    >
                      <template #icon>
                        <n-icon :component="CloudUploadOutline" />
                      </template>
                      批量入库 ({{ uploadFileList.length }})
                    </n-button>
                  </n-space>

                  <!-- 已选文件列表 -->
                  <div v-if="uploadFileList.length > 0">
                    <n-tag
                      v-for="(f, idx) in uploadFileList"
                      :key="idx"
                      closable
                      size="small"
                      style="margin: 2px"
                      @close="removeUploadFile(idx)"
                    >
                      {{ f.name }}
                    </n-tag>
                  </div>

                  <!-- 向量化进度/结果（从实时状态读取，关闭抽屉后后台仍继续） -->
                  <div v-if="currentKBVectorizingState">
                    <n-divider style="margin: 8px 0" />
                    <!-- 进行中：进度条 -->
                    <template v-if="currentKBVectorizingState.isVectorizing">
                      <n-space :size="8" align="center" style="margin-bottom: 6px">
                        <n-tag :bordered="false" type="warning" size="small" round>
                          向量化中 {{ currentKBVectorizingState.processedFiles }}/{{ currentKBVectorizingState.totalFiles }}
                        </n-tag>
                        <n-text depth="3" style="font-size: 12px">
                          已入库切片 {{ currentKBVectorizingState.totalChunks }}
                        </n-text>
                      </n-space>
                      <n-progress
                        type="line"
                        :height="6"
                        :border-radius="3"
                        status="warning"
                        :percentage="currentKBVectorizingState.totalFiles > 0
                          ? Math.round(currentKBVectorizingState.processedFiles / currentKBVectorizingState.totalFiles * 100)
                          : 0"
                      />
                    </template>
                    <!-- 已完成：结果汇总 + 明细表 -->
                    <template v-else>
                      <n-space :size="8" align="center" style="margin-bottom: 8px">
                        <n-tag :bordered="false" type="info" size="small">
                          共 {{ currentKBVectorizingState.processedFiles }} 个
                        </n-tag>
                        <n-tag :bordered="false" type="success" size="small">
                          成功 {{ currentKBVectorizingState.successCount }}
                        </n-tag>
                        <n-tag v-if="currentKBVectorizingState.failedCount > 0" :bordered="false" type="error" size="small">
                          失败 {{ currentKBVectorizingState.failedCount }}
                        </n-tag>
                        <n-tag :bordered="false" type="warning" size="small">
                          切片 {{ currentKBVectorizingState.totalChunks }}
                        </n-tag>
                        <n-text v-if="currentKBVectorizingState.error" type="error" style="font-size: 12px">
                          {{ currentKBVectorizingState.error }}
                        </n-text>
                      </n-space>
                      <n-data-table
                        v-if="currentKBVectorizingState.results && currentKBVectorizingState.results.length > 0"
                        size="small"
                        :columns="batchResultColumns"
                        :data="currentKBVectorizingState.results"
                        :max-height="200"
                        :row-key="(row) => row.filePath"
                      />
                    </template>
                  </div>
                </n-space>
              </n-tab-pane>

              <n-tab-pane name="text" tab="粘贴文本">
                <n-space vertical :size="8" style="width: 100%">
                  <n-input
                    v-model:value="inlineText"
                    type="textarea"
                    :rows="5"
                    placeholder="粘贴或输入文本内容（将自动按段落切片入库）"
                  />
                  <n-space :size="8">
                    <n-input
                      v-model:value="inlineSource"
                      placeholder="来源标记（可选，如：手动录入）"
                      style="width: 280px"
                    />
                    <n-button
                      type="primary"
                      @click="handleAddInline"
                      :loading="uploading"
                      :disabled="!inlineText.trim()"
                    >
                      入库
                    </n-button>
                  </n-space>
                </n-space>
              </n-tab-pane>
            </n-tabs>
          </n-card>

          <!-- 文档列表（后台分页） -->
          <n-card title="文档列表" size="small" :bordered="true">
            <n-data-table
              size="small"
              :columns="docColumns"
              :data="docList"
              :loading="docLoading"
              :row-key="(row) => row.id"
              :max-height="420"
              :pagination="docPagination"
              remote
            />
          </n-card>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<script setup>
import { ref, reactive, computed, h, onMounted, onUnmounted, nextTick, watch } from 'vue'
import {
  NButton, NIcon, NTag, NSpace, NPopconfirm, useMessage, NText, NCard,
  NInput, NAlert, NModal, NForm, NFormItem, NInputNumber, NDataTable,
  NDrawer, NDrawerContent, NDescriptions, NDescriptionsItem, NDivider,
  NEmpty, NTabs, NTabPane, NSelect, NPagination
} from 'naive-ui'
import {
  SearchOutline, AddOutline, RefreshOutline, TrashOutline,
  CheckmarkCircleOutline, FolderOpenOutline, CloudUploadOutline,
  CheckmarkCircleSharp, CloseCircleSharp, CubeOutline, HelpCircleOutline,
  ChatbubblesOutline, SendOutline,
  StopCircleOutline, ChevronDownOutline, ChevronUpOutline,
  ChevronForwardOutline, ChevronBackOutline,
  CopyOutline, ImageOutline, ShareSocialOutline
} from '@vicons/ionicons5'
import {
  LayoutSidebarLeftCollapse, LayoutSidebarRightCollapse,
  LayoutSidebarLeftExpand, LayoutSidebarRightExpand
} from '@vicons/tabler'
import {
  ListKnowledgeBases, CreateKnowledgeBase, DeleteKnowledgeBase,
  UploadKBFiles, AddKBDocument, SearchKnowledgeBase,
  ListKBDocuments, ListKBDocumentsPaged, DeleteKBDocument, PickKBFilePaths, ListAIServicesForKB,
  GetLongTermMemoryAiConfigId, SetLongTermMemoryAiConfigId,
  GetAllKBVectorizingStatuses, GetKBVectorizingStatus, GetKnowledgeBase,
  GetLongTermMemoryInfo, SearchLongTermMemory,
  SearchAllKnowledge, ChatWithAgentKBQA, AbortChatWithAgent, GetAiConfigs,
  GetConfig, ShareText, SaveImage
} from '../../wailsjs/go/main/App'
import html2canvas from 'html2canvas'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

// md-editor 主题：跟随应用暗色设置（与 skill-manager 一致）
const mdTheme = ref('light')

const message = useMessage()

// ============ 页签切换 ============
const activeTab = ref('kb')

// ============ 长期记忆检索 ============
const ltmInfo = ref(null)
const ltmQuery = ref('')
const ltmTopK = ref(5)
const ltmResults = ref([])
const ltmSearching = ref(false)
const ltmSearchedOnce = ref(false)
// 分页
const ltmCurrentPage = ref(1)
const ltmPageSize = ref(10)
const ltmPagedResults = computed(() => {
  const start = (ltmCurrentPage.value - 1) * ltmPageSize.value
  return ltmResults.value.slice(start, start + ltmPageSize.value)
})

async function loadLTMInfo() {
  try {
    ltmInfo.value = await GetLongTermMemoryInfo()
  } catch (e) {
    ltmInfo.value = { ready: false, error: String(e) }
  }
}

async function handleLTMSearch() {
  if (!ltmQuery.value.trim()) {
    message.warning('请输入查询语句')
    return
  }
  ltmSearching.value = true
  ltmSearchedOnce.value = true
  try {
    const results = await SearchLongTermMemory(ltmQuery.value, ltmTopK.value)
    ltmResults.value = results || []
    ltmCurrentPage.value = 1
    if (ltmResults.value.length === 0) {
      message.info('未找到相关历史问答')
    }
  } catch (e) {
    message.error(`检索失败: ${e}`)
    ltmResults.value = []
  } finally {
    ltmSearching.value = false
  }
}

// 切换到长期记忆页签时加载信息
watch(activeTab, (val) => {
  if (val === 'ltm' && !ltmInfo.value) {
    loadLTMInfo()
  }
  if (val === 'kbqa') {
    if (aiConfigOptions.value.length === 0) loadAiConfigs()
  }
})

// ============ 知识库问答（统一检索 + Agent 综合回答） ============
const KBQA_EVENT = 'kb-qa-message'
const aiConfigOptions = ref([])
const kbqaQuestion = ref('')
const kbqaTopK = ref(5)
const kbqaAiConfigId = ref(null)
const kbqaAgentMode = ref('plan_execute')
const kbqaAgentModeOptions = [
  { label: '🤖 自动', value: 'auto' },
  { label: '⚡ 快速', value: 'react' },
  { label: '🧠 规划', value: 'plan_execute' },
  { label: '🔬 DeepAgents', value: 'deepagents' },
]
const kbqaHits = ref([])
const kbqaSearching = ref(false)
const kbqaSearchedOnce = ref(false)
const kbqaAnswering = ref(false)
const kbqaAnswer = ref('')
const kbqaReasoning = ref('')
const kbqaAnsweredOnce = ref(false)
// 检索结果与 AI 回答的折叠状态（点击 header 按钮切换）
const hitsCollapsed = ref(false)
const answerCollapsed = ref(false)
// AI 回答自动滚动锚点
const kbqaAnswerEndRef = ref(null)
const kbqaAnswerWrapRef = ref(null)
// AI 回答分享/导出/复制 状态
const kbqaShareLoading = ref(false)
const kbqaExportLoading = ref(false)
// 命中片段分页
const kbqaCurrentPage = ref(1)
const kbqaPageSize = ref(10)
const kbqaPagedHits = computed(() => {
  const start = (kbqaCurrentPage.value - 1) * kbqaPageSize.value
  return kbqaHits.value.slice(start, start + kbqaPageSize.value)
})

async function loadAiConfigs() {
  try {
    const list = await GetAiConfigs()
    aiConfigOptions.value = (list || []).map(c => ({ label: c.name, value: c.ID }))
    if (kbqaAiConfigId.value == null && aiConfigOptions.value.length > 0) {
      kbqaAiConfigId.value = aiConfigOptions.value[0].value
    }
  } catch (e) {
    console.error('loadAiConfigs error:', e)
  }
}

async function handleKBQASearch() {
  if (!kbqaQuestion.value.trim()) {
    message.warning('请输入问题')
    return
  }
  kbqaSearching.value = true
  kbqaSearchedOnce.value = true
  kbqaHits.value = []
  kbqaCurrentPage.value = 1
  try {
    const hits = await SearchAllKnowledge(kbqaQuestion.value, kbqaTopK.value)
    kbqaHits.value = hits || []
    if (kbqaHits.value.length === 0) {
      message.info('未在任何知识库与历史经验中检索到相关内容')
    }
  } catch (e) {
    message.error(`统一检索失败: ${e}`)
    kbqaHits.value = []
  } finally {
    kbqaSearching.value = false
  }
}

// 检测 agent 框架最终返回的 {"response": "..."} JSON 汇总消息
// 流式 chunk 已逐段累积到 kbqaAnswer，框架最后会追加一条完整 response JSON，
// 若直接 += 会造成内容重复并显示 JSON 原文，因此解析后用最终完整版替换。
function tryParseFinalResponseJSON(content) {
  if (!content) return null
  const trimmed = content.trim()
  if (!trimmed.startsWith('{') || !trimmed.endsWith('}')) return null
  try {
    const obj = JSON.parse(trimmed)
    if (obj && typeof obj.response === 'string' && obj.response.trim() !== '') {
      return obj.response
    }
  } catch (_) {}
  return null
}

function handleKBQAStreamMessage(data) {
  if (data == null) return
  // DONE 标记：后端发送 content="agent-DONE" 的 assistant 消息作为结束信号
  if (data.role === 'assistant' && data.content === 'agent-DONE') {
    kbqaAnswering.value = false
    return
  }
  if (data.role === 'assistant') {
    if (data.content) {
      // 检测最终的 {"response": "..."} JSON：替换而非追加，避免重复
      const finalResp = tryParseFinalResponseJSON(data.content)
      if (finalResp !== null) {
        kbqaAnswer.value = finalResp
      } else {
        kbqaAnswer.value += data.content
      }
    }
    if (data.reasoning_content) kbqaReasoning.value += data.reasoning_content
    kbqaAnsweredOnce.value = true
  }
}

async function handleKBQAAsk() {
  if (!kbqaQuestion.value.trim()) {
    message.warning('请输入问题')
    return
  }
  if (kbqaAiConfigId.value == null) {
    message.warning('请选择 AI 服务')
    return
  }
  // 每次综合回答都重新检索，刷新左边命中片段区域（确保片段与当前问题一致）
  if (!kbqaSearching.value) {
    await handleKBQASearch()
  }
  kbqaAnswering.value = true
  kbqaAnsweredOnce.value = true
  kbqaAnswer.value = ''
  kbqaReasoning.value = ''
  const hitsJSON = JSON.stringify(kbqaHits.value || [])
  const mode = kbqaAgentMode.value === 'auto' ? '' : kbqaAgentMode.value
  try {
    ChatWithAgentKBQA(kbqaQuestion.value, kbqaAiConfigId.value, mode, hitsJSON)
  } catch (e) {
    kbqaAnswering.value = false
    message.error(`启动问答失败: ${e}`)
  }
}

function handleKBQAAbort() {
  try {
    AbortChatWithAgent()
  } catch (e) {
    console.error('abort error:', e)
  }
  kbqaAnswering.value = false
}

// 复制 AI 回答（markdown 原文）
async function copyKbqaAnswer() {
  const text = (kbqaAnswer.value || '').trim()
  if (!text) {
    message.warning('暂无可复制的回答内容')
    return
  }
  try {
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    message.success('已复制回答内容')
  } catch (e) {
    message.error('复制失败，请手动选择文本')
  }
}

// 分享 AI 回答到项目社区
function shareKbqaAnswerToCommunity() {
  const text = (kbqaAnswer.value || '').trim()
  if (!text) {
    message.warning('暂无可分享的回答内容')
    return
  }
  if (kbqaShareLoading.value) return
  kbqaShareLoading.value = true
  ShareText(text, '')
    .then((msg) => {
      message.success(msg || '已分享到社区')
    })
    .catch((err) => {
      message.error('分享失败: ' + (err?.message ?? err))
    })
    .finally(() => {
      kbqaShareLoading.value = false
    })
}

// 导出 AI 回答为图片：用 html2canvas 截取 MdPreview 预览区
async function exportKbqaAnswerImage() {
  if (kbqaExportLoading.value) return
  const text = (kbqaAnswer.value || '').trim()
  if (!text) {
    message.warning('暂无可导出的回答内容')
    return
  }
  kbqaExportLoading.value = true
  await nextTick()
  try {
    const wrap = kbqaAnswerWrapRef.value
    const target = (wrap && wrap.querySelector('.md-editor-preview')) || wrap || null
    if (!target) {
      message.error('未找到预览区域')
      return
    }
    // 临时解除父级滚动/高度限制，保证完整截取
    const savedStyles = []
    let el = target.parentElement
    while (el && el !== document.body) {
      const style = getComputedStyle(el)
      if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
        savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
        el.style.overflow = 'visible'
        el.style.overflowY = 'visible'
        el.style.height = 'auto'
        el.style.maxHeight = 'none'
      }
      el = el.parentElement
    }
    const savedTargetStyle = { height: target.style.height, maxHeight: target.style.maxHeight, overflow: target.style.overflow, overflowY: target.style.overflowY }
    target.style.height = 'auto'
    target.style.maxHeight = 'none'
    target.style.overflow = 'visible'
    target.style.overflowY = 'visible'
    await nextTick()
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: mdTheme.value === 'dark' ? '#1e1e1e' : '#ffffff'
    })
    target.style.height = savedTargetStyle.height
    target.style.maxHeight = savedTargetStyle.maxHeight
    target.style.overflow = savedTargetStyle.overflow
    target.style.overflowY = savedTargetStyle.overflowY
    savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
      el.style.overflow = overflow
      el.style.overflowY = overflowY
      el.style.height = height
      el.style.maxHeight = maxHeight
    })
    const dataUrl = canvas.toDataURL('image/png')
    const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
    const result = await SaveImage(`go-stock-kbqa-${safeTime}`, base64)
    if (result && !result.includes('异常') && !result.includes('无法')) {
      message.success('已导出图片：' + result)
    } else {
      message.info(result || '导出取消')
    }
  } catch (e) {
    message.error('导出图片失败: ' + (e?.message ?? e))
  } finally {
    kbqaExportLoading.value = false
  }
}

// AI 回答流式输出时自动滚动到最新内容
// - 仅在回答中(kbqaAnswering=true)时自动滚，避免用户手动上滚查看时被强制拉回
// - MdPreview 内部渲染为异步，nextTick 后再延迟一帧确保 DOM 已更新
watch(kbqaAnswer, () => {
  if (!kbqaAnswering.value) return
  nextTick(() => {
    const el = kbqaAnswerEndRef.value
    if (el && el.scrollIntoView) {
      el.scrollIntoView({ behavior: 'smooth', block: 'end' })
    }
  })
})
// 思考过程流式输出也跟随滚动
watch(kbqaReasoning, () => {
  if (!kbqaAnswering.value) return
  nextTick(() => {
    const el = kbqaAnswerEndRef.value
    if (el && el.scrollIntoView) {
      el.scrollIntoView({ behavior: 'smooth', block: 'end' })
    }
  })
})

// ============ 知识库列表 ============
const kbList = ref([])
const loading = ref(false)
const searchKeyword = ref('')

const filteredKBList = computed(() => {
  const kw = searchKeyword.value.trim().toLowerCase()
  if (!kw) return kbList.value
  return kbList.value.filter(kb =>
    (kb.name || '').toLowerCase().includes(kw) ||
    (kb.description || '').toLowerCase().includes(kw)
  )
})

async function loadKBList() {
  loading.value = true
  try {
    const list = await ListKnowledgeBases()
    kbList.value = list || []
  } catch (e) {
    message.error(`加载知识库列表失败: ${e}`)
  } finally {
    loading.value = false
  }
}

// ============ 创建知识库 ============
const showCreateModal = ref(false)
const submitting = ref(false)
const createFormRef = ref(null)
const createForm = reactive({
  name: '',
  description: '',
  aiConfigID: null,
  embeddingModel: ''
})
// 已配置向量模型的 AI 服务列表（原始数据，仅含 EmbeddingModel 非空的服务）
const embeddingServiceRawList = ref([])
const loadingAIServices = ref(false)
const createRules = {
  name: {
    required: true,
    message: '请输入知识库名称',
    trigger: ['blur', 'input']
  },
  aiConfigID: {
    required: true,
    type: 'number',
    message: '请选择向量模型',
    trigger: ['change', 'blur']
  }
}

// 向量模型下拉选项：仅显示设置中 ModelType="embedding" 的 AI 服务
// 每个选项 = 一个向量模型服务，label 形如 "服务名 [模型名]"
const embeddingServiceOptions = computed(() => {
  return embeddingServiceRawList.value.map(svc => ({
    label: `${svc.name}  [${svc.modelName}]`,
    value: svc.id
  }))
})

async function loadAIServices() {
  loadingAIServices.value = true
  try {
    const list = await ListAIServicesForKB()
    // 仅保留 ModelType="embedding" 的服务（专门的向量模型服务）
    embeddingServiceRawList.value = (list || []).filter(svc => svc.modelType === 'embedding')
  } catch (e) {
    embeddingServiceRawList.value = []
  } finally {
    loadingAIServices.value = false
  }
}

// 选择向量模型服务后，modelName 即向量模型名，传给后端
function onEmbeddingServiceChange(id) {
  const svc = embeddingServiceRawList.value.find(s => s.id === id)
  createForm.embeddingModel = svc ? svc.modelName : ''
}

function openCreateModal() {
  resetCreateForm()
  loadAIServices()
  showCreateModal.value = true
}

function resetCreateForm() {
  createForm.name = ''
  createForm.description = ''
  createForm.aiConfigID = null
  createForm.embeddingModel = ''
}

async function handleCreate() {
  try {
    await createFormRef.value?.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    await CreateKnowledgeBase(
      createForm.name.trim(),
      createForm.description.trim(),
      createForm.aiConfigID,
      createForm.embeddingModel
    )
    message.success('知识库创建成功')
    showCreateModal.value = false
    await loadKBList()
  } catch (e) {
    message.error(`创建失败: ${e}`)
  } finally {
    submitting.value = false
  }
}

async function handleDeleteKB(kb) {
  try {
    await DeleteKnowledgeBase(kb.name)
    message.success(`知识库 "${kb.name}" 已删除`)
    await loadKBList()
  } catch (e) {
    message.error(`删除失败: ${e}`)
  }
}

// ============ 文档管理抽屉 ============
const showDocDrawer = ref(false)
const currentKB = ref(null)
const docList = ref([])
const docLoading = ref(false)
const docTotal = ref(0)
// 文档列表分页（后台分页，仅加载当前页数据）
const docPagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50, 100],
  prefix: () => `共 ${docTotal.value} 条`,
  onChange: (page) => { docPagination.page = page; loadDocList() },
  onUpdatePageSize: (ps) => { docPagination.pageSize = ps; docPagination.page = 1; loadDocList() }
})
const drawerWidth = computed(() => Math.min(720, window.innerWidth - 80))

async function openDocDrawer(kb) {
  currentKB.value = kb
  showDocDrawer.value = true
  // 重置检索与上传状态
  searchQuery.value = ''
  searchResults.value = []
  searchedOnce.value = false
  uploadFileList.value = []
  inlineText.value = ''
  inlineSource.value = ''
  docPagination.page = 1
  await loadDocList()
}

async function loadDocList() {
  if (!currentKB.value) return
  docLoading.value = true
  try {
    const res = await ListKBDocumentsPaged(currentKB.value.name, docPagination.page, docPagination.pageSize)
    docList.value = (res && res.items) || []
    docTotal.value = (res && res.total) || 0
    docPagination.itemCount = docTotal.value
  } catch (e) {
    message.error(`加载文档列表失败: ${e}`)
    docList.value = []
    docTotal.value = 0
    docPagination.itemCount = 0
  } finally {
    docLoading.value = false
  }
}

async function handleDeleteDoc(doc) {
  try {
    await DeleteKBDocument(currentKB.value.name, doc.id)
    message.success('文档已删除')
    // 删除后若当前页已无数据且不是第1页，回退到前一页
    if (docList.value.length <= 1 && docPagination.page > 1) {
      docPagination.page--
    }
    await Promise.all([loadDocList(), loadKBList()])
  } catch (e) {
    message.error(`删除失败: ${e}`)
  }
}

// ============ 检索测试 ============
const searchQuery = ref('')
const searchTopK = ref(5)
const searchResults = ref([])
const searching = ref(false)
const searchedOnce = ref(false)

async function handleSearch() {
  if (!currentKB.value || !searchQuery.value.trim()) return
  searching.value = true
  searchedOnce.value = false
  try {
    const results = await SearchKnowledgeBase(
      currentKB.value.name,
      searchQuery.value.trim(),
      searchTopK.value
    )
    searchResults.value = results || []
  } catch (e) {
    message.error(`检索失败: ${e}`)
    searchResults.value = []
  } finally {
    searching.value = false
    searchedOnce.value = true
  }
}

// ============ 文件上传（支持多选批量导入） ============
const uploadFileList = ref([]) // 已选文件 {path, name}
const picking = ref(false)
const uploading = ref(false)

// 当前抽屉所打开 KB 的向量化状态（从全局实时状态读取，支持关抽屉后后台继续）
const currentKBVectorizingState = computed(() => {
  if (!currentKB.value) return null
  return vectorizingStatuses.value[currentKB.value.name] || null
})

// 多选文件：累加到 uploadFileList（允许重复选择追加，便于分批挑选）
async function handlePickFiles() {
  picking.value = true
  try {
    const paths = await PickKBFilePaths()
    if (paths && paths.length > 0) {
      for (const p of paths) {
        // 去重：避免重复添加同一文件
        if (!uploadFileList.value.some(f => f.path === p)) {
          uploadFileList.value.push({
            path: p,
            name: p.split(/[\\/]/).pop() || p
          })
        }
      }
      // 选择新文件时不清除上次向量化状态（用户可继续查看历史结果）
    }
  } catch (e) {
    message.error(`选择文件失败: ${e}`)
  } finally {
    picking.value = false
  }
}

function removeUploadFile(idx) {
  uploadFileList.value.splice(idx, 1)
}

async function handleUploadFiles() {
  if (uploadFileList.value.length === 0) return
  const kbName = currentKB.value.name
  const paths = uploadFileList.value.map(f => f.path)
  const fileCount = paths.length
  try {
    // 异步启动后台导入，立即返回
    await UploadKBFiles(kbName, paths)
    message.success(`已开始后台向量化 ${fileCount} 个文件，可关闭抽屉，列表将显示进度`)
    // 清空已选列表，启动轮询（后端会重置该 KB 的向量化状态）
    uploadFileList.value = []
    startVectorizingPoll()
  } catch (e) {
    message.error(`启动批量导入失败: ${e}`)
  }
}

// 批量导入结果表格列
const batchResultColumns = [
  {
    title: '',
    key: 'success',
    width: 40,
    align: 'center',
    render: (row) => h(NIcon, {
      color: row.success ? '#18a058' : '#d03050',
      size: 18
    }, {
      default: () => h(row.success ? CheckmarkCircleSharp : CloseCircleSharp)
    })
  },
  {
    title: '文件名',
    key: 'fileName',
    ellipsis: { tooltip: true }
  },
  {
    title: '切片数',
    key: 'chunkCount',
    width: 80,
    align: 'center'
  },
  {
    title: '失败原因',
    key: 'error',
    ellipsis: { tooltip: true },
    render: (row) => row.error
      ? h(NText, { type: 'error', style: 'font-size: 12px' }, { default: () => row.error })
      : h(NText, { depth: 3 }, { default: () => '-' })
  }
]

// ============ 内联文本入库 ============
const inlineText = ref('')
const inlineSource = ref('')

async function handleAddInline() {
  if (!inlineText.value.trim()) return
  uploading.value = true
  try {
    const ids = await AddKBDocument(
      currentKB.value.name,
      inlineText.value,
      inlineSource.value.trim() || '手动录入'
    )
    message.success(`文本入库成功，新增 ${ids.length} 个文档片段`)
    inlineText.value = ''
    inlineSource.value = ''
    await Promise.all([loadDocList(), loadKBList()])
  } catch (e) {
    message.error(`入库失败: ${e}`)
  } finally {
    uploading.value = false
  }
}

// ============ 表格列定义 ============
const kbColumns = [
  {
    title: '名称',
    key: 'name',
    width: 180,
    render: (row) => h(NText, { strong: true }, { default: () => row.name })
  },
  {
    title: '描述',
    key: 'description',
    ellipsis: { tooltip: true },
    render: (row) => row.description || h(NText, { depth: 3 }, { default: () => '未填写' })
  },
  {
    title: '文档数',
    key: 'documentCount',
    width: 120,
    align: 'center',
    render: (row) => {
      const st = getKBVectorizingState(row.name)
      const countTag = h(NTag, {
        type: row.documentCount > 0 ? 'success' : 'default',
        size: 'small',
        bordered: false
      }, { default: () => row.documentCount })
      // 向量化进行中：显示进度徽标
      if (st && st.isVectorizing) {
        return h(NSpace, { size: 4, vertical: false, align: 'center', wrap: false, justify: 'center' }, {
          default: () => [
            countTag,
            h(NTag, {
              type: 'warning',
              size: 'small',
              bordered: false,
              round: true
            }, { default: () => `向量化中 ${st.processedFiles}/${st.totalFiles}` })
          ]
        })
      }
      return countTag
    }
  },
  {
    title: '向量配置',
    key: 'embedding',
    width: 200,
    ellipsis: { tooltip: true },
    render: (row) => {
      const svc = row.aiConfigName || '默认'
      const model = row.embeddingModel || '默认'
      return h(NSpace, { size: 4, vertical: false, wrap: false, align: 'center' }, {
        default: () => [
          h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => svc }),
          h(NText, { depth: 3, style: 'font-size: 12px' }, { default: () => model })
        ]
      })
    }
  },
  {
    title: '创建时间',
    key: 'createdAt',
    width: 170,
    render: (row) => formatTime(row.createdAt)
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    fixed: 'right',
    render: (row) => h(NSpace, { size: 8 }, {
      default: () => [
        h(NButton, {
          size: 'small',
          type: 'primary',
          tertiary: true,
          onClick: () => openDocDrawer(row)
        }, {
          default: () => '管理文档',
          icon: () => h(NIcon, null, { default: () => h(FolderOpenOutline) })
        }),
        h(NPopconfirm, {
          onPositiveClick: () => handleDeleteKB(row)
        }, {
          default: () => `确认删除知识库 "${row.name}"？所有文档将被清除`,
          trigger: () => h(NButton, {
            size: 'small',
            type: 'error',
            tertiary: true
          }, {
            default: () => '删除',
            icon: () => h(NIcon, null, { default: () => h(TrashOutline) })
          })
        })
      ]
    })
  }
]

const docColumns = [
  {
    title: '来源',
    key: 'source',
    width: 160,
    ellipsis: { tooltip: true },
    render: (row) => row.source || '未知'
  },
  {
    title: '切片',
    key: 'chunkIndex',
    width: 80,
    align: 'center',
    render: (row) => `${row.chunkIndex + 1}/${row.totalChunks}`
  },
  {
    title: '内容预览',
    key: 'contentPreview',
    ellipsis: { tooltip: true },
    render: (row) => row.contentPreview || h(NText, { depth: 3 }, { default: () => '（无预览）' })
  },
  {
    title: '入库时间',
    key: 'createdAt',
    width: 160,
    render: (row) => row.createdAt
  },
  {
    title: '操作',
    key: 'actions',
    width: 80,
    fixed: 'right',
    render: (row) => h(NPopconfirm, {
      onPositiveClick: () => handleDeleteDoc(row)
    }, {
      default: () => '确认删除该文档片段？',
      trigger: () => h(NButton, {
        size: 'small',
        type: 'error',
        tertiary: true
      }, {
        default: () => '删除',
        icon: () => h(NIcon, null, { default: () => h(TrashOutline) })
      })
    })
  }
]

// ============ 工具函数 ============
function formatTime(t) {
  if (!t) return '-'
  // Go time.Time 序列化为 RFC3339 字符串
  if (typeof t === 'string') {
    // 2026-08-09T15:30:00+08:00 → 2026-08-09 15:30:00
    return t.replace('T', ' ').replace(/\+.*$/, '').replace(/Z$/, '')
  }
  // 兜底：可能是 Date 对象
  try {
    return new Date(t).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return String(t)
  }
}

// ============ 长期记忆向量服务配置 ============
const ltmAiConfigId = ref(0)
const ltmServiceOptions = ref([])
const loadingLtmServices = ref(false)

async function loadLtmConfig() {
  loadingLtmServices.value = true
  try {
    const [currentId, list] = await Promise.all([
      GetLongTermMemoryAiConfigId(),
      ListAIServicesForKB()
    ])
    ltmAiConfigId.value = currentId || 0
    // 选项：自动 + 所有 ModelType=embedding 的服务
    const opts = [{ label: '自动（优先向量模型类型服务）', value: 0 }]
    for (const svc of (list || []).filter(s => s.modelType === 'embedding')) {
      opts.push({ label: `${svc.name}  [${svc.modelName}]`, value: svc.id })
    }
    ltmServiceOptions.value = opts
  } catch (e) {
    ltmServiceOptions.value = [{ label: '自动（优先向量模型类型服务）', value: 0 }]
  } finally {
    loadingLtmServices.value = false
  }
}

async function onLtmServiceChange(id) {
  try {
    await SetLongTermMemoryAiConfigId(id || 0)
    message.success(id > 0 ? '长期记忆向量服务已指定' : '已切换为自动模式')
  } catch (e) {
    message.error(`保存失败: ${e}`)
    // 回滚：重新读取实际值
    ltmAiConfigId.value = await GetLongTermMemoryAiConfigId()
  }
}

// ============ 向量化状态轮询 ============
// 记录每个 KB 的向量化状态（name → status），用于列表显示进度徽标
const vectorizingStatuses = ref({})
let vectorizingPollTimer = null
// 记录已通知完成的 KB（避免重复 toast）
const notifiedFinishedKBs = new Set()

async function loadVectorizingStatuses() {
  try {
    const statuses = await GetAllKBVectorizingStatuses()
    vectorizingStatuses.value = statuses || {}
    // 检测刚完成的 KB：之前在轮询（有状态）且现在非进行中
    const anyActive = Object.values(statuses || {}).some(s => s.isVectorizing)
    // 对刚完成的 KB 弹通知
    for (const [name, st] of Object.entries(statuses || {})) {
      if (!st.isVectorizing && st.finishedAt && !notifiedFinishedKBs.has(name)) {
        notifiedFinishedKBs.add(name)
        if (st.error) {
          message.error(`知识库 "${name}" 向量化失败: ${st.error}`)
        } else if (st.failedCount === 0) {
          message.success(`知识库 "${name}" 向量化完成：${st.successCount} 个文件，${st.totalChunks} 个切片`)
        } else {
          message.warning(`知识库 "${name}" 向量化完成：成功 ${st.successCount}，失败 ${st.failedCount}`)
        }
        // 完成后刷新 KB 列表（文档数变化）
        loadKBList()
        // 若当前抽屉打开的正是该 KB，同步刷新文档列表与 KB 概览
        if (currentKB.value && currentKB.value.name === name && showDocDrawer.value) {
          loadDocList()
          // 刷新 currentKB 以更新 documentCount
          GetKnowledgeBase(name).then(info => { if (info) currentKB.value = info }).catch(() => {})
        }
      }
    }
    // 无进行中任务时停止轮询
    if (!anyActive && vectorizingPollTimer) {
      clearInterval(vectorizingPollTimer)
      vectorizingPollTimer = null
    }
  } catch (e) {
    // 静默失败，不打断用户
  }
}

function startVectorizingPoll() {
  // 立即查一次
  loadVectorizingStatuses()
  // 避免重复启动
  if (vectorizingPollTimer) return
  vectorizingPollTimer = setInterval(loadVectorizingStatuses, 2000)
}

// 获取某 KB 的向量化状态（列表列渲染用）
function getKBVectorizingState(name) {
  return vectorizingStatuses.value[name] || null
}

onMounted(() => {
  loadKBList()
  loadLtmConfig()
  GetConfig().then(result => {
    if (result && result.darkTheme) {
      mdTheme.value = 'dark'
    }
  }).catch(() => {})
  // 启动时检查是否有进行中的向量化任务（如刷新页面后）
  loadVectorizingStatuses().then(() => {
    const anyActive = Object.values(vectorizingStatuses.value).some(s => s.isVectorizing)
    if (anyActive) startVectorizingPoll()
  })
  EventsOn(KBQA_EVENT, handleKBQAStreamMessage)
})

onUnmounted(() => {
  if (vectorizingPollTimer) {
    clearInterval(vectorizingPollTimer)
    vectorizingPollTimer = null
  }
  EventsOff(KBQA_EVENT)
})
</script>

<style scoped>
/* MdPreview 预览区左对齐（与 skill-manager 一致） */
:deep(.md-editor) {
  text-align: left;
}

/* 内容区滚动容器：隐藏滚动条（参考 Home 题材组件 .thin-scroll） */
:deep(.thin-scroll) {
  scrollbar-width: none;
}
:deep(.thin-scroll::-webkit-scrollbar) {
  display: none;
}

/* 左右收缩：折叠侧的窄条样式 */
.kbqa-collapse-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  gap: 8px;
  padding: 10px 4px;
  width: 36px;
  height: 100%;
  min-height: 120px;
  cursor: pointer;
  user-select: none;
  border: 1px solid var(--n-border-color, #e0e0e6);
  border-radius: 6px;
  background: var(--n-color, #fff);
  transition: background 0.2s;
}
.kbqa-collapse-bar:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.03));
}
/* 竖排标题文字 */
.kbqa-vertical-text {
  writing-mode: vertical-rl;
  text-orientation: mixed;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.4;
  white-space: nowrap;
}
</style>
