package data

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ===== A股交易日/节假日本地判断核心 =====
// 数据源：timor.tech 年度节假日接口（一次返回全年法定假与调休补班数据）。
// 按年缓存 24 小时（单飞 double-check 防并发击穿），交易日判断、批量查询、
// 下一个交易日查找全部本地计算，避免逐日串行请求外部 API 导致工具响应过慢
// （旧实现 GetNextTradingDay 最坏情况要串行发起 30 次单日请求）。

type holidayYearEntry struct {
	Holiday bool   `json:"holiday"` // true=法定假（休市），false=调休补班（上班）
	Name    string `json:"name"`
	Wage    int    `json:"wage"`
	Rest    int    `json:"rest"`
}

type holidayYearResp struct {
	Code    int                         `json:"code"`
	Holiday map[string]holidayYearEntry `json:"holiday"` // key: "MM-DD"
}

type holidayYearTable struct {
	entries  map[string]holidayYearEntry
	loadedAt time.Time
}

var (
	holidayYearMu     sync.Mutex
	holidayYearTables = map[string]*holidayYearTable{}
)

// holidayYearTTL 年表缓存有效期：24 小时（节假日安排年内不变，跨天刷新一次兜底补发）
const holidayYearTTL = 24 * time.Hour

// getHolidayYearTable 取某年节假日年表（带 24h 内存缓存）。
func getHolidayYearTable(year int) (map[string]holidayYearEntry, error) {
	key := strconv.Itoa(year)
	holidayYearMu.Lock()
	if t, ok := holidayYearTables[key]; ok && time.Since(t.loadedAt) < holidayYearTTL {
		entries := t.entries
		holidayYearMu.Unlock()
		return entries, nil
	}
	holidayYearMu.Unlock()

	var resp holidayYearResp
	httpResp, err := SharedHTTPClient.R().SetResult(&resp).Get(fmt.Sprintf("https://timor.tech/api/holiday/year/%s/", key))
	if err != nil {
		return nil, fmt.Errorf("查询%d年节假日数据失败: %v", year, err)
	}
	if httpResp.StatusCode() != 200 || resp.Code != 0 {
		return nil, fmt.Errorf("查询%d年节假日数据失败: HTTP %d", year, httpResp.StatusCode())
	}

	holidayYearMu.Lock()
	defer holidayYearMu.Unlock()
	// double-check：并发请求期间可能已被其它协程写入
	if t, ok := holidayYearTables[key]; ok && time.Since(t.loadedAt) < holidayYearTTL {
		return t.entries, nil
	}
	entries := resp.Holiday
	if entries == nil {
		entries = map[string]holidayYearEntry{}
	}
	holidayYearTables[key] = &holidayYearTable{entries: entries, loadedAt: time.Now()}
	return entries, nil
}

// HolidayDayInfo 单日节假日信息（节假日名称/类型/工资倍数/连休天数）。
type HolidayDayInfo struct {
	IsHoliday bool   // true=法定假（休市），false=调休补班（上班）
	Name      string // 节假日/补班名称
	Wage      int    // 工资倍数：2=法定假（3倍工资），3=调休假（2倍工资），1=普通
	Rest      int    // 连休天数
}

// fetchHolidayDayFallback 年表不可用时的降级路径：单日 info 接口（1 次请求，与旧实现持平）。
func fetchHolidayDayFallback(date string) (*HolidayDayInfo, error) {
	var resp struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool   `json:"holiday"`
			Name    string `json:"name"`
			Wage    int    `json:"wage"`
			Rest    int    `json:"rest"`
		} `json:"holiday"`
	}
	httpResp, err := SharedHTTPClient.R().SetResult(&resp).Get("https://timor.tech/api/holiday/info/" + date)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode() != 200 || resp.Code != 0 {
		return nil, fmt.Errorf("查询节假日信息失败: HTTP %d", httpResp.StatusCode())
	}
	return &HolidayDayInfo{IsHoliday: resp.Holiday.Holiday, Name: resp.Holiday.Name, Wage: resp.Holiday.Wage, Rest: resp.Holiday.Rest}, nil
}

// IsTradingDayInfo 判断某日是否 A 股交易日。
// 返回：是否交易日、休市原因（交易日为空字符串）、节假日/补班名称（如有）、错误。
// 规则：周末（含调休补班）休市；工作日遇法定节假日休市，其余为交易日。
func IsTradingDayInfo(t time.Time) (bool, string, string, error) {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false, "周末休市", "", nil
	}
	if entries, err := getHolidayYearTable(t.Year()); err == nil {
		if e, ok := entries[t.Format("01-02")]; ok {
			if e.Holiday {
				return false, "法定节假日(" + e.Name + ")", e.Name, nil
			}
			// 调休补班日（工作日正常开市），名称供输出备注使用
			return true, "", e.Name, nil
		}
		return true, "", "", nil
	}
	// 年表不可用：降级单日接口
	info, ferr := fetchHolidayDayFallback(t.Format("2006-01-02"))
	if ferr != nil {
		return false, "", "", fmt.Errorf("节假日数据获取失败: %v", ferr)
	}
	if info.IsHoliday {
		return false, "法定节假日(" + info.Name + ")", info.Name, nil
	}
	return true, "", info.Name, nil
}

// NextTradingDayAfter 返回 start 之后（不含 start 当日）的第 1 个 A 股交易日。
// 连续多日数据异常时提前中止，避免网络故障时退化成逐日请求打满 maxDays。
func NextTradingDayAfter(start time.Time, maxDays int) (time.Time, error) {
	if maxDays <= 0 {
		maxDays = 30
	}
	failCount := 0
	for i := 1; i <= maxDays; i++ {
		d := start.AddDate(0, 0, i)
		trading, _, _, err := IsTradingDayInfo(d)
		if err != nil {
			failCount++
			if failCount >= 3 {
				return time.Time{}, err
			}
			continue
		}
		failCount = 0
		if trading {
			return d, nil
		}
	}
	return time.Time{}, fmt.Errorf("未来%d天内未找到交易日", maxDays)
}

// NthTradingDayAfter 返回 start 之后（不含 start 当日）的第 n 个 A 股交易日。
func NthTradingDayAfter(start time.Time, n int) (time.Time, error) {
	if n <= 0 {
		n = 1
	}
	t := start
	for i := 0; i < n; i++ {
		next, err := NextTradingDayAfter(t, 400)
		if err != nil {
			return time.Time{}, err
		}
		t = next
	}
	return t, nil
}

// HolidayInfoOn 查询某日节假日条目（供批量查询使用）：优先年表，降级单日接口。
// 找不到条目（普通工作日/周末）时返回 nil 而非错误。
func HolidayInfoOn(t time.Time) (*HolidayDayInfo, error) {
	if entries, err := getHolidayYearTable(t.Year()); err == nil {
		if e, ok := entries[t.Format("01-02")]; ok {
			return &HolidayDayInfo{IsHoliday: e.Holiday, Name: e.Name, Wage: e.Wage, Rest: e.Rest}, nil
		}
		return nil, nil
	}
	info, ferr := fetchHolidayDayFallback(t.Format("2006-01-02"))
	if ferr != nil {
		return nil, ferr
	}
	if !info.IsHoliday && info.Name == "" {
		return nil, nil
	}
	return info, nil
}
