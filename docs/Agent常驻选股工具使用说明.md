# Agent 常驻选股工具使用说明

> 适用版本：2026.11 之后（SearchStockByIndicators 等 5 个工具转为常驻）

## 一、什么是「常驻工具」

go-stock 的 AI Agent（对话/研报/机器人）默认按**问题关键词**动态裁剪工具：只有命中
「选股 / 筛选 / ETF」等关键词时，选股分组工具才会注入模型上下文。问法一旦模糊
（如"帮我看看现在有什么机会"、"验证一下这只票的板块强度"），模型就看不到这些工具。

以下 5 个工具已登记进跨领域常驻白名单（`backend/agent/tools/tool_groups.go` 的
`alwaysVisibleTools`），**任何问题（包括闲聊）都对模型可见**，不再依赖措辞命中：

| 工具 | 作用 | 常驻理由 |
|---|---|---|
| `SearchStockByIndicators` | 自然语言问句选股（东财AI选股，问财回退） | 入口级筛选能力，个股验证/策略回溯/对比筛选随时可能用到 |
| `FilterStocks` | 结构化技术指标/人气/连涨连跌筛选（东财数据中心） | 与指标选股同属入口级过滤能力 |
| `SearchBk` | 板块/概念/指数问句查询（东财AI选股） | 分析个股所属板块强弱或轮动时随时可用 |
| `SearchETF` | ETF 问句查询（东财AI选股） | ETF 是行情/轮动/替代持仓讨论的常用载体 |
| `HotStrategyTable` | 当前热门选股策略排行榜（东财选股热榜） | 机会发现类问题常隐式依赖市场有效策略概览 |

## 一（续）、与东方财富选股器（xuangu.eastmoney.com）功能对照

| 东财选股器功能 | 本项目集成 | 说明 |
|---|---|---|
| AI 问句选股（股票） | `SearchStockByIndicators` | np-tjxg-g smart-tag v3 接口；东财 503 故障时自动回退同花顺问财（需 IwencaiApiKey） |
| AI 问句选股（板块/概念） | `SearchBk` | np-tjxg-b 接口 |
| AI 问句选股（ETF） | `SearchETF` | np-tjxg-b 接口 |
| 热门策略榜 | `HotStrategyTable` | np-ipick 热榜，无需任何 Key |
| 条件选股器（加强版） | `FilterStocks`（部分） | 仅覆盖「行情技术面 + 人气&涨停」子集；基本面/消息面/范围/概念风格等条件超出覆盖范围时改用问句选股 |
| 策略广场（同花顺回测） | 未暴露为工具 | `data.SearchStockApi.StrategySquare()` 已实现，未包装 |
| 选股圈 / 大盘星图 / 个股人气榜 | 未集成 | 人气榜场景可用 `HotStockTable`（雪球热股）替代 |

同分组的 `SelectAStock / SelectETF / HotStockTable` 等**仍按关键词动态注入**，不属于常驻。

## 二、工具详解

### 1. SearchStockByIndicators —— 自然语言问句选股

- **数据源**：东方财富 AI 选股（xuangu.eastmoney.com，需 **QgqpBId**）；东财服务故障（503）时自动回退同花顺问财（可选配 **IwencaiApiKey**）
- **问句覆盖面**（对应选股器六大页签）：
  - **范围**：市场板块（沪/深/北交所/科创板/创业板/ST）、上市时间、概念/风格（554+）、行业（496+）、地区、指数成份、企业性质
  - **基本面**：估值（市盈率/市净率/市值等）、每股指标、盈利能力（ROE/毛利率等）、成长能力、资本结构与偿债、股东股本（户数/户均持股等）、分红
  - **行情技术面**：技术指标（MACD/KDJ/RSI/CCI/BOLL/OBV 等）、经典/分时 K 线形态（早晨之星/出水芙蓉/放量突破等）、量价形态（巨量/地量/量价齐升）、资金流向（主力净流入/沪深股通/DDX/增仓占比）、换手率/量比、竞价异动、创阶段新高/新低
  - **消息面**：限售解禁、定增、重组、质押、商誉、业绩预告、机构调研/评级、龙虎榜、股东变动、监管函/风险警示、融资融券
  - **人气&涨停**：股吧人气排名（连涨/连跌/创新高）、连板/N天M板、涨停方式（一字板/回封板/地天板等）、封单强度、曾涨停、特色指标（冲涨停/核按钮等）
- **参数**：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `words` | string | 是 | 选股问句，可组合范围/基本面/行情技术面/消息面/人气涨停五大类条件；**同一时段多条件用「且」连接**，支持相对时间（今日/近3日/上周/近一月），如：`上周涨停且放量 且 市盈率小于30` |

- **调用示例**：
  - `SearchStockByIndicators(words="macd金叉 且 kdj超卖 且 市盈率小于30")`
  - `SearchStockByIndicators(words="连续3天放量上涨 流通市值大于100亿")`
- **返回**：Markdown 表格 `### 工具筛选出的相关股票数据`（列名由选股接口按条件动态返回，通常含代码/名称/现价等）
- **结果缓存**：300 秒（相同 `words` 命中缓存直接返回）

### 2. FilterStocks —— 结构化指标筛选

- **数据源**：东方财富数据中心（无需额外 Key）
- **与 SearchStockByIndicators 的分工**：参数是**结构化布尔/整数字段**（不做自然语言解析），
  适合模型把用户条件精确映射为开关；条件超出字段覆盖范围（财务指标、概念板块等）时用 SearchStockByIndicators。
- **参数**（全部可选，未传 = 不启用该条件）：

| 参数 | 类型 | 说明 |
|---|---|---|
| `keyword` | string | 股票名称/代码关键词 |
| `page` / `pageSize` | integer | 分页，默认 1 / 20 |
| 技术指标 | boolean | `macdGoldenFork`（MACD金叉）、`kdjGoldenFork`（KDJ金叉）、`breakThrough`（放量突破）、`breakUpMa5Days`（上穿5日线）、`longAvgArray`（均线多头排列）、`shortAvgArray`（均线空头排列） |
| 资金 | boolean | `lowFundsInflow`（低位资金净流入）、`highFundsOutflow`（高位资金净流出） |
| K 线形态 | boolean | `oneDayangLine`/`twoDayangLines`（大阳线）、`riseSun`（旭日东升）、`powerFulgun`（强势多方炮）、`restoreJustice`（拨云见日）、`upper8Days`（八连阳）、`upper9Days`（九连阳）、`upper4Days`（四串阳）、`down7Days`（七连阴）、`morningStar`（早晨之星）、`eveningStar`（黄昏之星）、`firstDawn`（曙光初现）、`blackCloudTops`（乌云盖顶）、`bearishEngulfing`（穿头破脚）、`reversingHammer`（倒转锤头）、`shootingStar`（射击之星）、`pregnant`（身怀六甲）、`narrowFinish`（窄幅整理）、`heavenRule`（天量法则）、`upsideVolume`（放量上攻）、`upperLargeVolume`（连涨放量）、`downNarrowVolume`（下跌无量） |
| 人气/连涨跌 | integer | `uppDays`（人气排名连涨 3/5/7 天）、`concernRank7Days`（7日关注排名 10/50/100 内）、`upNday`（连涨 3/5/8 天）、`downNday`（连跌 3/5/8/10/14 天） |

- **调用示例**：
  - `FilterStocks(macdGoldenFork=true, lowFundsInflow=true)` —— MACD 金叉 + 低位资金净流入
  - `FilterStocks(upNday=5, keyword="白酒")` —— 白酒板块连涨 5 天以上
- **返回**：股票列表（代码/名称/现价/涨幅等），空结果返回提示文案

### 3. SearchBk —— 板块/概念/指数查询

- **数据源**：东方财富 AI 选股（需 **QgqpBId**）
- **参数**：`words`（string，必填）—— 板块/概念/指数问句，多条件用「且」连接
- **调用示例**：
  - `SearchBk(words="今日资金净流入前十的板块")`
  - `SearchBk(words="半导体概念 且 今日涨幅大于1%")`
- **返回**：Markdown 表格 `### 工具筛选出的相关板块/概念数据`
- **典型场景**：个股分析中判断其所属板块当日强弱、板块轮动方向

### 4. SearchETF —— ETF 查询

- **数据源**：东方财富 AI 选股（需 **QgqpBId**）
- **参数**：`words`（string，必填）—— ETF 问句，多条件用「且」连接
- **调用示例**：
  - `SearchETF(words="规模最大的科创50ETF")`
  - `SearchETF(words="近一月涨幅超过10%的行业ETF")`
- **返回**：Markdown 表格 `### 工具筛选出的相关ETF数据`

### 5. HotStrategyTable —— 热门选股策略排行榜

- **数据源**：东方财富选股热榜（np-ipick），**无需任何 API Key**
- **参数**：无
- **调用示例**：`HotStrategyTable()`
- **返回**：当前热门选股策略 Markdown 表格（策略名 + 近期收益表现）
- **典型场景**：用户问"最近什么策略有效 / 现在有什么机会"时，先取策略表再结合其他工具验证

## 三、前置配置

| 依赖 | 影响工具 | 配置位置 |
|---|---|---|
| **QgqpBId**（东财用户标识 Cookie） | SearchStockByIndicators / SearchBk / SearchETF | 设置页「东财唯一标识」 |
| **IwencaiApiKey**（可选回退） | SearchStockByIndicators | 设置页问财 API Key；仅当东财 503 时生效 |
| 无额外依赖 | FilterStocks / HotStrategyTable | — |

未配置 QgqpBId 时工具仍会注册/可见，但执行会返回提示文案；FilterStocks、HotStrategyTable 不受影响。

## 四、执行机制备忘（开发者）

- **注册**：执行体在 `backend/data/tool_search_etf_stock.go` 等文件 `init()` 中
  `registerToolHandler` 注册；schema 在 **两处** 手工定义需同步维护：
  - Agent 层：`backend/agent/tools/data_tools_wrapper.go`（`GetAllDataTools`）
  - 数据层：`backend/data/tools.go`（`Tools`，飞书/钉钉机器人用）
- **常驻登记**：`backend/agent/tools/tool_groups.go` → `alwaysVisibleTools`
  （数据层常驻 = 不登记进 `dataToolGroupMap`，未登记工具对所有问题保留）
- **缓存 TTL**：常驻后查不到 GroupScreening 组默认值，已在
  `backend/agent/tools/data_tools_cache.go` 的 `cacheTTLByTool` 显式配置 300s
- **API Key 依赖**：`backend/data/tool_registry.go` → `toolRequiredKey`
