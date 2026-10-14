package data

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// OKX 行情榜单：基于公共接口（无需鉴权），供交易面板「行情榜单」区块使用。
// - gainers/losers/volume：/api/v5/market/tickers(SWAP) 本地排序，一次请求覆盖全部 USDT 永续；
// - funding：/api/v5/public/funding-rate 无批量接口，对成交额头部合约并发查询（OKX 公共接口限频约 20次/2s，8 并发可控）；
// - longshort：/api/v5/rubik/stat/contracts/long-short-account-ratio 仅支持主流币种，对成交额头部币种并发查询，失败即跳过。

// OKXRankItem 榜单行（USDT 永续）。
type OKXRankItem struct {
	InstId         string  `json:"instId"`
	Base           string  `json:"base"`           // 展示名（去掉 -USDT-SWAP 后缀）
	Last           float64 `json:"last"`           // 最新价
	ChangePct      float64 `json:"changePct"`      // 24h 涨跌幅 %
	VolUsd         float64 `json:"volUsd"`         // 24h 成交额（折 USDT）
	FundingRate    float64 `json:"fundingRate"`    // 当期资金费率 %（仅 funding 榜填充）
	NextFundingTs  int64   `json:"nextFundingTs"`  // 下次资金费结算时间（毫秒，仅 funding 榜）
	LongShortRatio float64 `json:"longShortRatio"` // 多空持仓人数比（仅 longshort 榜）
}

const rankSwapSuffix = "-USDT-SWAP"

// 榜单结果缓存：模拟盘/实盘两个面板实例 + 前端 60s 轮询共用，避免重复组装。
var (
	rankCache   sync.Map // kind -> rankCacheEntry
	rankCacheMu sync.Mutex
)

type rankCacheEntry struct {
	items     []OKXRankItem
	expiresAt time.Time
}

// GetOKXRankings 按类型返回榜单：gainers 涨幅榜 / losers 跌幅榜 / volume 成交额榜 / funding 资金费率榜 / longshort 多空比榜。
// 结果缓存 30s。
func GetOKXRankings(kind string, limit int) []OKXRankItem {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "gainers", "losers", "volume", "funding", "longshort":
	default:
		return []OKXRankItem{}
	}
	if limit <= 0 || limit > 50 {
		limit = 15
	}
	cacheKey := k + ":" + strconv.Itoa(limit)
	if e, ok := rankCache.Load(cacheKey); ok {
		if en, ok := e.(rankCacheEntry); ok && time.Now().Before(en.expiresAt) {
			return en.items
		}
	}
	var items []OKXRankItem
	switch k {
	case "gainers", "losers", "volume":
		items = rankFromTickers(k, limit)
	case "funding":
		items = rankFunding(limit)
	case "longshort":
		items = rankLongShort(limit)
	}
	if items == nil {
		items = []OKXRankItem{}
	}
	rankCache.Store(cacheKey, rankCacheEntry{items: items, expiresAt: time.Now().Add(30 * time.Second)})
	return items
}

// rankFromTickers 基于全量 SWAP 行情本地排序出涨跌幅/成交额榜。
func rankFromTickers(kind string, limit int) []OKXRankItem {
	tickers := GetOKXTickers("SWAP")
	items := make([]OKXRankItem, 0, len(tickers))
	for _, t := range tickers {
		if !strings.HasSuffix(t.InstId, rankSwapSuffix) {
			continue
		}
		last, _ := strconv.ParseFloat(t.Last, 64)
		open, _ := strconv.ParseFloat(t.Open24h, 64)
		volCcy, _ := strconv.ParseFloat(t.VolCcy24h, 64)
		if last <= 0 || open <= 0 {
			continue
		}
		items = append(items, OKXRankItem{
			InstId:    t.InstId,
			Base:      strings.TrimSuffix(t.InstId, rankSwapSuffix),
			Last:      last,
			ChangePct: (last - open) / open * 100,
			VolUsd:    volCcy * last, // SWAP 的 volCcy24h 以基础币计，折 USDT 名义
		})
	}
	switch kind {
	case "gainers":
		sort.Slice(items, func(i, j int) bool { return items[i].ChangePct > items[j].ChangePct })
	case "losers":
		sort.Slice(items, func(i, j int) bool { return items[i].ChangePct < items[j].ChangePct })
	case "volume":
		sort.Slice(items, func(i, j int) bool { return items[i].VolUsd > items[j].VolUsd })
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

// fundingRateResp /api/v5/public/funding-rate 单合约响应。
type fundingRateResp struct {
	InstId          string `json:"instId"`
	FundingRate     string `json:"fundingRate"`
	NextFundingTime string `json:"nextFundingTime"`
}

// rankFunding 资金费率榜：成交额前 60 合约并发查当期费率，按费率降序。
func rankFunding(limit int) []OKXRankItem {
	candidates := rankFromTickers("volume", 60)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sem := make(chan struct{}, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make([]OKXRankItem, 0, len(candidates))
	for _, b := range candidates {
		wg.Add(1)
		go func(b OKXRankItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			body, err := okxCachedRequest(ctx, "funding-rate:"+b.InstId, 60*time.Second,
				http.MethodGet, "/api/v5/public/funding-rate", url.Values{"instId": {b.InstId}})
			if err != nil {
				return
			}
			var fr fundingRateResp
			if err := okxData(body, &fr); err != nil {
				return
			}
			rate, _ := strconv.ParseFloat(fr.FundingRate, 64)
			next, _ := strconv.ParseInt(fr.NextFundingTime, 10, 64)
			mu.Lock()
			b.FundingRate = rate * 100
			b.NextFundingTs = next
			out = append(out, b)
			mu.Unlock()
		}(b)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].FundingRate > out[j].FundingRate })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// rankLongShort 多空持仓人数比榜：Rubik 接口仅支持主流币种，取成交额前 30 币种并发查询最新值，按多空比降序。
func rankLongShort(limit int) []OKXRankItem {
	candidates := rankFromTickers("volume", 30)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sem := make(chan struct{}, 6)
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := make([]OKXRankItem, 0, len(candidates))
	for _, b := range candidates {
		wg.Add(1)
		go func(b OKXRankItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			body, err := okxCachedRequest(ctx, "long-short:"+b.Base, 120*time.Second,
				http.MethodGet, "/api/v5/rubik/stat/contracts/long-short-account-ratio",
				url.Values{"ccy": {b.Base}, "period": {"5m"}})
			if err != nil {
				return // 非主流币种 Rubik 不支持，静默跳过
			}
			var resp struct {
				Data [][]string `json:"data"`
			}
			if err := okxData(body, &resp); err != nil || len(resp.Data) == 0 {
				return
			}
			ratio, _ := strconv.ParseFloat(resp.Data[len(resp.Data)-1][1], 64)
			if ratio <= 0 {
				return
			}
			mu.Lock()
			b.LongShortRatio = ratio
			out = append(out, b)
			mu.Unlock()
		}(b)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].LongShortRatio > out[j].LongShortRatio })
	if len(out) > limit {
		out = out[:limit]
	}
	if len(out) < limit {
		logger.SugaredLogger.Debugf("OKX多空比榜单仅覆盖 %d 个主流币种", len(out))
	}
	return out
}
