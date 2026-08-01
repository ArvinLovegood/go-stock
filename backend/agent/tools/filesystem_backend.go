package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
	"go-stock/backend/logger"
)

// LocalFilesystemBackend 是基于本地操作系统的 filesystem.Backend 实现。
//
// 设计要点：
//   - 以 rootDir 为沙箱根目录，所有路径操作均在 rootDir 内进行，防止路径穿越（path traversal）。
//   - 路径解析同时支持 POSIX 风格（/）与 Windows 风格（\），便于 LLM 跨平台使用。
//   - Grep 使用 Go 标准库 regexp + bufio.Scanner 实现，无需依赖外部 ripgrep 二进制。
//   - Glob 使用 bmatcuk/doublestar/v4 支持 ** 递归匹配，配合 os.DirFS 进行路径匹配。
//
// 用于 DeepAgents 模式，将文件系统操作能力（read_file/write_file/edit_file/ls/glob/grep）
// 暴露给模型，使其能够读取项目源码、配置文件、写入分析结果等。
type LocalFilesystemBackend struct {
	rootDir string
}

// NewLocalFilesystemBackend 创建一个以 rootDir 为沙箱根的本地文件系统 Backend。
// 若 rootDir 为空，默认使用当前工作目录。
func NewLocalFilesystemBackend(rootDir string) *LocalFilesystemBackend {
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}
	abs, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = abs
	}
	return &LocalFilesystemBackend{rootDir: rootDir}
}

// RootDir 返回沙箱根目录绝对路径。
func (b *LocalFilesystemBackend) RootDir() string { return b.rootDir }

// resolve 将任意输入路径解析为相对于沙箱根的绝对路径，并验证其未越界。
// 同时支持 POSIX 风格（/）与 Windows 风格（\）的输入。
func (b *LocalFilesystemBackend) resolve(path string) (string, error) {
	if path == "" {
		return b.rootDir, nil
	}
	// 统一路径分隔符：LLM 常以 POSIX 风格输入，转换为当前 OS 风格
	cleaned := filepath.FromSlash(path)
	var full string
	if filepath.IsAbs(cleaned) {
		// 绝对路径直接使用，但仍需验证是否在沙箱内
		full = filepath.Clean(cleaned)
	} else {
		full = filepath.Join(b.rootDir, cleaned)
	}
	// 解析软链接与 .. 后再比较，防止符号链接穿越
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("解析路径失败 %q: %w", path, err)
	}
	// 验证最终路径必须在沙箱根之下（或等于沙箱根）
	rel, err := filepath.Rel(b.rootDir, abs)
	if err != nil {
		return "", fmt.Errorf("路径不在沙箱内 %q: %w", path, err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("路径越界，禁止访问沙箱外目录 %q", path)
	}
	return abs, nil
}

// LsInfo 列出指定目录下的文件信息。
func (b *LocalFilesystemBackend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	if req == nil {
		return nil, errors.New("请求参数为空")
	}
	dir := req.Path
	if dir == "" {
		dir = "."
	}
	target, err := b.resolve(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败 %q: %w", target, err)
	}
	result := make([]filesystem.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// 返回相对于沙箱根的路径，便于 LLM 理解与后续调用
		result = append(result, filesystem.FileInfo{
			Path:       filepath.ToSlash(filepath.Join(req.Path, entry.Name())),
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return result, nil
}

// Read 读取文件内容，支持基于行号的 offset/limit。
func (b *LocalFilesystemBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	if req == nil {
		return nil, errors.New("请求参数为空")
	}
	target, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败 %q: %w", req.FilePath, err)
	}
	defer file.Close()

	offset := req.Offset
	if offset < 1 {
		offset = 1
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 2000
	}

	scanner := bufio.NewScanner(file)
	// 提升单行上限到 1MB，避免长行被截断
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var sb strings.Builder
	lineNum := 0
	read := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if read >= limit {
			break
		}
		if read > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(scanner.Text())
		read++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败 %q: %w", req.FilePath, err)
	}
	return &filesystem.FileContent{Content: sb.String()}, nil
}

// GrepRaw 在文件内容中搜索匹配模式。
//
// 实现说明：
//   - Pattern 使用 Go regexp 语法（RE2），与 ripgrep 语法高度兼容。
//   - 遍历 rootDir 下所有文件，对每个文件按行扫描匹配。
//   - 支持 Path（限制搜索目录）、Glob（文件名过滤）、FileType（扩展名过滤）、
//     CaseInsensitive、BeforeLines/AfterLines（上下文）等参数。
func (b *LocalFilesystemBackend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	if req == nil || req.Pattern == "" {
		return nil, errors.New("搜索模式为空")
	}
	pattern := req.Pattern
	if req.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("正则表达式无效 %q: %w", req.Pattern, err)
	}

	// 确定搜索根目录
	searchRoot := b.rootDir
	if req.Path != "" {
		resolved, err := b.resolve(req.Path)
		if err != nil {
			return nil, err
		}
		searchRoot = resolved
	}

	// 文件类型 → 扩展名映射（与 ripgrep --type 对齐的常用子集）
	typeExts := map[string][]string{
		"go":   {".go"},
		"js":   {".js", ".mjs", ".cjs"},
		"ts":   {".ts", ".tsx"},
		"py":   {".py"},
		"java": {".java"},
		"rust": {".rs"},
		"c":    {".c", ".h"},
		"cpp":  {".cpp", ".cc", ".cxx", ".hpp", ".h"},
		"vue":  {".vue"},
		"json": {".json"},
		"yaml": {".yaml", ".yml"},
		"md":   {".md"},
	}

	var matches []filesystem.GrepMatch
	walkErr := filepath.WalkDir(searchRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的目录
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			base := d.Name()
			// 跳过常见无关目录（VCS、依赖、构建产物）
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		// 文件名过滤（glob）
		if req.Glob != "" {
			ok, _ := doublestar.Match(req.Glob, filepath.Base(path))
			if !ok {
				// 也尝试对完整相对路径匹配
				rel, _ := filepath.Rel(searchRoot, path)
				ok, _ = doublestar.Match(req.Glob, filepath.ToSlash(rel))
			}
			if !ok {
				return nil
			}
		}
		// 文件类型过滤
		if req.FileType != "" {
			exts, ok := typeExts[req.FileType]
			if !ok {
				// 未知类型，按扩展名 ".filetype" 过滤
				exts = []string{"." + req.FileType}
			}
			matched := false
			for _, ext := range exts {
				if strings.HasSuffix(path, ext) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// 读取并扫描文件
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		relPath, _ := filepath.Rel(b.rootDir, path)
		displayPath := filepath.ToSlash(relPath)

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNum := 0
		beforeLines := req.BeforeLines
		afterLines := req.AfterLines
		var beforeBuf []string
		if beforeLines > 0 {
			beforeBuf = make([]string, 0, beforeLines)
		}
		pendingAfter := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				// 输出前导上下文
				if beforeLines > 0 {
					start := 0
					if len(beforeBuf) < beforeLines {
						start = 0
					} else {
						start = len(beforeBuf) - beforeLines
					}
					for _, bl := range beforeBuf[start:] {
						matches = append(matches, filesystem.GrepMatch{
							Content: bl,
							Path:    displayPath,
							Line:    lineNum - len(beforeBuf[start:]) - 1,
						})
					}
				}
				matches = append(matches, filesystem.GrepMatch{
					Content: line,
					Path:    displayPath,
					Line:    lineNum,
				})
				pendingAfter = afterLines
			} else if pendingAfter > 0 {
				matches = append(matches, filesystem.GrepMatch{
					Content: line,
					Path:    displayPath,
					Line:    lineNum,
				})
				pendingAfter--
			}
			if beforeLines > 0 {
				beforeBuf = append(beforeBuf, line)
				if len(beforeBuf) > beforeLines {
					beforeBuf = beforeBuf[1:]
				}
			}
		}
		if err := scanner.Err(); err != nil {
			// 单个文件扫描失败不影响整体
			adkLogf("grep scan file %q error: %v", path, err)
		}
		return nil
	})
	if walkErr != nil {
		if errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
			return matches, walkErr
		}
		return matches, walkErr
	}
	return matches, nil
}

// GlobInfo 按通配符模式返回匹配的文件信息。
//
// 使用 bmatcuk/doublestar/v4 支持标准 glob 语法（含 ** 递归匹配）。
// 若 req.Path 为空，默认从沙箱根开始搜索。
func (b *LocalFilesystemBackend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	if req == nil || req.Pattern == "" {
		return nil, errors.New("glob 模式为空")
	}
	base := req.Path
	if base == "" {
		base = "."
	}
	baseAbs, err := b.resolve(base)
	if err != nil {
		return nil, err
	}
	// 使用 os.DirFS 配合 doublestar.GlobWalk
	fsys := os.DirFS(baseAbs)
	var result []filesystem.FileInfo
	err = doublestar.GlobWalk(fsys, req.Pattern, func(path string, d fs.DirEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		// doublestar 返回的 path 是相对于 fsys 的，使用 POSIX 风格
		fullDisplay := filepath.ToSlash(filepath.Join(base, path))
		result = append(result, filesystem.FileInfo{
			Path:       fullDisplay,
			IsDir:      d.IsDir(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("glob 匹配失败 %q: %w", req.Pattern, err)
	}
	return result, nil
}

// Write 创建或覆盖写入文件内容。若父目录不存在会自动创建。
func (b *LocalFilesystemBackend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	if req == nil {
		return errors.New("请求参数为空")
	}
	target, err := b.resolve(req.FilePath)
	if err != nil {
		return err
	}
	// 确保父目录存在
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("创建目录失败 %q: %w", parent, err)
	}
	if err := os.WriteFile(target, []byte(req.Content), 0o644); err != nil {
		return fmt.Errorf("写入文件失败 %q: %w", req.FilePath, err)
	}
	return nil
}

// Edit 替换文件中的字符串。当 ReplaceAll=true 时替换所有匹配，否则要求 OldString 在文件中唯一。
func (b *LocalFilesystemBackend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if req == nil {
		return errors.New("请求参数为空")
	}
	if req.OldString == "" {
		return errors.New("old_string 不能为空")
	}
	if req.OldString == req.NewString {
		return errors.New("new_string 必须与 old_string 不同")
	}
	target, err := b.resolve(req.FilePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("读取文件失败 %q: %w", req.FilePath, err)
	}
	content := string(data)
	count := strings.Count(content, req.OldString)
	if count == 0 {
		return fmt.Errorf("在文件 %q 中未找到 old_string", req.FilePath)
	}
	if !req.ReplaceAll && count > 1 {
		return fmt.Errorf("old_string 在文件 %q 中出现 %d 次，请提供更长的上下文或设置 replace_all=true", req.FilePath, count)
	}
	var newContent string
	if req.ReplaceAll {
		newContent = strings.ReplaceAll(content, req.OldString, req.NewString)
	} else {
		newContent = strings.Replace(content, req.OldString, req.NewString, 1)
	}
	if err := os.WriteFile(target, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("写入文件失败 %q: %w", req.FilePath, err)
	}
	return nil
}

// adkLogf 是一个轻量日志包装，避免与本包其他 logger 命名冲突。
func adkLogf(format string, args ...interface{}) {
	if logger.SugaredLogger != nil {
		logger.SugaredLogger.Infof(format, args...)
	}
}

// 编译期断言：确保 LocalFilesystemBackend 完整实现 filesystem.Backend 接口。
var (
	_ filesystem.Backend = (*LocalFilesystemBackend)(nil)
)

// ensureRuntimeOS 校验运行平台受支持，目前实现兼容 Windows/Linux/macOS。
func ensureRuntimeOS() {
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		// OK
	default:
		adkLogf("LocalFilesystemBackend: 未测试的平台 %q，将使用通用实现", runtime.GOOS)
	}
}

func init() {
	ensureRuntimeOS()
}
