# 文档更新变更日志（2026.09）

> 本轮更新将文档对齐至 2026.09.15 的代码状态，覆盖 2026.08.26 - 2026.09.15 期间的 24 个功能提交（共 47 个提交）：AI 视觉理解、提示词模板回测、推荐回测统计、每日复盘与盘前策略独立菜单、游资动向与游资席位识别、部委政策新闻、K线六项技术指标、Agent 自我进化层、Excel 导入导出、macOS 整体更新、注册双重验证等。
>
> **涉及文件**：
> - [README.md](../README.md)
> - [docs/go-stock使用手册.md](./go-stock使用手册.md)
>
> **核对依据**：git 提交 `53e22ab`/`ac73e82`（提示词回测与推荐回测统计）、`956fd75`/`b90e149`（视觉理解）、`4917b98`（复盘策略菜单）、`785cf02`/`4e44e4d`（政策新闻）、`f0c0d2d`/`0dad4b4`/`219656f`（游资席位）、`12e1140`（K线指标）、`57134be`（自我进化层）等，及前端 [App.vue](../frontend/src/App.vue)、[PromptBacktest.vue](../frontend/src/components/PromptBacktest.vue)、[RecommendBacktestStats.vue](../frontend/src/components/RecommendBacktestStats.vue)、[DailyReview.vue](../frontend/src/components/DailyReview.vue)、[LhbHotMoneyDaily.vue](../frontend/src/components/LhbHotMoneyDaily.vue)、[PolicyNewsList.vue](../frontend/src/components/PolicyNewsList.vue) 实际实现。

---

## 一、README.md 变更

### 1. ✨ 简介更新

- AI 智能体条目补充「支持视觉理解图片对话（截图问股/K线图分析）」
- K线条目补充「成交量分布/神奇九转/自动背离」
- 资讯条目补充「部委政策新闻」与「龙虎榜与游资动向识别」
- 新增条目「支持复盘策略体系：每日复盘、盘前策略、提示词模板回测、推荐回测统计」

### 2. 🧩 重大功能开发计划新增 8 行

| 功能 | 说明 |
|------|------|
| AI 视觉理解（图片对话） | 截图问股/K线图分析，上传/粘贴/URL多方式输入，多模型自动适配（2026.09） |
| 提示词模板回测 | 历史行情重放+防未来数据机制，胜率/回撤/夏普/Jaccard稳定性等指标（2026.09） |
| 推荐回测统计 | AI推荐个股按持有期计算实际收益并与沪深300对比（2026.09） |
| 每日复盘与盘前策略 | 独立菜单，AI自动生成，交易日定时执行+飞书/钉钉推送（2026.09） |
| 游资动向与游资席位识别 | 龙虎榜席位明细+游资席位映射表（2026.08版）+游资名录维护（2026.09） |
| 部委政策新闻 | 政策新闻聚合+部门筛选，政策工具接入AI智能体（2026.09） |
| K线六项技术指标 | 成交量分布VPVR/神奇九转/自动背离/BBI/涨跌停价位线/威斯波浪（2026.08） |
| Agent 自我进化层 | 内置SOUL.md进化规则种子，任务后自动内省沉淀经验（2026.09） |

### 3. 👀 更新日志新增 9 条（2026.08.27 - 2026.09.15）

- **2026.09.15**：提示词模板回测与推荐回测统计（试验阶段）、AI推荐自动保存、AI诊断默认开启、VIP兑换码修复
- **2026.09.08**：AI 视觉理解（图片对话）、板块/概念资金流成分股 AI 工具
- **2026.09.06**：每日复盘与盘前策略独立菜单
- **2026.09.05**：Agent 自我进化层、Excel 导出、板块成分股查看、交易模板下载
- **2026.09.04**：部委政策新闻模块
- **2026.09.02**：游资动向与游资席位识别
- **2026.08.30**：macOS .app-bundle 整体更新、注册双重验证、画像纠正学习
- **2026.08.28**：K线六项技术指标
- **2026.08.27**：Markdown 转图片工具

---

## 二、用户手册变更（go-stock使用手册.md）

### 1. 新增第 8 章「复盘策略」（独立一级菜单章节）

| 小节 | 内容 |
|------|------|
| 8.1 每日复盘 | AI 生成复盘报告（市场概况/主线板块/交易复盘/龙虎榜资金/明日展望），交易日 18:00 定时生成+飞书/钉钉推送 |
| 8.2 盘前策略 | 盘前必读生成，与复盘形成「盘前预判→盘后验证」闭环 |
| 8.3 提示词回测（beta） | 回测配置、防未来数据机制、输出指标（胜率/超额胜率/夏普/Jaccard 等）、任务管理 |
| 8.4 推荐回测统计 | 5日持有期实际收益 vs 沪深300，按评级/提示词/模板/最佳模型多维统计 |

### 2. 章节编号顺延

插入第 8 章后，原第 8-16 章顺延为第 9-17 章（AI智能体→9、浮动AI助手→10、研究中心→11、设置→12、关于与赞助→13、使用技巧→14、快捷操作与常见问题→15、技术架构→16、更新日志→17），目录同步更新，手册更新日期改为 2026.09。

### 3. 第 6 章市场行情（15 → 17 个子标签页）

| 变更 | 说明 |
|------|------|
| 新增 6.2 政策新闻 | PolicyNewsList：部门筛选、关键词搜索、AI 政策工具 |
| 新增 6.11 游资动向 | LhbHotMoneyDaily：当日龙虎榜席位明细按游资/机构聚合，结果缓存 10 分钟 |
| 6.10 龙虎榜扩充 | 补充游资席位映射表（2026.08版）与「游资名录」维护抽屉说明 |
| 编号顺延 | 原 6.2-6.16 顺延为 6.3-6.18（全球股指→6.3 … 实时数据刷新机制→6.18） |
| 板块/概念资金流向 | 补充「点击名称查看成分股（主力净流入/涨跌幅/成交额排序）」 |

### 4. 其他章节更新

| 位置 | 变更 |
|------|------|
| 1.1 核心特性 | 补充视觉理解、K线扩展指标、政策新闻、游资识别、复盘策略体系、自我进化层、Excel 导入导出 |
| 3.2 导航菜单 | 市场行情 15 → 17 个子菜单；新增「📅 复盘策略」行 |
| 9.2 底部输入区 | 新增图片按钮（视觉理解）行；修正过时引用「见8.3」→「见9.3」 |
| 9.3 Agent 模式 | 「系统默认模式」标注从 plan_execute 修正为 DeepAgents（与 agent-chat.vue / FloatingAgentAssistant.vue 实际默认值一致） |
| 9.4 AI 工具调用 | 新增 GetBkFundFlowRank / GetBkConstituentStocks、政策新闻检索、游资动向查询 |
| 11.2 股票推荐记录 | 补充 AI 推荐自动保存机制说明及与推荐回测统计的关联 |
| 12.3 AI模型服务 | 配置表新增「视觉理解」开关行 |
| 17 更新日志 | 标题改为「2026.06 - 2026.09」，17.1/17.2/17.3/17.6/17.7/17.8/17.9/17.10 共补充 25 条 2026.09 条目 |

---

## 三、本轮文档对齐的关键功能（代码依据）

| 功能 | 关键文件 |
|------|----------|
| 提示词回测 | [prompt_backtest_engine.go](../backend/agent/prompt_backtest_engine.go)、[PromptBacktest.vue](../frontend/src/components/PromptBacktest.vue) |
| 推荐回测统计 | [recommend_backtest.go](../backend/agent/recommend_backtest.go)、[RecommendBacktestStats.vue](../frontend/src/components/RecommendBacktestStats.vue) |
| AI 推荐自动保存 | [auto_recommend_saver.go](../backend/agent/auto_recommend_saver.go)（工具调用主路径 + JSON 解析兜底路径双机制） |
| 每日复盘/盘前策略 | [daily_review_api.go](../backend/agent/daily_review_api.go)、[cron_task_api.go](../backend/agent/cron_task_api.go)（交易日 18:00 定时任务） |
| 游资席位识别 | [HotMoneySeatsManager.vue](../frontend/src/components/HotMoneySeatsManager.vue)、[LhbHotMoneyDaily.vue](../frontend/src/components/LhbHotMoneyDaily.vue) |
| 政策新闻 | [PolicyNewsList.vue](../frontend/src/components/PolicyNewsList.vue) |
| K线六项指标 | [volumeProfilePrimitive.js](../frontend/src/components/kline/volumeProfilePrimitive.js)、[tdSequentialPrimitive.js](../frontend/src/components/kline/tdSequentialPrimitive.js)、[divergencePrimitive.js](../frontend/src/components/kline/divergencePrimitive.js) |
| Agent 自我进化 | [soul_seed.md](../backend/agent/soul_seed.md)、[agent_self_evolution.go](../backend/agent/agent_self_evolution.go) |
| 视觉理解 | 详见 [AI对话与视觉理解功能说明.md](./AI对话与视觉理解功能说明.md)（已有独立文档，本轮仅补入口引用） |

---

*本轮文档核对基于 2026.09.15 的代码状态（提交 53e22ab），由 AI 助手逐一核对 git 历史与前后端实现后更新。*
