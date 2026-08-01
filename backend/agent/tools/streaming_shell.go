package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
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
//   - 安全考量：不限制命令内容（保持 Shell 应有的灵活性），但工作目录已限定，
//     且超时机制防止资源耗尽型攻击。
//
// 用于 DeepAgents 模式，将 execute 工具暴露给模型，支持运行构建、测试、
// 脚本等命令以辅助代码分析与项目理解。
type LocalStreamingShell struct {
	workDir string
	timeout time.Duration
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
	return &LocalStreamingShell{workDir: s.workDir, timeout: timeout}
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
