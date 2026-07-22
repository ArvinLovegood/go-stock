package data

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// DecodeSponsorInfo is the single safe seam for sponsor payload decryption.
// lancet's AES helper panics on invalid padding, so malformed data and local
// development builds without an injected key must be converted to errors.
func DecodeSponsorInfo(sponsorCode, keyHex string) (info map[string]any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			info = nil
			err = fmt.Errorf("decrypt sponsor payload: %v", recovered)
		}
	}()
	encrypted, err := hex.DecodeString(strings.TrimSpace(sponsorCode))
	if err != nil || len(encrypted) == 0 {
		return nil, fmt.Errorf("invalid sponsor payload encoding")
	}
	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil || (len(key) != 16 && len(key) != 24 && len(key) != 32) {
		return nil, fmt.Errorf("invalid sponsor decryption key")
	}
	raw := cryptor.AesEcbDecrypt(encrypted, key)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty sponsor payload")
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("decode sponsor payload: %w", err)
	}
	return info, nil
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
	info, err := DecodeSponsorInfo(sponsorCode, keyHex)
	if err != nil {
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
