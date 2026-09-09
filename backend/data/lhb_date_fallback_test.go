package data

import (
	"strings"
	"testing"
)

func TestLatestLhbTradeDateFallback(t *testing.T) {
	t.Logf("回退日期: %s", LatestLhbTradeDate())
	detail := NewLhbSeatApi().GetLhbSeatDetail("600077", "")
	t.Logf("空日期查询实际使用: %s", detail.TradeDate)
	if detail.TradeDate != LatestLhbTradeDate() {
		t.Errorf("空日期未回退: got %s", detail.TradeDate)
	}
}

// 空数据时 Markdown 输出必须标注实际查询日期（回退日期），不能标今天，避免 AI 搞错数据日期
func TestLhbSeatDetailMarkdownEmptyCarriesDate(t *testing.T) {
	fallback := LatestLhbTradeDate()
	// 用一只当日极大概率未上榜的股票触发空数据分支
	md := NewLhbSeatApi().GetLhbSeatDetailToMarkdown("600000", "")
	t.Logf("空数据 Markdown: %s", md)
	if !strings.Contains(md, fallback) {
		t.Errorf("空数据输出未携带回退日期 %s", fallback)
	}
	if strings.Contains(md, "当日未上榜") && !strings.Contains(md, fallback) {
		t.Errorf("空数据输出日期错误")
	}
}
