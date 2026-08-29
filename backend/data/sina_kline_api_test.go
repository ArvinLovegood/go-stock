package data

import (
	"fmt"
	"os"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
)

func init() {
	if os.Getenv("TEST_DB_SKIP_INIT") == "1" {
		return
	}
	db.Init("../../data/stock.db")
}

func TestSinaKLineApi_GetDayKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewSinaKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "101", 10)
	logger.SugaredLogger.Infof("Sina 日K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Sina 日K数据为空")
		return
	}
	first := (*kLines)[0]
	logger.SugaredLogger.Infof("Sina 日K首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
		first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
	last := (*kLines)[len(*kLines)-1]
	logger.SugaredLogger.Infof("Sina 日K末条: day=%s open=%s close=%s high=%s low=%s vol=%s",
		last.Day, last.Open, last.Close, last.High, last.Low, last.Volume)

	todayStr := time.Now().Format("2006-01-02")
	if len(last.Day) >= 10 && last.Day[:10] == todayStr {
		t.Logf("✅ 末条数据是今天(%s)的实时K线", todayStr)
	} else {
		t.Logf("⚠️ 末条数据日期=%s, 今天=%s (可能非交易时段)", last.Day[:10], todayStr)
	}
}

func TestSinaKLineApi_TodayKLineAllPeriods(t *testing.T) {
	config := GetSettingConfig()
	api := NewSinaKLineApi(config)
	periods := []struct {
		klt   string
		label string
	}{
		{"101", "日线"}, {"102", "周线"},
	}
	for _, p := range periods {
		t.Run(p.label, func(t *testing.T) {
			data := api.GetKLineData("600519.SH", p.klt, 10)
			if data == nil || len(*data) == 0 {
				t.Errorf("[%s] 无数据", p.label)
				return
			}
			last := (*data)[len(*data)-1]
			t.Logf("[%s] klt=%s 末条: day=%s close=%s vol=%s",
				p.label, p.klt, last.Day, last.Close, last.Volume)
			todayStr := time.Now().Format("2006-01-02")
			if len(last.Day) >= 10 && last.Day[:10] == todayStr {
				fmt.Printf("  ✅ %s 包含今日数据\n", p.label)
			}
		})
	}
}

func TestSinaKLineApi_Get5MinKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewSinaKLineApi(config)
	kLines := api.GetKLineData("000001.SZ", "5", 10)
	logger.SugaredLogger.Infof("Sina 5分钟K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Sina 5分钟K数据为空")
	}
}

func TestTencentKLineApi_GetDayKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewTencentKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "101", 10)
	logger.SugaredLogger.Infof("Tencent 日K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Tencent 日K数据为空")
		return
	}
	first := (*kLines)[0]
	logger.SugaredLogger.Infof("Tencent 日K首条: day=%s open=%s close=%s high=%s low=%s vol=%s",
		first.Day, first.Open, first.Close, first.High, first.Low, first.Volume)
}

func TestTencentKLineApi_GetWeekKLine(t *testing.T) {
	config := GetSettingConfig()
	api := NewTencentKLineApi(config)
	kLines := api.GetKLineData("600519.SH", "102", 10)
	logger.SugaredLogger.Infof("Tencent 周K: 获取到 %d 条数据", len(*kLines))
	if len(*kLines) == 0 {
		t.Error("Tencent 周K数据为空")
	}
}

func TestFetchKLineWithFallback(t *testing.T) {
	result := FetchKLineWithFallback("600519.SH", "贵州茅台", "101", 10, "")
	logger.SugaredLogger.Infof("Fallback 日K: source=%s, 数据条数=%d", result.Source, len(*result.Data))
	if len(*result.Data) == 0 {
		t.Error("Fallback 日K数据为空")
	}
	if result.Source == "" {
		t.Error("Fallback 未识别数据源")
	}
}
