package data

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/cryptor"
)

// DefaultSponsorAESKeyHex 与 main.checkDir 在 BuildKey 为空时的回退值一致，
// 供 ai-assistant-web 等独立进程解密本地配置中的赞助码。
const DefaultSponsorAESKeyHex = ""

// SponsorDecryptKeyHex 由主程序在启动时同步为 ldflags 注入的 BuildKey；为空则使用 DefaultSponsorAESKeyHex。
var SponsorDecryptKeyHex string

// safeAesEcbDecrypt 包装 cryptor.AesEcbDecrypt，捕获 padding 非法 / 密钥不匹配时的 panic，
// 解密失败返回 nil，避免独立进程因赞助码与 BuildKey 不匹配而崩溃。
func safeAesEcbDecrypt(encrypted, key []byte) (result []byte) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
		}
	}()
	return cryptor.AesEcbDecrypt(encrypted, key)
}

// EffectiveSponsorVipLevel 根据设置中的 sponsorCode 解析 VIP 等级，并按 vipAuthTime / vipStartTime / vipEndTime 判断是否当前有效。
// 与 app.isVip 时间判断逻辑保持一致。
func EffectiveSponsorVipLevel() (level int, active bool) {
	keyHex := strings.TrimSpace(SponsorDecryptKeyHex)
	if keyHex == "" {
		keyHex = DefaultSponsorAESKeyHex
	}
	sponsorCode := strings.TrimSpace(GetSettingConfig().SponsorCode)
	if sponsorCode == "" {
		return 0, false
	}
	encrypted, err := hex.DecodeString(sponsorCode)
	if err != nil {
		return 0, false
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return 0, false
	}
	raw := safeAesEcbDecrypt(encrypted, key)
	if len(raw) == 0 {
		return 0, false
	}
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return 0, false
	}
	lvl64, _ := convertor.ToInt(info["vipLevel"])
	lvl := int(lvl64)
	vipStartTime, err1 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipStartTime"]), time.Local)
	vipEndTime, err2 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipEndTime"]), time.Local)
	vipAuthTime, err3 := time.ParseInLocation("2006-01-02 15:04:05", convertor.ToString(info["vipAuthTime"]), time.Local)
	if err1 != nil || err2 != nil || err3 != nil {
		return lvl, false
	}
	now := time.Now()
	if now.After(vipAuthTime) && now.After(vipStartTime) && now.Before(vipEndTime) {
		return lvl, true
	}
	return lvl, false
}
