# go-stock MCP 服务使用指南

本文档说明如何在 go-stock 中配置、管理与使用 MCP（Model Context Protocol）服务，让 AI 助手在对话中调用外部工具（如飞书/钉钉/Slack 消息推送、Webhook、外部 API 查询等）。

> 支持的 MCP 传输协议：**StreamableHTTP**（默认）、**SSE**。鉴权支持**静态 Headers** 与 **OAuth 2.1**（浏览器授权）两种方式。

---

## 一、MCP 是什么，能做什么

MCP 是一个开放协议，允许 AI 应用以标准方式连接外部工具服务器。在 go-stock 中接入 MCP 服务器后：

- AI 助手（Agent 对话）可以**自动调用**这些服务器提供的工具；
- 例如接入一个「飞书机器人 MCP」，就能对 AI 说「把这份分析结论发送到群里」；
- 工具列表、参数 Schema 会自动同步给模型，无需写任何代码。

---

## 二、入口位置

MCP 服务管理界面有两个入口：

| 入口 | 路径 |
| --- | --- |
| 独立管理页 | 左侧导航 → **MCP 服务**（路由 `/mcp-servers`） |
| 研究页内嵌 | **深入研究** 页（路由 `/research`）中的 MCP 服务管理区块 |

界面提供：服务器列表（分页/搜索/按状态筛选）、新增/编辑/删除、启用开关、连接测试、OAuth 授权、工具详情查看。

---

## 三、添加 MCP 服务器（详细步骤）

### 步骤 1：打开新增表单

进入 MCP 服务管理页 → 点击「新增服务器」按钮。

### 步骤 2：填写基本信息

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| **名称** | ✅ | 服务器唯一标识（如 `feishu-bot`）。⚠️ 提问中包含该名称可提高工具命中率（见第六节） |
| **描述** | — | 用途说明，帮助自己和 AI 理解 |
| **URL** | ✅ | MCP 服务器地址，如 `https://mcp.example.com/mcp` |
| **类型** | — | `streamable-http`（默认）/ `sse`，按服务器文档选择 |
| **命令 / 参数** | ✅（表单校验） | 预留的本地 stdio 启动命令字段（当前连接实际走 HTTP，按需填写） |
| **启用** | — | 开关，默认开启。关闭后 AI 不再加载该服务器的工具 |

### 步骤 3：配置鉴权（二选一）

**方式 A — 静态 Headers（默认，`鉴权方式 = 无`）**

适合 API Key / Token 直接放在请求头的场景。界面提供表单式键值对编辑（无需手写 JSON）：

- 常见写法：`Authorization: Bearer sk-xxx` 或 `X-api-key: xxx`

Header 值支持以下**模板变量**（避免明文凭证进数据库）：

| 变量 | 展开结果 |
| --- | --- |
| `{{env.VAR_NAME}}` | 读取环境变量（推荐放 API Token，如 `{{env.FEISHU_TOKEN}}`） |
| `{{sessionId}}` / `{{conversationId}}` | 当前会话 ID（为空自动生成 UUID） |
| `{{uuid}}` | 每次调用生成新 UUID |

> `{{env.*}}` 变量在启动 go-stock 的终端环境中读取。找不到时替换为空串，鉴权失败请先检查变量是否已导出。

**方式 B — OAuth 2.1（`鉴权方式 = OAuth 2.1`）**

适合支持标准 MCP OAuth 的服务端：

1. 鉴权方式选「OAuth 2.1（浏览器授权）」并保存；
2. 点击「授权」→ 自动打开系统浏览器完成授权（动态客户端注册 + PKCE，loopback 回调）；
3. 凭证加密存储（AES-GCM），access token 临近过期时**自动用 refresh_token 续期**，无需人工干预。

> ⚠️ OAuth 凭证字段仅由授权流程内部写入，普通编辑保存不会覆盖或泄露。

### 步骤 4：测试连接

点击「测试连接」，系统会执行：

1. 建立 MCP 连接（initialize）；
2. 拉取工具列表（tools/list）；
3. 将每个工具的名称、描述、参数 Schema 持久化到本地数据库（`mcp_server_tools` 表）。

成功提示形如：`连接成功!发现 3 个工具: send_message, query_board, push_notice`。

状态含义：

| 状态 | 含义 | 处理 |
| --- | --- | --- |
| `available` 可用 | 连接与工具发现均成功 | 无 |
| `untested` 未测试 | 尚未测过 | 测试一次 |
| `unauthorized` 未授权 | 鉴权失败 | OAuth 重新授权 / 检查 Headers 或 env 变量 |
| `unavailable` 不可用 | 连不上或无工具 | 检查 URL、网络、服务器日志 |

### 步骤 5：确认启用

保持「启用」开关打开。只有 `enable = true` 的服务器，其工具才会进入 AI 的可调用范围。

---

## 四、验证工具是否就绪

在服务器列表中**点击行**可展开该服务器的工具清单；点「查看」可看单个工具的参数详情（参数名、类型、是否必填、枚举值、描述）。

也可以在 Agent 对话中直接问：「你现在有哪些可用的 MCP 工具？」

---

## 五、在 AI 对话中调用

配置完成后无需任何额外操作，正常与 AI 助手对话即可。模型会根据你的问题自主决定是否调用 MCP 工具，并在回复中展示调用过程与结果。

提问示例（以接入飞书机器人为例）：

> 「用 feishu-bot 把贵州茅台今日的资金流向摘要发送出去」

## 六、不同 Agent 模式下的工具加载机制

MCP 工具的注入策略随 Agent 模式不同而不同（这也是「为什么有时候工具没被调用」的原因）：

| 模式 | 机制 | 触发条件 |
| --- | --- | --- |
| **DeepAgents**（默认） | MCP 工具放入**动态工具池**，由 Eino ADK ToolSearch 按需检索 | 核心工具常驻；MCP 工具需模型主动执行 `tool_search` 后再调用 |
| **React / PlanExecute / auto** | **按问题匹配注入** | 满足以下任一条件才注入：① 问题中包含**启用的服务器名称**（如 `feishu-bot`）；② 问题含 MCP 相关信号词（mcp / 工具 / 调用 / 发送 / 通知 / 消息 / 服务器 / server / webhook / api / 机器人 / slack / 钉钉 / 飞书 / 企业微信 等）；③ 工具名/工具描述与问题关键词命中 |

**提高命中率的实用建议：**

- 提问时**直接点名服务器名**（如「用 slack-server 发消息…」）；
- 或使用信号词（「调用」「发送」「通知」等）；
- 服务器名称建议起**语义相关**的名字（如 `weather-query` 而非 `srv1`）。

## 七、让 AI 自己管理 MCP 服务器

go-stock 内置了一组 **MCP 管理工具**（`../backend/agent/tools/mcp_skill_tools.go`），AI 助手可以在对话中直接帮你增删改查 MCP 服务器：

| 工具名 | 功能 |
| --- | --- |
| `ListMCPServers` | 按名称/状态/启用筛选服务器列表（分页） |
| `GetMCPServerDetail` | 按 ID 查服务器详情 |
| `CreateMCPServer` | 新建服务器（名称+URL 必填） |
| `UpdateMCPServer` | 更新配置（按 ID） |
| `DeleteMCPServer` | 删除服务器 |

对话示例：

> 「帮我添加一个 MCP 服务器，名字叫 weather，URL 是 https://mcp.weather.com/mcp」
> 「看下现在启用了哪些 MCP 服务器」

> 💡 用这种方式添加后，仍需在界面里点一次「测试连接」同步工具清单（或让工具缓存 TTL 到期自动重建）。

## 八、性能：工具缓存

为避免每个问题都重建 MCP 客户端（多服务器时初始化开销可达数秒），go-stock 内置了**全局工具缓存**：

- 按服务器 ID 缓存已发现的工具列表；
- 配置变更（URL / Headers / 类型 / 启用状态等）时**指纹自动失效**，立即重建；
- 另有 10 分钟 TTL 兜底；
- 旧客户端延迟关闭，不影响进行中的对话。

即：**改完配置立刻生效，无需重启应用**。

---

## 九、常见问题

### 1. 测试连接报「鉴权失败」（unauthorized）

- 静态 Headers：检查 Token 是否过期、Header 名是否正确、`{{env.*}}` 变量是否已导出；
- OAuth：重新点击「授权」走一遍浏览器流程。

### 2. 连接成功但 AI 不调用工具

- 确认服务器「启用」开关已打开；
- 按第六节调整提问方式（点名服务器名 / 使用信号词）；
- DeepAgents 模式下模型需先 `tool_search`，复杂问题多给它一点耐心。

### 3. 「unavailable」但 URL 浏览器能打开

- 确认类型选对：部分旧服务器只支持 `sse`，新服务器用 `streamable-http`；
- 检查服务器是否限制来源 IP / UA；
- 查看 `logs/wails.log` 中的详细错误。

### 4. 工具列表为空（「连接成功！但未发现可用工具」）

服务器实现了 MCP 协议但未注册任何工具，属服务器侧问题，与 go-stock 无关。

### 5. 想让某个服务器彻底不参与

关闭列表中的「启用」开关即可（保留配置，随时可再开）；删除则会连同工具记录一起清除。

---

## 十、技术细节（开发者参考）

| 内容 | 位置 |
| --- | --- |
| 服务器数据模型（`mcp_servers` 表） | `../backend/models/models.go` → `MCPServer` |
| 工具记录模型（`mcp_server_tools` 表） | `../backend/models/models.go` → `MCPServerTool` |
| 增删改查 / 测试连接 / OAuth | `../backend/data/mcp_server_api.go` |
| Header 模板变量展开 / OAuth 凭证合并 | `../backend/data/mcp_server_api.go` → `ExpandHeaderVars` / `ResolveMCPHeaders` |
| 工具缓存（指纹 + TTL） | `../backend/agent/mcp_tools_cache.go` |
| 各模式工具注入逻辑 | `../backend/agent/agent.go` → `maybeGetMCPTools` / ToolSearch 集成 |
| AI 端 MCP 管理工具（List/Create/Update/Delete） | `../backend/agent/tools/mcp_skill_tools.go` |
| 前端管理界面 | `../frontend/src/components/mcp-server-manager.vue` |
| Wails API 包装 | `../app.go`（`Create/Update/Delete/Enable/TestMCPServer` 等） |

## 许可证

详见项目根目录的 LICENSE 文件。
