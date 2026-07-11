package data

import "strings"

// @Author spark
// @Date 2026/3/7 18:48
// @Desc
//-----------------------------------------------------------------------------------

// toolSchemaStockCodes 可选多只股票，与 code/stockCode 合并解析（去重）；也可仅在 code/stockCode 中用英文逗号分隔。
var toolSchemaStockCodes = map[string]any{
	"type": "array",
	"items": map[string]any{
		"type": "string",
	},
	"description": "可选，多只股票代码列表；与主字段合并后去重。也可仅在主字段中用英文逗号分隔多只。",
}

func Tools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股，支持多只股票查询（用,分隔）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "选股条件描述，支持K线形态、技术指标、财务指标等。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchBk",
			Description: "根据自然语言查询板块/概念/指数整体数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "板块/概念/指数查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchETF",
			Description: "根据自然语言查询ETF数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "ETF查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockKLine",
			Description: "获取股票日K线数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "string",
						"description": "日K数据条数",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"days", "stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLine",
			Description: "获取股票 K 线数据。支持日/周/月/季/年 K 线及 1/5/15/30/60 分钟线，可选前复权(qfq)或后复权(hfq)。股票代码格式：A股 000001.SZ、600000.SH，港股 00700.HK 等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120。",
					},
					"adjustFlag": map[string]any{
						"type":        "string",
						"description": "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（日K为天数，周K为周数，月K为月数，分钟为天数内分钟数等）。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLineWithMA",
			Description: "获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。股票代码格式同 GetEastMoneyKLine。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（如 60 表示最近 60 根）。",
					},
					"maPeriods": map[string]any{
						"type":        "string",
						"description": "均线周期，逗号分隔，如 \"5,10,20,60\"。不传则默认 5,10,20,60,120。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "InteractiveAnswer",
			Description: "获取投资者与上市公司互动问答的数据,反映当前投资者关注的热点问题",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"page": map[string]any{
						"type":        "string",
						"description": "分页号",
					},
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索关键词（可输入股票名称或者当前热门板块/行业/概念/标的/事件等）",
					},
				},
				Required: []string{"page", "pageSize"},
			},
		},
	})

	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "QueryBKDictInfo",
	//		Description: "获取所有板块/行业名称或者代码(bkCode,bkName)",
	//	},
	//})

	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "GetIndustryResearchReport",
	//		Description: "获取行业/板块研究报告,请先使用QueryBKDictInfo工具获取行业代码，然后输入行业代码调用",
	//		Parameters: FunctionParameters{
	//			Type: "object",
	//			Properties: map[string]any{
	//				"bkCode": map[string]any{
	//					"type":        "string",
	//					"description": "板块/行业代码",
	//				},
	//			},
	//			Required: []string{"bkCode"},
	//		},
	//	},
	//})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockResearchReport",
			Description: "获取市场分析师的股票研究报告。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotStrategyTable",
			Description: "获取当前热门选股策略",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotStockTable",
			Description: "当前热门股票排名",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
				},
				Required: []string{"pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMoneyData",
			Description: "今日股票资金流入排名",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
				},
				Required: []string{"pageSize"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "GetMutualTop10Deal",
			Description: "获取:北向资金（沪股通、深股通）南向资金（港股通）交易日期对应十大成交股数据（注意：当日数据 17:00–18:00 左右更新）。" +
				"MUTUAL_TYPE=001 表示沪股通十大成交股；" +
				"002 表示港股通(沪)十大成交股；" +
				"003 表示深股通十大成交股；" +
				"004 表示港股通(深)十大成交股。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"mutualType": map[string]any{
						"type": "string",
						"description": "互联互通通道类型：" +
							"001=沪股通十大成交股，" +
							"002=港股通(沪)十大成交股，" +
							"003=深股通十大成交股，" +
							"004=港股通(深)十大成交股",
					},
					"tradeDate": map[string]any{
						"type":        "string",
						"description": "交易日期，格式：YYYY-MM-DD，例如 2026-03-16（注意：当日数据 17:00–18:00 左右更新）",
					},
					"page": map[string]any{
						"type":        "number",
						"description": "页码，从 1 开始，默认 1",
					},
					"pageSize": map[string]any{
						"type":        "number",
						"description": "每页条数，默认 10",
					},
				},
				Required: []string{"mutualType", "tradeDate"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockConceptInfo",
			Description: "获取股票所属概念详细信息。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"code"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockFinancialInfo",
			Description: "获取股票财务报表信息。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockHolderNum",
			Description: "获取股票股东人数信息(股东人数与股价比( 注:股票价格通常与股东人数成反比，股东人数越少代表筹码越集中，股价越有可能上涨))。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockHistoryMoneyData",
			Description: "获取股票历史资金流向数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockRZRQInfo",
			Description: "获取股票融资融券信息，包括融资余额、融券余额、两融余额、融资净买入等。适用于 A 股两融标的。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。如：601138.SH、000001.SZ 或 sh601138、sz000001。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetIndustryValuation",
			Description: "获取行业/板块平均估值和中值（PE,PEG等）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"bkName": map[string]any{
						"type":        "string",
						"description": "行业/板块名称,如：半导体",
					},
				},
				Required: []string{"bkName"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxCompanyInfo",
			Description: "通过通达信协议获取股票F10公司资料，包括公司简介、股本结构、财务摘要、除权除息等完整信息。当东方财富F10接口不可用或需要补充数据时可使用此工具。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxFinanceInfo",
			Description: "通过通达信协议获取股票财务信息，包括每股收益、总资产、净资产、营业收入、净利润、股东人数等核心财务指标。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxXDXRInfo",
			Description: "通过通达信协议获取股票除权除息信息，包括分红、配股、送转股等历史记录及股本变动情况。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxSymbolBelongBoard",
			Description: "通过通达信MAC接口获取股票所属板块信息，包括行业板块、概念板块等，以及板块涨跌幅、涨停/跌停家数等数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾，港股以.HK结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetMACCapitalFlow",
			Description: "通过通达信MAC接口获取个股资金流向数据，包括今日主力/散户流入流出及净流入、5日主力买卖净额与超大/大/中/小单净流入（单位：元）。主要支持A股。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxCompanyCategory",
			Description: "通过通达信协议获取股票F10分类信息。不传category参数时返回所有可用分类名称列表；传入category参数时返回该分类的详细内容。可用分类包括：最新提示、公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "F10分类名称，如：公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级、最新提示。不传或为空时返回所有可用分类列表。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "CailianpressWeb",
	//		Description: "财经新闻资讯搜索",
	//		Parameters: &FunctionParameters{
	//			Type: "object",
	//			Properties: map[string]any{
	//				"searchWords": map[string]any{
	//					"type": "string",
	//					"description": "搜索关键词（不要使用分隔符如空格逗号），为空时返回最新10条新闻资讯" +
	//						"板块/概念名称：半导体\n" +
	//						"股票名称：中科曙光\n" +
	//						"政策：十五五规划\n",
	//				},
	//			},
	//			Required: []string{},
	//		},
	//	},
	//})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNewsListData",
			Description: "获取新闻资讯",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索时的关键词，可为空",
					},
					"startTime": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "每页条数（未传 page/pageSize 时生效，默认 20）",
					},
					"page": map[string]any{
						"type":        "number",
						"description": "页码，从 1 开始",
					},
					"pageSize": map[string]any{
						"type":        "number",
						"description": "每页条数，与 page 配合使用",
					},
				},
				Required: []string{"startTime"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GlobalStockIndexesReadable",
			Description: "获取全球主要指数概览，并输出为 AI 易读的 Markdown 结构化文本。",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SendToDingDing",
			Description: "将指定标题和内容以 Markdown 形式发送到钉钉机器人。用于把分析结果、摘要或通知推送到钉钉群。需在设置中开启钉钉推送并配置机器人 Webhook。通知内容需尽可能精简。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "消息标题，会显示为「go-stock {title}」",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
					},
				},
				Required: []string{"title", "message"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SendToFeishu",
			Description: "将指定标题和内容以 Markdown 卡片形式发送到飞书自定义机器人。用于把分析结果、摘要或通知推送到飞书群。需在设置中开启飞书推送并配置机器人 Webhook（可选填写签名校验 Secret）。通知内容需尽可能精简。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "消息标题，会显示为卡片标题「go-stock {title}」",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
					},
				},
				Required: []string{"title", "message"},
			},
		},
	})

	//CreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "CreateAiRecommendStocks",
			Description: "创建/保存AI推荐股票记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"modelName": map[string]any{
						"type":        "string",
						"description": "模型名称",
					},
					"rating": map[string]any{
						"type":        "string",
						"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾，",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称",
					},
					"bkCode": map[string]any{
						"type":        "string",
						"description": "板块/行业代码",
					},
					"bkName": map[string]any{
						"type":        "string",
						"description": "板块/概念/行业名称",
					},
					"stockPrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票价格",
					},
					"stockPrePrice": map[string]any{
						"type":        "string",
						"description": "前一交易日股票价格",
					},
					"stockClosePrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票收盘价格",
					},
					"recommendReason": map[string]any{
						"type":        "string",
						"description": "推荐理由/驱动因素/逻辑",
					},
					"recommendBuyPrice": map[string]any{
						"type":        "string",
						"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
					},
					"recommendBuyPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高买入价",
					},
					"recommendBuyPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低买入价",
					},
					"recommendStopProfitPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
					},
					"recommendStopProfitPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高止盈价",
					},
					"recommendStopProfitPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低止盈价",
					},

					"recommendStopLossPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止损价",
					},
					"riskRemarks": map[string]any{
						"type":        "string",
						"description": "风险提示",
					},
					"remarks": map[string]any{
						"type":        "string",
						"description": "操作总结/备注",
					},
				},
				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	//BatchCreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "BatchCreateAiRecommendStocks",
			Description: "批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stocks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"modelName": map[string]any{
									"type":        "string",
									"description": "模型名称",
								},
								"rating": map[string]any{
									"type":        "string",
									"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
								},
								"stockCode": map[string]any{
									"type":        "string",
									"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾，",
								},
								"stockName": map[string]any{
									"type":        "string",
									"description": "股票名称",
								},
								"bkCode": map[string]any{
									"type":        "string",
									"description": "板块/行业代码",
								},
								"bkName": map[string]any{
									"type":        "string",
									"description": "板块/概念/行业名称",
								},
								"stockPrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票价格",
								},
								"stockPrePrice": map[string]any{
									"type":        "string",
									"description": "前一交易日股票价格",
								},
								"stockClosePrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票收盘价格",
								},
								"recommendReason": map[string]any{
									"type":        "string",
									"description": "推荐理由/驱动因素/逻辑",
								},
								"recommendBuyPrice": map[string]any{
									"type":        "string",
									"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
								},
								"recommendBuyPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低买入价",
								},
								"recommendBuyPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高买入价",
								},
								"recommendStopProfitPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
								},
								"recommendStopProfitPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低止盈价",
								},
								"recommendStopProfitPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高止盈价",
								},
								"recommendStopLossPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止损价",
								},
								"riskRemarks": map[string]any{
									"type":        "string",
									"description": "风险提示",
								},
								"remarks": map[string]any{
									"type":        "string",
									"description": "操作总结/备注",
								},
							},
						},
					},
				},

				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "AiRecommendStocks",
			Description: "获取近期AI分析/推荐股票明细列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26 23:59:59）",
					},
					"page": map[string]any{
						"type":        "string",
						"description": "分页号（如：1）",
					},
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小(如： 1500)",
					},
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"startDate", "endDate", "page", "pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "StockNotice",
			Description: "获取上市公司公告列表。可查询一只或多只股票的最新公告（如业绩预告、重大事项、募集资金、减持、增持、监管问题、财务异常等），多只股票用英文逗号分隔。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stock_list": map[string]any{
						"type":        "string",
						"description": "股票代码，多只用英文逗号分隔。例如：600584,600900 或 002046,601138",
					},
				},
				Required: []string{"stock_list"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetSecuritiesCompanyOpinion",
			Description: "获取券商/机构的市场分析观点/要点",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26）",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCurrentTime",
			Description: "获取当前本地时间（格式：YYYY-MM-DD HH:mm:ss）及星期几",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "SetTradingPrice",
			Description: "设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数。设置后会同步到行情界面显示。" +
				"开仓价：买入的目标价格；止盈价：预期卖出获利价格；止损价：亏损到该价格时必须卖出止损。" +
				"注意：所有价格参数必须为正数，0 表示不设置该价格。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 000001.SZ、600000.SH（沪市）、00700.HK（港股）。注意：上海以.SH结尾，深圳以.SZ结尾，港股以.HK结尾，北交所以.BJ结尾。",
					},
					"entryPrice": map[string]any{
						"type":        "number",
						"description": "开仓价/买入价（目标买入价格），0 表示不设置",
					},
					"takeProfitPrice": map[string]any{
						"type":        "number",
						"description": "止盈价（预期卖出价格），0 表示不设置",
					},
					"stopLossPrice": map[string]any{
						"type":        "number",
						"description": "止损价（亏损止损价格），0 表示不设置",
					},
					"costPrice": map[string]any{
						"type":        "number",
						"description": "成本价（持仓成本价格），0 表示不设置",
					},
				},
				Required: []string{"stockCode", "entryPrice", "takeProfitPrice", "stopLossPrice", "costPrice"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetMarketData",
			Description: "获取市场行情数据，包括指数行情、涨跌分布和今日申购信息",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FilterStocks",
			Description: "根据技术指标或者关注排名或者连涨/连跌天数筛选股票。支持MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌天数等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"description": "股票名称或代码关键词搜索",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
					"macdGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "MACD金叉",
					},
					"kdjGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "KDJ金叉",
					},
					"breakThrough": map[string]any{
						"type":        "boolean",
						"description": "放量突破",
					},
					"lowFundsInflow": map[string]any{
						"type":        "boolean",
						"description": "低位资金净流入",
					},
					"highFundsOutflow": map[string]any{
						"type":        "boolean",
						"description": "高位资金净流出",
					},
					"breakUpMa5Days": map[string]any{
						"type":        "boolean",
						"description": "向上突破5日均线",
					},
					"longAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线多头排列",
					},
					"shortAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线空头排列",
					},
					"upperLargeVolume": map[string]any{
						"type":        "boolean",
						"description": "连涨放量",
					},
					"downNarrowVolume": map[string]any{
						"type":        "boolean",
						"description": "下跌无量",
					},
					"morningStar": map[string]any{
						"type":        "boolean",
						"description": "早晨之星",
					},
					"eveningStar": map[string]any{
						"type":        "boolean",
						"description": "黄昏之星",
					},
					"upNday": map[string]any{
						"type":        "integer",
						"description": "连涨天数：3/5/8天及以上",
					},
					"downNday": map[string]any{
						"type":        "integer",
						"description": "连跌天数：3/5/8/10/14天及以上",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockCodeInfo",
			Description: "查询股票/指数信息(名称、代码、拼音、交易所等)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWord": map[string]any{
						"type":        "string",
						"description": "股票搜索关键词",
					},
				},
				Required: []string{"searchWord"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockNews",
			Description: "按关键词搜索相关市场资讯/新闻(财联社)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWords": map[string]any{
						"type":        "string",
						"description": "搜索关键词(多个关键词使用空格分隔)",
					},
				},
				Required: []string{"searchWords"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockInfo",
			Description: "获取股票详细信息，包括实时行情、基本数据等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMinuteData",
			Description: "获取股票分时数据（当日分钟级成交量和价格）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChanges",
			Description: "获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"changeTypes": map[string]any{
						"type":        "string",
						"description": "异动类型，多个用逗号分隔。如：火箭发射,快速反弹,大笔买入,封涨停板,加速下跌,高台跳水,大笔卖出,封跌停板",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChangeHistoryList",
			Description: "查询股票异动历史记录。可以根据股票代码、异动类型、日期范围等条件筛选历史异动数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选，支持模糊匹配",
					},
					"changeType": map[string]any{
						"type":        "integer",
						"description": "异动类型代码",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式：YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式：YYYY-MM-DD",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFollowedStocks",
			Description: "获取用户关注/自选的股票列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupId": map[string]any{
						"type":        "integer",
						"description": "股票分组ID，不传则返回所有关注/自选的股票",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "FollowStock",
			Description: "关注（新增自选）一只股票，并可同时设置其附加信息：分组、概念标签、成本价、持仓量、止盈止损价位等。" +
				"分组/概念不存在时自动创建，概念名称忽略大小写去重；美股代码 us 前缀会被自动归一化。" +
				"若该股票已关注，仍会继续设置附加信息（幂等）。" +
				"未传的可选参数会被跳过。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 000001.SZ、sh600519、00700.HK、usaapl。上海.SH、深圳.SZ、港股.HK、北交所.BJ、美股 us 前缀。",
					},
					"groupNames": map[string]any{
						"type":        "string",
						"description": "可选，分组名称，多个用英文逗号分隔（如 白酒,消费）。不存在则自动创建。",
					},
					"conceptNames": map[string]any{
						"type":        "string",
						"description": "可选，概念标签名称，多个用英文逗号分隔（如 AI,芯片,新能源）。自动去重创建。",
					},
					"costPrice": map[string]any{
						"type":        "number",
						"description": "可选，持仓成本价，大于 0 生效。",
					},
					"volume": map[string]any{
						"type":        "integer",
						"description": "可选，持仓数量（股），大于 0 生效。",
					},
					"entryPrice": map[string]any{
						"type":        "number",
						"description": "可选，开仓价（价位线），大于 0 生效。",
					},
					"takeProfitPrice": map[string]any{
						"type":        "number",
						"description": "可选，止盈价（价位线），大于 0 生效。",
					},
					"stopLossPrice": map[string]any{
						"type":        "number",
						"description": "可选，止损价（价位线），大于 0 生效。",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisHistory",
			Description: "查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称筛选",
					},
					"question": map[string]any{
						"type":        "string",
						"description": "问题关键词搜索",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式：YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式：YYYY-MM-DD",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisDetail",
			Description: "根据ID获取历史AI分析报告的详细内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"id": map[string]any{
						"type":        "integer",
						"description": "分析报告ID",
					},
				},
				Required: []string{"id"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisContent",
			Description: "根据股票代码获取最新的AI分析报告内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotStockList",
			Description: "获取雪球热门股票排行榜",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"marketType": map[string]any{
						"type":        "string",
						"description": "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)，默认10",
					},
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotEventList",
			Description: "获取雪球热门话题/事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetIndustryMoneyRank",
			Description: "获取行业资金流向排名（按行业分类）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fenlei": map[string]any{
						"type":        "string",
						"description": "行业分类：0=所有行业,1=行业分类,2=概念板块,3=地域板块，默认1",
					},
					"sort": map[string]any{
						"type":        "string",
						"description": "排序字段：netamount=净流入,netbuy=主力净流入,change=涨跌幅，默认netamount",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetLongTigerList",
			Description: "获取龙虎榜数据（营业部排行榜）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-03-28，默认今天",
					},
				},
				Required: []string{"date"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEconomicData",
			Description: "获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dataType": map[string]any{
						"type":        "string",
						"description": "数据类型：gdp=国内生产总值,cpi=居民消费价格指数,ppi=工业生产者出厂价格指数,pmi=采购经理指数",
					},
				},
				Required: []string{"dataType"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetInvestCalendar",
			Description: "获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"yearMonth": map[string]any{
						"type":        "string",
						"description": "年月，格式：2026-03，不传则查询当月",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockNotice",
			Description: "获取个股公告信息",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCodes": map[string]any{
						"type":        "string",
						"description": "股票代码列表，逗号分隔，如：600519,000001",
					},
				},
				Required: []string{"stockCodes"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchFund",
			Description: "搜索基金信息，支持按基金代码或名称模糊搜索",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"description": "搜索关键词（基金代码或名称）",
					},
				},
				Required: []string{"keyword"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundInfo",
			Description: "获取基金详细信息，包括净值、涨跌幅、评级等",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundKLine",
			Description: "获取基金K线数据，支持多周期(日K/周K/月K/年K等)。场内基金(ETF/LOF)使用4层数据源fallback，场外基金从东方财富历史净值接口获取",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 510050(场内ETF)、000001(场外基金)",
					},
					"klt": map[string]any{
						"type":        "string",
						"description": "K线周期: 101=日K, 102=周K, 103=月K, 104=年K",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回数据条数，默认100",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundHistoryNetValue",
			Description: "获取基金历史净值数据。场外基金从东方财富API获取，场内基金(ETF/LOF)从K线收盘价换算",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
					"pageIndex": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式 YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式 YYYY-MM-DD",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFundTop10Holdings",
			Description: "获取基金前十大重仓持股信息，包括股票代码、名称、持仓占比、实时股价和涨跌幅",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fundCode": map[string]any{
						"type":        "string",
						"description": "基金代码，如 000001",
					},
				},
				Required: []string{"fundCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryIwencai",
			Description: "同花顺问财行情数据查询。支持自然语言查询股票、ETF、指数等实时价格、涨跌幅、成交量、技术指标等行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句，如：同花顺最新价格、主力资金流向、上证指数行情等",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectAStock",
			Description: "A股智能选股(同花顺i问财)。通过自然语言查询进行A股股票筛选，支持行情指标、技术形态、财务指标、行业概念等多条件组合筛选。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言选股条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectSector",
			Description: "选板块(同花顺i问财)。通过自然语言查询板块/概念信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询板块条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryMacro",
			Description: "宏观数据查询(同花顺i问财)。查询GDP、CPI、PPI、利率、汇率、社融、M2、PMI等宏观经济指标数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryZhishu",
			Description: "指数数据查询(同花顺i问财)。查询上证指数、沪深300、创业板指等指数行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryEvent",
			Description: "事件数据查询(同花顺i问财)。查询业绩预告、增发配股、股权质押、限售解禁、机构调研等事件数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchNews",
			Description: "财经新闻搜索(同花顺i问财)。搜索财经领域新闻资讯，覆盖官媒、主流财经媒体等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchInvestor",
			Description: "投资者关系活动搜索(同花顺i问财)。搜索上市公司投资者关系活动记录。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchReport",
			Description: "研报搜索(同花顺i问财)。搜索主流投研机构发布的研究报告。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryInsResearch",
			Description: "机构研究与评级查询(同花顺i问财)。查询研报评级、业绩预测、ESG评级、券商金股等机构观点数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinanceSearch",
			Description: "金融资讯搜索(东方财富妙想)。支持自然语言搜索全网最新公告、研报、财经新闻。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言搜索查询",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinancialQA",
			Description: "金融问答。针对金融领域专业问题进行回答，包括股票分析、财务指标解读、投资策略等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "金融相关问题",
					},
				},
				Required: []string{"question"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockLatestFinance",
			Description: "获取股票最新财务主要数据，包括每股收益(EPS)、每股净资产(BPS)、净资产收益率(ROE)、营业收入、净利润及同比/环比增速等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ、600000.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockQtrMainFinance",
			Description: "获取股票季度主要财务指标，包括EPS、BPS、营业收入、净利润、同比增长率、ROE、毛利率等按季度列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOrgPredict",
			Description: "获取股票机构预测数据，包括各券商/机构对未来数年的EPS和PE预测明细。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockPredictSummary",
			Description: "获取股票机构预测汇总，按年度汇总多家机构的EPS预测均值、增长率和PE估值。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockValuationPercentile",
			Description: "获取股票估值百分位数据，展示当前PE在历史30%/50%/70%分位的值，判断估值高低。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMarginTrading",
			Description: "获取股票融资融券数据，包括融资买入额、融资余额、融券卖出量、融券余额等按日列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBlockTrade",
			Description: "获取股票大宗交易数据，包括成交价、溢价率、成交金额、买方/卖方营业部等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockHolderTrend",
			Description: "获取股票户均持股趋势数据，展示股东户数和户均持股数量随时间的变化趋势。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBillboard",
			Description: "获取股票龙虎榜数据，包括上榜日期、上榜原因、买入/卖出总额等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOperationDeptTrade",
			Description: "获取股票营业部买卖明细，展示各营业部在龙虎榜上的买入/卖出金额和占比。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "ComparableCompanyAnalysis",
			Description: "可比公司分析(东方财富妙想)。对指定公司进行可比公司分析，包括财务指标对比和估值对比，帮助判断公司相对估值水平。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "公司名称或股票代码，如：贵州茅台、东方财富",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotspotDiscovery",
			Description: "市场热点发现(东方财富妙想)。发现当前A股市场热点板块和题材，包括热点逻辑分析和相关个股。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "热点的自然语言描述，如：今日热点、新能源热点、AI概念热点",
					},
				},
				Required: []string{"question"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitLadder",
			Description: "获取连板梯队数据，包括连板统计和连板梯队详情。适用于分析连板高度、市场情绪、龙头股识别等场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotPlates",
			Description: "获取涨停热门板块数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotStocks",
			Description: "获取涨停热门个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitExplodedStocks",
			Description: "获取炸板(封板失败)个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyChangeStats",
			Description: "获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetChangeRank",
			Description: "获取异动排行数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"tradeDate": map[string]any{
						"type":        "string",
						"description": "交易日期，格式：YYYY-MM-DD",
					},
					"changeType": map[string]any{
						"type":        "string",
						"description": "异动类型",
					},
				},
				Required: []string{"tradeDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayInfo",
			Description: "查询指定日期的节假日信息。返回该日期是否为节假日、节假日名称、是否需要补班等信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "IsTradingDay",
			Description: "判断指定日期是否为A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNextTradingDay",
			Description: "获取下一个A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "起始日期，格式：YYYY-MM-DD。不传则从今天开始",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "获取N个交易日后的日期",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnLives",
			Description: "获取华尔街见闻实时快讯。支持全球7x24、A股、美股、港股、外汇、商品、黄金、原油、债券、加密货币等频道。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"channel": map[string]any{
						"type":        "string",
						"description": "频道：global-channel=全球7x24, a-stock-channel=A股, us-stock-channel=美股, hk-stock-channel=港股, forex-channel=外汇, commodity-channel=商品, goldc-channel=黄金, oil-channel=原油, bond-channel=债券, crypto-channel=加密货币。默认global-channel",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "条数，默认20，最大50",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnMarketReal",
			Description: "获取华尔街见闻全球实时行情报价。包含美元指数、欧元/美元、美元/日元、离岸人民币、现货黄金、WTI原油等品种。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCodes": map[string]any{
						"type":        "string",
						"description": "品种代码(逗号分隔)，可选：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油。留空返回全部。",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnKline",
			Description: "获取华尔街见闻K线数据。支持美元指数、外汇、黄金、原油等品种的各周期K线。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCode": map[string]any{
						"type":        "string",
						"description": "品种代码：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油",
					},
					"periodType": map[string]any{
						"type":        "integer",
						"description": "K线周期(秒)：60=1分钟, 300=5分钟, 900=15分钟, 1800=30分钟, 3600=1小时, 14400=4小时, 86400=日线。默认300",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "K线条数，默认50",
					},
				},
				Required: []string{"prodCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnCalendar",
			Description: "获取华尔街见闻财经日历。包含全球重要经济数据公布时间、预期值、前值等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查看未来几天内的财经日历，默认3天",
					},
				},
			},
		},
	})

	tools = appendAgentParityTools(tools)

	// 根据 API Key 配置过滤工具，未配置对应 Key 的工具不注册
	tools = FilterToolsByApiKey(tools)

	return tools
}

type dataToolGroup string

const (
	dataToolGroupBase          dataToolGroup = "base"
	dataToolGroupStockAnalysis dataToolGroup = "stock_analysis"
	dataToolGroupMarket        dataToolGroup = "market"
	dataToolGroupScreening     dataToolGroup = "screening"
	dataToolGroupMoneyFlow     dataToolGroup = "money_flow"
	dataToolGroupNewsResearch  dataToolGroup = "news_research"
	dataToolGroupAIAnalysis    dataToolGroup = "ai_analysis"
	dataToolGroupOperations    dataToolGroup = "operations"
)

var dataToolGroupMap = map[string]dataToolGroup{
	"QueryStockCodeInfo": dataToolGroupBase,
	"GetCurrentTime":     dataToolGroupBase,
	"GetHolidayInfo":     dataToolGroupBase,
	"GetHolidayYear":     dataToolGroupBase,
	"GetHolidayBatch":    dataToolGroupBase,
	"IsTradingDay":       dataToolGroupBase,
	"GetNextTradingDay":  dataToolGroupBase,
	"GetFollowedStocks":  dataToolGroupBase,

	"GetStockInfo":                dataToolGroupStockAnalysis,
	"GetStockKLine":               dataToolGroupStockAnalysis,
	"GetEastMoneyKLine":           dataToolGroupStockAnalysis,
	"GetEastMoneyKLineWithMA":     dataToolGroupStockAnalysis,
	"GetStockMinuteData":          dataToolGroupStockAnalysis,
	"GetStockFinancialInfo":       dataToolGroupStockAnalysis,
	"GetStockHolderNum":           dataToolGroupStockAnalysis,
	"GetStockRZRQInfo":            dataToolGroupStockAnalysis,
	"GetStockConceptInfo":         dataToolGroupStockAnalysis,
	"GetIndustryValuation":        dataToolGroupStockAnalysis,
	"GetTdxCompanyInfo":           dataToolGroupStockAnalysis,
	"GetTdxFinanceInfo":           dataToolGroupStockAnalysis,
	"GetTdxXDXRInfo":              dataToolGroupStockAnalysis,
	"GetTdxCompanyCategory":       dataToolGroupStockAnalysis,
	"GetTdxSymbolBelongBoard":     dataToolGroupStockAnalysis,
	"GetStockLatestFinance":       dataToolGroupStockAnalysis,
	"GetStockQtrMainFinance":      dataToolGroupStockAnalysis,
	"GetStockOrgPredict":          dataToolGroupStockAnalysis,
	"GetStockPredictSummary":      dataToolGroupStockAnalysis,
	"GetStockValuationPercentile": dataToolGroupStockAnalysis,
	"GetStockMarginTrading":       dataToolGroupStockAnalysis,
	"GetStockBlockTrade":          dataToolGroupStockAnalysis,
	"GetStockHolderTrend":         dataToolGroupStockAnalysis,
	"GetStockBillboard":           dataToolGroupStockAnalysis,
	"GetStockOperationDeptTrade":  dataToolGroupStockAnalysis,
	"ComparableCompanyAnalysis":   dataToolGroupStockAnalysis,
	"FinancialQA":                 dataToolGroupStockAnalysis,
	"GetAIAnalysisContent":        dataToolGroupStockAnalysis,
	"GetStockResearchReport":      dataToolGroupStockAnalysis,
	"GetIndustryResearchReport":   dataToolGroupStockAnalysis,
	"InteractiveAnswer":           dataToolGroupStockAnalysis,
	"GetSecuritiesCompanyOpinion": dataToolGroupStockAnalysis,
	"StockNotice":                 dataToolGroupStockAnalysis,
	"GetStockNotice":              dataToolGroupStockAnalysis,
	"SearchInvestor":              dataToolGroupStockAnalysis,
	"SearchReport":                dataToolGroupStockAnalysis,
	"QueryInsResearch":            dataToolGroupStockAnalysis,
	"QueryBasicInfo":              dataToolGroupStockAnalysis,
	"QueryFinance":                dataToolGroupStockAnalysis,
	"QueryIndustry":               dataToolGroupStockAnalysis,
	"QueryManagement":             dataToolGroupStockAnalysis,
	"QueryFundFinance":            dataToolGroupStockAnalysis,
	"QueryBusinessData":           dataToolGroupStockAnalysis,
	"StockEarningsReview":         dataToolGroupStockAnalysis,
	"IndustryResearch":            dataToolGroupStockAnalysis,
	"TrackingReport":              dataToolGroupStockAnalysis,
	"FinanceDataQuery":            dataToolGroupStockAnalysis,

	"GetMarketData":              dataToolGroupMarket,
	"GlobalStockIndexesReadable": dataToolGroupMarket,
	"GetStockChanges":            dataToolGroupMarket,
	"GetStockChangeHistoryList":  dataToolGroupMarket,
	"GetDailyChangeStats":        dataToolGroupMarket,
	"GetChangeRank":              dataToolGroupMarket,
	"QueryIwencai":               dataToolGroupMarket,
	"QueryMacro":                 dataToolGroupMarket,
	"QueryZhishu":                dataToolGroupMarket,
	"QueryEvent":                 dataToolGroupMarket,
	"QueryFutures":               dataToolGroupMarket,
	"QueryStockConnect":          dataToolGroupMarket,
	"HotspotDiscovery":           dataToolGroupMarket,
	"GetWallstreetcnMarketReal":  dataToolGroupMarket,
	"GetWallstreetcnKline":       dataToolGroupMarket,
	"GetDailyDimensionStats":     dataToolGroupMarket,
	"GetTypeStatsByDate":         dataToolGroupMarket,

	"SearchStockByIndicators": dataToolGroupScreening,
	"SearchBk":                dataToolGroupScreening,
	"SearchETF":               dataToolGroupScreening,
	"HotStrategyTable":        dataToolGroupScreening,
	"HotStockTable":           dataToolGroupScreening,
	"FilterStocks":            dataToolGroupScreening,
	"SelectAStock":            dataToolGroupScreening,
	"SelectSector":            dataToolGroupScreening,
	"SelectETF":               dataToolGroupScreening,
	"SelectFundManager":       dataToolGroupScreening,
	"SelectConvertibleBond":   dataToolGroupScreening,
	"SelectFundCompany":       dataToolGroupScreening,
	"SelectFund":              dataToolGroupScreening,
	"SelectFuturesOption":     dataToolGroupScreening,
	"SelectHKStock":           dataToolGroupScreening,
	"SelectUSStock":           dataToolGroupScreening,

	"GetStockMoneyData":        dataToolGroupMoneyFlow,
	"GetMutualTop10Deal":       dataToolGroupMoneyFlow,
	"GetStockHistoryMoneyData": dataToolGroupMoneyFlow,
	"GetIndustryMoneyRank":     dataToolGroupMoneyFlow,
	"GetMACCapitalFlow":        dataToolGroupMoneyFlow,

	"GetNewsListData":          dataToolGroupNewsResearch,
	"QueryStockNews":           dataToolGroupNewsResearch,
	"GetInvestCalendar":        dataToolGroupNewsResearch,
	"GetLongTigerList":         dataToolGroupNewsResearch,
	"GetHotStockList":          dataToolGroupNewsResearch,
	"GetHotEventList":          dataToolGroupNewsResearch,
	"SearchNews":               dataToolGroupNewsResearch,
	"SearchAnnouncement":       dataToolGroupNewsResearch,
	"FinanceSearch":            dataToolGroupNewsResearch,
	"GetUplimitLadder":         dataToolGroupNewsResearch,
	"GetUplimitHotPlates":      dataToolGroupNewsResearch,
	"GetUplimitHotStocks":      dataToolGroupNewsResearch,
	"GetUplimitExplodedStocks": dataToolGroupNewsResearch,
	"GetUplimitPlateStocks":    dataToolGroupNewsResearch,
	"GetWallstreetcnLives":     dataToolGroupNewsResearch,
	"GetWallstreetcnCalendar":  dataToolGroupNewsResearch,

	"CreateAiRecommendStocks":      dataToolGroupAIAnalysis,
	"BatchCreateAiRecommendStocks": dataToolGroupAIAnalysis,
	"AiRecommendStocks":            dataToolGroupAIAnalysis,
	"GetAIAnalysisHistory":         dataToolGroupAIAnalysis,
	"GetAIAnalysisDetail":          dataToolGroupAIAnalysis,

	"SetTradingPrice":        dataToolGroupOperations,
	"FollowStock":            dataToolGroupOperations,
	"SendDingDingMessage":    dataToolGroupOperations,
	"SendToDingDing":         dataToolGroupOperations,
	"SendFeishuMessage":      dataToolGroupOperations,
	"SendToFeishu":           dataToolGroupOperations,
	"SearchFund":             dataToolGroupOperations,
	"GetFundInfo":            dataToolGroupOperations,
	"GetFundKLine":           dataToolGroupOperations,
	"GetFundHistoryNetValue": dataToolGroupOperations,
	"GetFundTop10Holdings":   dataToolGroupOperations,
	"GetEconomicData":        dataToolGroupOperations,

	// 分组与概念标签管理（16 个工具）
	"GetStockGroups":          dataToolGroupOperations,
	"CreateStockGroup":        dataToolGroupOperations,
	"UpdateStockGroup":        dataToolGroupOperations,
	"DeleteStockGroup":        dataToolGroupOperations,
	"AddStockToGroup":         dataToolGroupOperations,
	"RemoveStockFromGroup":    dataToolGroupOperations,
	"BatchMoveStocksToGroup":  dataToolGroupOperations,
	"GetStockConcepts":        dataToolGroupOperations,
	"CreateStockConcept":      dataToolGroupOperations,
	"UpdateStockConcept":      dataToolGroupOperations,
	"DeleteStockConcept":      dataToolGroupOperations,
	"AddStockToConcept":       dataToolGroupOperations,
	"RemoveStockFromConcept":  dataToolGroupOperations,
	"BatchAddStocksToConcept": dataToolGroupOperations,
	"MergeStockConcepts":      dataToolGroupOperations,
	"ReorganizeStockGroups":   dataToolGroupOperations,
}

type dataToolGroupKeywords struct {
	group    dataToolGroup
	keywords []string
}

var dataToolGroupKeywordsList = []dataToolGroupKeywords{
	{dataToolGroupStockAnalysis, []string{
		"股票", "股价", "个股", "行情", "K线", "k线", "日K", "周K", "月K", "分时", "实时", "价格",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS", "现金流", "负债率",
		"股东", "持股", "融资融券", "融券", "融资", "概念", "基本面", "技术面", "估值",
		"基本资料", "上市日期", "股本结构", "股东户数", "实控人", "主营业务", "主要客户",
		"供应商", "经营数据", "业绩点评", "财报分析", "行业研究", "跟踪报告", "金融数据查询",
		"研报", "研究报告", "机构预测", "券商预测", "目标价", "可比公司", "同行对比",
		"分析", "诊断", "评估", "怎么样", "怎么看", "能买吗", "值得买吗",
	}},
	{dataToolGroupMarket, []string{
		"大盘", "市场", "指数", "涨跌分布", "涨停", "跌停", "上涨家数", "下跌家数", "异动",
		"热点", "题材", "宏观", "GDP", "CPI", "PPI", "PMI", "社融", "M2", "LPR",
		"美元指数", "黄金", "原油", "外汇", "美股", "港股", "A股", "全球",
		"期货", "期权", "波动率", "持仓", "北向资金", "南向资金", "沪深港通", "AH溢价",
	}},
	{dataToolGroupScreening, []string{
		"筛选", "选股", "条件选股", "指标选股", "智能选股", "选板块", "板块排行",
		"选ETF", "ETF", "形态选股", "MACD金叉", "KDJ金叉", "放量突破", "连涨", "连跌",
		"基金经理", "基金公司", "选基金", "基金筛选", "基金排名", "可转债", "转债",
		"选期货", "选期权", "港股筛选", "美股筛选", "选港股", "选美股",
		"多头排列", "空头排列", "热门策略", "热门股票",
	}},
	{dataToolGroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出", "北向", "南向", "沪股通", "深股通",
		"港股通", "主力", "外资", "行业资金", "板块资金",
	}},
	{dataToolGroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "最新动态", "政策", "券商", "机构观点", "评级",
		"互动", "问答", "投资者关系", "调研", "财经日历", "龙虎榜", "连板", "梯队",
		"公告搜索", "分红公告", "回购公告", "重组公告", "涨停股", "涨停明细",
		"炸板", "华尔街见闻", "见闻快讯", "7x24", "非农", "美联储", "降息", "加息",
	}},
	{dataToolGroupAIAnalysis, []string{
		"AI分析", "AI推荐", "历史分析", "分析报告", "推荐股票", "买入评级", "增持", "减持",
		"止盈", "止损", "买入价",
	}},
	{dataToolGroupOperations, []string{
		"预警", "价位", "开仓", "成本价", "钉钉", "QQ", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
		"关注", "自选", "加自选", "加入分组", "设置概念", "概念标签", "归类", "持仓", "持仓量", "止盈价", "止损价",
	}},
}

func ToolsForQuestion(question string) []Tool {
	allTools := Tools(nil)
	groups := classifyDataToolGroups(question)
	return filterDataToolsByGroups(allTools, groups)
}

func classifyDataToolGroups(question string) map[dataToolGroup]bool {
	matched := map[dataToolGroup]bool{
		dataToolGroupBase: true,
	}
	lowerQ := strings.ToLower(question)
	for _, groupKeywords := range dataToolGroupKeywordsList {
		for _, keyword := range groupKeywords.keywords {
			if strings.Contains(lowerQ, strings.ToLower(keyword)) {
				matched[groupKeywords.group] = true
				break
			}
		}
	}
	if len(matched) <= 1 {
		matched[dataToolGroupStockAnalysis] = true
		matched[dataToolGroupMarket] = true
		matched[dataToolGroupNewsResearch] = true
	}
	return matched
}

func filterDataToolsByGroups(allTools []Tool, groups map[dataToolGroup]bool) []Tool {
	filtered := make([]Tool, 0, len(allTools))
	for _, tool := range allTools {
		group, exists := dataToolGroupMap[tool.Function.Name]
		if !exists || groups[group] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func appendAgentParityTools(tools []Tool) []Tool {
	for _, def := range []struct {
		name        string
		description string
	}{
		{"SendDingDingMessage", "将指定标题和内容以 Markdown 形式发送到钉钉机器人。等同于 SendToDingDing。"},
		{"SendFeishuMessage", "将指定标题和内容以 Markdown 卡片形式发送到飞书机器人。等同于 SendToFeishu。"},
	} {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        def.name,
				Description: def.description,
				Parameters: &FunctionParameters{
					Type: "object",
					Properties: map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "消息标题",
						},
						"message": map[string]any{
							"type":        "string",
							"description": "消息正文，通知内容需尽可能精简",
						},
					},
					Required: []string{"title", "message"},
				},
			},
		})
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayYear",
			Description: "查询指定年份的所有节假日数据，包括日期、名称、连休天数、补班安排等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"year": map[string]any{
						"type":        "string",
						"description": "查询年份，格式：YYYY。不传则查询当前年份",
					},
				},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayBatch",
			Description: "批量查询多个日期的节假日信息。适合需要一次性查询多个日期是否为节假日的场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dates": map[string]any{
						"type":        "string",
						"description": "查询日期列表，多个日期用逗号分隔，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"dates"},
			},
		},
	})

	for _, def := range []struct {
		name        string
		description string
	}{
		{"QueryBasicInfo", "基本资料查询。查询股票、指数、基金、期货、期权、转债、债券、理财、保险等基础信息、发行主体、机构资料、费率、上市地点、上市日期等。"},
		{"QueryFinance", "财务数据查询。查询营业收入、净利润、毛利率、净利率、ROE、ROA、负债率、现金流、市盈率、市净率等财务指标。"},
		{"QueryIndustry", "行业数据查询。查询行业估值、行业财务指标、行业盈利数据、行业行情数据、板块排名等行业维度数据。"},
		{"QueryFutures", "期货期权数据查询。查询期货期权行情、波动率、库存产销、会员持仓、榜单、行权等数据。"},
		{"SelectETF", "ETF智能筛选。按行情、跟踪指数、估值、费率、规模、份额变化等条件筛选ETF。"},
		{"QueryManagement", "公司股东股本查询。查询股本结构、股权结构、股东户数、前十大股东、实控人、质押、高管等。"},
		{"QueryStockConnect", "沪深港通资金流查询。查询北向资金、南向资金、沪股通、深股通、港股通、北向持股变动、AH溢价等。"},
		{"SelectFundManager", "智能选基金经理。根据历史业绩、管理规模、投资风格、风险控制等维度筛选基金经理。"},
		{"SelectConvertibleBond", "智能选可转债。按转股溢价率、正股表现、评级、剩余期限等条件筛选可转债。"},
		{"SelectFundCompany", "智能选基金公司。根据管理规模、旗下产品业绩、投研实力、风险评级等维度筛选基金公司。"},
		{"SelectFund", "智能选基金。根据基金类型、业绩、基金经理、风险、持仓、资产配置等维度筛选基金。"},
		{"SelectFuturesOption", "智能选期货期权。通过行情、波动率、产销、会员持仓、榜单、行权等条件筛选期货期权。"},
		{"SelectHKStock", "智能选港股。通过行情指标、财务指标、行业概念、陆港通等条件筛选港股。"},
		{"SelectUSStock", "智能选美股。通过行情指标、财务指标、行业概念、业绩预测、研报评级等条件筛选美股。"},
		{"QueryFundFinance", "基金理财查询。对基金做业绩、持仓、风险、评级、获奖、基金经理、基金公司综合分析。"},
		{"QueryBusinessData", "公司经营数据查询。查询主营业务构成、主要客户、供应商、参控股公司、股权投资、重大合同等经营数据。"},
	} {
		tools = append(tools, newQueryTool(def.name, def.description, "query"))
	}

	for _, def := range []struct {
		name        string
		description string
	}{
		{"SearchAnnouncement", "公告搜索。搜索A股、港股、基金、ETF等金融标的公告，包括定期财务报告、分红派息、回购增持、资产重组等。"},
		{"StockEarningsReview", "个股业绩点评。获取上市公司业绩点评报告，包含营收分析、利润分析、财务指标解读等深度内容。"},
		{"IndustryResearch", "行业研究报告生成。根据行业关键词生成深度行业研究报告。"},
		{"TrackingReport", "个股或行业跟踪报告。根据股票或行业关键词生成跟踪报告。"},
		{"FinanceDataQuery", "金融数据查询。基于东方财富数据库，支持自然语言查询A股、港股、美股、基金、债券等结构化金融数据。"},
	} {
		tools = append(tools, newSimpleQueryTool(def.name, def.description, "query"))
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyDimensionStats",
			Description: "按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dimension": map[string]any{
						"type":        "string",
						"description": "查询维度：stock=股票，industry=行业，concept=概念，type=异动类型",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
				Required: []string{"dimension", "name"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTypeStatsByDate",
			Description: "查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"date"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitPlateStocks",
			Description: "获取指定板块的涨停股详情，包括板块内所有涨停股票的代码、名称、连板数、封单比、成交额、市值、概念板块等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"plate_name": map[string]any{
						"type":        "string",
						"description": "板块名称，如：人工智能、机器人、芯片等",
					},
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD，默认今天",
					},
				},
				Required: []string{"plate_name"},
			},
		},
	})

	// === 分组与概念标签管理（16 个工具）===
	// 1. GetStockGroups
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockGroups",
			Description: "获取所有股票分组列表，以及每个分组下的股票代码。可用于查看分组结构、确认分组ID。",
		},
	})
	// 2. CreateStockGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "CreateStockGroup",
			Description: "创建股票分组（按名称查找/创建，已存在则幂等返回）。返回分组ID。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupName": map[string]any{
						"type":        "string",
						"description": "分组名称",
					},
				},
				Required: []string{"groupName"},
			},
		},
	})
	// 3. UpdateStockGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "UpdateStockGroup",
			Description: "重命名股票分组。需先获取分组ID（可用 GetStockGroups 查询）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupId": map[string]any{
						"type":        "integer",
						"description": "分组ID",
					},
					"groupName": map[string]any{
						"type":        "string",
						"description": "新的分组名称",
					},
				},
				Required: []string{"groupId", "groupName"},
			},
		},
	})
	// 4. DeleteStockGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "DeleteStockGroup",
			Description: "删除股票分组，级联删除该分组下的股票归属关系（不会取消关注股票本身）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupId": map[string]any{
						"type":        "integer",
						"description": "分组ID",
					},
				},
				Required: []string{"groupId"},
			},
		},
	})
	// 5. AddStockToGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "AddStockToGroup",
			Description: "将一只股票加入指定分组（按分组名查找/创建，幂等）。仅建立归属关系，不会触发关注动作。" +
				"美股代码 us 前缀会被自动归一化。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 000001.SZ、sh600519、00700.HK、usaapl。",
					},
					"groupName": map[string]any{
						"type":        "string",
						"description": "分组名称，不存在则自动创建。",
					},
				},
				Required: []string{"stockCode", "groupName"},
			},
		},
	})
	// 6. RemoveStockFromGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "RemoveStockFromGroup",
			Description: "将一只股票从指定分组中移出（仅解除归属，不会取消关注）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码",
					},
					"groupName": map[string]any{
						"type":        "string",
						"description": "分组名称",
					},
				},
				Required: []string{"stockCode", "groupName"},
			},
		},
	})
	// 7. BatchMoveStocksToGroup
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "BatchMoveStocksToGroup",
			Description: "批量将多只股票加入同一分组（按分组名查找/创建，幂等）。美股代码 us 前缀自动归一化。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCodes": map[string]any{
						"type":        "array",
						"description": "股票代码列表",
						"items": map[string]any{
							"type": "string",
						},
					},
					"groupName": map[string]any{
						"type":        "string",
						"description": "分组名称，不存在则自动创建",
					},
				},
				Required: []string{"stockCodes", "groupName"},
			},
		},
	})
	// 8. GetStockConcepts
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockConcepts",
			Description: "获取概念标签列表。可选传入 stockCode 只查该股票的概念；不传则返回全部概念-股票归属。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "可选，股票代码，传入则只返回该股票的概念标签",
					},
				},
			},
		},
	})
	// 9. CreateStockConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "CreateStockConcept",
			Description: "创建概念标签（按名称查找/去重创建，已存在则幂等返回）。返回概念ID。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"conceptName": map[string]any{
						"type":        "string",
						"description": "概念名称",
					},
				},
				Required: []string{"conceptName"},
			},
		},
	})
	// 10. UpdateStockConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "UpdateStockConcept",
			Description: "重命名概念标签。重名（忽略大小写）会被拒绝。需先获取概念ID（可用 GetStockConcepts 查询）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"conceptId": map[string]any{
						"type":        "integer",
						"description": "概念ID",
					},
					"conceptName": map[string]any{
						"type":        "string",
						"description": "新的概念名称",
					},
				},
				Required: []string{"conceptId", "conceptName"},
			},
		},
	})
	// 11. DeleteStockConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "DeleteStockConcept",
			Description: "删除概念标签，级联删除该概念下的所有股票归属关系。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"conceptId": map[string]any{
						"type":        "integer",
						"description": "概念ID",
					},
				},
				Required: []string{"conceptId"},
			},
		},
	})
	// 12. AddStockToConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "AddStockToConcept",
			Description: "为一只股票打上概念标签（按概念名查找/去重创建，幂等）。不会触发关注动作。" +
				"美股代码 us 前缀会被自动归一化。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码",
					},
					"conceptName": map[string]any{
						"type":        "string",
						"description": "概念名称，不存在则自动去重创建",
					},
				},
				Required: []string{"stockCode", "conceptName"},
			},
		},
	})
	// 13. RemoveStockFromConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "RemoveStockFromConcept",
			Description: "移除一只股票的概念标签。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码",
					},
					"conceptName": map[string]any{
						"type":        "string",
						"description": "概念名称",
					},
				},
				Required: []string{"stockCode", "conceptName"},
			},
		},
	})
	// 14. BatchAddStocksToConcept
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "BatchAddStocksToConcept",
			Description: "批量为多只股票打上同一概念标签（按概念名查找/去重创建，幂等）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCodes": map[string]any{
						"type":        "array",
						"description": "股票代码列表",
						"items": map[string]any{
							"type": "string",
						},
					},
					"conceptName": map[string]any{
						"type":        "string",
						"description": "概念名称，不存在则自动去重创建",
					},
				},
				Required: []string{"stockCodes", "conceptName"},
			},
		},
	})
	// 15. MergeStockConcepts
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "MergeStockConcepts",
			Description: "合并概念标签：将源概念下的所有股票转移到目标概念（目标不存在则创建），然后删除源概念。用于整理重复/相似的概念。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"sourceConceptName": map[string]any{
						"type":        "string",
						"description": "源概念名称（将被删除）",
					},
					"targetConceptName": map[string]any{
						"type":        "string",
						"description": "目标概念名称（将保留并接收所有股票）",
					},
				},
				Required: []string{"sourceConceptName", "targetConceptName"},
			},
		},
	})
	// 16. ReorganizeStockGroups
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "ReorganizeStockGroups",
			Description: "批量重新整理多只股票的分组归属（一次操作多只股票、多个分组）。可选先清除各股票的现有分组归属再加入新分组。" +
				"assignments 每项为 {stockCode, groupName}。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"assignments": map[string]any{
						"type":        "array",
						"description": "归属分配列表，每项含 stockCode 和 groupName",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"stockCode": map[string]any{"type": "string", "description": "股票代码"},
								"groupName": map[string]any{"type": "string", "description": "目标分组名称"},
							},
						},
					},
					"clearExisting": map[string]any{
						"type":        "boolean",
						"description": "可选，是否先清除各股票现有的全部分组归属，默认 false",
					},
				},
				Required: []string{"assignments"},
			},
		},
	})

	return tools
}

func newQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{queryField},
			},
		},
	}
}

func newSimpleQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
						"type":        "string",
						"description": "自然语言查询语句或关键词",
					},
					"reportDate": map[string]any{
						"type":        "string",
						"description": "仅 StockEarningsReview 可用：报告期，格式YYYY-MM-DD",
					},
				},
				Required: []string{queryField},
			},
		},
	}
}
