package data

import (
	"testing"

	"go-stock/backend/db"
)

// TestSponsorVipProbe 复现生产构建的赞助码门控：注入与 build-prod.bat 相同的 BuildKey，
// 读取 D:/go-stock/data/stock.db（cleanup_stock_codes_test.go 的 init 已指向该库）中的赞助码，
// 输出 EffectiveSponsorVipStatus 的完整判定（等级/生效/原因/时间）。
func TestSponsorVipProbe(t *testing.T) {
	SponsorDecryptKeyHex = "8171b192a21b4d95a42fdcd54478e3ed"
	if db.Dao == nil {
		t.Skip("db 未初始化")
	}
	st := EffectiveSponsorVipStatus()
	t.Logf("level=%d active=%v reason=%q user=%q", st.Level, st.Active, st.Reason, st.User)
	t.Logf("startTime=%q endTime=%q authTime=%q", st.StartTime, st.EndTime, st.AuthTime)
}
