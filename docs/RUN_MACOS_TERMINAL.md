# go-stock macOS 终端手动运行二进制指南

本文档说明如何在 macOS 终端中手动运行**已编译好的 go-stock 二进制文件**，适用于从 GitHub Releases 下载的裸可执行文件，或本地 / CI 交叉编译产物。

> go-stock 基于 Wails 框架构建。前端资源、图标、股票基础数据等已通过 `//go:embed` 编译进二进制，**单个文件即可独立运行**，无需安装额外依赖。更完整的运行说明（含 `.app` 封装、通知设置）见 [RUN_MACOS.md](RUN_MACOS.md)。

## 一、前置条件

| 项目 | 要求 |
| --- | --- |
| 系统版本 | macOS **10.13.0** 或更高 |
| CPU 架构 | 与二进制文件匹配（见下表） |
| 终端 | 系统自带 Terminal 或 iTerm2 均可 |

## 二、详细步骤

### 步骤 1：确认 CPU 架构

```bash
uname -m
```

| 架构标识 | 适用芯片 | `uname -m` 输出 | 对应文件名 |
| --- | --- | --- | --- |
| `darwin/arm64` | Apple Silicon（M1/M2/M3/M4） | `arm64` | `go-stock-darwin-arm64` |
| `darwin/amd64` | Intel（x86_64） | `x86_64` | `go-stock-darwin-intel` |
| `darwin/universal` | 通用二进制，两种芯片均支持 | 任意 | `go-stock-darwin-universal` |

> - Apple Silicon 优先选 `arm64` 原生版本，性能最佳。
> - 在 Apple Silicon 上运行 `intel` 版本会经 Rosetta 2 转译，需先执行 `softwareupdate --install-rosetta`，不推荐。

### 步骤 2：获取二进制文件

按来源三选一：

**方式 A — 从 GitHub Releases 下载**（https://github.com/ArvinLovegood/go-stock/releases）

```bash
# 推荐 .zip（体积小，解压后通常保留执行权限）
curl -LO https://github.com/ArvinLovegood/go-stock/releases/latest/download/go-stock-darwin-arm64.zip
unzip go-stock-darwin-arm64.zip
```

**方式 B — 本地 `wails build` 产物**

```bash
# 默认产出应用包，内层即裸二进制
cp build/bin/go-stock.app/Contents/MacOS/go-stock ./go-stock
# 交叉编译 / CI 产物为裸二进制，直接位于 build/bin/go-stock
```

**方式 C — scp / AirDrop / U盘 传输**

传输后权限和扩展属性可能丢失，后续步骤 4、5 必做。

> 本地自编译产物通常无隔离属性，步骤 5（xattr）可省略；经浏览器下载、传输的一律建议执行。

### 步骤 3：放入固定目录

go-stock 的数据库和日志生成在**运行时的工作目录**，换目录启动等于换一套数据。建议建立专属目录：

```bash
mkdir -p ~/go-stock
mv go-stock-darwin-arm64 ~/go-stock/
cd ~/go-stock
```

### 步骤 4：添加执行权限

```bash
chmod +x go-stock-darwin-arm64
```

> 下载 / 传输后的文件常丢失执行位，缺失时运行会报 `permission denied`。

### 步骤 5：清除隔离属性（绕过 Gatekeeper）

经浏览器或 AirDrop 下载的文件会被打上 `com.apple.quarantine` 标记，首次运行被 Gatekeeper 拦截，提示「无法打开，因为无法验证开发者」或「已损坏，应该移到废纸篓」——**并非文件真的损坏**：

```bash
# 方式一：仅移除隔离标记
xattr -d com.apple.quarantine go-stock-darwin-arm64

# 方式二：递归清除所有扩展属性（更彻底，推荐）
xattr -c go-stock-darwin-arm64
```

> 不熟悉命令行也可走图形界面：双击文件 → 关闭安全提示 → **系统设置 → 隐私与安全性** → 底部点「仍要打开」。

### 步骤 6：运行

```bash
./go-stock-darwin-arm64
```

> **必须带 `..` 前缀**（或使用绝对路径），否则 shell 只会去 PATH 中查找，报 `command not found`。

首次运行会弹窗请求**通知权限**，请选「允许」（价格预警、定时任务推送依赖它）。

## 三、运行时行为

| 事项 | 说明 |
| --- | --- |
| 前台运行 | Wails 应用会阻塞终端直到退出；需查看日志输出时保持前台，不要 Ctrl+Z |
| 窗口 | 自动按屏幕 80% 宽 × 90% 高居中显示 |
| 退出 | 关闭窗口 → 确认对话框 → 退出；或 `Command+Q` |
| 单实例锁 | 已在运行时再启动只弹通知「程序已经在运行了」，不开第二个窗口 |
| 通知归属 | 裸二进制无 `CFBundleIdentifier`，通知设置中显示为文件名而非「go-stock」（详见 RUN_MACOS.md 的 `.app` 封装章节） |

## 四、运行时文件与目录

在**启动时的工作目录**下生成：

| 路径 | 说明 |
| --- | --- |
| `../data` | 数据目录，启动时自动创建 |
| `../data/stock.db` | SQLite 数据库（WAL 模式） |
| `../data/stock.db-wal` / `../data/stock.db-shm` | SQLite WAL 日志 / 共享内存文件 |
| `../data/dict` | 情绪分析分词字典目录 |
| `logs/wails.log` | 应用运行日志 |

> 💡 **始终从同一目录启动**（推荐 `~/go-stock/`），避免数据分散或「数据丢失」错觉。

## 五、停止与重启

```bash
# 正常退出：窗口关闭确认，或 Command+Q

# 进程残留时强制结束
pkill -f go-stock-darwin
```

## 六、常见问题速查

| 现象 | 原因与解法 |
| --- | --- |
| `command not found` | 未带 `..` 前缀，用 `./go-stock-darwin-arm64` 运行 |
| `permission denied` | 缺执行权限 → `chmod +x` |
| 「已损坏，应该移到废纸篓」/「无法验证开发者」 | 隔离标记未清 → `xattr -c`（文件没坏） |
| 启动闪退 / 无窗口 | 保持前台运行查看终端报错。常见：①架构不匹配（M 芯片跑 intel 版未装 Rosetta）→ 换 arm64/universal；②工作目录无写权限（无法建 `../data`）→ 移到 `~/go-stock/` |
| `db connection error` | `../data` 目录无法创建，确认启动目录可写：`mkdir -p data && ls -ld data` |
| 「程序已经在运行了」但找不到窗口 | 单实例锁生效，旧实例残留后台 → `pkill -f go-stock-darwin` 后重启 |
| 换目录后数据「丢失」 | 从不同目录启动读取了不同 `../data`，回到原目录启动 |
| 通知不弹出 | 检查 系统设置 → 通知 中对应条目；检查应用内「本地推送」开关 |

## 七、一键脚本

以 arm64 + Release 下载为例，全部步骤合并：

```bash
set -e

# 1. 下载并解压
curl -LO https://github.com/ArvinLovegood/go-stock/releases/latest/download/go-stock-darwin-arm64.zip
unzip -o go-stock-darwin-arm64.zip

# 2. 放入专属目录
mkdir -p ~/go-stock
mv go-stock-darwin-arm64 ~/go-stock/
cd ~/go-stock

# 3. 赋权 + 清除隔离属性
chmod +x go-stock-darwin-arm64
xattr -c go-stock-darwin-arm64

# 4. 运行
./go-stock-darwin-arm64
```

## 许可证

详见项目根目录的 LICENSE 文件。
