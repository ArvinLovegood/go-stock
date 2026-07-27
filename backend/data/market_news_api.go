package data

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/util"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/coocood/freecache"
	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/robertkrimen/otto"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

// @Author spark
// @Date 2025/4/23 14:54
// @Desc
// -----------------------------------------------------------------------------------
type MarketNewsApi struct {
}

func NewMarketNewsApi() *MarketNewsApi {
	return &MarketNewsApi{}
}

func (m MarketNewsApi) TelegraphList(crawlTimeOut int64) *[]models.Telegraph {
	//https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraph&os=web&sv=8.7.9"
	res := map[string]any{}
	_, _ = SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetResult(&res).
		Get(clsURL)
	var telegraphs []models.Telegraph

	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		data, ok := res["data"].(map[string]any)
		if !ok {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		rollData, ok := data["roll_data"].([]any)
		if !ok || len(rollData) == 0 {
			return m.GetNewTelegraph(crawlTimeOut)
		}
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			ctime, _ := convertor.ToInt(news["ctime"])
			dataTime := time.Unix(ctime, 0).Local()

			shareURL := ""
			if su, ok2 := news["shareurl"].(string); ok2 && su != "" {
				shareURL = su
			} else if id, ok2 := news["id"]; ok2 {
				shareURL = fmt.Sprintf("https://www.cls.cn/telegraph/%v", id)
			}

			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			level, _ := news["level"].(string)

			telegraph := models.Telegraph{
				Title:           title,
				Content:         content,
				Time:            dataTime.Format("15:04:05"),
				DataTime:        &dataTime,
				Url:             shareURL,
				Source:          "财联社电报",
				IsRed:           level != "C",
				SentimentResult: AnalyzeSentiment(content).Description,
			}
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt > 0 {
				continue
			}
			telegraphs = append(telegraphs, telegraph)
			db.Dao.Model(&models.Telegraph{}).Create(&telegraph)
			if news["subjects"] == nil {
				continue
			}
			subjects, ok := news["subjects"].([]any)
			if !ok {
				continue
			}
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag := &models.Tags{
					Name: name,
					Type: "subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "subject").FirstOrCreate(&tag)
				db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
					TelegraphId: telegraph.ID,
					TagId:       tag.ID,
				})
			}
		}
	} else {
		return m.GetNewTelegraph(crawlTimeOut)
	}

	return &telegraphs
}

func (m MarketNewsApi) GetNewTelegraph(crawlTimeOut int64) *[]models.Telegraph {
	clsURL := "https://www.cls.cn/api/cache?app=CailianpressWeb&name=telegraphList&os=web&sv=8.7.9"
	res := map[string]any{}
	_, _ = SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetResult(&res).
		Get(clsURL)
	var telegraphs []models.Telegraph

	if v, _ := convertor.ToInt(res["errno"]); v == 0 {
		if res["data"] == nil {
			return &telegraphs
		}
		data, ok := res["data"].(map[string]any)
		if !ok {
			return &telegraphs
		}
		rollData, ok := data["roll_data"].([]any)
		if !ok {
			return &telegraphs
		}
		for _, v := range rollData {
			news, ok := v.(map[string]any)
			if !ok {
				continue
			}
			ctime, _ := convertor.ToInt(news["ctime"])
			dataTime := time.Unix(ctime, 0).Local()

			shareURL := ""
			if su, ok2 := news["shareurl"].(string); ok2 && su != "" {
				shareURL = su
			} else if id, ok2 := news["id"]; ok2 {
				shareURL = fmt.Sprintf("https://www.cls.cn/telegraph/%v", id)
			}

			title, _ := news["title"].(string)
			content, _ := news["content"].(string)
			level, _ := news["level"].(string)

			telegraph := models.Telegraph{
				Title:           title,
				Content:         content,
				Time:            dataTime.Format("15:04:05"),
				DataTime:        &dataTime,
				Url:             shareURL,
				Source:          "财联社电报",
				IsRed:           level != "C",
				SentimentResult: AnalyzeSentiment(content).Description,
			}
			cnt := int64(0)
			if telegraph.Title == "" {
				db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
			} else {
				db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
			}
			if cnt > 0 {
				continue
			}
			telegraphs = append(telegraphs, telegraph)
			db.Dao.Model(&models.Telegraph{}).Create(&telegraph)
			if news["subjects"] == nil {
				continue
			}
			subjects, ok := news["subjects"].([]any)
			if !ok {
				continue
			}
			for _, subject := range subjects {
				subMap, ok := subject.(map[string]any)
				if !ok {
					continue
				}
				name, ok := subMap["subject_name"].(string)
				if !ok || name == "" {
					continue
				}
				tag := &models.Tags{
					Name: name,
					Type: "subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "subject").FirstOrCreate(&tag)
				db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tag.ID).FirstOrCreate(&models.TelegraphTags{
					TelegraphId: telegraph.ID,
					TagId:       tag.ID,
				})
			}
		}
	}
	return &telegraphs
}
func (m MarketNewsApi) GetNewsList(source string, limit int) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("data_time desc,time desc").Limit(limit).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("data_time desc,time desc").Limit(limit).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}
func (m MarketNewsApi) GetNewsList2(source string, limit int) *[]*models.Telegraph {
	NewMarketNewsApi().TelegraphList(30)
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("data_time desc,is_red desc").Limit(limit).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("data_time desc,is_red desc").Limit(limit).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}

func (m MarketNewsApi) GetTelegraphList(source string) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("data_time desc,time desc").Limit(50).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("data_time desc,time desc").Limit(50).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}
func (m MarketNewsApi) GetTelegraphListWithPaging(source string, page, pageSize int) *[]*models.Telegraph {
	// 计算偏移量
	offset := (page - 1) * pageSize

	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=?", source).Order("data_time desc,time desc").Limit(pageSize).Offset(offset).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Order("data_time desc,time desc").Limit(pageSize).Offset(offset).Find(news)
	}
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
	}
	return news
}

func (m MarketNewsApi) GetSinaNews(crawlTimeOut uint) *[]models.Telegraph {
	news := &[]models.Telegraph{}
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://zhibo.sina.com.cn/api/zhibo/feed?callback=callback&page=1&page_size=20&zhibo_id=152&tag_id=0&dire=f&dpc=1&pagesize=20&id=4161089&type=0&_=" + strconv.FormatInt(time.Now().Unix(), 10))
	js := string(response.Body())
	js = strutil.ReplaceWithMap(js, map[string]string{
		"try{callback(":  "var data=",
		");}catch(e){};": ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	_, err := vm.Run(js)
	if err != nil {
		logger.SugaredLogger.Error(err)
	}
	vm.Run("var result = data.result;")
	//vm.Run("var resultStr =JSON.stringify(data);")
	vm.Run("var resultData = result.data;")
	vm.Run("var feed = resultData.feed;")
	vm.Run("var feedStr = JSON.stringify(feed);")

	value, _ := vm.Get("feedStr")
	//resultStr, _ := vm.Get("resultStr")

	//logger.SugaredLogger.Info(resultStr)
	feed := make(map[string]any)
	err = json.Unmarshal([]byte(value.String()), &feed)
	if err != nil {
		logger.SugaredLogger.Errorf("json.Unmarshal error:%v", err.Error())
	}
	var telegraphs []models.Telegraph

	if feed["list"] != nil {
		for _, item := range feed["list"].([]any) {
			telegraph := models.Telegraph{Source: "新浪财经"}
			data := item.(map[string]any)
			//logger.SugaredLogger.Infof("%s:%s", data["create_time"], data["rich_text"])
			telegraph.Content = data["rich_text"].(string)
			telegraph.Title = strutil.SubInBetween(data["rich_text"].(string), "【", "】")
			telegraph.Time = strings.Split(data["create_time"].(string), " ")[1]
			dataTime, _ := time.ParseInLocation("2006-01-02 15:04:05", data["create_time"].(string), time.Local)
			if &dataTime != nil {
				telegraph.DataTime = &dataTime
			}
			tags := data["tag"].([]any)
			telegraph.SubjectTags = lo.Map(tags, func(tagItem any, index int) string {
				name := tagItem.(map[string]any)["name"].(string)
				tag := &models.Tags{
					Name: name,
					Type: "sina_subject",
				}
				db.Dao.Model(tag).Where("name=? and type=?", name, "sina_subject").FirstOrCreate(&tag)
				return name
			})
			if _, ok := lo.Find(telegraph.SubjectTags, func(item string) bool { return item == "焦点" }); ok {
				telegraph.IsRed = true
			}
			//logger.SugaredLogger.Infof("telegraph.SubjectTags:%v %s", telegraph.SubjectTags, telegraph.Content)

			if telegraph.Content != "" {
				telegraph.SentimentResult = AnalyzeSentiment(telegraph.Content).Description
				cnt := int64(0)
				if telegraph.Title == "" {
					db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
				} else {
					db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
				}
				if cnt == 0 {
					db.Dao.Create(&telegraph)
					telegraphs = append(telegraphs, telegraph)
					for _, tag := range telegraph.SubjectTags {
						tagInfo := &models.Tags{}
						db.Dao.Model(models.Tags{}).Where("name=? and type=?", tag, "sina_subject").First(&tagInfo)
						if tagInfo.ID > 0 {
							db.Dao.Model(models.TelegraphTags{}).Where("telegraph_id=? and tag_id=?", telegraph.ID, tagInfo.ID).FirstOrCreate(&models.TelegraphTags{
								TelegraphId: telegraph.ID,
								TagId:       tagInfo.ID,
							})
						}
					}
				}
			}
		}
		return &telegraphs
	}

	return news

}

func (m MarketNewsApi) GlobalStockIndexes(crawlTimeOut uint) map[string]any {
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(crawlTimeOut)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/mstats").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/rank/indexRankDetail2")
	js := string(response.Body())
	res := make(map[string]any)
	json.Unmarshal([]byte(js), &res)
	return res["data"].(map[string]any)
}

// GlobalStockIndexesReadable 获取全球指数并转换为 AI 易读的 Markdown 文本。
func (m MarketNewsApi) GlobalStockIndexesReadable(crawlTimeOut uint) string {
	data := m.GlobalStockIndexes(crawlTimeOut)
	return m.GlobalStockIndexesToReadable(data)
}

// GlobalStockIndexesToReadable 将 GlobalStockIndexes 返回的 JSON 转为 AI 易读格式（Markdown）。
//
//	输入示例：map[string]any{
//	  "america": []any{...},
//	  "asia":    []any{...},
//	  "europe":  []any{...},
//	  "other":   []any{...},
//	  "common":  []any{...},
//	}
func (m MarketNewsApi) GlobalStockIndexesToReadable(data map[string]any) string {
	if len(data) == 0 {
		return "暂无全球指数数据。"
	}
	type regionDef struct {
		Key   string
		Title string
	}
	regions := []regionDef{
		{Key: "common", Title: "重点关注"},
		{Key: "asia", Title: "亚洲市场"},
		{Key: "america", Title: "美洲市场"},
		{Key: "europe", Title: "欧洲市场"},
		{Key: "other", Title: "其他市场"},
	}

	stateText := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "open":
			return "开盘"
		case "close":
			return "收盘"
		default:
			if v == "" {
				return "-"
			}
			return v
		}
	}

	var sb strings.Builder
	sb.WriteString("# 全球主要指数概览\n")
	sb.WriteString("> 数据来源：腾讯财经，已按区域整理。\n\n")

	written := 0
	for _, region := range regions {
		raw, ok := data[region.Key]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			continue
		}
		written++
		sb.WriteString("## ")
		sb.WriteString(region.Title)
		sb.WriteString("\n")
		sb.WriteString("| 指数 | 地区 | 最新点位 | 涨跌幅(%) | 状态 |\n")
		sb.WriteString("| --- | --- | ---: | ---: | --- |\n")

		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			name := convertor.ToString(row["name"])
			location := convertor.ToString(row["location"])
			zxj := convertor.ToString(row["zxj"])
			zdf := convertor.ToString(row["zdf"])
			state := stateText(convertor.ToString(row["state"]))
			if name == "" {
				name = convertor.ToString(row["code"])
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", name, location, zxj, zdf, state))
		}
		sb.WriteString("\n")
	}

	if written == 0 {
		return "暂无可解析的全球指数数据。"
	}
	return sb.String()
}

// CacheGlobalStockIndexes 将全球指数数据缓存到数据库
func (m MarketNewsApi) CacheGlobalStockIndexes(crawlTimeOut uint) error {
	data := m.GlobalStockIndexes(crawlTimeOut)
	if len(data) == 0 {
		return fmt.Errorf("获取全球指数数据失败")
	}

	// 定义区域映射
	regions := map[string]string{
		"america": "美洲",
		"asia":    "亚洲",
		"europe":  "欧洲",
		"common":  "重点关注",
		"other":   "其他",
	}

	for regionKey, regionName := range regions {
		raw, ok := data[regionKey]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			continue
		}

		for _, item := range list {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}

			index := models.GlobalStockIndex{
				Code:       convertor.ToString(row["code"]),
				Name:       convertor.ToString(row["name"]),
				Location:   convertor.ToString(row["location"]),
				Qtcode:     convertor.ToString(row["qtcode"]),
				State:      convertor.ToString(row["state"]),
				Zdf:        convertor.ToString(row["zdf"]),
				Zxj:        convertor.ToString(row["zxj"]),
				Img:        convertor.ToString(row["img"]),
				Region:     regionKey,
				RegionName: regionName,
			}

			// 如果已存在则更新，不存在则创建
			existing := models.GlobalStockIndex{}
			query := db.Dao.Model(&models.GlobalStockIndex{}).Where("qtcode = ?", index.Qtcode)
			if err := query.First(&existing).Error; err == nil {
				// 记录已存在，更新
				db.Dao.Model(&existing).Updates(map[string]any{
					"name":     index.Name,
					"location": index.Location,
					"state":    index.State,
					"zdf":      index.Zdf,
					"zxj":      index.Zxj,
					"img":      index.Img,
					"region":   index.Region,
				})
			} else {
				// 记录不存在，创建
			}
			db.Dao.Where(models.GlobalStockIndex{Qtcode: index.Qtcode}).FirstOrCreate(&index)
		}
	}

	logger.SugaredLogger.Info("全球指数缓存完成")
	return nil
}

// GetCachedGlobalStockIndexes 从数据库获取缓存的全球指数数据
func (m MarketNewsApi) GetCachedGlobalStockIndexes(region string) *[]models.GlobalStockIndex {
	indexes := &[]models.GlobalStockIndex{}
	query := db.Dao.Model(&models.GlobalStockIndex{})
	if region != "" && region != "all" {
		query = query.Where("region = ?", region)
	}
	query.Order("region, zdf desc").Find(indexes)
	return indexes
}

// GetCachedGlobalStockIndexesReadable 获取缓存的全球指数并转换为易读格式
func (m MarketNewsApi) GetCachedGlobalStockIndexesReadable(region string) string {
	data := m.GetCachedGlobalStockIndexes(region)
	if data == nil || len(*data) == 0 {
		return "暂无全球指数数据。"
	}

	type regionDef struct {
		Key   string
		Title string
	}
	regions := []regionDef{
		{Key: "common", Title: "重点关注"},
		{Key: "asia", Title: "亚洲市场"},
		{Key: "america", Title: "美洲市场"},
		{Key: "europe", Title: "欧洲市场"},
		{Key: "other", Title: "其他市场"},
	}

	stateText := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "open":
			return "开盘"
		case "close":
			return "收盘"
		default:
			if v == "" {
				return "-"
			}
			return v
		}
	}

	var sb strings.Builder
	sb.WriteString("# 全球主要指数概览\n")
	sb.WriteString("> 数据来源：腾讯财经，已按区域整理。\n\n")

	// 按区域分组
	indexesByRegion := make(map[string][]models.GlobalStockIndex)
	for _, idx := range *data {
		indexesByRegion[idx.Region] = append(indexesByRegion[idx.Region], idx)
	}

	written := 0
	for _, regionDef := range regions {
		list, ok := indexesByRegion[regionDef.Key]
		if !ok || len(list) == 0 {
			continue
		}
		written++
		sb.WriteString("## ")
		sb.WriteString(regionDef.Title)
		sb.WriteString("\n")
		sb.WriteString("| 指数 | 地区 | 最新点位 | 涨跌幅(%) | 状态 |\n")
		sb.WriteString("| --- | --- | ---: | ---: | --- |\n")

		for _, idx := range list {
			name := idx.Name
			if name == "" {
				name = idx.Code
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				name, idx.Location, idx.Zxj, idx.Zdf, stateText(idx.State)))
		}
		sb.WriteString("\n")
	}

	if written == 0 {
		return "暂无可解析的全球指数数据。"
	}
	return sb.String()
}

func (m MarketNewsApi) GetIndustryRank(sort string, cnt int) map[string]any {

	url := fmt.Sprintf("https://proxy.finance.qq.com/ifzqgtimg/appstock/app/mktHs/rank?l=%d&p=1&t=01/averatio&ordertype=&o=%s", cnt, sort)
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Referer", "https://stockapp.finance.qq.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := make(map[string]any)
	json.Unmarshal([]byte(js), &res)
	return res
}

func (m MarketNewsApi) GetIndustryMoneyRankSina(fenlei, sort string) []map[string]any {
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_bk?page=1&num=20&sort=%s&asc=0&fenlei=%s", sort, fenlei)

	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetMoneyRankSina(sort string) []map[string]any {
	if sort == "" {
		sort = "netamount"
	}
	url := fmt.Sprintf("https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_bkzj_ssggzj?page=1&num=20&sort=%s&asc=0&bankuai=&shichang=", sort)
	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res
}

func (m MarketNewsApi) GetStockMoneyTrendByDay(stockCode string, days int) []map[string]any {
	url := fmt.Sprintf("http://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/MoneyFlow.ssl_qsfx_zjlrqs?page=1&num=%d&sort=opendate&asc=0&daima=%s", days, stockCode)

	response, _ := SharedHTTPClient.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "vip.stock.finance.sina.com.cn").
		SetHeader("Referer", "https://finance.sina.com.cn").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").Get(url)
	js := string(response.Body())
	res := &[]map[string]any{}
	err := json.Unmarshal([]byte(js), &res)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return *res
	}
	return *res

}

func (m MarketNewsApi) LongTiger(date string) *[]models.LongTigerRankData {
	ranks := &[]models.LongTigerRankData{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get"
	//logger.SugaredLogger.Infof("url:%s", url)
	params := make(map[string]string)
	params["callback"] = "callback"
	params["sortColumns"] = "TURNOVERRATE,TRADE_DATE,SECURITY_CODE"
	params["sortTypes"] = "-1,-1,1"
	params["pageSize"] = "500"
	params["pageNumber"] = "1"
	params["reportName"] = "RPT_DAILYBILLBOARD_DETAILSNEW"
	params["columns"] = "SECURITY_CODE,SECUCODE,SECURITY_NAME_ABBR,TRADE_DATE,EXPLAIN,CLOSE_PRICE,CHANGE_RATE,BILLBOARD_NET_AMT,BILLBOARD_BUY_AMT,BILLBOARD_SELL_AMT,BILLBOARD_DEAL_AMT,ACCUM_AMOUNT,DEAL_NET_RATIO,DEAL_AMOUNT_RATIO,TURNOVERRATE,FREE_MARKET_CAP,EXPLANATION,D1_CLOSE_ADJCHRATE,D2_CLOSE_ADJCHRATE,D5_CLOSE_ADJCHRATE,D10_CLOSE_ADJCHRATE,SECURITY_TYPE_CODE"
	params["source"] = "WEB"
	params["client"] = "WEB"
	params["filter"] = fmt.Sprintf("(TRADE_DATE<='%s')(TRADE_DATE>='%s')", date, date)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/stock/tradedetail.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetQueryParams(params).
		Get(url)
	if err != nil {
		return ranks
	}
	js := string(resp.Body())
	//logger.SugaredLogger.Infof("resp:%s", js)

	js = strutil.ReplaceWithMap(js, map[string]string{
		"callback(": "var data=",
		");":        ";",
	})
	//logger.SugaredLogger.Info(js)
	vm := otto.New()
	_, err = vm.Run(js)
	_, err = vm.Run("var data = JSON.stringify(data);")
	value, err := vm.Get("data")
	//logger.SugaredLogger.Infof("resp-json:%s", value.String())
	data := gjson.Get(value.String(), "result.data")
	//logger.SugaredLogger.Infof("resp:%v", data)
	err = json.Unmarshal([]byte(data.String()), ranks)
	if err != nil {
		logger.SugaredLogger.Error(err)
		return ranks
	}
	for _, rankData := range *ranks {
		temp := &models.LongTigerRankData{}
		db.Dao.Model(temp).Where(&models.LongTigerRankData{
			TRADEDATE: rankData.TRADEDATE,
			SECUCODE:  rankData.SECUCODE,
		}).First(temp)
		if temp.SECURITYTYPECODE == "" {
			db.Dao.Model(temp).Create(&rankData)
		}
	}
	return ranks
}

func (m MarketNewsApi) IndustryResearchReport(industryCode string, days int) []any {
	beginDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	if strutil.Trim(industryCode) != "" {
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	//logger.SugaredLogger.Infof("IndustryResearchReport-name:%s", industryCode)
	params := map[string]string{
		"industry":     "*",
		"industryCode": industryCode,
		"beginTime":    beginDate,
		"endTime":      endDate,
		"pageNo":       "1",
		"pageSize":     "50",
		"p":            "1",
		"pageNum":      "1",
		"pageNumber":   "1",
		"qType":        "1",
	}

	url := "https://reportapi.eastmoney.com/report/list"

	//logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap["data"].([]any)
}
func (m MarketNewsApi) StockResearchReport(stockCode string, days int) []any {
	beginDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	if strutil.ContainsAny(stockCode, []string{"."}) {
		stockCode = strings.Split(stockCode, ".")[0]
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	} else {
		stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
			"sh":  "",
			"sz":  "",
			"gb_": "",
			"us":  "",
			"us_": "",
		})
		beginDate = time.Now().Add(-time.Duration(days) * 365 * time.Hour).Format("2006-01-02")
	}

	//logger.SugaredLogger.Infof("StockResearchReport-stockCode:%s", stockCode)

	type Req struct {
		BeginTime    string      `json:"beginTime"`
		EndTime      string      `json:"endTime"`
		IndustryCode string      `json:"industryCode"`
		RatingChange string      `json:"ratingChange"`
		Rating       string      `json:"rating"`
		OrgCode      interface{} `json:"orgCode"`
		Code         string      `json:"code"`
		Rcode        string      `json:"rcode"`
		PageSize     int         `json:"pageSize"`
		PageNo       int         `json:"pageNo"`
		P            int         `json:"p"`
		PageNum      int         `json:"pageNum"`
		PageNumber   int         `json:"pageNumber"`
	}

	url := "https://reportapi.eastmoney.com/report/list2"

	//logger.SugaredLogger.Infof("beginDate:%s endDate:%s", beginDate, endDate)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/stock.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetBody(&Req{
			Code:         stockCode,
			IndustryCode: "*",
			BeginTime:    beginDate,
			EndTime:      endDate,
			PageNo:       1,
			PageSize:     50,
			P:            1,
			PageNum:      1,
			PageNumber:   1,
		}).Post(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return respMap["data"].([]any)
}

func (m MarketNewsApi) StockNotice(stock_list string) []any {
	var stockCodes []string
	for _, stockCode := range strings.Split(stock_list, ",") {
		if strutil.ContainsAny(stockCode, []string{"."}) {
			stockCode = strings.Split(stockCode, ".")[0]
			stockCodes = append(stockCodes, stockCode)
		} else {
			stockCode = strutil.ReplaceWithMap(stockCode, map[string]string{
				"sh":  "",
				"sz":  "",
				"gb_": "",
				"us":  "",
				"us_": "",
			})
			stockCodes = append(stockCodes, stockCode)
		}
	}

	url := "https://np-anotice-stock.eastmoney.com/api/security/ann?page_size=50&page_index=1&ann_type=SHA%2CCYB%2CSZA%2CBJA%2CINV&client_source=web&f_node=0&stock_list=" + strings.Join(stockCodes, ",")
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "np-anotice-stock.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/notices/hsa/5.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	respMap := map[string]any{}

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	return (respMap["data"].(map[string]any))["list"].([]any)
}

func (m MarketNewsApi) EMDictCode(code string, cache *freecache.Cache) []any {
	respMap := map[string]any{}

	d, _ := cache.Get([]byte(code))
	if d != nil {
		json.Unmarshal(d, &respMap)
		return respMap["data"].([]any)
	}

	url := "https://reportapi.eastmoney.com/report/bk"

	params := map[string]string{
		"bkCode": code,
	}
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/industry.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetQueryParams(params).Get(url)

	if err != nil {
		return []any{}
	}
	json.Unmarshal(resp.Body(), &respMap)
	//logger.SugaredLogger.Infof("resp:%+v", respMap["data"])
	cache.Set([]byte(code), resp.Body(), 60*60*24)
	return respMap["data"].([]any)
}

func (m MarketNewsApi) TradingViewNews() *[]models.Telegraph {
	client := SharedHTTPClient
	config := GetSettingConfig()
	if config.HttpProxyEnabled && config.HttpProxy != "" {
		client.SetProxy(config.HttpProxy)
	}
	TVNews := &[]models.TVNews{}
	news := &[]models.Telegraph{}
	//	url := "https://news-mediator.tradingview.com/news-flow/v2/news?filter=lang:zh-Hans&filter=area:WLD&client=screener&streaming=false"
	//url := "https://news-mediator.tradingview.com/news-flow/v2/news?filter=area%3AWLD&filter=lang%3Azh-Hans&client=screener&streaming=false"
	url := "https://news-mediator.tradingview.com/news-flow/v2/news?filter=lang%3Azh-Hans&client=screener&streaming=false"

	resp, err := client.SetTimeout(time.Duration(15)*time.Second).R().
		SetHeader("Host", "news-mediator.tradingview.com").
		SetHeader("Origin", "https://cn.tradingview.com").
		SetHeader("Referer", "https://cn.tradingview.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		//logger.SugaredLogger.Errorf("TradingViewNews err:%s", err.Error())
		return news
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	if err != nil {
		return news
	}
	items, err := json.Marshal(respMap["items"])
	if err != nil {
		return news
	}
	json.Unmarshal(items, TVNews)

	for i, a := range *TVNews {
		if i > 10 {
			break
		}
		detail := NewMarketNewsApi().TradingViewNewsDetail(a.Id)
		dataTime := time.Unix(int64(a.Published), 0).Local()
		description := ""
		sentimentResult := ""
		if detail != nil {
			description = detail.ShortDescription
			sentimentResult = AnalyzeSentiment(description).Description
		}
		if a.Title == "" {
			continue
		}
		telegraph := &models.Telegraph{
			Title:           a.Title,
			Content:         description,
			DataTime:        &dataTime,
			IsRed:           false,
			Time:            dataTime.Format("15:04:05"),
			Source:          "外媒",
			Url:             fmt.Sprintf("https://cn.tradingview.com/news/%s", a.Id),
			SentimentResult: sentimentResult,
		}
		cnt := int64(0)
		if telegraph.Title == "" {
			db.Dao.Model(telegraph).Where("content=?", telegraph.Content).Count(&cnt)
		} else {
			db.Dao.Model(telegraph).Where("title=?", telegraph.Title).Count(&cnt)
		}
		if cnt > 0 {
			continue
		}
		db.Dao.Model(&models.Telegraph{}).Where("time=? and title=? and source=?", telegraph.Time, telegraph.Title, "外媒").FirstOrCreate(&telegraph)
		*news = append(*news, *telegraph)
	}
	return news
}
func (m MarketNewsApi) TradingViewNewsDetail(id string) *models.TVNewsDetail {
	//https://news-headlines.tradingview.com/v3/story?id=panews%3A9be7cf057e3f9%3A0&lang=zh-Hans
	newsDetail := &models.TVNewsDetail{}
	newsUrl := fmt.Sprintf("https://news-headlines.tradingview.com/v3/story?id=%s&lang=zh-Hans", url.QueryEscape(id))

	client := SharedHTTPClient
	config := GetSettingConfig()
	if config.HttpProxyEnabled && config.HttpProxy != "" {
		client.SetProxy(config.HttpProxy)
	}
	request := client.SetTimeout(time.Duration(3) * time.Second).R()
	_, err := request.
		SetHeader("Host", "news-headlines.tradingview.com").
		SetHeader("Origin", "https://cn.tradingview.com").
		SetHeader("Referer", "https://cn.tradingview.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:146.0) Gecko/20100101 Firefox/146.0").
		//SetHeader("TE", "trailers").
		//SetHeader("Priority", "u=4").
		//SetHeader("Connection", "keep-alive").
		SetResult(newsDetail).
		Get(newsUrl)
	if err != nil {
		logger.SugaredLogger.Errorf("TradingViewNewsDetail err:%s", err.Error())
		return newsDetail
	}
	//logger.SugaredLogger.Infof("resp:%+v", newsDetail)
	return newsDetail
}

func (m MarketNewsApi) XUEQIUHotStock(size int, marketType string) *[]models.HotItem {
	url := fmt.Sprintf("https://stock.xueqiu.com/v5/stock/hot_stock/list.json?page=1&size=%d&_type=%s&type=%s", size, marketType, marketType)
	empty := &[]models.HotItem{}

	const maxRetries = 2
	for attempt := 0; attempt < maxRetries; attempt++ {
		cookieHeader, cookieErr := FetchXueqiuCookiesViaChromedp("", 30*time.Second, "https://xueqiu.com/hq#hot")
		if cookieErr != nil {
			logger.SugaredLogger.Warnf("雪球 chromedp 获取 cookie 失败 (attempt %d): %v", attempt+1, cookieErr)
		}

		res := &models.XUEQIUHot{}
		request := SharedHTTPClient.SetTimeout(time.Duration(30) * time.Second).R()
		request.SetHeader("Host", "stock.xueqiu.com").
			SetHeader("Origin", "https://xueqiu.com").
			SetHeader("Referer", "https://xueqiu.com/").
			SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
		if cookieErr == nil && cookieHeader != "" {
			request.SetHeader("Cookie", cookieHeader)
		}
		_, err := request.SetResult(res).Get(url)
		if err != nil {
			logger.SugaredLogger.Errorf("XUEQIUHotStock err (attempt %d):%s", attempt+1, err.Error())
			if attempt < maxRetries-1 {
				InvalidateXueqiuCookieCache()
				time.Sleep(time.Second)
				continue
			}
			return empty
		}
		if res.ErrorCode != 0 {
			logger.SugaredLogger.Errorf("XUEQIUHotStock API error (attempt %d): code=%d, desc=%s", attempt+1, res.ErrorCode, res.ErrorDescription)
			if attempt < maxRetries-1 {
				InvalidateXueqiuCookieCache()
				time.Sleep(time.Second)
				continue
			}
			return empty
		}
		return &res.Data.Items
	}
	return empty
}

func (m MarketNewsApi) HotEvent(size int) *[]models.HotEvent {
	cookieHeader, cookieErr := FetchXueqiuCookiesViaChromedp("", 30*time.Second, "https://xueqiu.com/hq#hot")
	if cookieErr != nil {
		logger.SugaredLogger.Warnf("雪球 chromedp 获取 cookie 失败: %v", cookieErr)
	}

	events := &[]models.HotEvent{}
	sprintf := fmt.Sprintf("https://xueqiu.com/hot_event/list.json?count=%d", size)
	request := SharedHTTPClient.SetTimeout(time.Duration(30) * time.Second).R()
	request.SetHeader("Host", "xueqiu.com").
		SetHeader("Referer", "https://xueqiu.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
	if cookieErr == nil && cookieHeader != "" {
		request.SetHeader("Cookie", cookieHeader)
	}
	resp, err := request.Get(sprintf)
	if err != nil {
		logger.SugaredLogger.Errorf("HotEvent err:%s", err.Error())
		return events
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	if err != nil {
		logger.SugaredLogger.Errorf("HotEvent json unmarshal err:%s", err.Error())
		return events
	}
	items, err := json.Marshal(respMap["list"])
	if err != nil {
		return events
	}
	json.Unmarshal(items, events)
	return events
}

func (m MarketNewsApi) HotTopic(size int) []any {
	url := "https://gubatopic.eastmoney.com/interface/GetData.aspx?path=newtopic/api/Topic/HomePageListRead"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "gubatopic.eastmoney.com").
		SetHeader("Origin", "https://gubatopic.eastmoney.com").
		SetHeader("Referer", "https://gubatopic.eastmoney.com/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetFormData(map[string]string{
			"param": fmt.Sprintf("ps=%d&p=1&type=0", size),
			"path":  "newtopic/api/Topic/HomePageListRead",
			"env":   "2",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("HotTopic err:%s", err.Error())
		return []any{}
	}
	//logger.SugaredLogger.Infof("HotTopic:%s", resp.Body())
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	return respMap["re"].([]any)

}

func (m MarketNewsApi) InvestCalendar(yearMonth string) []any {
	if yearMonth == "" {
		yearMonth = time.Now().Format("2006-01")
	}

	// 韭研公社网站 JS 逆向：token = md5("Uu0KfOB8iUP69d3c:" + timestamp)
	// 无需 SESSION cookie，匿名访问即可
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signSrc := "Uu0KfOB8iUP69d3c:" + timestamp
	md5Sum := md5.Sum([]byte(signSrc))
	token := hex.EncodeToString(md5Sum[:])

	url := "https://app.jiuyangongshe.com/jystock-app/api/v1/timeline/list"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "app.jiuyangongshe.com").
		SetHeader("Origin", "https://www.jiuyangongshe.com").
		SetHeader("Referer", "https://www.jiuyangongshe.com/timeline").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetHeader("Content-Type", "application/json").
		SetHeader("platform", "3").
		SetHeader("Accept-Language", "en-US, zh; q=0.9, en; q=0.8").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		SetHeader("timestamp", timestamp).
		SetHeader("token", token).
		SetBody(map[string]string{
			"date":  yearMonth,
			"grade": "0",
		}).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("InvestCalendar err:%s", err.Error())
		return []any{}
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	if err != nil {
		logger.SugaredLogger.Errorf("InvestCalendar unmarshal err:%s,body:%s", err.Error(), resp.Body())
		return []any{}
	}
	// errCode != 0 时 data 为空对象 {}，需类型断言保护避免 panic
	if data, ok := respMap["data"].([]any); ok {
		return data
	}
	logger.SugaredLogger.Errorf("InvestCalendar unexpected response:%s", resp.Body())
	return []any{}

}

func (m MarketNewsApi) ClsCalendar() []any {
	url := "https://www.cls.cn/api/calendar/web/list?app=CailianpressWeb&flag=0&os=web&sv=8.4.6&type=0&sign=4b839750dc2f6b803d1c8ca00d2b40be"
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "www.cls.cn").
		SetHeader("Origin", "https://www.cls.cn").
		SetHeader("Referer", "https://www.cls.cn/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ClsCalendar err:%s", err.Error())
		return []any{}
	}
	respMap := map[string]any{}
	err = json.Unmarshal(resp.Body(), &respMap)
	return respMap["data"].([]any)
}

func (m MarketNewsApi) GetGDP() *models.GDPResp {
	res := &models.GDPResp{}

	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CDOMESTICL_PRODUCT_BASE%2CFIRST_PRODUCT_BASE%2CSECOND_PRODUCT_BASE%2CTHIRD_PRODUCT_BASE%2CSUM_SAME%2CFIRST_SAME%2CSECOND_SAME%2CTHIRD_SAME&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_GDP&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GDP err:%s", err.Error())
		return res
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GDP:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("GDP err:%s", err.Error())
		return res
	}
	data, _ := val.Object().Value().Export()
	//logger.SugaredLogger.Infof("GDP:%v", data)
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	json.Unmarshal(marshal, &res)
	//logger.SugaredLogger.Infof("GDP:%+v", res)
	return res
}

func (m MarketNewsApi) GetCPI() *models.CPIResp {
	res := &models.CPIResp{}

	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CNATIONAL_SAME%2CNATIONAL_BASE%2CNATIONAL_SEQUENTIAL%2CNATIONAL_ACCUMULATE%2CCITY_SAME%2CCITY_BASE%2CCITY_SEQUENTIAL%2CCITY_ACCUMULATE%2CRURAL_SAME%2CRURAL_BASE%2CRURAL_SEQUENTIAL%2CRURAL_ACCUMULATE&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_CPI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetCPI err:%s", err.Error())
		return res
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GetCPI:%s", body)
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		logger.SugaredLogger.Errorf("GetCPI err:%s", err.Error())
		return res
	}
	data, _ := val.Object().Value().Export()
	//logger.SugaredLogger.Infof("GetCPI:%v", data)
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	json.Unmarshal(marshal, &res)
	//logger.SugaredLogger.Infof("GetCPI:%+v", res)
	return res
}

// GetPPI PPI
func (m MarketNewsApi) GetPPI() *models.PPIResp {
	res := &models.PPIResp{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE,TIME,BASE,BASE_SAME,BASE_ACCUMULATE&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_PPI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetPPI err:%s", err.Error())
		return res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		return res
	}
	data, _ := val.Object().Value().Export()
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	json.Unmarshal(marshal, &res)
	return res
}

func (m MarketNewsApi) GetPMI() *models.PMIResp {
	res := &models.PMIResp{}
	url := "https://datacenter-web.eastmoney.com/api/data/v1/get?callback=data&columns=REPORT_DATE%2CTIME%2CMAKE_INDEX%2CMAKE_SAME%2CNMAKE_INDEX%2CNMAKE_SAME&pageNumber=1&pageSize=20&sortColumns=REPORT_DATE&sortTypes=-1&source=WEB&client=WEB&reportName=RPT_ECONOMY_PMI&p=1&pageNo=1&pageNum=1&_=" + strconv.FormatInt(time.Now().Unix(), 10)
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "datacenter-web.eastmoney.com").
		SetHeader("Origin", "https://datacenter.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/cjsj/gdp.html").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		return res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, err := vm.Run(body)
	if err != nil {
		return res
	}
	data, _ := val.Object().Value().Export()
	marshal, err := json.Marshal(data)
	if err != nil {
		return res
	}
	json.Unmarshal(marshal, &res)
	return res

}
func (m MarketNewsApi) GetIndustryReportInfo(infoCode string) string {
	url := "https://data.eastmoney.com/report/zw_industry.jshtml?infocode=" + infoCode
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("Origin", "https://data.eastmoney.com").
		SetHeader("Referer", "https://data.eastmoney.com/report/industry.jshtml").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetIndustryReportInfo err:%s", err.Error())
		return ""
	}
	body := resp.Body()
	////logger.SugaredLogger.Debugf("GetIndustryReportInfo:%s", body)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	title, _ := doc.Find("div.c-title").Html()
	content, _ := doc.Find("div.ctx-content").Html()
	//logger.SugaredLogger.Infof("GetIndustryReportInfo:\n%s\n%s", title, content)
	markdown, err := util.HTMLToMarkdown(title + content)
	if err != nil {
		return ""
	}
	//logger.SugaredLogger.Infof("GetIndustryReportInfo markdown:\n%s", markdown)
	return markdown
}

func (receiver MarketNewsApi) GetSecuritiesCompanyOpinion(startDate string, endDate string) *models.SecuritiesCompanyOpinionResp {
	res := models.SecuritiesCompanyOpinionResp{}

	url := fmt.Sprintf("https://reportapi.eastmoney.com/report/jg?cb=data&pageSize=50&beginTime=%s&endTime=%s&pageNo=1&fields=&qType=4&orgCode=&author=&p=1&pageNum=1&pageNumber=1&_=%d", startDate, endDate, time.Now().Unix())
	resp, err := SharedHTTPClient.SetTimeout(time.Duration(30)*time.Second).R().
		SetHeader("Host", "reportapi.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("GetSecuritiesCompanyOpinion err:%s", err.Error())
		return &res
	}
	body := resp.Body()
	vm := otto.New()
	vm.Run("function data(res){return res};")

	val, _ := vm.Run(body)

	data, _ := val.Object().Value().Export()
	marshal, _ := json.Marshal(data)

	json.Unmarshal(marshal, &res)

	for _, d := range (&res).Data {
		//logger.SugaredLogger.Debugf("PublishDate: %s,OrgSName: %s,Title: %s,EncodeUrl: %s", d.PublishDate, d.OrgSName, d.Title, d.EncodeUrl)
		markdown := receiver.GetSecuritiesCompanyOpinionContent(d.OrgSName, d.EncodeUrl)
		d.OpinionData = markdown
	}
	return &res
}

func (m MarketNewsApi) GetSecuritiesCompanyOpinionContent(OrgSName, encodeUrl string) string {
	url := "https://data.eastmoney.com/report/zw_brokerreport.jshtml?encodeUrl=" + encodeUrl
	resp, _ := SharedHTTPClient.R().
		SetHeader("Host", "data.eastmoney.com").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		Get(url)
	body := resp.Body()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	title, _ := doc.Find("div.c-title").Html()
	content, _ := doc.Find("div.ctx-content").Html()
	markdown, err := util.HTMLToMarkdown("<h1>" + OrgSName + "</h1>" + title + content)
	if err != nil {
	}
	return markdown
}

func (m MarketNewsApi) ReutersNew() *models.ReutersNews {
	client := SharedHTTPClient
	config := GetSettingConfig()
	if config.HttpProxyEnabled && config.HttpProxy != "" {
		client.SetProxy(config.HttpProxy)
	}
	news := &models.ReutersNews{}
	//url := "https://www.reuters.com/pf/api/v3/content/fetch/articles-by-section-alias-or-id-v1?query={\"arc-site\":\"reuters\",\"fetch_type\":\"collection\",\"offset\":0,\"section_id\":\"/world/\",\"size\":9,\"uri\":\"/world/\",\"website\":\"reuters\"}&d=300&mxId=00000000&_website=reuters"
	url := "https://www.reuters.com/pf/api/v3/content/fetch/recent-stories-by-sections-v1?query=%7B%22section_ids%22%3A%22%2Fworld%2F%22%2C%22size%22%3A4%2C%22website%22%3A%22reuters%22%7D&d=334&mxId=00000000&_website=reuters"
	_, err := client.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "www.reuters.com").
		SetHeader("Origin", "https://www.reuters.com").
		SetHeader("Referer", "https://www.reuters.com/world/china/").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0").
		SetResult(news).
		Get(url)
	if err != nil {
		logger.SugaredLogger.Errorf("ReutersNew err:%s", err.Error())
		return news
	}
	//logger.SugaredLogger.Infof("Articles:%+v", news.Result.Articles)
	return news
}

func (m MarketNewsApi) InteractiveAnswer(page int, pageSize int, keyWord string) *models.InteractiveAnswer {
	client := SharedHTTPClient
	config := GetSettingConfig()
	if config.HttpProxyEnabled && config.HttpProxy != "" {
		client.SetProxy(config.HttpProxy)
	}
	url := fmt.Sprintf("https://irm.cninfo.com.cn/newircs/index/search?_t=%d", time.Now().Unix())
	answers := &models.InteractiveAnswer{}
	//logger.SugaredLogger.Infof("请求url:%s", url)
	_, err := client.SetTimeout(time.Duration(5)*time.Second).R().
		SetHeader("Host", "irm.cninfo.com.cn").
		SetHeader("Origin", "https://irm.cninfo.com.cn").
		SetHeader("Referer", "https://irm.cninfo.com.cn/views/interactiveAnswer").
		SetHeader("handleError", "true").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:142.0) Gecko/20100101 Firefox/142.0").
		SetFormData(map[string]string{
			"pageNo":      convertor.ToString(page),
			"pageSize":    convertor.ToString(pageSize),
			"searchTypes": "11",
			"highLight":   "true",
			"keyWord":     keyWord,
		}).
		SetResult(answers).
		Post(url)
	if err != nil {
		logger.SugaredLogger.Errorf("InteractiveAnswer-err:%+v", err)
	}
	//logger.SugaredLogger.Debugf("InteractiveAnswer-resp:%s", resp.Body())
	return answers

}

func (m MarketNewsApi) CailianpressWeb(searchWords string) *models.CailianpressWeb {
	res := &models.CailianpressWeb{}
	_, err := SharedHTTPClient.SetTimeout(time.Second*10).R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Host", "www.cls.cn").
		SetHeader("Origin", "https://www.cls.cn").
		SetHeader("Referer", "https://www.cls.cn/telegraph").
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36 Edg/117.0.2045.60").
		SetBody(fmt.Sprintf(`{"app":"CailianpressWeb","os":"web","sv":"8.4.6","category":"","keyword":"%s"}`, searchWords)).
		SetResult(res).
		Post("https://www.cls.cn/api/csw?app=CailianpressWeb&os=web&sv=8.4.6&sign=9f8797a1f4de66c2370f7a03990d2737")
	if err != nil {
		return nil
	}
	logger.SugaredLogger.Debug(res)

	return res
}

func (m MarketNewsApi) GetNews24HoursList(source string, limit int) *[]*models.Telegraph {
	news := &[]*models.Telegraph{}
	if source != "" {
		db.Dao.Model(news).Preload("TelegraphTags").Where("source=? and created_at>?", source, time.Now().Add(-24*time.Hour)).Order("data_time desc,is_red desc").Limit(limit).Find(news)
	} else {
		db.Dao.Model(news).Preload("TelegraphTags").Where("created_at>?", time.Now().Add(-24*time.Hour)).Order("data_time desc,is_red desc").Limit(limit).Find(news)
	}
	// 内容去重
	uniqueNews := make([]*models.Telegraph, 0)
	seenContent := make(map[string]bool)
	for _, item := range *news {
		tags := &[]models.Tags{}
		db.Dao.Model(&models.Tags{}).Where("id in ?", lo.Map(item.TelegraphTags, func(item models.TelegraphTags, index int) uint {
			return item.TagId
		})).Find(&tags)
		tagNames := lo.Map(*tags, func(item models.Tags, index int) string {
			return item.Name
		})
		item.SubjectTags = tagNames
		//logger.SugaredLogger.Infof("tagNames %v ，SubjectTags：%s", tagNames, item.SubjectTags)
		// 使用内容作为去重键值，可以考虑只使用内容的前几个字符或哈希值
		contentKey := strings.TrimSpace(item.Content)
		if contentKey != "" && !seenContent[contentKey] {
			seenContent[contentKey] = true
			uniqueNews = append(uniqueNews, item)
		}
	}
	return &uniqueNews
}

// GetNewsListData 分页获取新闻列表，page 从 1 开始，pageSize 为每页条数（<=0 时默认 20）。返回本页去重后的列表与总条数。
func (m MarketNewsApi) GetNewsListData(keyWord string, startTime time.Time, page, pageSize int) (*[]*models.Telegraph, int64) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	whereCond := "created_at>? and (title like ? or content like ?)"
	args := []any{startTime, "%" + keyWord + "%", "%" + keyWord + "%"}
	var total int64
	db.Dao.Model(&models.Telegraph{}).Where(whereCond, args...).Count(&total)
	offset := (page - 1) * pageSize
	news := &[]*models.Telegraph{}
	db.Dao.Model(news).Preload("TelegraphTags").Where(whereCond, args...).Order("data_time desc,is_red desc").Offset(offset).Limit(pageSize).Find(news)
	// 内容去重
	uniqueNews := make([]*models.Telegraph, 0)
	seenContent := make(map[string]bool)
	for _, item := range *news {
		contentKey := strings.TrimSpace(item.Content)
		if item.Title != "" {
			contentKey = strings.TrimSpace(item.Title)
		}
		if contentKey != "" && !seenContent[contentKey] {
			seenContent[contentKey] = true
			uniqueNews = append(uniqueNews, item)
		}
	}
	return &uniqueNews, total
}

func (m MarketNewsApi) GetUplimitHot(date string, limit int) map[string]any {
	if limit <= 0 {
		limit = 20
	}
	if date == "" {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		date = time.Now().In(loc).Format("2006-01-02")
	}
	apiUrl := fmt.Sprintf("https://api.zizizaizai.com/v3/open/review/uplimit/hot?date1=%s&limit=%d", date, limit)
	resp, err := SharedHTTPClient.SetTimeout(15*time.Second).R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36").
		SetHeader("Accept", "application/json").
		Get(apiUrl)
	if err != nil {
		logger.SugaredLogger.Errorf("GetUplimitHot error: %v", err)
		return map[string]any{"code": 50000, "message": "请求失败"}
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.SugaredLogger.Errorf("GetUplimitHot unmarshal error: %v", err)
		return map[string]any{"code": 50000, "message": "数据解析失败"}
	}
	return result
}
