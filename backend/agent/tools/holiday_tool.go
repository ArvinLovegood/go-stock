package tools

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/tidwall/gjson"
)

type HolidayTool struct {
	name        string
	description string
	params      map[string]*schema.ParameterInfo
	handler     func(args string) (string, error)
}

func NewHolidayTool(name, description string, params map[string]*schema.ParameterInfo, handler func(args string) (string, error)) *HolidayTool {
	return &HolidayTool{
		name:        name,
		description: description,
		params:      params,
		handler:     handler,
	}
}

func (t *HolidayTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(t.params),
	}, nil
}

func (t *HolidayTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	logger.SugaredLogger.Infof("Tool %s called with args: %s", t.name, argumentsInJSON)
	return t.handler(argumentsInJSON)
}

type HolidayInfo struct {
	Code    int `json:"code"`
	Holiday struct {
		Holiday       bool   `json:"holiday"`
		Name          string `json:"name"`
		Wage          int    `json:"wage"`
		Date          string `json:"date"`
		Rest          int    `json:"rest"`
		After         bool   `json:"after"`
		Target        string `json:"target"`
		TargetWeekday string `json:"targetWeekday"`
	} `json:"holiday"`
}

type HolidayYearInfo struct {
	Code    int `json:"code"`
	Holiday map[string]struct {
		Holiday       bool   `json:"holiday"`
		Name          string `json:"name"`
		Wage          int    `json:"wage"`
		Date          string `json:"date"`
		Rest          int    `json:"rest"`
		After         bool   `json:"after"`
		Target        string `json:"target"`
		TargetWeekday string `json:"targetWeekday"`
	} `json:"holiday"`
}

func GetHolidayTools() []tool.BaseTool {
	var tools []tool.BaseTool

	tools = append(tools, NewHolidayTool(
		"GetHolidayInfo",
		"查询指定日期的节假日信息。返回该日期是否为节假日、节假日名称、是否需要补班等信息。支持查询中国法定节假日（元旦、春节、清明、五一、端午、中秋、国庆等）。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：YYYY-MM-DD，如：2026-01-01。不传则查询今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			client := data.SharedHTTPClient
			apiURL := fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date)

			var result HolidayInfo
			resp, err := client.R().
				SetResult(&result).
				Get(apiURL)

			if err != nil {
				return "", fmt.Errorf("查询节假日信息失败: %v", err)
			}

			if resp.StatusCode() != 200 || result.Code != 0 {
				return fmt.Sprintf("查询失败，日期：%s", date), nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s 节假日信息\n\n", date))
			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")

			if result.Holiday.Holiday {
				md.WriteString(fmt.Sprintf("| 是否节假日 | ✅ 是 |\n"))
				md.WriteString(fmt.Sprintf("| 节假日名称 | %s |\n", result.Holiday.Name))
			} else {
				md.WriteString(fmt.Sprintf("| 是否节假日 | ❌ 否（工作日） |\n"))
				if result.Holiday.Name != "" {
					md.WriteString(fmt.Sprintf("| 备注 | %s |\n", result.Holiday.Name))
				}
			}

			wageDesc := ""
			switch result.Holiday.Wage {
			case 1:
				wageDesc = "普通工作日"
			case 2:
				wageDesc = "法定节假日（加班3倍工资）"
			case 3:
				wageDesc = "法定节假日调休（加班2倍工资）"
			}
			if wageDesc != "" {
				md.WriteString(fmt.Sprintf("| 工资倍数 | %d倍（%s） |\n", result.Holiday.Wage, wageDesc))
			}

			if result.Holiday.Rest > 0 {
				md.WriteString(fmt.Sprintf("| 连休天数 | %d天 |\n", result.Holiday.Rest))
			}

			if result.Holiday.Target != "" {
				md.WriteString(fmt.Sprintf("| 关联节日 | %s |\n", result.Holiday.Target))
			}

			if result.Holiday.TargetWeekday != "" {
				md.WriteString(fmt.Sprintf("| 关联星期 | %s |\n", result.Holiday.TargetWeekday))
			}

			return md.String(), nil
		},
	))

	tools = append(tools, NewHolidayTool(
		"GetHolidayYear",
		"查询指定年份的所有节假日数据。返回该年份所有法定节假日的详细信息，包括日期、名称、连休天数、补班安排等。",
		map[string]*schema.ParameterInfo{
			"year": {
				Type:     "string",
				Desc:     "查询年份，格式：YYYY，如：2026。不传则查询当前年份",
				Required: false,
			},
		},
		func(args string) (string, error) {
			year := gjson.Get(args, "year").String()
			if year == "" {
				year = time.Now().Format("2006")
			}

			client := data.SharedHTTPClient
			apiURL := fmt.Sprintf("https://timor.tech/api/holiday/year/%s/", year)

			var result HolidayYearInfo
			resp, err := client.R().
				SetResult(&result).
				Get(apiURL)

			if err != nil {
				return "", fmt.Errorf("查询年度节假日信息失败: %v", err)
			}

			if resp.StatusCode() != 200 || result.Code != 0 {
				return fmt.Sprintf("查询失败，年份：%s", year), nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s年 节假日安排\n\n", year))

			type holidayRow struct {
				Date      string `md:"日期"`
				Name      string `md:"节日名称"`
				Rest      int    `md:"连休天数"`
				Wage      string `md:"工资倍数"`
				Target    string `md:"关联节日"`
				IsHoliday string `md:"类型"`
			}

			var rows []holidayRow
			for _, h := range result.Holiday {
				wageDesc := ""
				switch h.Wage {
				case 1:
					wageDesc = "1倍"
				case 2:
					wageDesc = "2倍"
				case 3:
					wageDesc = "3倍"
				default:
					wageDesc = fmt.Sprintf("%d倍", h.Wage)
				}

				isHoliday := "工作日"
				if h.Holiday {
					isHoliday = "节假日"
				}

				rows = append(rows, holidayRow{
					Date:      h.Date,
					Name:      h.Name,
					Rest:      h.Rest,
					Wage:      wageDesc,
					Target:    h.Target,
					IsHoliday: isHoliday,
				})
			}

			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Date < rows[j].Date
			})

			md.WriteString("| 日期 | 节日名称 | 类型 | 连休天数 | 工资倍数 | 关联节日 |\n")
			md.WriteString("|------|----------|------|----------|----------|----------|\n")
			for _, row := range rows {
				md.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %s | %s |\n",
					row.Date, row.Name, row.IsHoliday, row.Rest, row.Wage, row.Target))
			}

			return md.String(), nil
		},
	))

	tools = append(tools, NewHolidayTool(
		"GetHolidayBatch",
		"批量查询多个日期的节假日信息。适合需要一次性查询多个日期是否为节假日的场景。",
		map[string]*schema.ParameterInfo{
			"dates": {
				Type:     "string",
				Desc:     "查询日期列表，多个日期用逗号分隔，格式：YYYY-MM-DD，如：2026-01-01,2026-01-02,2026-01-03",
				Required: true,
			},
		},
		func(args string) (string, error) {
			datesStr := gjson.Get(args, "dates").String()
			if datesStr == "" {
				return "请提供要查询的日期列表", nil
			}

			dates := strings.Split(datesStr, ",")
			if len(dates) == 0 {
				return "请提供要查询的日期列表", nil
			}

			// 走年度节假日表缓存本地判断（见 data.HolidayInfoOn），避免逐日串行请求外部 API
			type batchRow struct {
				Date      string `md:"日期"`
				IsHoliday string `md:"是否节假日"`
				Name      string `md:"节日名称"`
				Wage      string `md:"工资倍数"`
			}

			var rows []batchRow

			for _, date := range dates {
				date = strings.TrimSpace(date)
				if date == "" {
					continue
				}

				t, err := time.Parse("2006-01-02", date)
				if err != nil {
					rows = append(rows, batchRow{Date: date, IsHoliday: "日期格式错误", Name: "-", Wage: "-"})
					continue
				}

				info, err := data.HolidayInfoOn(t)
				if err != nil {
					rows = append(rows, batchRow{Date: date, IsHoliday: "查询失败", Name: "-", Wage: "-"})
					continue
				}
				if info == nil {
					rows = append(rows, batchRow{Date: date, IsHoliday: "否", Name: "-", Wage: "-"})
					continue
				}

				isHoliday := "否"
				if info.IsHoliday {
					isHoliday = "是"
				}

				wageDesc := ""
				switch info.Wage {
				case 1:
					wageDesc = "1倍"
				case 2:
					wageDesc = "3倍"
				case 3:
					wageDesc = "2倍"
				default:
					wageDesc = fmt.Sprintf("%d倍", info.Wage)
				}

				rows = append(rows, batchRow{
					Date:      date,
					IsHoliday: isHoliday,
					Name:      info.Name,
					Wage:      wageDesc,
				})
			}

			var md strings.Builder
			md.WriteString("### 批量节假日查询结果\n\n")
			md.WriteString("| 日期 | 是否节假日 | 节日名称 | 工资倍数 |\n")
			md.WriteString("|------|------------|----------|----------|\n")
			for _, row := range rows {
				md.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
					row.Date, row.IsHoliday, row.Name, row.Wage))
			}

			return md.String(), nil
		},
	))

	tools = append(tools, NewHolidayTool(
		"IsTradingDay",
		"判断指定日期是否为A股交易日。规则：周一至周五且非法定节假日为交易日，周末（含调休补班）和法定节假日为休市日。",
		map[string]*schema.ParameterInfo{
			"date": {
				Type:     "string",
				Desc:     "查询日期，格式：YYYY-MM-DD，如：2026-01-02。不传则查询今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			date := gjson.Get(args, "date").String()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			parsedDate, err := time.Parse("2006-01-02", date)
			if err != nil {
				return "", fmt.Errorf("日期格式错误: %v", err)
			}

			// 走年度节假日表缓存本地判断（见 data.IsTradingDayInfo），无需请求外部 API
			trading, reason, holidayName, err := data.IsTradingDayInfo(parsedDate)
			if err != nil {
				return "", fmt.Errorf("查询节假日信息失败: %v", err)
			}

			weekday := parsedDate.Weekday()
			weekdayName := map[time.Weekday]string{
				time.Monday: "周一", time.Tuesday: "周二", time.Wednesday: "周三",
				time.Thursday: "周四", time.Friday: "周五", time.Saturday: "周六", time.Sunday: "周日",
			}[weekday]

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s 交易日判断\n\n", date))

			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")
			md.WriteString(fmt.Sprintf("| 日期 | %s |\n", date))
			md.WriteString(fmt.Sprintf("| 星期 | %s |\n", weekdayName))

			if holidayName != "" && trading {
				md.WriteString(fmt.Sprintf("| 备注 | %s（调休补班日，正常交易） |\n", holidayName))
			}

			if trading {
				md.WriteString("| 是否交易日 | ✅ 是 |\n")
				md.WriteString("| 说明 | 该日期为A股交易日，可以进行股票买卖 |\n")
			} else {
				md.WriteString("| 是否交易日 | ❌ 否 |\n")
				md.WriteString(fmt.Sprintf("| 原因 | %s |\n", reason))
				md.WriteString("| 说明 | 该日期非A股交易日，股市休市 |\n")
			}

			return md.String(), nil
		},
	))

	tools = append(tools, NewHolidayTool(
		"GetNextTradingDay",
		"获取指定日期之后的下一个A股交易日。规则：周一至周五且非法定节假日为交易日，周末（含调休补班）和法定节假日为休市日。",
		map[string]*schema.ParameterInfo{
			"startDate": {
				Type:     "string",
				Desc:     "起始日期，格式：YYYY-MM-DD，如：2026-01-01。不传则从今天开始",
				Required: false,
			},
			"days": {
				Type:     "integer",
				Desc:     "查找天数范围，默认30天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			startDate := gjson.Get(args, "startDate").String()
			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 30
			}

			var currentDate time.Time
			var err error
			if startDate == "" {
				currentDate = time.Now()
			} else {
				currentDate, err = time.Parse("2006-01-02", startDate)
				if err != nil {
					return "", fmt.Errorf("日期格式错误: %v", err)
				}
			}

			// 走年度节假日表缓存本地查找（见 data.NextTradingDayAfter），避免逐日串行请求外部 API
			var nextTradingDay *time.Time
			next, err := data.NextTradingDayAfter(currentDate, days)
			if err == nil {
				nextTradingDay = &next
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### 下一个交易日查询\n\n"))
			md.WriteString(fmt.Sprintf("**起始日期**: %s\n\n", currentDate.Format("2006-01-02")))

			if nextTradingDay != nil {
				weekdayName := map[time.Weekday]string{
					time.Monday: "周一", time.Tuesday: "周二", time.Wednesday: "周三",
					time.Thursday: "周四", time.Friday: "周五", time.Saturday: "周六", time.Sunday: "周日",
				}[nextTradingDay.Weekday()]
				md.WriteString(fmt.Sprintf("**下一个交易日**: %s（%s）\n\n",
					nextTradingDay.Format("2006-01-02"), weekdayName))
			} else {
				md.WriteString(fmt.Sprintf("**结果**: 在未来%d天内未找到交易日（%v）\n", days, err))
			}

			return md.String(), nil
		},
	))

	return tools
}
