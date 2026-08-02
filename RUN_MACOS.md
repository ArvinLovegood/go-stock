# go-stock macOS 运行指南

本文档说明如何在 macOS 上运行已编译好的 go-stock 二进制程序。

> go-stock 基于 Wails 框架构建。GitHub Releases 中提供的 macOS 产物是 **裸 Mach-O 可执行文件**（如 `go-stock-darwin-arm64`），不是 `.app` 应用包。前端资源、图标、股票基础数据等已通过 `//go:embed` 编译进二进制，**单个文件即可独立运行**，无需额外依赖。

## 一、前置条件

### 1. 系统版本

macOS **10.13.0** 或更高版本（见 `build/darwin/Info.plist` 中 `LSMinimumSystemVersion`）。

### 2. 确认 CPU 架构

go-stock 为 macOS 提供三种架构的可执行文件，运行前请先确认你的 Mac 芯片类型：

```bash
uname -m
```

| 架构标识 | 适用芯片 | `uname -m` 输出 | 对应下载文件 |
| --- | --- | --- | --- |
| `darwin/arm64` | Apple Silicon（M1/M2/M3/M4） | `arm64` | `go-stock-darwin-arm64` |
| `darwin/amd64` | Intel（x86_64） | `x86_64` | `go-stock-darwin-intel` |
| `darwin/universal` | 通用二进制，同时支持上述两种 | 任意 | `go-stock-darwin-universal` |

> - Apple Silicon 用户优先选 `arm64` 原生版本，性能最佳。
> - `universal` 版本可在任意 Mac 上运行，但体积约为单架构的两倍（约 249 MB）。
> - 在 Apple Silicon 上运行 `intel` 版本会通过 Rosetta 2 转译，性能下降且需先安装 Rosetta（`softwareupdate --install-rosetta`），不推荐。

## 二、下载产物

从 GitHub Releases 下载对应架构的文件，两种格式任选其一：

- **裸可执行文件**：`go-stock-darwin-arm64`（约 120 MB，下载后需手动添加执行权限）
- **压缩包**：`go-stock-darwin-arm64.zip`（约 31 MB，解压后通常已带执行权限）

下载地址：https://github.com/ArvinLovegood/go-stock/releases

> 建议下载 `.zip` 版本，体积更小且解压后通常保留执行权限；裸文件下载后需额外 `chmod +x`。

## 三、运行步骤

以下以 Apple Silicon（arm64）为例，Intel / universal 用户请替换文件名。

### 1. 解压（如下载的是 .zip）

```bash
unzip go-stock-darwin-arm64.zip
```

解压后得到可执行文件 `go-stock-darwin-arm64`。

### 2. 添加执行权限

从浏览器下载的裸文件、或解压后权限丢失时，需要手动赋予执行权限：

```bash
chmod +x go-stock-darwin-arm64
```

### 3. 移除隔离属性（绕过 Gatekeeper）

通过浏览器下载的文件会被 macOS 打上 `com.apple.quarantine` 隔离标记，首次运行时 Gatekeeper 会拦截，提示「无法打开，因为无法验证开发者」或「已损坏，应该移到废纸篓」。

> 这并非文件真的损坏，而是 go-stock 未使用 Apple Developer ID 证书签名（仅 Ad-hoc 签名）。执行以下命令清除隔离标记即可：

```bash
xattr -d com.apple.quarantine go-stock-darwin-arm64
# 或递归清除所有扩展属性（更彻底）
xattr -c go-stock-darwin-arm64
```

> 若不熟悉命令行，也可走图形界面放行：双击文件 → 出现安全提示后关闭 → **系统设置 → 隐私与安全性** → 滚动到底部点击「仍要打开」。

### 4. 运行

```bash
./go-stock-darwin-arm64
```

首次运行会弹窗请求通知权限，请选择「允许」（用于价格预警、定时任务推送）。

应用窗口将自动按屏幕尺寸的 80% 宽 × 90% 高居中显示。关闭窗口时会弹出确认对话框，选择「确定」退出。

### 一键脚本

将上述步骤合并，可直接执行：

```bash
# 下载（以 arm64 zip 为例）
curl -LO https://github.com/ArvinLovegood/go-stock/releases/latest/download/go-stock-darwin-arm64.zip

# 解压并赋权
unzip go-stock-darwin-arm64.zip
chmod +x go-stock-darwin-arm64

# 清除隔离属性
xattr -c go-stock-darwin-arm64

# 运行
./go-stock-darwin-arm64
```

## 四、运行时文件与目录

go-stock 启动后会在 **当前工作目录**（即执行命令时所在目录）下创建以下文件：

| 路径 | 说明 |
| --- | --- |
| `data/` | 数据目录，启动时自动创建 |
| `data/stock.db` | SQLite 数据库（WAL 模式），存储股票、分组、配置等 |
| `data/stock.db-wal` | SQLite WAL 日志文件 |
| `data/stock.db-shm` | SQLite 共享内存文件 |
| `data/dict/` | 情绪分析分词字典目录 |
| `logs/wails.log` | 应用运行日志 |

> 💡 由于数据库和日志路径相对于工作目录，**建议固定从同一目录启动**。推荐做法：为二进制新建一个专属目录（如 `~/go-stock/`），把可执行文件放进去，每次从该目录运行，避免数据分散。例如：
>
> ```bash
> mkdir -p ~/go-stock && mv go-stock-darwin-arm64 ~/go-stock/
> cd ~/go-stock && ./go-stock-darwin-arm64
> ```

## 五、通知权限

go-stock 在 macOS 上通过以下方式发送系统通知：

- **价格预警 / 启动提示**：调用 `osascript -e 'display notification ...'`（见 `backend/data/alert_darwin_api.go`）。
- **应用事件通知**：通过 `beeep` 库发送。

首次运行涉及通知的功能时，macOS 会弹出授权弹窗，请选择「允许」，否则价格预警、定时任务推送等功能将无法生效。

如误选了「拒绝」，可在 **系统设置 → 通知** 中找到对应条目并重新开启。

> ⚠️ 由于裸二进制没有 `CFBundleIdentifier`（应用包标识符），在通知设置中它可能显示为可执行文件名（如 `go-stock-darwin-arm64`）而非「go-stock」。如需更规范的应用名、Dock 图标和通知归属，可参考下文「进阶：封装为 .app 应用包」。

## 六、单实例限制

go-stock 启用了单实例锁（`SingleInstanceLock`，唯一标识 `go-stock`）。重复启动时不会打开第二个窗口，而是弹出系统通知「程序已经在运行了」。

如需强制重启，先完全退出运行中的实例（窗口关闭确认 → 退出，或 `Command+Q`），再重新启动。若进程残留，可强制结束：

```bash
pkill -f go-stock-darwin-arm64
```

## 七、常见问题

### 1. 提示「无法打开，因为无法验证开发者」/「已损坏，应该移到废纸篓」

Gatekeeper 拦截未签名应用，并非文件损坏。执行：

```bash
xattr -c go-stock-darwin-arm64
```

### 2. 提示 `command not found` 或 `permission denied`

可执行文件缺少执行权限，执行：

```bash
chmod +x go-stock-darwin-arm64
./go-stock-darwin-arm64
```

> 注意必须带 `./` 前缀（或使用绝对路径）来运行当前目录下的可执行文件，否则 shell 会去 PATH 中查找。

### 3. 运行后无窗口 / 闪退

在终端直接运行可查看具体错误输出：

```bash
./go-stock-darwin-arm64
```

常见原因：

- **架构不匹配**：在 Apple Silicon 上运行了 `intel` 版本且未安装 Rosetta 2。请改用 `arm64` 或 `universal` 版本。
- **工作目录无写权限**：当前目录无法创建 `data/` 和 `logs/`。请将可执行文件移动到有写权限的目录（如用户主目录下的 `~/go-stock/`）再运行。
- **隔离属性未清除**：见问题 1。

### 4. 通知不弹出

- 检查 **系统设置 → 通知** 中对应条目是否开启。
- 检查应用内设置中「本地推送」开关是否启用（见 `alert_darwin_api.go` 中 `LocalPushEnable` 判断）。

### 5. 数据库初始化失败

错误信息包含 `db connection error` 时，通常是 `data/` 目录无法创建或无写权限。确认启动目录可写：

```bash
mkdir -p data
ls -ld data
```

### 6. 提示「程序已经在运行了」但找不到窗口

单实例锁生效，旧实例仍在后台。强制结束后重启：

```bash
pkill -f go-stock-darwin
```

### 7. 切换目录后数据「丢失」

数据库和日志位于启动时的工作目录。从不同目录启动会读取到不同的 `data/`，看起来像数据丢失。**始终从同一目录启动**即可（推荐 `~/go-stock/`）。

## 八、进阶：封装为 .app 应用包（可选）

裸二进制可以正常运行，但缺少 Dock 图标、应用名显示不规范、通知归属不清晰。如需更原生的 macOS 体验，可手动封装为 `.app` 应用包：

```bash
# 1. 创建应用包目录结构
APP_DIR="go-stock.app"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

# 2. 拷贝可执行文件并重命名为 go-stock（Info.plist 中 CFBundleExecutable 指定的名称）
cp go-stock-darwin-arm64 "${APP_DIR}/Contents/MacOS/go-stock"
chmod +x "${APP_DIR}/Contents/MacOS/go-stock"

# 3. 写入 Info.plist
cat > "${APP_DIR}/Contents/Info.plist" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleName</key>
    <string>go-stock</string>
    <key>CFBundleExecutable</key>
    <string>go-stock</string>
    <key>CFBundleIdentifier</key>
    <string>com.sparkmemory.go-stock</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13.0</string>
    <key>NSHighResolutionCapable</key>
    <string>true</string>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsArbitraryLoads</key>
        <true/>
    </dict>
</dict>
</plist>
EOF

# 4. 清除隔离属性
xattr -cr "${APP_DIR}"

# 5. 运行
open "${APP_DIR}"
```

封装后可拖入 `/Applications/` 作为常规应用使用，Dock 图标会显示为系统默认图标（如需自定义图标，可将 `.icns` 文件放入 `Contents/Resources/` 并在 Info.plist 中添加 `CFBundleIconFile` 字段）。

> 注意：封装为 `.app` 后，工作目录变为 `/Applications/go-stock.app/Contents/MacOS/`（系统目录，只读），数据库写入会失败。建议在应用内配置中将数据目录指向可写位置，或继续使用裸二进制方式从 `~/go-stock/` 运行。**对大多数用户，直接运行裸二进制是更简单的选择。**

## 九、技术支持

如有问题，请提交 Issue 至：https://github.com/ArvinLovegood/go-stock/issues

## 许可证

详见项目根目录的 LICENSE 文件。
