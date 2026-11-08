package agent

import (
	"testing"
)

func TestClassifyComplexityIntentRouting(t *testing.T) {
	tests := []struct {
		question string
		expected Mode
	}{
		{"今天茅台股价多少", React},
		{"查询一下平安银行的代码", React},
		{"全面分析贵州茅台的投资价值", PlanExecute},
		{"综合分析当前市场热点和投资机会", PlanExecute},
		{"帮我查一下今天大盘行情", React},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", PlanExecute},
		// DeepAgents 自动升级：显式深度研究/多步/报告产出信号
		{"深度研究宁德时代、比亚迪和阳光电源的竞争格局，并写一份对比研究报告", DeepAgents},
		{"分步执行：先筛选低估值蓝筹，再分析主力资金流向，最后生成投资报告", DeepAgents},
		// DeepAgents 自动升级：综合报告类 + 超长（>120 字）且无多主体词
		{"请全面分析贵州茅台的投资价值，从公司基本面、财务状况、估值水平、行业竞争格局、产品提价能力、渠道变革、管理层战略、品牌护城河、潜在风险等角度展开详细分析，再结合当前市场环境与流动性状况给出明确的操作建议、目标价区间和仓位管理参考，并说明主要假设的局限性。", DeepAgents},
		// DeepAgents 自动升级：综合报告类 + 多主体交叉
		{"全面对比宁德时代和比亚迪这两家公司的投资价值，分别从财务、估值、行业地位多维度展开详细分析并给出操作建议", DeepAgents},
	}
	for _, tt := range tests {
		got := classifyComplexity(tt.question)
		if got != tt.expected {
			t.Errorf("classifyComplexity(%q) = %s, want %s", tt.question, got, tt.expected)
		}
	}
}
