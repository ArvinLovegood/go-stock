# go-stock Code Wiki

## 1. 项目概述

### 1.1 项目简介

**go-stock** 是一款基于 **Wails** 和 **Vue.js** 开发的AI赋能股票分析工具。该工具结合大语言模型能力，为用户提供智能化的股票分析服务，支持A股、港股、美股市场数据查询与分析。

### 1.2 核心功能

| 功能模块 | 描述 | 状态 |
|---------|------|------|
| 股票实时行情 | 获取A股、港股、美股实时价格数据 | ✅ |
| AI股票分析 | 基于大语言模型的股票分析报告生成 | ✅ |
| 智能选股 | 技术指标筛选、条件选股 | ✅ |
| 基金数据 | 基金净值、估值、持仓查询 | ✅ |
| 市场资讯 | 实时新闻推送、公告查询 | ✅ |
| 价格预警 | 自定义价格预警线设置 | ✅ |
| 龙虎榜 | 营业部交易数据 | ✅ |
| 全球指数 | 全球主要市场指数概览 | ✅ |
| 宏观经济 | GDP、CPI、PPI、PMI等宏观数据 | ✅ |
| 投资日历 | 财报发布、股东大会、IPO等事件 | ✅ |

### 1.3 支持的大模型平台

- OpenAI / 兼容OpenAI格式的模型
- Ollama（本地大模型）
- LMStudio（本地大模型）
- DeepSeek
- 火山方舟（VolcArk）
- 阿里云通义千问（DashScope）
- Gemini
- OpenRouter
- Claude

---

## 2. 项目架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        go-stock 应用                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐     ┌──────────────────────────────┐    │
│  │    Frontend      │     │           Backend            │    │
│  │   (Vue.js)       │◄───►│                              │    │
│  │  ┌────────────┐  │     │  ┌───────────────────────┐   │    │
│  │  │  UI组件层   │  │     │  │       Agent层         │   │    │
│  │  └────────────┘  │     │  │  - AI代理管理         │   │    │
│  │  ┌────────────┐  │     │  │  - 工具调用          │   │    │
│  │  │  路由层     │  │     │  │  - 聊天模型工厂      │   │    │
│  │  └────────────┘  │     │  └───────────────────────┘   │    │
│  │  ┌────────────┐  │     │  ┌───────────────────────┐   │    │
│  │  │  API调用层  │  │     │  │       Data层          │   │    │
│  │  └────────────┘  │     │  │  - 股票数据API       │   │    │
│  └──────────────────┘     │  │  - 基金数据API       │   │    │
│                           │  │  - 资讯数据API       │   │    │
│                           │  │  - 爬虫模块          │   │    │
│                           │  │  - AI工具函数        │   │    │
│                           │  └───────────────────────┘   │    │
│                           │  ┌───────────────────────┐   │    │
│                           │  │       DB层           │   │    │
│                           │  │  - SQLite ORM        │   │    │
│                           │  │  - 数据迁移          │   │    │
│                           │  └───────────────────────┘   │    │
│                           │  ┌───────────────────────┐   │    │
│                           │  │      Models层         │   │    │
│                           │  │  - 数据结构体定义     │   │    │
│                           │  └───────────────────────┘   │    │
│                           └──────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│                      外部依赖                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────────┐  │
│  │东方财富API│ │新浪财经API│ │同花顺API │ │大语言模型API     │  │
│  │腾讯财经API│ │财联社API │ │雪球API   │ │Tushare数据源    │  │
│  └──────────┘ └──────────┘ └──────────┘ └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 模块职责说明

| 模块 | 职责 | 文件路径 |
|------|------|----------|
| **Agent层** | AI代理管理、工具调用、聊天模型工厂 | `backend/agent/` |
| **Data层** | 股票/基金数据获取、爬虫、工具函数 | `backend/data/` |
| **DB层** | 数据库连接、ORM操作、数据迁移 | `backend/db/` |
| **Models层** | 数据结构体定义、数据库模型 | `backend/models/` |
| **Logger层** | 日志记录 | `backend/logger/` |
| **MachineID层** | 机器唯一标识生成 | `backend/machineid/` |
| **Frontend** | 前端界面、用户交互 | `frontend/` |
| **AI-Assistant-Web** | 独立的AI助手Web服务 | `ai-assistant-web/` |

---

## 3. 核心模块详解

### 3.1 主应用入口

#### 3.1.1 main.go

主程序入口，负责：
- 初始化应用环境
- 设置数据库连接
- 启动Wails应用
- 注册定时任务
- 初始化股票基础数据

**关键函数：**

| 函数名 | 功能说明 |
|--------|----------|
| `main()` | 程序入口，初始化应用 |
| `AutoMigrate()` | 自动迁移数据库表结构 |
| `initStockData()` | 初始化A股基础数据 |
| `initStockDataHK()` | 初始化港股基础数据 |
| `initStockDataUS()` | 初始化美股基础数据 |
| `checkDir()` | 检查并创建必要目录 |
| `cacheCookies()` | 预缓存东财Cookie |

**文件位置:** [main.go](file:///workspace/main.go)

#### 3.1.2 app.go

主应用结构体，封装业务逻辑：
- 股票监控定时任务
- 价格预警机制
- 消息推送
- 自动更新检查
- 赞助码验证

**核心结构体：**

```go
type App struct {
    ctx                context.Context
    cache              *freecache.Cache
    cron               *cron.Cron
    cronEntrys         map[string]cron.EntryID
    AiTools            []data.Tool
    stockAlertMu       sync.Mutex
    stockAlertLastSent map[string]time.Time
    priceAtAlertReset  map[string]float64
}
```

**关键方法：**

| 方法名 | 功能说明 |
|--------|----------|
| `domReady()` | DOM加载完成后的初始化 |
| `NewsPush()` | 新闻推送 |
| `MonitorStockPrices()` | 监控股票价格 |
| `MonitorAiRecommendStockPrices()` | 监控AI推荐股票价格 |
| `MonitorFollowedStockCostPrices()` | 监控自选股成本价 |
| `MonitorFundPrices()` | 监控基金价格 |
| `CheckUpdate()` | 检查应用更新 |
| `CheckSponsorCode()` | 验证赞助码 |

**文件位置:** [app.go](file:///workspace/app.go)

---

### 3.2 AI代理模块 (backend/agent/)

#### 3.2.1 agent.go

AI代理核心模块，实现两种代理模式：

**代理模式：**

| 模式 | 描述 | 适用场景 |
|------|------|----------|
| **React模式** | 单步推理，直接调用工具 | 简单查询、实时数据获取 |
| **PlanExecute模式** | 多步规划执行 | 复杂分析、深度研究 |

**复杂度分类逻辑：**

```go
func classifyComplexity(question string) AgentMode {
    // 简单问题模式：包含关键词如"今天"、"当前"、"最新"、"查询"等，且字数<30
    // 复杂问题模式：包含关键词如"全面分析"、"深度分析"、"投资建议"等
    // 工具组数量>=4或字数>80也判定为复杂问题
}
```

**核心函数：**

| 函数名 | 功能说明 |
|--------|----------|
| `GetStockAiAgent()` | 获取AI代理实例 |
| `classifyComplexity()` | 根据问题复杂度选择代理模式 |
| `createReactAgent()` | 创建React模式代理 |
| `createPlanExecuteAgent()` | 创建PlanExecute模式代理 |
| `getToolsByQuestion()` | 根据问题动态筛选工具 |
| `buildSkillPrompt()` | 构建技能提示词 |
| `errorRecoveryMiddleware()` | 工具调用错误恢复中间件 |

**文件位置:** [backend/agent/agent.go](file:///workspace/backend/agent/agent.go)

#### 3.2.2 chat_model_factory.go

聊天模型工厂，支持多种大模型平台的自动识别和创建：

**支持的模型提供商：**

| 提供商 | 识别方式 | 配置方式 |
|--------|----------|----------|
| OpenAI兼容 | 默认回退 | base_url配置 |
| 火山方舟 | URL包含volces.com | API Key |
| 通义千问 | URL包含dashscope | API Key |
| OpenRouter | URL包含openrouter | API Key |
| Anthropic | URL包含anthropic | API Key |
| Ollama | URL包含ollama或端口11434 | 本地端口 |
| Gemini | URL包含googleapis或模型名 | API Key |
| DeepSeek | URL包含deepseek或模型名 | API Key |

**核心函数：**

| 函数名 | 功能说明 |
|--------|----------|
| `createChatModel()` | 根据配置创建聊天模型 |
| `detectChatModelProvider()` | 自动检测模型提供商 |
| `buildProxyHTTPClient()` | 构建带代理的HTTP客户端 |

**文件位置:** [backend/agent/chat_model_factory.go](file:///workspace/backend/agent/chat_model_factory.go)

#### 3.2.3 agent/tools/ - AI工具集

AI工具定义，供大模型调用。工具按功能分类如下：

**股票数据工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `QueryStockCodeInfo` | 查询股票基本信息 |
| `GetStockInfo` | 获取股票详细信息 |
| `GetStockKLine` | 获取日K线数据 |
| `GetEastMoneyKLine` | 获取多周期K线数据 |
| `GetEastMoneyKLineWithMA` | 获取带均线的K线数据 |
| `GetStockMinuteData` | 获取分时数据 |
| `GetStockFinancialInfo` | 获取财务报表 |
| `GetStockHolderNum` | 获取股东人数 |
| `GetStockRZRQInfo` | 获取融资融券信息 |
| `GetStockHistoryMoneyData` | 获取历史资金流向 |
| `GetStockConceptInfo` | 获取股票所属概念 |

**选股与筛选工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `SearchStockByIndicators` | 根据指标筛选股票 |
| `FilterStocks` | 根据技术指标筛选股票 |
| `SelectAStock` | A股智能选股(i问财) |
| `HotStrategyTable` | 获取热门选股策略 |
| `HotStockTable` | 当前热门股票排名 |

**市场数据工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `GetMarketData` | 获取市场行情数据 |
| `GlobalStockIndexesReadable` | 获取全球主要指数概览 |
| `GetStockMoneyData` | 今日股票资金流入排名 |
| `GetMutualTop10Deal` | 获取北向/南向资金十大成交股 |
| `GetLongTigerList` | 获取龙虎榜数据 |
| `GetIndustryMoneyRank` | 获取行业资金流向排名 |

**资讯与新闻工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `QueryStockNews` | 搜索股票相关新闻 |
| `GetNewsListData` | 获取新闻资讯 |
| `GetStockNotice` | 获取上市公司公告 |
| `InteractiveAnswer` | 获取投资者互动问答 |
| `GetSecuritiesCompanyOpinion` | 获取券商观点 |

**基金工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `SearchFund` | 搜索基金信息 |
| `GetFundInfo` | 获取基金详细信息 |
| `GetFundKLine` | 获取基金K线数据 |
| `GetFundHistoryNetValue` | 获取基金历史净值 |
| `GetFundTop10Holdings` | 获取基金十大重仓股 |

**宏观经济工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `GetEconomicData` | 获取宏观经济数据 |
| `GetInvestCalendar` | 获取投资日历 |
| `QueryMacro` | 宏观数据查询(i问财) |

**AI分析工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `CreateAiRecommendStocks` | 创建AI推荐股票记录 |
| `BatchCreateAiRecommendStocks` | 批量创建AI推荐股票 |
| `AiRecommendStocks` | 获取AI推荐股票列表 |
| `GetAIAnalysisHistory` | 查询历史AI分析报告 |
| `GetAIAnalysisDetail` | 获取分析报告详情 |

**其他工具：**

| 工具名 | 功能说明 |
|--------|----------|
| `GetCurrentTime` | 获取当前时间 |
| `SendToDingDing` | 发送钉钉通知 |
| `SetTradingPrice` | 设置交易预警价位 |
| `GetFollowedStocks` | 获取自选股列表 |

**文件位置:** [backend/agent/tools/](file:///workspace/backend/agent/tools/)

---

### 3.3 数据模块 (backend/data/)

#### 3.3.1 stock_data_api.go

股票数据API，提供实时行情和基本信息获取：

**核心结构体：**

```go
type StockInfo struct {
    Code     string  // 股票代码
    Name     string  // 股票名称
    Price    string  // 当前价格
    Open     string  // 开盘价
    High     string  // 最高价
    Low      string  // 最低价
    Volume   string  // 成交量
    Amount   string  // 成交额
    PreClose string  // 昨日收盘价
    // 买卖五档数据...
}

type FollowedStock struct {
    StockCode   string
    Name        string
    CostPrice   float64
    Volume      int64
    AlarmPrice  float64
    // ...
}
```

**关键方法：**

| 方法名 | 功能说明 |
|--------|----------|
| `GetStockCodeRealTimeData()` | 获取实时行情数据 |
| `Follow()` | 关注股票 |
| `UnFollow()` | 取消关注 |
| `SetCostPriceAndVolume()` | 设置成本价和持仓数量 |
| `GetFollowList()` | 获取关注列表 |
| `GetStockList()` | 搜索股票列表 |

**数据来源：**
- A股/港股：腾讯财经API (`qt.gtimg.cn`)
- 美股：新浪财经API (`hq.sinajs.cn`)

**文件位置:** [backend/data/stock_data_api.go](file:///workspace/backend/data/stock_data_api.go)

#### 3.3.2 eastmoney_kline_api.go

东方财富K线数据API：

**支持的K线类型：**
- 日K、周K、月K、季K、年K
- 1分钟、5分钟、15分钟、30分钟、60分钟

**文件位置:** [backend/data/eastmoney_kline_api.go](file:///workspace/backend/data/eastmoney_kline_api.go)

#### 3.3.3 openai_api.go

AI配置封装：

```go
type OpenAi struct {
    BaseUrl          string  // API基础URL
    ApiKey           string  // API密钥
    Model            string  // 模型名称
    MaxTokens        int     // 最大Token数
    Temperature      float64 // 温度参数
    TimeOut          int     // 超时时间(秒)
    Prompt           string  // 提示词模板
    CrawlTimeOut     int64   // 爬虫超时时间
    KDays            int64   // K线天数
}
```

**文件位置:** [backend/data/openai_api.go](file:///workspace/backend/data/openai_api.go)

#### 3.3.4 tools.go

AI工具定义注册中心，包含约60+个工具函数定义。

**文件位置:** [backend/data/tools.go](file:///workspace/backend/data/tools.go)

---

### 3.4 数据库模块 (backend/db/)

#### 3.4.1 db.go

数据库连接管理：

```go
func Init(sqlitePath string) {
    // SQLite配置
    // - busy_timeout: 10秒
    // - journal_mode: WAL模式
    // - synchronous: NORMAL
    // - cache_size: 512KB
}
```

**数据库优化策略：**
- WAL模式提升并发写入性能
- 限制连接数避免锁竞争（最大5个连接）
- 自动迁移数据表
- 禁用外键约束以提高写入性能
- 启用预编译语句缓存

**文件位置:** [backend/db/db.go](file:///workspace/backend/db/db.go)

---

### 3.5 模型定义 (backend/models/)

#### 3.5.1 models.go

核心数据模型：

| 模型 | 用途 |
|------|------|
| `StockBasic` | 股票基础信息 |
| `StockInfoHK` | 港股基础信息 |
| `StockInfoUS` | 美股基础信息 |
| `FollowedStock` | 关注的股票 |
| `AiRecommendStocks` | AI推荐股票记录 |
| `AIResponseResult` | AI分析结果 |
| `Telegraph` | 财联社电报 |
| `PromptTemplate` | 提示词模板 |
| `CronTask` | 定时任务 |
| `MCPServer` | MCP服务器配置 |
| `Skill` | 自定义技能 |
| `TradingRecord` | 交易记录 |
| `LongTigerRankData` | 龙虎榜数据 |

**文件位置:** [backend/models/models.go](file:///workspace/backend/models/models.go)

---

## 4. 核心数据流程

### 4.1 股票实时行情获取流程

```
用户请求 → App.GetStockInfo → StockDataApi.GetStockCodeRealTimeData → 
    分类处理(A股/港股/美股) → 
        A股/港股: 调用腾讯API (qt.gtimg.cn)
        美股: 调用新浪API (hq.sinajs.cn) → 
    解析响应数据 → 
    保存到数据库 → 
    返回结果
```

### 4.2 AI分析流程

```
用户提问 → Agent.GetStockAiAgent → 
    分类问题复杂度 → 
        简单问题 → React模式代理 → 直接调用工具 → 返回结果
        复杂问题 → PlanExecute模式代理 → 规划步骤 → 执行 → 总结 → 返回结果
```

### 4.3 价格预警流程

```
定时任务触发 → MonitorStockPrices → 
    获取关注股票列表 → 
    获取实时价格 → 
    对比预警价格 → 
    触发条件满足 → 
        Windows通知 / 钉钉推送 / 消息推送
```

### 4.4 消息推送流程

```
新闻源 → syncNews() → 
    过滤筛选(外媒资讯/财联社/新浪) → 
    情感分析 → 
    保存到数据库 → 
    发送通知 → 
        前端事件推送 → 
        系统通知 → 
        钉钉推送
```

---

## 5. 技术栈

### 5.1 后端技术

| 技术 | 版本 | 用途 |
|------|------|------|
| Go | 1.26 | 主语言 |
| Wails | 2.11.0 | 桌面应用框架 |
| GORM | 1.31.1 | ORM框架 |
| SQLite | 内置 | 数据库 |
| Eino | 0.8.x | AI代理框架 |
| Resty | 2.17.2 | HTTP客户端 |
| Chromedp | 0.15.1 | 网页爬虫 |
| Robfig/Cron | 3.0.1 | 定时任务 |
| Freecache | 1.2.7 | 内存缓存 |

### 5.2 前端技术

| 技术 | 版本 | 用途 |
|------|------|------|
| Vue.js | 3.x | 前端框架 |
| NaiveUI | 最新 | UI组件库 |
| Vite | 6.x | 构建工具 |
| TypeScript | 最新 | 类型安全 |
| WailsJS | 内置 | Go-Vue通信 |

### 5.3 AI框架

| 框架 | 用途 |
|------|------|
| Eino | AI代理框架 |
| Eino-ext/model/openai | OpenAI模型支持 |
| Eino-ext/model/ollama | Ollama本地模型 |
| Eino-ext/model/gemini | Gemini模型 |
| Eino-ext/model/deepseek | DeepSeek模型 |
| Eino-ext/model/qwen | 通义千问模型 |

---

## 6. 依赖关系

### 6.1 核心依赖关系图

```
main.go
    ├── app.go (主应用逻辑)
    │       ├── backend/agent/agent.go (AI代理)
    │       │       ├── backend/agent/chat_model_factory.go (模型工厂)
    │       │       └── backend/agent/tools/ (工具集)
    │       ├── backend/data/stock_data_api.go (股票数据)
    │       ├── backend/data/openai_api.go (AI配置)
    │       └── backend/db/db.go (数据库)
    ├── backend/db/db.go (数据库初始化)
    │       └── backend/models/models.go (数据模型)
    └── ai-assistant-web/server.go (独立Web服务)
```

### 6.2 Go模块依赖

**核心依赖：**

| 包 | 版本 | 用途 |
|----|------|------|
| github.com/wailsapp/wails/v2 | 2.11.0 | 桌面应用框架 |
| gorm.io/gorm | 1.31.1 | ORM |
| github.com/cloudwego/eino | 0.8.11 | AI代理 |
| github.com/go-resty/resty/v2 | 2.17.2 | HTTP客户端 |
| github.com/chromedp/chromedp | 0.15.1 | 爬虫 |
| github.com/robfig/cron/v3 | 3.0.1 | 定时任务 |
| github.com/coocood/freecache | 1.2.7 | 缓存 |

**AI模型扩展：**

| 包 | 版本 | 用途 |
|----|------|------|
| github.com/cloudwego/eino-ext/components/model/openai | 0.1.13 | OpenAI |
| github.com/cloudwego/eino-ext/components/model/ollama | 0.1.9 | Ollama |
| github.com/cloudwego/eino-ext/components/model/gemini | 0.1.30 | Gemini |
| github.com/cloudwego/eino-ext/components/model/deepseek | 0.1.3 | DeepSeek |
| github.com/cloudwego/eino-ext/components/model/qwen | 0.1.9 | 通义千问 |

---

## 7. 运行方式

### 7.1 环境要求

- Go 1.26+
- Node.js 18+
- Wails CLI (可选，用于开发)

### 7.2 开发模式

```bash
# 安装依赖
go mod download

# 前端安装依赖
cd frontend && npm install && cd ..

# 开发模式运行
wails dev
```

### 7.3 构建

```bash
# Windows
wails build -platform windows/amd64

# macOS
wails build -platform darwin/universal

# Linux
wails build -platform linux/amd64
```

### 7.4 构建脚本

项目提供了预定义的构建脚本：

| 脚本 | 平台 | 路径 |
|------|------|------|
| build-windows.sh | Windows | scripts/build-windows.sh |
| build-macos.sh | macOS通用 | scripts/build-macos.sh |
| build-macos-arm.sh | macOS ARM | scripts/build-macos-arm.sh |
| build-macos-intel.sh | macOS Intel | scripts/build-macos-intel.sh |
| build-linux.sh | Linux | scripts/build-linux.sh |

### 7.5 目录结构

```
go-stock/
├── ai-assistant-web/      # AI助手独立Web服务
│   ├── cmd/ai-assistant-web/main.go
│   ├── frontend/          # Web前端
│   └── server.go
├── backend/               # 后端核心代码
│   ├── agent/            # AI代理模块
│   │   ├── tools/        # AI工具定义
│   │   ├── agent.go
│   │   ├── agent_api.go
│   │   └── chat_model_factory.go
│   ├── data/             # 数据API模块
│   │   ├── stock_data_api.go
│   │   ├── eastmoney_kline_api.go
│   │   ├── openai_api.go
│   │   └── tools.go
│   ├── db/               # 数据库模块
│   ├── logger/           # 日志模块
│   ├── machineid/        # 机器标识
│   └── models/           # 数据模型
├── build/                # 构建资源
├── frontend/             # 主应用前端
│   ├── src/
│   │   ├── components/   # UI组件
│   │   ├── router/       # 路由
│   │   └── App.vue
│   └── wailsjs/          # Wails通信层
├── scripts/              # 构建脚本
├── app.go                # 主应用逻辑
├── main.go               # 入口文件
└── go.mod                # Go依赖
```

---

## 8. 关键配置说明

### 8.1 AI配置

在应用设置中配置：

| 配置项 | 说明 |
|--------|------|
| Base URL | 大模型API地址 |
| API Key | 认证密钥 |
| Model Name | 模型名称 |
| Temperature | 创意程度(0-1) |
| Max Tokens | 最大响应长度 |

### 8.2 数据来源

| 数据类型 | 来源 |
|----------|------|
| 实时行情 | 腾讯财经、新浪财经 |
| K线数据 | 东方财富 |
| 财务数据 | 同花顺i问财 |
| 资讯 | 财联社、新浪财经、TradingView |
| 基金数据 | 东方财富 |

### 8.3 数据库配置

```go
// 默认配置
sqlitePath = "data/stock.db?_busy_timeout=10000&_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-524288"
```

**参数说明：**
- `_busy_timeout=10000`: 锁等待超时10秒
- `_journal_mode=WAL`: WAL日志模式
- `_synchronous=NORMAL`: 同步模式
- `_cache_size=-524288`: 缓存大小512KB

---

## 9. 扩展能力

### 9.1 MCP工具支持

支持通过MCP（Model Context Protocol）扩展工具：

```go
// 加载MCP工具
mcpTools := getMCPTools()
if len(mcpTools) > 0 {
    allTools = append(allTools, mcpTools...)
}
```

### 9.2 自定义技能

支持自定义技能扩展：

```go
type Skill struct {
    Name           string  // 技能名称
    Description    string  // 描述
    SystemPrompt   string  // 系统提示词
    TriggerKeywords string // 触发关键词
    MCPServerIDs   string  // 关联MCP服务器
}
```

### 9.3 定时任务

支持自定义定时任务：

```go
type CronTask struct {
    Name        string  // 任务名称
    CronExpr    string  // Cron表达式
    TaskType    string  // 任务类型
    Target      string  // 目标
    Params      string  // 参数(JSON)
    Enable      bool    // 是否启用
    Status      string  // 状态
    Description string  // 描述
}
```

---

## 10. 安全与合规

### 10.1 数据安全

- API Key本地存储（加密）
- 敏感信息日志脱敏
- 本地数据库加密
- 赞助码AES加密存储

### 10.2 免责声明

本工具仅供学习研究使用，不构成投资建议。投资有风险，入市需谨慎。

---

## 11. 版本历史

| 版本 | 日期 | 主要更新 |
|------|------|----------|
| v2026.05.20 | 2026-05-20 | 多周期K线图、SATS指标 |
| v2026.05.15 | 2026-05-15 | 基金K线、十大重仓持仓 |
| v2026.04.12 | 2026-04-12 | AgentMode支持React和PlanExecute |
| v2026.04.11 | 2026-04-11 | MCP工具调用支持 |
| v2026.03.10 | 2026-03-10 | AI助手功能 |
| v2026.02.20 | 2026-02-20 | 港股、美股支持 |
| v2026.01.15 | 2026-01-15 | 基金数据模块 |

---

**文档版本**: v1.0  
**生成日期**: 2026-06-04  
**项目地址**: [go-stock](https://github.com/ArvinLovegood/go-stock)