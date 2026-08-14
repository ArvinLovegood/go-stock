package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"go-stock/backend/logger"
	"gopkg.in/yaml.v3"
)

// skillFileName 是技能定义文件的固定文件名。
const skillFileName = "SKILL.md"

// tolerantSkillBackend 是 skill.Backend 的容错实现。
//
// 背景：eino 内置的 filesystemBackend.list() 在加载 SKILL.md 时，任意一个技能的
// frontmatter 解析失败都会让 List/Get 返回错误。该错误会经由 skill 工具的 Info()
// 传播到 NewToolNode，导致整个 DeepAgents Agent 构建工具列表失败，表现为：
//
//	[NodeRunError] failed to convert tool list from call option: ...
//	failed to unmarshal frontmatter: yaml: unmarshal errors:
//	line 2: cannot unmarshal !!seq into string
//
// 即一个手写/导入错误的 SKILL.md 会拖垮整个 Agent 运行。
//
// 本实现复用 eino 的 frontmatter 提取与解析逻辑，但在单个技能解析失败时仅记录
// 警告日志并跳过该技能，保证其余技能正常加载、Agent 不受影响。
type tolerantSkillBackend struct {
	backend filesystem.Backend
	baseDir string
}

// newTolerantSkillBackend 创建容错技能后端，扫描 baseDir 下的一级子目录 SKILL.md。
func newTolerantSkillBackend(backend filesystem.Backend, baseDir string) *tolerantSkillBackend {
	return &tolerantSkillBackend{backend: backend, baseDir: baseDir}
}

// List 列出所有可正常解析的技能 frontmatter。
// 单个技能解析失败时记录警告并跳过，永不让整个列表加载失败。
func (b *tolerantSkillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	skills, errs := b.listTolerant(ctx)
	for _, e := range errs {
		logger.SugaredLogger.Warnf("skill 中间件: 跳过格式错误的技能文件: %v", e)
	}
	matters := make([]skill.FrontMatter, 0, len(skills))
	for _, s := range skills {
		matters = append(matters, s.FrontMatter)
	}
	return matters, nil
}

// Get 按名称获取技能。仅返回可正常解析的技能；若技能被跳过则视为未找到。
func (b *tolerantSkillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	skills, errs := b.listTolerant(ctx)
	for _, e := range errs {
		logger.SugaredLogger.Warnf("skill 中间件: 跳过格式错误的技能文件: %v", e)
	}
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}
	return skill.Skill{}, fmt.Errorf("skill not found: %s", name)
}

// listTolerant 扫描 baseDir 下的一级子目录 SKILL.md，逐个加载；
// 单个技能加载失败时计入 errs 并跳过，不影响其余技能。
func (b *tolerantSkillBackend) listTolerant(ctx context.Context) ([]skill.Skill, []error) {
	entries, err := b.backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{
		Pattern: "*/" + skillFileName,
		Path:    b.baseDir,
	})
	if err != nil {
		// 目录不存在或 glob 失败：视为无技能，不阻塞 Agent。
		return nil, []error{fmt.Errorf("glob 技能文件失败 (baseDir=%s): %w", b.baseDir, err)}
	}

	var skills []skill.Skill
	var errs []error
	for _, entry := range entries {
		filePath := entry.Path
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(b.baseDir, filePath)
		}
		s, loadErr := b.loadSkillFromFile(ctx, filePath)
		if loadErr != nil {
			errs = append(errs, fmt.Errorf("加载 %s 失败: %w", filePath, loadErr))
			continue
		}
		skills = append(skills, s)
	}
	return skills, errs
}

// loadSkillFromFile 读取并解析单个 SKILL.md，逻辑与 eino filesystem_backend 一致。
func (b *tolerantSkillBackend) loadSkillFromFile(ctx context.Context, path string) (skill.Skill, error) {
	fileContent, err := b.backend.Read(ctx, &filesystem.ReadRequest{
		FilePath: path,
	})
	if err != nil {
		return skill.Skill{}, fmt.Errorf("读取文件失败: %w", err)
	}

	data := stripSkillLineNumbers(fileContent.Content)

	frontmatter, content, err := parseSkillFrontmatter(data)
	if err != nil {
		return skill.Skill{}, err
	}

	var fm skill.FrontMatter
	if err = yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return skill.Skill{}, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	return skill.Skill{
		FrontMatter:   fm,
		Content:       strings.TrimSpace(content),
		BaseDirectory: filepath.Dir(path),
	}, nil
}

// stripSkillLineNumbers 去除行号前缀（首个制表符之前的内容），与 eino 实现一致。
// 某些文件系统后端会在每行前加 "行号\t" 前缀，此处做兼容处理。
func stripSkillLineNumbers(data string) string {
	lines := strings.Split(data, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		idx := strings.Index(line, "\t")
		if idx != -1 {
			line = line[idx+1:]
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// parseSkillFrontmatter 从 SKILL.md 内容中分离 frontmatter 与正文，与 eino 实现一致。
func parseSkillFrontmatter(data string) (frontmatter string, content string, err error) {
	const delimiter = "---"
	data = strings.TrimSpace(data)
	if !strings.HasPrefix(data, delimiter) {
		return "", "", fmt.Errorf("文件未以 frontmatter 分隔符 %q 开头", delimiter)
	}
	rest := data[len(delimiter):]
	endIdx := strings.Index(rest, "\n"+delimiter)
	if endIdx == -1 {
		return "", "", fmt.Errorf("未找到 frontmatter 结束分隔符 %q", delimiter)
	}
	frontmatter = strings.TrimSpace(rest[:endIdx])
	content = rest[endIdx+len("\n"+delimiter):]
	content = strings.TrimPrefix(content, "\n")
	return frontmatter, content, nil
}
