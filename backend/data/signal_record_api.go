package data

import (
	"go-stock/backend/db"
	"go-stock/backend/models"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SignalRecordService 后台买卖点信号流水的落库与查询。
type SignalRecordService struct{}

func NewSignalRecordService() *SignalRecordService {
	return &SignalRecordService{}
}

// SaveSignalRecords 批量落库，按 (code,klt,family,kind,bar_time) 去重。
// 前端在同一根 K 线上重复上报时静默跳过，不会产生重复流水。
func (s *SignalRecordService) SaveSignalRecords(items []models.SignalRecord) error {
	if len(items) == 0 {
		return nil
	}
	return db.Dao.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "code"}, {Name: "klt"}, {Name: "family"}, {Name: "kind"}, {Name: "bar_time"},
		},
		DoNothing: true,
	}).CreateInBatches(items, 50).Error
}

// GetSignalRecordPage 分页查询流水：Keyword 模糊匹配代码/名称，
// StartTime/EndTime 按信号所在 K 线时间（bar_time，Unix 秒）筛选，结果按 K 线时间倒序。
// 分页参数缺失时兜底默认值，避免漏传时把整表拉走。
func (s *SignalRecordService) GetSignalRecordPage(query models.SignalRecordQuery) (*models.SignalRecordPageData, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 || query.PageSize > 200 {
		query.PageSize = 20
	}

	dbQuery := db.Dao.Model(&models.SignalRecord{})
	if kw := strings.TrimSpace(query.Keyword); kw != "" {
		like := "%" + kw + "%"
		dbQuery = dbQuery.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	if query.StartTime > 0 {
		dbQuery = dbQuery.Where("bar_time >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		dbQuery = dbQuery.Where("bar_time <= ?", query.EndTime)
	}

	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	list := make([]models.SignalRecord, 0)
	if err := dbQuery.Order("bar_time DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&list).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / query.PageSize
	if int(total)%query.PageSize > 0 {
		totalPages++
	}

	return &models.SignalRecordPageData{
		List:       list,
		Total:      total,
		Page:       query.Page,
		PageSize:   query.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ClearSignalRecords 清空流水（面板的「清空」按钮）。
func (s *SignalRecordService) ClearSignalRecords() error {
	return db.Dao.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.SignalRecord{}).Error
}

// signalStatAcc 统计累加器：同一「代码|周期」内的买点排队等卖点来平。
type signalStatAcc struct {
	item    models.SignalStatItem
	pending []float64 // 待平仓的买入价，FIFO 逐笔配对
}

func (a *signalStatAcc) close(ret float64) {
	a.item.Trades++
	if ret > 0 {
		a.item.Wins++
	}
	a.item.Return += ret
}

func (a *signalStatAcc) finish() models.SignalStatItem {
	it := a.item
	it.Open = len(a.pending)
	if it.Trades > 0 {
		it.WinRate = float64(it.Wins) / float64(it.Trades)
	}
	return it
}

// GetSignalStats 统计区间内的买卖点信号按「买点开仓、卖点平仓」操作下来的收益与胜率。
//
// 只统计 buysell 族（TEMA 转折是趋势提示，不构成买卖对）；无价格的记录（早期数据）直接跳过。
// 配对在「代码|周期」内进行——同一只票配了多个周期时，各周期的信号各自成笔，不跨周期混算；
// 展示与汇总再按代码合并。未配到卖点的买点计入 Open（持仓中），不参与胜率。
func (s *SignalRecordService) GetSignalStats(query models.SignalStatQuery) (*models.SignalStatResult, error) {
	dbQuery := db.Dao.Model(&models.SignalRecord{}).Where("family = ?", "buysell")
	if query.StartTime > 0 {
		dbQuery = dbQuery.Where("bar_time >= ?", query.StartTime)
	}
	if query.EndTime > 0 {
		dbQuery = dbQuery.Where("bar_time <= ?", query.EndTime)
	}

	list := make([]models.SignalRecord, 0)
	// 必须按时间正序配对：先买后卖
	if err := dbQuery.Order("bar_time ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}

	overall := &signalStatAcc{}
	byKey := make(map[string]*signalStatAcc)
	keys := make([]string, 0)

	for i := range list {
		r := list[i]
		if r.Price == nil {
			continue
		}
		key := r.Code + "|" + r.Klt
		acc, ok := byKey[key]
		if !ok {
			acc = &signalStatAcc{item: models.SignalStatItem{Code: r.Code, Name: r.Name}}
			byKey[key] = acc
			keys = append(keys, key)
		}
		if r.Kind == "buy" {
			acc.pending = append(acc.pending, *r.Price)
			continue
		}
		if len(acc.pending) == 0 {
			// 卖点没有对应买点（区间起点之前的买点未纳入统计），不成笔
			continue
		}
		buy := acc.pending[0]
		acc.pending = acc.pending[1:]
		ret := (*r.Price - buy) / buy
		acc.close(ret)
		overall.close(ret)
	}

	// 按代码合并（同一只票的多个周期合并成一条）
	byCode := make(map[string]models.SignalStatItem)
	codes := make([]string, 0)
	for _, key := range keys {
		it := byKey[key].finish()
		m, ok := byCode[it.Code]
		if !ok {
			m = models.SignalStatItem{Code: it.Code, Name: it.Name}
			codes = append(codes, it.Code)
		}
		if m.Name == "" {
			m.Name = it.Name
		}
		m.Trades += it.Trades
		m.Wins += it.Wins
		m.Return += it.Return
		m.Open += it.Open
		byCode[it.Code] = m
	}

	stocks := make([]models.SignalStatItem, 0, len(codes))
	for _, code := range codes {
		it := byCode[code]
		if it.Trades > 0 {
			it.WinRate = float64(it.Wins) / float64(it.Trades)
		}
		stocks = append(stocks, it)
	}
	// 收益高的排前面，便于直接看哪只票贡献最大
	sort.SliceStable(stocks, func(i, j int) bool {
		if stocks[i].Return != stocks[j].Return {
			return stocks[i].Return > stocks[j].Return
		}
		return stocks[i].Code < stocks[j].Code
	})

	return &models.SignalStatResult{Overall: overall.finish(), Stocks: stocks}, nil
}
