package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/random"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/tidwall/gjson"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (o *OpenAi) NewSummaryStockNewsStreamWithTools(userQuestion string, sysPromptId *int, tools []Tool, thinking bool, history []map[string]interface{}) <-chan map[string]any {
	ch := make(chan map[string]any, 512)
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewSummaryStockNewsStream panic", err)
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine panic: %s", err)
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine panic context: model=%s timeout=%d", o.Model, o.TimeOut)
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		sysPrompt += `

【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率等）——必须调用工具获取最新财报数据
3. 新闻资讯——必须调用工具获取最新新闻
4. 宏观经济数据——必须调用工具获取最新数据
任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息。如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"，绝不能编造数据。`

		sysPrompt += "最后必须调用CreateAiRecommendStocks工具函数保存ai股票推荐记录。"

		msg := []map[string]interface{}{
			{
				"role":    "system",
				"content": sysPrompt,
			},
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":              "assistant",
			"reasoning_content": "使用工具查询",
			"content":           "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})
		wg := &sync.WaitGroup{}

		//wg.Go(func() {
		//	datas := NewMarketNewsApi().InteractiveAnswer(1, 100, "")
		//	content := util.MarkdownTableWithTitle("当前最新投资者互动数据", datas.Results)
		//	msg = append(msg, map[string]interface{}{
		//		"role":    "user",
		//		"content": "投资者互动数据",
		//	})
		//	msg = append(msg, map[string]interface{}{
		//		"role":              "assistant",
		//		"reasoning_content": "使用工具查询",
		//		"content":           content,
		//	})
		//})

		wg.Go(func() {
			var market strings.Builder
			res := NewMarketNewsApi().GetGDP()
			md := util.MarkdownTableWithTitle("国内生产总值(GDP)", res.GDPResult.Data)
			market.WriteString(md)
			res2 := NewMarketNewsApi().GetCPI()
			md2 := util.MarkdownTableWithTitle("居民消费价格指数(CPI)", res2.CPIResult.Data)
			market.WriteString(md2)
			res3 := NewMarketNewsApi().GetPPI()
			md3 := util.MarkdownTableWithTitle("工业品出厂价格指数(PPI)", res3.PPIResult.Data)
			market.WriteString(md3)
			res4 := NewMarketNewsApi().GetPMI()
			md4 := util.MarkdownTableWithTitle("采购经理人指数(PMI)", res4.PMIResult.Data)
			market.WriteString(md4)

			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "国内宏观经济数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "\n# 国内宏观经济数据：\n" + market.String(),
			})
		})

		wg.Go(func() {
			md := strings.Builder{}
			res := NewMarketNewsApi().ClsCalendar()
			for _, a := range res {
				bytes, err := json.Marshal(a)
				if err != nil {
					continue
				}
				date := gjson.Get(string(bytes), "calendar_day")
				md.WriteString("\n### 事件/会议日期：" + date.String())
				list := gjson.Get(string(bytes), "items")
				list.ForEach(func(key, value gjson.Result) bool {
					//logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
					md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
					return true
				})
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "近期重大事件/会议",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "近期重大事件/会议如下：\n" + md.String(),
			})
		})

		wg.Wait()

		//for _, m := range TrimAiAssistantHistoryForAPI(history) {
		//	msg = append(msg, m)
		//}
		if userQuestion == "" {
			userQuestion = "请根据当前时间，总结和分析股票市场新闻中的投资机会"
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": userQuestion,
		})
		AskAiWithTools(o, errors.New(""), msg, ch, userQuestion, tools, thinking)
	}()
	return ch
}

func (o *OpenAi) NewSummaryStockNewsStream(userQuestion string, sysPromptId *int, think bool, history []map[string]interface{}) <-chan map[string]any {
	ch := make(chan map[string]any, 512)
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewSummaryStockNewsStream panic", err)
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic :%s", err)
				logger.SugaredLogger.Errorf("NewSummaryStockNewsStream goroutine  panic  config:%s", o.String())
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		msg := []map[string]interface{}{
			{
				"role":    "system",
				"content": sysPrompt,
			},
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})
		wg := &sync.WaitGroup{}
		wg.Add(3)

		go func() {
			defer wg.Done()
			md := strings.Builder{}
			res := NewMarketNewsApi().ClsCalendar()
			for _, a := range res {
				bytes, err := json.Marshal(a)
				if err != nil {
					continue
				}
				date := gjson.Get(string(bytes), "calendar_day")
				md.WriteString("\n### 事件/会议日期：" + date.String())
				list := gjson.Get(string(bytes), "items")
				list.ForEach(func(key, value gjson.Result) bool {
					//logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
					md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
					return true
				})
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "近期重大事件/会议",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "近期重大事件/会议如下：\n" + md.String(),
			})
		}()

		go func() {
			defer wg.Done()
			datas := NewMarketNewsApi().InteractiveAnswer(1, 100, "")
			content := util.MarkdownTableWithTitle("当前最新投资者互动数据", datas.Results)
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "投资者互动数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": content,
			})
		}()

		go func() {
			defer wg.Done()
			markdownTable := ""
			res := NewSearchStockApi("").HotStrategy()
			bytes, _ := json.Marshal(res)
			strategy := &models.HotStrategy{}
			json.Unmarshal(bytes, strategy)
			for _, data := range strategy.Data {
				data.Chg = mathutil.RoundToFloat(100*data.Chg, 2)
			}
			markdownTable = util.MarkdownTableWithTitle("当前热门选股策略", strategy.Data)
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "当前热门选股策略",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": markdownTable,
			})
		}()

		wg.Wait()

		// 资讯条数过多会单独占满 context；限制在合理范围（原先 200–1000 极易撑爆上下文）
		news := NewMarketNewsApi().GetNews24HoursList("", random.RandInt(20, 40))
		messageText := strings.Builder{}
		for _, telegraph := range *news {
			messageText.WriteString("## " + telegraph.Time + ":" + "\n")
			messageText.WriteString("### " + telegraph.Content + "\n")
		}

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "市场资讯",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": messageText.String(),
		})

		//for _, m := range TrimAiAssistantHistoryForAPI(history) {
		//	msg = append(msg, m)
		//}
		if userQuestion == "" {
			userQuestion = "请根据当前时间，总结和分析股票市场新闻中的投资机会"
		}
		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": userQuestion,
		})
		AskAi(o, errors.New(""), msg, ch, userQuestion, think)
	}()
	return ch
}

func (o *OpenAi) NewChatStream(stock, stockCode, userQuestion string, sysPromptId *int, tools []Tool, thinking bool) <-chan map[string]any {
	ch := make(chan map[string]any, 512)

	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Error("NewChatStream panic", err)
		}
	}()
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic :%s", err)
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic  stock:%s stockCode:%s", stock, stockCode)
				logger.SugaredLogger.Errorf("NewChatStream goroutine  panic  context:model=%s timeout=%d", o.Model, o.TimeOut)
			}
		}()
		defer close(ch)

		sysPrompt := ""
		if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = o.Prompt
		} else {
			sysPrompt = NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}
		if sysPrompt == "" {
			sysPrompt = o.Prompt
		}

		msg := []map[string]interface{}{
			{
				"role":    "system",
				"content": sysPrompt,
			},
		}

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": "当前时间",
		})
		msg = append(msg, map[string]interface{}{
			"role":    "assistant",
			"content": "当前本地时间是:" + time.Now().Format("2006-01-02 15:04:05"),
		})

		replaceTemplates := map[string]string{
			"{{stockName}}": RemoveAllBlankChar(stock),
			"{{stockCode}}": RemoveAllBlankChar(stockCode),
			"{stockName}":   RemoveAllBlankChar(stock),
			"{stockCode}":   RemoveAllBlankChar(stockCode),
			"stockName":     RemoveAllBlankChar(stock),
			"stockCode":     RemoveAllBlankChar(stockCode),
		}
		followedStock := NewStockDataApi().GetFollowedStockByStockCode(stockCode)
		stockData, err := NewStockDataApi().GetStockCodeRealTimeData(stockCode)
		if err == nil && len(*stockData) > 0 {
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": fmt.Sprintf("当前%s[%s]价格是多少？", stock, stockCode),
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": fmt.Sprintf("截止到%s,当前%s[%s]价格是%s", (*stockData)[0].Date+" "+(*stockData)[0].Time, stock, stockCode, (*stockData)[0].Price),
			})
		}
		if followedStock.CostPrice > 0 {
			replaceTemplates["{{costPrice}}"] = convertor.ToString(followedStock.CostPrice)
			replaceTemplates["{costPrice}"] = convertor.ToString(followedStock.CostPrice)
			replaceTemplates["costPrice"] = convertor.ToString(followedStock.CostPrice)
		}

		question := ""
		if userQuestion == "" {
			question = strutil.ReplaceWithMap(o.QuestionTemplate, replaceTemplates)
		} else {
			question = strutil.ReplaceWithMap(userQuestion, replaceTemplates)
		}

		wg := &sync.WaitGroup{}
		wg.Add(8)

		go func() {
			defer wg.Done()
			datas := NewMarketNewsApi().InteractiveAnswer(1, 100, stock)
			content := util.MarkdownTableWithTitle("当前最新投资者互动数据", datas.Results)
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "投资者互动数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           content,
			})
		}()

		go func() {
			defer wg.Done()
			var market strings.Builder
			res := NewMarketNewsApi().GetGDP()
			md := util.MarkdownTableWithTitle("国内生产总值(GDP)", res.GDPResult.Data)
			market.WriteString(md)
			res2 := NewMarketNewsApi().GetCPI()
			md2 := util.MarkdownTableWithTitle("居民消费价格指数(CPI)", res2.CPIResult.Data)
			market.WriteString(md2)
			res3 := NewMarketNewsApi().GetPPI()
			md3 := util.MarkdownTableWithTitle("工业品出厂价格指数(PPI)", res3.PPIResult.Data)
			market.WriteString(md3)
			res4 := NewMarketNewsApi().GetPMI()
			md4 := util.MarkdownTableWithTitle("采购经理人指数(PMI)", res4.PMIResult.Data)
			market.WriteString(md4)

			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "国内宏观经济数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "\n# 国内宏观经济数据：\n" + market.String(),
			})
		}()

		go func() {
			defer wg.Done()
			md := strings.Builder{}
			res := NewMarketNewsApi().ClsCalendar()
			for _, a := range res {
				bytes, err := json.Marshal(a)
				if err != nil {
					continue
				}
				date := gjson.Get(string(bytes), "calendar_day")
				md.WriteString("\n### 事件/会议日期：" + date.String())
				list := gjson.Get(string(bytes), "items")
				list.ForEach(func(key, value gjson.Result) bool {
					//logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
					md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
					return true
				})
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "近期重大事件/会议",
			})
			msg = append(msg, map[string]interface{}{
				"role":              "assistant",
				"reasoning_content": "使用工具查询",
				"content":           "近期重大事件/会议如下：\n" + md.String(),
			})
		}()

		go func() {
			defer wg.Done()
			//logger.SugaredLogger.Infof("NewChatStream getKLineData stock:%s stockCode:%s", stock, stockCode)
			if strutil.HasPrefixAny(stockCode, []string{"sz", "sh", "hk", "us", "gb_"}) {
				K := &[]KLineData{}
				//logger.SugaredLogger.Infof("NewChatStream getKLineData stock:%s stockCode:%s", stock, stockCode)
				if strutil.HasPrefixAny(stockCode, []string{"sz", "sh"}) {
					K = NewStockDataApi().GetKLineData(stockCode, "240", o.KDays)
				}
				if strutil.HasPrefixAny(stockCode, []string{"hk", "us", "gb_"}) {
					K = NewStockDataApi().GetHK_KLineData(stockCode, "day", o.KDays)
				}
				Kmap := &[]map[string]any{}
				for _, kline := range *K {
					mapk := make(map[string]any, 6)
					mapk["日期"] = kline.Day
					mapk["开盘价"] = kline.Open
					mapk["最高价"] = kline.High
					mapk["最低价"] = kline.Low
					mapk["收盘价"] = kline.Close
					Volume, _ := convertor.ToFloat(kline.Volume)
					mapk["成交量(万手)"] = Volume / 10000.00 / 100.00
					*Kmap = append(*Kmap, mapk)
				}
				jsonData, _ := json.Marshal(Kmap)
				markdownTable, _ := JSONToMarkdownTable(jsonData)
				msg = append(msg, map[string]interface{}{
					"role":    "user",
					"content": stock + "日K数据",
				})
				msg = append(msg, map[string]interface{}{
					"role":    "assistant",
					"content": "## " + stock + "日K数据如下：\n" + markdownTable,
				})
				//logger.SugaredLogger.Infof("getKLineData=\n%s", markdownTable)
			}
		}()

		go func() {
			defer wg.Done()
			messages := SearchStockPriceInfo(stock, stockCode, o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				//logger.SugaredLogger.Error("获取股票价格失败")
				ch <- map[string]any{
					"code":         1,
					"question":     question,
					"extraContent": "***❗获取股票价格失败,分析结果可能不准确***<hr>",
				}
				go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票价格失败,分析结果可能不准确")
				return
			}
			price := ""
			for _, message := range *messages {
				price += message + ";"
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "股价数据",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": "\n## " + stock + "股价数据：\n" + price,
			})
			//logger.SugaredLogger.Infof("SearchStockPriceInfo stock:%s stockCode:%s", stock, stockCode)
			//logger.SugaredLogger.Infof("SearchStockPriceInfo assistant:%s", "\n## "+stock+"股价数据：\n"+price)
		}()

		go func() {
			defer wg.Done()
			if tools != nil && len(tools) > 0 {
				return
			}
			if checkIsIndexBasic(stock) {
				return
			}
			messages := GetFinancialReportsByXUEQIU(stockCode, o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				//logger.SugaredLogger.Error("获取股票财报失败")
				ch <- map[string]any{
					"code":         1,
					"question":     question,
					"extraContent": "***❗获取股票财报失败,分析结果可能不准确***<hr>",
				}
				go runtime.EventsEmit(o.ctx, "warnMsg", "❗获取股票财报失败,分析结果可能不准确")
				return
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "财报数据",
			})
			for _, message := range *messages {
				msg = append(msg, map[string]interface{}{
					"role":    "assistant",
					"content": stock + message,
				})
			}
		}()

		go func() {
			defer wg.Done()
			messages := NewMarketNewsApi().GetNews24HoursList("", random.RandInt(80, 200))
			if messages == nil || len(*messages) == 0 {
				//logger.SugaredLogger.Error("获取市场资讯失败")
				return
			}
			var messageText strings.Builder
			for _, telegraph := range *messages {
				messageText.WriteString("## " + telegraph.Time + ":" + "\n")
				messageText.WriteString("### " + telegraph.Content + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": "市场资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": messageText.String(),
			})
		}()

		go func() {
			defer wg.Done()
			messages := SearchStockInfo(stock, "telegram", o.CrawlTimeOut)
			if messages == nil || len(*messages) == 0 {
				//logger.SugaredLogger.Error("获取股票电报资讯失败")
				return
			}
			var newsText strings.Builder
			for _, message := range *messages {
				newsText.WriteString(message + "\n")
			}
			msg = append(msg, map[string]interface{}{
				"role":    "user",
				"content": stock + "相关新闻资讯",
			})
			msg = append(msg, map[string]interface{}{
				"role":    "assistant",
				"content": newsText.String(),
			})
		}()

		wg.Wait()

		msg = append(msg, map[string]interface{}{
			"role":    "user",
			"content": question,
		})

		if tools != nil && len(tools) > 0 {
			AskAiWithTools(o, errors.New(""), msg, ch, question, tools, thinking)
		} else {
			AskAi(o, errors.New(""), msg, ch, question, thinking)
		}
	}()
	return ch
}
