package data

import (
	"encoding/json"
	"github.com/coocood/freecache"
	"github.com/tidwall/gjson"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"strings"
	"testing"
)

// @Author spark
// @Date 2025/4/23 17:58
// @Desc
//-----------------------------------------------------------------------------------

func TestGetSinaNews(t *testing.T) {
	db.Init("../../data/stock.db")
	NewMarketNewsApi().GetSinaNews(30)
	//NewMarketNewsApi().GetNewTelegraph(30)

}

func TestGlobalStockIndexes(t *testing.T) {
	resp := NewMarketNewsApi().GlobalStockIndexes(30)
	bytes, err := json.Marshal(resp)
	if err != nil {
		return
	}
	logger.SugaredLogger.Debugf("resp: %+v", string(bytes))
}

func TestGetIndustryRank(t *testing.T) {
	res := NewMarketNewsApi().GetIndustryRank("0", 10)
	for s, a := range res["data"].([]any) {
		logger.SugaredLogger.Debugf("key: %+v, value: %+v", s, a)

	}
}
func TestGetIndustryMoneyRankSina(t *testing.T) {
	res := NewMarketNewsApi().GetIndustryMoneyRankSina("0", "netamount")
	for i, re := range res {
		logger.SugaredLogger.Debugf("key: %+v, value: %+v", i, re)

	}
}
func TestGetMoneyRankSina(t *testing.T) {
	res := NewMarketNewsApi().GetMoneyRankSina("r3_net")
	for i, re := range res {
		logger.SugaredLogger.Debugf("key: %+v, value: %+v", i, re)
	}
}

func TestGetStockMoneyTrendByDay(t *testing.T) {
	res := NewMarketNewsApi().GetStockMoneyTrendByDay("sh600438", 360)
	for i, re := range res {
		logger.SugaredLogger.Debugf("key: %+v, value: %+v", i, re)
	}
}
func TestTopStocksRankingList(t *testing.T) {
	NewMarketNewsApi().TopStocksRankingList("2025-05-19")
}

func TestLongTiger(t *testing.T) {
	db.Init("../../data/stock.db")

	NewMarketNewsApi().LongTiger("2025-06-08")
}

func TestStockResearchReport(t *testing.T) {
	db.Init("../../data/stock.db")
	resp := NewMarketNewsApi().StockResearchReport("600584.sh", 7)
	for _, a := range resp {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}
}

func TestIndustryResearchReport(t *testing.T) {
	db.Init("../../data/stock.db")
	resp := NewMarketNewsApi().IndustryResearchReport("", 7)
	for _, a := range resp {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestStockNotice(t *testing.T) {
	db.Init("../../data/stock.db")
	resp := NewMarketNewsApi().StockNotice("600584,600900")
	for _, a := range resp {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestEMDictCode(t *testing.T) {
	db.Init("../../data/stock.db")
	resp := NewMarketNewsApi().EMDictCode("016", freecache.NewCache(100))
	for _, a := range resp {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestTradingViewNews(t *testing.T) {
	db.Init("../../data/stock.db")
	resp := NewMarketNewsApi().TradingViewNews()
	for _, a := range *resp {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}
}

func TestXUEQIUHotStock(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewMarketNewsApi().XUEQIUHotStock(50, "10")
	for _, a := range *res {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestHotEvent(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewMarketNewsApi().HotEvent(50)
	for _, a := range *res {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestHotTopic(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewMarketNewsApi().HotTopic(10)
	for _, a := range res {
		logger.SugaredLogger.Debugf("value: %+v", a)
	}

}

func TestInvestCalendar(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewMarketNewsApi().InvestCalendar("2025-06")
	for _, a := range res {
		bytes, err := json.Marshal(a)
		if err != nil {
			continue
		}
		date := gjson.Get(string(bytes), "date")
		list := gjson.Get(string(bytes), "list")

		logger.SugaredLogger.Debugf("value: %+v,list: %+v", date.String(), list)
	}
}

func TestClsCalendar(t *testing.T) {
	db.Init("../../data/stock.db")
	res := NewMarketNewsApi().ClsCalendar()
	md := strings.Builder{}
	for _, a := range res {
		bytes, err := json.Marshal(a)
		if err != nil {
			continue
		}
		//logger.SugaredLogger.Debugf("value: %+v", string(bytes))
		date := gjson.Get(string(bytes), "calendar_day")
		md.WriteString("\n### 事件/会议日期：" + date.String())
		list := gjson.Get(string(bytes), "items")
		//logger.SugaredLogger.Debugf("value: %+v,list: %+v", date.String(), list)
		list.ForEach(func(key, value gjson.Result) bool {
			logger.SugaredLogger.Debugf("key: %+v,value: %+v", key.String(), gjson.Get(value.String(), "title"))
			md.WriteString("\n- " + gjson.Get(value.String(), "title").String())
			return true
		})
	}
	logger.SugaredLogger.Debugf("md:\n %s", md.String())
}
