package data

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"go-stock/backend/logger"
	"time"
)

// @Author spark
// @Date 2025/6/28 21:02
// @Desc
// -----------------------------------------------------------------------------------
type SearchStockApi struct {
	words string
}

func NewSearchStockApi(words string) *SearchStockApi {
	return &SearchStockApi{words: words}
}
func (s SearchStockApi) SearchStock() map[string]any {
	url := "https://np-tjxg-g.eastmoney.com/api/smart-tag/stock/v3/pw/search-code"
	resp, err := resty.New().SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "np-tjxg-g.eastmoney.com").
		SetHeader("Origin", "https://xuangu.eastmoney.com").
		SetHeader("Referer", "https://xuangu.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetBody(fmt.Sprintf(`{
				"keyWord": "%s",
				"pageSize": 50000,
				"pageNo": 1,
				"fingerprint": "e38b5faabf9378c8238e57219f0ebc9b",
				"gids": [],
				"matchWord": "",
				"timestamp": "1751113883290349",
				"shareToGuba": false,
				"requestId": "8xTWgCDAjvQ5lmvz5mDA3Ydk2AE4yoiJ1751113883290",
				"needCorrect": true,
				"removedConditionIdList": [],
				"xcId": "xc0af28549ab330013ed",
				"ownSelectAll": false,
				"dxInfo": [],
				"extraCondition": ""
				}`, s.words)).Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("SearchStock-err:%+v", err)
		return map[string]any{}
	}
	respMap := map[string]any{}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap
}
