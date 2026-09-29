package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/schema"
	"go-stock/backend/logger"
)

// LocalStreamingShell 是基于本地操作系统 shell 的 filesystem.StreamingShell 实现。
//
// 设计要点：
//   - 平台自适应：Windows 使用 PowerShell（powershell.exe -Command），
//     Linux/macOS 使用 /bin/sh -c，确保跨平台一致体验。
//   - 工作目录：固定为构造时传入的 workDir，避免命令任意穿越文件系统。
//   - 流式输出：合并 stdout/stderr，按行读取并通过 schema.Pipe 实时推送，
//     模型可即时获得命令执行进度。
//   - 超时控制：默认 60 秒，可通过 WithTimeout 调整，避免长时间挂起。
//   - 输出上限：合并输出超过 4MB 自动截断，防止内存被刷爆。
//   - 安全考量：危险命令拦截（见 dangerousShellCommandRules，系统级破坏与
//     远程下载执行两类操作在股票分析场景没有正当用途，且是提示词注入攻击的
//     常见落地手段）；可选只读模式（WithReadOnly）进一步禁止重定向写入与
//     变更命令。二者均为纵深防御的一层，不能替代工作目录沙箱与超时控制。
//
// 用于 DeepAgents 模式，将 execute 工具暴露给模型，支持运行构建、测试、
// 脚本等命令以辅助代码分析与项目理解。
type LocalStreamingShell struct {
	workDir  string
	timeout  time.Duration
	readOnly bool
}

// shellCommandRule 描述一条命令安全检查规则：命令命中 pattern 即以 reason 拒绝执行。
type shellCommandRule struct {
	pattern *regexp.Regexp
	reason  string
}

// dangerousShellCommandRules 危险命令拦截规则（大小写不敏感，跨平台生效）。
var dangerousShellCommandRules = []shellCommandRule{
	{regexp.MustCompile(`(?i)\brm\s+(-\w*[rf]\w*\s+)+\s*(--\S+\s+)*(/|/\*|~|\$HOME)\s*$`), "递归强制删除根/家目录"},
	{regexp.MustCompile(`(?i)\bmkfs\b`), "格式化文件系统"},
	{regexp.MustCompile(`(?i)\bdd\s+[^|;&]*\bof=/dev/`), "dd 直写块设备"},
	{regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:`), "fork 炸弹"},
	{regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff)\b`), "关机/重启"},
	{regexp.MustCompile(`(?i)\binit\s+[06]\b`), "切换运行级别（关机/重启）"},
	{regexp.MustCompile(`(?i)\b(curl|wget|iwr|Invoke-WebRequest)\b[^|;&]*\|\s*(sudo\s+)?(bash|sh|zsh|powershell|pwsh|iex|Invoke-Expression)\b`), "下载并直接执行远程脚本"},
	{regexp.MustCompile(`(?i)(DownloadString|DownloadFile)\s*\(`), "PowerShell 远程下载执行"},
	{regexp.MustCompile(`(?i)\biex\b`), "PowerShell Invoke-Expression 动态执行"},
	{regexp.MustCompile(`(?i)\bformat\s+[a-z]:`), "格式化磁盘分区"},
	{regexp.MustCompile(`(?i)\b(diskpart|bcdedit|Clear-Disk|Format-Volume|Remove-Partition)\b`), "磁盘/启动配置操作"},
	{regexp.MustCompile(`(?i)\b(del|erase)\s+[^|;&]*?/[sqf]+[^|;&]*?\b[a-z]:\\`), "递归强制删除磁盘目录"},
	{regexp.MustCompile(`(?i)\b(rd|rmdir)\s+[^|;&]*?/s[^|;&]*?\b[a-z]:\\`), "递归删除磁盘目录"},
	{regexp.MustCompile(`(?i)\bRemove-Item\b[^|;&]*(-Recurse[^|;&]*([a-z]:\\|\$env:)|([a-z]:\\|\$env:)[^|;&]*-Recurse)`), "PowerShell 递归删除系统/环境目录"},
	{regexp.MustCompile(`(?i)\b(Stop-Computer|Restart-Computer)\b`), "关机/重启"},
	{regexp.MustCompile(`(?i)\breg\s+(delete|add|import)\b[^|;&]*\bHKLM\b`), "修改机器级注册表"},
	{regexp.MustCompile(`(?i)\bSet-ExecutionPolicy\b`), "修改 PowerShell 执行策略"},
	{regexp.MustCompile(`(?i)\bnet\s+(user|localgroup)\b`), "修改本地账户/组"},
	{regexp.MustCompile(`(?i)\b(certutil|bitsadmin)\b[^|;&]*(-urlcache|/transfer)`), "系统工具下载远程文件"},
}

// readOnlyShellCommandRules 只读模式下的追加拦截规则：拒绝重定向写入与常见变更命令。
// 基于模式匹配，属于 best-effort；构建/测试等需要写文件的场景不应开启只读模式。
var readOnlyShellCommandRules = []shellCommandRule{
	{regexp.MustCompile(`>>?\s*[^&\s]`), "只读模式：禁止输出重定向写入文件"},
	// 动词后必须跟空白字符，避免误伤 "README.md" 这类扩展名命中 \bmd\b 的情况
	{regexp.MustCompile(`(?i)\b(rm|rmdir|rd|del|erase|mv|move|rename|ren|cp|copy|xcopy|robocopy|mkdir|md|touch|tee|truncate|ln)\s`), "只读模式：禁止文件变更命令"},
	{regexp.MustCompile(`(?i)\b(Set-Content|Out-File|Add-Content|New-Item|Rename-Item|Move-Item|Copy-Item|Remove-Item|Clear-Content|New-ItemProperty|Set-ItemProperty|Remove-ItemProperty)\b`), "只读模式：禁止 PowerShell 变更命令"},
	{regexp.MustCompile(`(?i)\bgit\s+(add|commit|push|pull|fetch|reset|checkout|switch|restore|clean|merge|rebase|tag|stash|apply|am|config|init|clone|submodule|rm|mv)\b`), "只读模式：禁止 git 变更操作"},
	{regexp.MustCompile(`(?i)\b(npm|pnpm|yarn|pip|pip3|go|cargo|mvn|gradle)\s+(install|add|get|update|upgrade|remove|uninstall|publish)\b`), "只读模式：禁止包管理变更"},
	{regexp.MustCompile(`(?i)\b(apt|apt-get|yum|dnf|brew|choco|scoop|winget)\b`), "只读模式：禁止系统包管理器"},
}

// NewLocalStreamingShell 创建一个本地流式 Shell。
// workDir 为命令执行的工作目录；timeout 为单条命令最大执行时长。
func NewLocalStreamingShell(workDir string, timeout time.Duration) *LocalStreamingShell {
	if workDir == "" {
		workDir = "."
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &LocalStreamingShell{workDir: workDir, timeout: timeout}
}

// WithTimeout 设置新的命令超时时长，返回新的实例（便于链式调用）。
func (s *LocalStreamingShell) WithTimeout(timeout time.Duration) *LocalStreamingShell {
	return &LocalStreamingShell{workDir: s.workDir, timeout: timeout, readOnly: s.readOnly}
}

// WithReadOnly 返回开启只读模式的新实例（便于链式调用）。
// 只读模式下除危险命令拦截外，额外拒绝重定向写入与常见变更命令
// （见 readOnlyShellCommandRules），适合纯分析场景；构建/测试需写文件，不应开启。
func (s *LocalStreamingShell) WithReadOnly() *LocalStreamingShell {
	return &LocalStreamingShell{workDir: s.workDir, timeout: s.timeout, readOnly: true}
}

// validateCommand 在执行前检查命令：危险命令（始终拦截）与只读模式限制。
// 返回非空字符串表示拒绝原因。
func (s *LocalStreamingShell) validateCommand(command string) string {
	for _, rule := range dangerousShellCommandRules {
		if rule.pattern.MatchString(command) {
			return rule.reason
		}
	}
	if s.readOnly {
		for _, rule := range readOnlyShellCommandRules {
			if rule.pattern.MatchString(command) {
				return rule.reason
			}
		}
	}
	return ""
}

// ExecuteStreaming 执行一条 shell 命令并流式返回输出。
//
// 实现流程：
//  1. 根据运行平台构造 exec.Cmd（PowerShell/sh）
//  2. 用 io.Pipe 合并 stdout/stderr
//  3. 启动单独 goroutine 执行 cmd.Wait()，结束后关闭 pw（让 scanner 收到 EOF）
//  4. 主 goroutine 按行读取并流式推送
//  5. scanner 退出后，从 waitCh 获取 exit code，发送执行结果摘要
func (s *LocalStreamingShell) ExecuteStreaming(ctx context.Context, req *filesystem.ExecuteRequest) (*schema.StreamReader[*filesystem.ExecuteResponse], error) {
	if req == nil || strings.TrimSpace(req.Command) == "" {
		return nil, errors.New("命令为空")
	}

	if reason := s.validateCommand(req.Command); reason != "" {
		logger.SugaredLogger.Warnf("shell 命令被安全策略拦截: cmd=%q reason=%s", req.Command, reason)
		return nil, fmt.Errorf("命令被安全策略拦截（%s），已拒绝执行", reason)
	}

	execCtx, cancel := context.WithTimeout(ctx, s.timeout)

	cmd, err := s.buildCommand(execCtx, req.Command)
	if err != nil {
		cancel()
		return nil, err
	}

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		cancel()
		_ = pw.Close()
		return nil, fmt.Errorf("启动命令失败: %w", err)
	}

	sr, sw := schema.Pipe[*filesystem.ExecuteResponse](20)

	go func() {
		defer cancel()   // 释放 context，避免泄漏
		defer sw.Close() // 关闭 stream writer，通知消费端 EOF
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("streaming shell panic: %v", r)
				sw.Send(&filesystem.ExecuteResponse{
					Output: fmt.Sprintf("\n[内部错误: %v]", r),
				}, nil)
			}
		}()

		// 单独 goroutine 等待进程退出后关闭 pw
		// 否则 scanner.Scan() 会永远阻塞（io.Pipe 不会自动关闭）
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
			_ = pw.Close()
		}()

		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

		var totalBytes int
		const maxOutputBytes = 4 * 1024 * 1024
		truncated := false

		for scanner.Scan() {
			line := scanner.Text()
			lineBytes := len(line) + 1
			if totalBytes+lineBytes > maxOutputBytes {
				if !truncated {
					sw.Send(&filesystem.ExecuteResponse{
						Output:    "\n[输出超过 4MB 限制，已截断]",
						Truncated: true,
					}, nil)
					truncated = true
				}
				continue
			}
			totalBytes += lineBytes
			sw.Send(&filesystem.ExecuteResponse{
				Output: line + "\n",
			}, nil)
		}

		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			sw.Send(&filesystem.ExecuteResponse{
				Output: fmt.Sprintf("\n[读取输出错误: %v]", err),
			}, nil)
		}

		// 获取进程退出状态（此时 pw 已关闭，scanner 已退出）
		waitErr := <-waitCh
		exitCode := 0
		var summary string

		switch {
		case waitErr == nil:
			summary = "\n[命令执行完成: exit code 0]"
		default:
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				summary = fmt.Sprintf("\n[命令执行失败: exit code %d]", exitCode)
			} else if execCtx.Err() == context.DeadlineExceeded {
				exitCode = -1
				summary = fmt.Sprintf("\n[命令执行超时，已终止（上限 %s）]", s.timeout)
			} else {
				exitCode = -1
				summary = fmt.Sprintf("\n[命令执行错误: %v]", waitErr)
			}
		}

		// 发送执行结果摘要，方便模型和调试时判断结果
		sw.Send(&filesystem.ExecuteResponse{
			Output: summary,
		}, nil)
		sw.Send(&filesystem.ExecuteResponse{
			ExitCode: &exitCode,
		}, nil)

		logger.SugaredLogger.Debugf("shell 执行完成: cmd=%q, exit=%d, bytes=%d, truncated=%v",
			req.Command, exitCode, totalBytes, truncated)
	}()

	return sr, nil
}

// buildCommand 根据运行平台构造对应的 exec.Cmd。
func (s *LocalStreamingShell) buildCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShell：-NoProfile 加速启动并避免用户配置干扰，-Command 执行命令
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command)
	case "linux", "darwin":
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	default:
		return nil, fmt.Errorf("不支持的平台: %s", runtime.GOOS)
	}
	cmd.Dir = s.workDir
	// 继承当前进程环境变量，确保 PATH 等可用
	// cmd.Env = os.Environ() // 默认即继承，无需显式设置
	// 隐藏子进程控制台窗口（go-stock 为 Wails GUI 应用，无控制台宿主，
	// 否则 Windows 会为 powershell.exe 自动创建可见 PowerShell 窗口）。
	// 平台差异通过 applyHiddenWindow 分文件实现（windows/unix）。
	applyHiddenWindow(cmd)
	return cmd, nil
}

// 编译期断言：确保 LocalStreamingShell 完整实现 filesystem.StreamingShell 接口。
var _ filesystem.StreamingShell = (*LocalStreamingShell)(nil)

// ShellInfo 返回 Shell 的描述信息（用于日志诊断）。
func (s *LocalStreamingShell) ShellInfo() string {
	shell := "sh"
	if runtime.GOOS == "windows" {
		shell = "powershell"
	}
	return fmt.Sprintf("shell=%s, workdir=%s, timeout=%s", shell, s.workDir, s.timeout)
}
