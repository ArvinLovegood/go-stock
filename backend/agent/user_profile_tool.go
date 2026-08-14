package agent

// user_profile_tool.go — 将用户画像读写暴露为 Agent 可调用的工具。
//
// 背景：画像文件（user_profile.md）存放在程序（可执行文件）所在目录的 memory
// 子目录下。DeepAgents 若用原始文件工具写画像，需依赖文件系统工具可用且沙箱
// 允许该路径；此前因工具裁剪/沙箱限制导致「user_profile.md cannot be updated」。
// 因此把画像更新收敛为专用工具，走 writeUserProfileAtomic 安全写入，任何模式
// （React/PlanExecute/DeepAgents）都能可靠更新画像。
//
// 安全：更新内容经 mergeProfileContent 只提取固定维度字段，忽略模型输出的其它
// 文本/指令，与 validateGeneratedProfile 的防注入目标一致。

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"

	"go-stock/backend/logger"
)

// profileLabels 画像固定维度（与 validateGeneratedProfile / user_profile.go 一致）。
var profileLabels = []string{
	"关注市场", "关注标的", "持仓与成本", "风险偏好",
	"常用分析维度", "偏好格式", "需规避项",
}

// userProfileTool 实现 tool.BaseTool，封装画像读写为 Agent 可调用工具。
type userProfileTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
	run         func(ctx context.Context, args string) (string, error)
}

// NewGetUserProfileTool 构造读取画像工具。
func NewGetUserProfileTool() tool.BaseTool {
	return &userProfileTool{
		name: "GetUserProfile",
		description: "读取当前用户画像（user_profile.md）内容。当用户询问'你对我的偏好/画像了解多少'、" +
			"或需要先了解用户已记录的风险偏好/关注标的/需规避项再回答时调用。返回画像内容；未生成时提示为空。",
		params: map[string]*schema.ParameterInfo{},
		run: func(ctx context.Context, args string) (string, error) {
			content := NewUserProfileLearner().Get()
			if strings.TrimSpace(content) == "" {
				return "当前尚未生成用户画像，无已记录偏好。", nil
			}
			return content, nil
		},
	}
}

// NewUpdateUserProfileTool 构造更新画像工具。
func NewUpdateUserProfileTool() tool.BaseTool {
	return &userProfileTool{
		name: "UpdateUserProfile",
		description: "更新/写入用户画像（user_profile.md）。当用户明确表达希望记住的偏好、风格或需规避项" +
			"（例如'把我的风险偏好设为稳健'、'以后回答用表格'、'我主要关注新能源板块'）时调用，将内容持久化。" +
			"内容会与现有画像合并，同名维度以本次为准。参数 content 传入形如 '- 风险偏好：稳健' 的一行或多行，或完整画像 markdown。",
		params: map[string]*schema.ParameterInfo{
			"content": {
				Type:     "string",
				Desc:     "要写入的用户画像内容：形如 '- 风险偏好：稳健' 的一行或多行（支持维度：关注市场/关注标的/持仓与成本/风险偏好/常用分析维度/偏好格式/需规避项），或完整画像 markdown。将与现有画像合并，同名维度以本次为准。",
				Required: true,
			},
		},
		run: func(ctx context.Context, args string) (string, error) {
			logger.SugaredLogger.Infof("Tool UpdateUserProfile called with args: %s", args)
			incoming := strings.TrimSpace(gjson.Get(args, "content").String())
			if incoming == "" {
				return "content 参数不能为空，请提供要写入的用户画像内容。", nil
			}
			// 若画像曾被禁用，写入后自动启用，使本次更新生效。
			if !IsUserProfileEnabled() {
				if err := SetUserProfileEnabled(true); err != nil {
					logger.SugaredLogger.Warnf("启用用户画像失败: %v", err)
				}
			}
			existing := NewUserProfileLearner().Get()
			merged := mergeProfileContent(existing, incoming)
			if err := NewUserProfileLearner().Save(merged); err != nil {
				return fmt.Sprintf("更新用户画像失败: %v", err), nil
			}
			return "用户画像已更新：\n" + merged, nil
		},
	}
}

// Info 返回工具元信息。
func (t *userProfileTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

// InvokableRun 执行工具。
func (t *userProfileTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	if t.run == nil {
		return "", fmt.Errorf("工具 %s 未实现", t.name)
	}
	return t.run(ctx, argumentsInJSON)
}

// parseProfileValues 从画像文本提取「维度：值」映射，仅保留固定维度。
func parseProfileValues(content string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimPrefix(line, "- ")
		idx := strings.Index(body, "：")
		if idx <= 0 {
			continue
		}
		label := strings.TrimSpace(body[:idx])
		if !containsStr(profileLabels, label) {
			continue
		}
		value := strings.TrimSpace(body[idx+len("："):])
		if value != "" {
			m[label] = value
		}
	}
	return m
}

// mergeProfileContent 将 incoming 合并进 existing（同名维度以 incoming 为准），
// 只输出固定维度，缺失的标为"未明确"。天然忽略模型输出的其它文本，防注入。
func mergeProfileContent(existing, incoming string) string {
	vals := parseProfileValues(existing)
	for k, v := range parseProfileValues(incoming) {
		vals[k] = v
	}
	lines := make([]string, 0, len(profileLabels))
	for _, label := range profileLabels {
		v := vals[label]
		if v == "" {
			v = "未明确"
		}
		lines = append(lines, "- "+label+"："+v)
	}
	return "## 用户画像\n" + strings.Join(lines, "\n")
}

// containsStr 判断切片是否包含指定字符串。
func containsStr(list []string, s string) bool {
	for _, it := range list {
		if it == s {
			return true
		}
	}
	return false
}

// 兼容编译期检查：确保 userProfileTool 实现 tool.BaseTool 接口。
var _ tool.BaseTool = (*userProfileTool)(nil)
