package data

import (
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/go-resty/resty/v2"
	"go-stock/backend/logger"
	"time"
)

// @Author spark
// @Date 2025/2/17 12:33
// @Desc
//-----------------------------------------------------------------------------------

type TushareApi struct {
	client *resty.Client
	config *Settings
}

func NewTushareApi(config *Settings) *TushareApi {
	return &TushareApi{
		client: resty.New(),
		config: config,
	}
}

// GetDaily tushare A股日线行情
func (receiver TushareApi) GetDaily(tsCode, startDate, endDate string, crawlTimeOut int64) string {
	logger.SugaredLogger.Debugf("tushare daily request: ts_code=%s, start_date=%s, end_date=%s", tsCode, startDate, endDate)
	fields := "ts_code,trade_date,open,high,low,close,pre_close,change,pct_chg,vol,amount"
	resp := &TushareStockBasicResponse{}
	_, err := receiver.client.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("content-type", "application/json").
		SetBody(&TushareRequest{
			ApiName: "daily",
			Token:   receiver.config.TushareToken,
			Params: map[string]any{
				"ts_code":    tsCode,
				"start_date": startDate,
				"end_date":   endDate,
			},
			Fields: fields}).
		SetResult(resp).
		Post(tushareApiUrl)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return ""
	}
	res := ""
	if resp.Data.Items != nil && len(resp.Data.Items) > 0 {
		fieldsStr := slice.JoinFunc(resp.Data.Fields, ",", func(s string) string {
			return "\"" + convertor.ToString(s) + "\""
		})
		res += fieldsStr + "\n"
		for _, item := range resp.Data.Items {
			//logger.SugaredLogger.Debugf("%s", slice.Join(item, ","))
			t := slice.JoinFunc(item, ",", func(s any) any {
				return "\"" + convertor.ToString(s) + "\""
			})
			res += t + "\n"
		}
	}
	logger.SugaredLogger.Debugf("tushare response: %s", res)
	return res
}
