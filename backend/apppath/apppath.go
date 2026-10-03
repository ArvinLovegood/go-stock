// Package apppath 统一解析运行时可写目录（数据库、日志、外置配置文件）。
//
// 历史实现把 "data"、"logs" 等路径写成相对当前工作目录（cwd）的硬编码字符串。
// macOS 上由 LaunchServices（访达双击 / Dock / open）启动的 .app，cwd 固定为只读的
// "/"，于是 mkdir 必然失败、sqlite 打不开、进程 exit(1)，表现为双击后闪退。
//
// 本包给出一个稳定的绝对基准目录（base），DataDir/LogsDir 都挂在其下，
// 并且优先级设计上尽量保持老用户的目录不变：
//
//  1. 环境变量 GO_STOCK_ROOT_DIR（显式覆盖，与 agent 沙箱根语义保持一致）；
//  2. 当前工作目录下已存在 data/stock.db —— 沿用历史行为，终端启动的老用户无感；
//  3. 可执行文件所在目录下已存在 data/stock.db —— 沿用便携安装 / 双击启动的历史行为；
//  4. 全新安装：macOS 的 .app 包内不适合持久化数据，落到
//     ~/Library/Application Support/go-stock；其它平台优先落在可执行文件同级目录
//     （与发布包内 data/ 目录布局一致），不可写时再回退系统配置目录；
//  5. 以上都不可用时兜底到系统临时目录。
package apppath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const appName = "go-stock"

var (
	once sync.Once
	base string
)

// BaseDir 返回程序运行数据的基准目录。
func BaseDir() string {
	once.Do(func() { base = resolve() })
	return base
}

// DataDir 返回数据库与外置配置文件所在目录。
func DataDir() string { return filepath.Join(BaseDir(), "data") }

// LogsDir 返回日志文件所在目录。
func LogsDir() string { return filepath.Join(BaseDir(), "logs") }

// File 返回数据目录下指定文件的绝对路径。
func File(elem ...string) string {
	return filepath.Join(append([]string{DataDir()}, elem...)...)
}

// ExeDir 返回可执行文件所在目录，获取失败时返回空串。
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return ""
	}
	return filepath.Dir(exe)
}

// Ensure 创建数据目录与日志目录，任一失败返回错误（便于调用方显式暴露原因）。
func Ensure() error {
	for _, dir := range []string{DataDir(), LogsDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s failed: %w", dir, err)
		}
	}
	return nil
}

// resolve 按优先级挑选基准目录。
func resolve() string {
	if root := strings.TrimSpace(os.Getenv("GO_STOCK_ROOT_DIR")); root != "" {
		return root
	}
	if wd, err := os.Getwd(); err == nil && hasData(wd) {
		return wd
	}
	exeDir := ExeDir()
	if hasData(exeDir) {
		return exeDir
	}
	// 全新安装：macOS 的 .app 包内目录不适合存放需要跨版本保留的数据。
	if runtime.GOOS == "darwin" && isAppBundle(exeDir) {
		if dir := userConfigDir(); usable(dir) {
			return dir
		}
	}
	if usable(exeDir) {
		return exeDir
	}
	if dir := userConfigDir(); usable(dir) {
		return dir
	}
	return filepath.Join(os.TempDir(), appName)
}

// hasData 判断目录下是否已存在历史数据库文件，作为“老用户数据位置”的标记。
func hasData(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "data", "stock.db"))
	return err == nil
}

// usable 通过尝试创建 data/logs 子目录来验证基准目录是否可写。
func usable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		return false
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return false
	}
	return true
}

// isAppBundle 判断可执行文件是否位于 macOS .app 包内（Contents/MacOS）。
func isAppBundle(exeDir string) bool {
	if exeDir == "" {
		return false
	}
	return strings.HasSuffix(filepath.ToSlash(exeDir), "/Contents/MacOS")
}

func userConfigDir() string {
	cfg, err := os.UserConfigDir()
	if err != nil || cfg == "" {
		return ""
	}
	return filepath.Join(cfg, appName)
}
