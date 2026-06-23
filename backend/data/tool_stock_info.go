package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/util"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("GetIndustryValuation", handleGetIndustryValuation)
	registerToolHandler("GetStockConceptInfo", handleGetStockConceptInfo)
	registerToolHandler("GetStockFinancialInfo", handleGetStockFinancialInfo)
	registerToolHandler("GetStockHolderNum", handleGetStockHolderNum)
	registerToolHandler("GetStockHistoryMoneyData", handleGetStockHistoryMoneyData)
	registerToolHandler("SetTradingPrice", handleSetTradingPrice)
	registerToolHandler("GetTdxCompanyInfo", handleGetTdxCompanyInfo)
	registerToolHandler("GetTdxFinanceInfo", handleGetTdxFinanceInfo)
	registerToolHandler("GetTdxXDXRInfo", handleGetTdxXDXRInfo)
	registerToolHandler("GetTdxCompanyCategory", handleGetTdxCompanyCategory)
	registerToolHandler("GetTdxSymbolBelongBoard", handleGetTdxSymbolBelongBoard)
}

// handleGetIndustryValuation 处理 GetIndustryValuation 工具调用
func handleGetIndustryValuation(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetIndustryValuation，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	bkName := gjson.Get(funcArguments, "bkName").String()
	res := NewStockDataApi().GetIndustryValuation(bkName)
	md := util.MarkdownTableWithTitle(bkName+"行业估值", res.Result.Data)
	//logger.SugaredLogger.Infof("%s", md)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

func handleGetTdxCompanyCategory(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetTdxCompanyCategory，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	category := gjson.Get(funcArguments, "category").String()

	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		if category == "" {
			cats := api.GetF10CategoryList(stockCode)
			if cats == nil || len(*cats) == 0 {
				return stockCode + "：获取分类列表失败"
			}
			var sb strings.Builder
			sb.WriteString("## " + stockCode + " F10可用分类列表（通达信）\n\n")
			sb.WriteString("| 序号 | 分类名称 |\n|---|---|\n")
			for i, c := range *cats {
				sb.WriteString(fmt.Sprintf("| %d | %s |\n", i+1, c.Name))
			}
			sb.WriteString("\n> 提示：传入 category 参数可获取对应分类的详细内容。\n")
			return sb.String()
		}

		section := api.GetF10CategoryContent(stockCode, category)
		if section == nil || section.Content == "" {
			return stockCode + "：分类 '" + category + "' 获取失败或内容为空"
		}
		var sb strings.Builder
		sb.WriteString("## " + stockCode + " - " + section.Name + "（通达信）\n\n")
		sb.WriteString(section.Content + "\n")
		return sb.String()
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

// handleGetTdxSymbolBelongBoard 处理 GetTdxSymbolBelongBoard 工具调用
func handleGetTdxSymbolBelongBoard(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetTdxSymbolBelongBoard，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		items := api.GetMACSymbolBelongBoard(stockCode)
		if items == nil || len(*items) == 0 {
			return stockCode + "：获取所属板块信息失败或无数据"
		}
		return util.MarkdownTableWithTitle(stockCode+" 所属板块（通达信MAC）", *items)
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

func handleGetTdxCompanyInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetTdxCompanyInfo，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		bundle := api.GetF10Data(stockCode)
		var sb strings.Builder
		sb.WriteString("## " + stockCode + " F10公司资料（通达信）\n\n")
		for _, s := range bundle.Sections {
			sb.WriteString("### " + s.Name + "\n\n")
			sb.WriteString(s.Content + "\n\n")
		}
		if bundle.Finance != nil {
			f := bundle.Finance
			sb.WriteString("### 财务摘要\n\n")
			sb.WriteString(fmt.Sprintf("| 指标 | 值 |\n|---|---|\n"))
			sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
			if f.IPODate != "" {
				sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
			}
			sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
			sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
			sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
			sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
			sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
			sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
			sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
			sb.WriteString(fmt.Sprintf("| 营业成本(万元) | %.2f |\n", f.OperatingCost))
			sb.WriteString(fmt.Sprintf("| 营业利润(万元) | %.2f |\n", f.OperatingProfit))
			sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
			sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
			sb.WriteString(fmt.Sprintf("| 资本公积金(万元) | %.2f |\n", f.CapitalReserve))
			sb.WriteString(fmt.Sprintf("| 未分配利润(万元) | %.2f |\n", f.UndistributedProfit))
			sb.WriteString("\n")
		}
		if len(bundle.XDXR) > 0 {
			sb.WriteString("### 除权除息\n\n")
			sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 |\n|---|---|---|---|---|---|\n")
			for _, x := range bundle.XDXR {
				fh := "-"
				if x.Fenhong != nil {
					fh = fmt.Sprintf("%.4f", *x.Fenhong)
				}
				szg := "-"
				if x.Songzhuangu != nil {
					szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
				}
				pgj := "-"
				if x.Peigujia != nil {
					pgj = fmt.Sprintf("%.2f", *x.Peigujia)
				}
				pg := "-"
				if x.Peigu != nil {
					pg = fmt.Sprintf("%.4f", *x.Peigu)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n", x.Date, x.Name, fh, szg, pgj, pg))
			}
			sb.WriteString("\n")
		}
		return sb.String()
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

func handleGetTdxFinanceInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetTdxFinanceInfo，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		f := api.GetFinanceInfo(stockCode)
		if f == nil {
			return stockCode + "：获取财务信息失败"
		}
		var sb strings.Builder
		sb.WriteString("## " + stockCode + " 财务信息（通达信）\n\n")
		sb.WriteString("| 指标 | 值 |\n|---|---|\n")
		sb.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", f.Code))
		if f.IPODate != "" {
			sb.WriteString(fmt.Sprintf("| 上市日期 | %s |\n", f.IPODate))
		}
		if f.UpdatedDate != "" {
			sb.WriteString(fmt.Sprintf("| 更新日期 | %s |\n", f.UpdatedDate))
		}
		sb.WriteString(fmt.Sprintf("| 每股收益 | %.4f |\n", f.EPS))
		sb.WriteString(fmt.Sprintf("| 每股净资产 | %.4f |\n", f.NetAssetsPerShare))
		sb.WriteString(fmt.Sprintf("| 流通股本(万股) | %.2f |\n", f.FloatShares))
		sb.WriteString(fmt.Sprintf("| 总股本(万股) | %.2f |\n", f.TotalShares))
		sb.WriteString(fmt.Sprintf("| 总资产(万元) | %.2f |\n", f.TotalAssets))
		sb.WriteString(fmt.Sprintf("| 流动资产(万元) | %.2f |\n", f.CurrentAssets))
		sb.WriteString(fmt.Sprintf("| 固定资产(万元) | %.2f |\n", f.FixedAssets))
		sb.WriteString(fmt.Sprintf("| 无形资产(万元) | %.2f |\n", f.IntangibleAssets))
		sb.WriteString(fmt.Sprintf("| 股东人数 | %.0f |\n", f.ShareholderCount))
		sb.WriteString(fmt.Sprintf("| 流动负债(万元) | %.2f |\n", f.CurrentLiabilities))
		sb.WriteString(fmt.Sprintf("| 长期负债(万元) | %.2f |\n", f.LongTermLiabilities))
		sb.WriteString(fmt.Sprintf("| 资本公积金(万元) | %.2f |\n", f.CapitalReserve))
		sb.WriteString(fmt.Sprintf("| 净资产(万元) | %.2f |\n", f.TotalEquity))
		sb.WriteString(fmt.Sprintf("| 营业收入(万元) | %.2f |\n", f.OperatingRevenue))
		sb.WriteString(fmt.Sprintf("| 营业成本(万元) | %.2f |\n", f.OperatingCost))
		sb.WriteString(fmt.Sprintf("| 应收账款(万元) | %.2f |\n", f.AccountsReceivable))
		sb.WriteString(fmt.Sprintf("| 营业利润(万元) | %.2f |\n", f.OperatingProfit))
		sb.WriteString(fmt.Sprintf("| 投资收益(万元) | %.2f |\n", f.InvestmentIncome))
		sb.WriteString(fmt.Sprintf("| 净现金流(万元) | %.2f |\n", f.NetCashFlow))
		sb.WriteString(fmt.Sprintf("| 存货(万元) | %.2f |\n", f.Inventory))
		sb.WriteString(fmt.Sprintf("| 利润总额(万元) | %.2f |\n", f.TotalProfit))
		sb.WriteString(fmt.Sprintf("| 税后利润(万元) | %.2f |\n", f.AfterTaxProfit))
		sb.WriteString(fmt.Sprintf("| 净利润(万元) | %.2f |\n", f.NetProfit))
		sb.WriteString(fmt.Sprintf("| 未分配利润(万元) | %.2f |\n", f.UndistributedProfit))
		sb.WriteString("\n")
		return sb.String()
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

func handleGetTdxXDXRInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetTdxXDXRInfo，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewTdxKLineApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		items := api.GetXDXRInfo(stockCode)
		if items == nil || len(*items) == 0 {
			return stockCode + "：暂无除权除息数据"
		}
		var sb strings.Builder
		sb.WriteString("## " + stockCode + " 除权除息信息（通达信）\n\n")
		sb.WriteString("| 日期 | 类别 | 分红(每股) | 送转股 | 配股价 | 配股 | 缩股 | 变动前流通股本 | 变动前总股本 | 变动后流通股本 | 变动后总股本 |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, x := range *items {
			fh := "-"
			if x.Fenhong != nil {
				fh = fmt.Sprintf("%.4f", *x.Fenhong)
			}
			szg := "-"
			if x.Songzhuangu != nil {
				szg = fmt.Sprintf("%.4f", *x.Songzhuangu)
			}
			pgj := "-"
			if x.Peigujia != nil {
				pgj = fmt.Sprintf("%.2f", *x.Peigujia)
			}
			pg := "-"
			if x.Peigu != nil {
				pg = fmt.Sprintf("%.4f", *x.Peigu)
			}
			sg := "-"
			if x.Suogu != nil {
				sg = fmt.Sprintf("%.4f", *x.Suogu)
			}
			preFS := "-"
			if x.PreFloatShares != nil {
				preFS = fmt.Sprintf("%.2f", *x.PreFloatShares)
			}
			preTS := "-"
			if x.PreTotalShares != nil {
				preTS = fmt.Sprintf("%.2f", *x.PreTotalShares)
			}
			postFS := "-"
			if x.PostFloatShares != nil {
				postFS = fmt.Sprintf("%.2f", *x.PostFloatShares)
			}
			postTS := "-"
			if x.PostTotalShares != nil {
				postTS = fmt.Sprintf("%.2f", *x.PostTotalShares)
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
				x.Date, x.Name, fh, szg, pgj, pg, sg, preFS, preTS, postFS, postTS))
		}
		sb.WriteString("\n")
		return sb.String()
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

func handleSetTradingPrice(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：SetTradingPrice，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	stockCode := gjson.Get(funcArguments, "stockCode").String()
	entryPrice := gjson.Get(funcArguments, "entryPrice").Float()
	takeProfitPrice := gjson.Get(funcArguments, "takeProfitPrice").Float()
	stopLossPrice := gjson.Get(funcArguments, "stopLossPrice").Float()
	costPrice := gjson.Get(funcArguments, "costPrice").Float()

	result := NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)

	var content string
	if result == "设置成功" {
		content = fmt.Sprintf("✅ 价位线设置成功！\n\n📈 %s\n💰 开仓价：%.2f\n🎯 止盈价：%.2f\n🛑 止损价：%.2f\n💵 成本价：%.2f", stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice)
	} else {
		content = fmt.Sprintf("❌ 价位线设置失败：%s", result)
	}

	//logger.SugaredLogger.Infof("%s", content)

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		content,
	)

	return nil
}

// handleGetStockConceptInfo 处理 GetStockConceptInfo 工具调用
func handleGetStockConceptInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetStockConceptInfo，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "code")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 code 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewStockDataApi()
	md := parallelStockToolSections(codes, func(code string) string {
		res := api.GetStockConceptInfo(code)
		return util.MarkdownTableWithTitle(code+" 股票所属概念详细信息", res.Result.Data)
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

// handleGetStockFinancialInfo 处理 GetStockFinancialInfo 工具调用
func handleGetStockFinancialInfo(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetStockFinancialInfo，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewStockDataApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		res := api.GetStockFinancialInfo(stockCode)
		return util.MarkdownTableWithTitle("股票"+stockCode+"财务报表信息", res.Result.Data)
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

// handleGetStockHolderNum 处理 GetStockHolderNum 工具调用
func handleGetStockHolderNum(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetStockHolderNum，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewStockDataApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		res := api.GetStockHolderNum(stockCode)
		return util.MarkdownTableWithTitle("股票"+stockCode+"股东人数信息", res.Result.Data)
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}

// handleGetStockHistoryMoneyData 处理 GetStockHistoryMoneyData 工具调用
func handleGetStockHistoryMoneyData(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	ctx.Ch <- map[string]any{
		"code":              1,
		"question":          ctx.Question,
		"chatId":            ctx.StreamResponseID,
		"model":             ctx.Model,
		"reasoning_content": "\r\n```\r\n🔧 开始调用工具：GetStockHistoryMoneyData，\n参数：" + funcArguments + "\r\n```\r\n",
		"time":              time.Now().Format(time.DateTime),
	}

	codes := parseStockCodesFromToolArgs(funcArguments, "stockCode")
	if len(codes) == 0 {
		appendToolMessages(
			ctx.Messages,
			ctx.CurrentAIContent.String(),
			ctx.ReasoningContentText.String(),
			ctx.CurrentCallID,
			ctx.FuncName,
			funcArguments,
			"参数 stockCode 或 stockCodes 不能为空，请传入股票代码（多只可用英文逗号分隔）。",
		)
		return nil
	}

	api := NewStockDataApi()
	md := parallelStockToolSections(codes, func(stockCode string) string {
		res := api.GetStockHistoryMoneyData(stockCode)
		return util.MarkdownTableWithTitle("股票"+stockCode+"历史资金流向数据", res)
	})

	appendToolMessages(
		ctx.Messages,
		ctx.CurrentAIContent.String(),
		ctx.ReasoningContentText.String(),
		ctx.CurrentCallID,
		ctx.FuncName,
		funcArguments,
		md,
	)

	return nil
}
