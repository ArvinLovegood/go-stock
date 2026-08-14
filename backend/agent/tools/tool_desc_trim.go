package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// mcpToolWrapper 包装外部 MCP 工具，仅作为 ToolSearch 动态检索的类型标记，不改动其描述内容。
// 描述完整原样透传（不做精简），以保证模型在选择工具时能看到完整的原始语义。
// 执行仍委托给原始工具。
type mcpToolWrapper struct {
	inner tool.BaseTool
}

func (w *mcpToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w *mcpToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if it, ok := w.inner.(tool.InvokableTool); ok {
		return it.InvokableRun(ctx, argumentsInJSON, opts...)
	}
	return "", fmt.Errorf("mcpToolWrapper: 内部工具不支持 InvokableTool")
}

// MarkMCPTools 将外部 MCP 工具包装为动态检索标记（描述内容保持原样）。
func MarkMCPTools(tools []tool.BaseTool) []tool.BaseTool {
	if len(tools) == 0 {
		return tools
	}
	out := make([]tool.BaseTool, 0, len(tools))
	for _, t := range tools {
		if t == nil {
			continue
		}
		out = append(out, &mcpToolWrapper{inner: t})
	}
	return out
}

// IsDynamicTool 判断工具是否属于应放入 ToolSearch 动态检索的"大型/可选"工具。
// 当前识别外部 MCP 工具（经 MarkMCPTools 包装为 *mcpToolWrapper 的类型）。
// 核心数据工具（查询/行情/记忆/知识库/画像等）返回 false，保持常驻可见。
func IsDynamicTool(t tool.BaseTool) bool {
	if t == nil {
		return false
	}
	_, ok := t.(*mcpToolWrapper)
	return ok
}
