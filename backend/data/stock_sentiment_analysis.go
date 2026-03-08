package data

import (
	"bufio"
	_ "embed"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/fileutil"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-ego/gse"
)

const basefreq float64 = 100

// 金融情感词典，包含股票市场相关的专业词汇
var (
	seg gse.Segmenter

	// 正面金融词汇及其权重
	positiveFinanceWords = map[string]float64{
		"涨": 1.0, "上涨": 2.0, "涨停": 3.0, "牛市": 3.0, "反弹": 2.0, "新高": 2.5,
		"利好": 2.5, "增持": 2.0, "买入": 2.0, "推荐": 1.5, "看多": 2.0,
		"盈利": 2.0, "增长": 2.0, "超预期": 2.5, "强劲": 1.5, "回升": 1.5,
		"复苏": 2.0, "突破": 2.0, "创新高": 3.0, "回暖": 1.5, "上扬": 1.5,
		"利好消息": 3.0, "收益增长": 2.5, "利润增长": 2.5, "业绩优异": 2.5,
		"潜力股": 2.0, "绩优股": 2.0, "强势": 1.5, "走高": 1.5, "攀升": 1.5,
		"大涨": 2.5, "飙升": 3.0, "井喷": 3.0, "暴涨": 3.0,
	}

	// 负面金融词汇及其权重
	negativeFinanceWords = map[string]float64{
		"跌": 2.0, "下跌": 2.0, "跌停": 3.0, "熊市": 3.0, "回调": 2.5, "新低": 2.5,
		"利空": 2.5, "减持": 2.0, "卖出": 2.0, "看空": 2.0, "亏损": 2.5,
		"下滑": 2.0, "萎缩": 2.0, "不及预期": 2.5, "疲软": 1.5, "恶化": 2.0,
		"衰退": 2.0, "跌破": 2.0, "创新低": 3.0, "走弱": 2.5, "下挫": 2.5,
		"利空消息": 3.0, "收益下降": 2.5, "利润下滑": 2.5, "业绩不佳": 2.5,
		"垃圾股": 2.0, "风险股": 2.0, "弱势": 2.5, "走低": 2.5, "缩量": 2.5,
		"大跌": 2.5, "暴跌": 3.0, "崩盘": 3.0, "跳水": 3.0, "重挫": 3.0, "跌超": 2.5, "跌逾": 2.5, "跌近": 3.0,
		"被抓": 3.0, "被抓捕": 3.0, "回吐": 3.0, "转跌": 3.0,
	}

	// 否定词，用于反转情感极性
	negationWords = map[string]struct{}{
		"不": {}, "没": {}, "无": {}, "非": {}, "未": {}, "别": {}, "勿": {},
	}

	// 程度副词，用于调整情感强度
	degreeWords = map[string]float64{
		"非常": 1.8, "极其": 2.2, "太": 1.8, "很": 1.5,
		"比较": 0.8, "稍微": 0.6, "有点": 0.7, "显著": 1.5,
		"大幅": 1.8, "急剧": 2.0, "轻微": 0.6, "小幅": 0.7, "逾": 1.8, "超": 1.8,
	}

	// 转折词，用于识别情感转折
	transitionWords = map[string]struct{}{
		"但是": {}, "然而": {}, "不过": {}, "却": {}, "可是": {},
	}
)

//go:embed data/dict/base.txt
var baseDict string

//go:embed data/dict/zh/s_1.txt
var zhDict string

func InitAnalyzeSentiment() {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Error(fmt.Sprintf("panic: %v", r))
		}
	}()
	// 加载简体中文词典
	//err := seg.LoadDict("zh_s")
	//if err != nil {
	//	logger.SugaredLogger.Error(err.Error())
	//}

	err := seg.LoadDictEmbed(baseDict)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
	} else {
		logger.SugaredLogger.Info("加载默认词典成功")
	}
	seg.CalcToken()

	stocks := &[]StockBasic{}
	db.Dao.Model(&StockBasic{}).Find(stocks)
	for _, stock := range *stocks {
		if strutil.Trim(stock.Name) == "" {
			continue
		}
		err := seg.AddToken(stock.Name, basefreq+100, "n")
		if strutil.Trim(stock.BKName) != "" {
			err = seg.AddToken(stock.BKName, basefreq+100, "n")
		}
		if err != nil {
			logger.SugaredLogger.Errorf("添加%s失败:%s", stock.Name, err.Error())
		}
	}
	logger.SugaredLogger.Info("加载股票名称词典成功")

	stockhks := &[]models.StockInfoHK{}
	db.Dao.Model(&models.StockInfoHK{}).Find(stockhks)
	for _, stock := range *stockhks {
		if strutil.Trim(stock.Name) == "" {
			continue
		}
		err := seg.AddToken(stock.Name, basefreq+100, "n")
		if strutil.Trim(stock.BKName) != "" {
			err = seg.AddToken(stock.BKName, basefreq+100, "n")
		}
		if err != nil {
			logger.SugaredLogger.Errorf("添加%s失败:%s", stock.Name, err.Error())
		}
	}
	logger.SugaredLogger.Info("加载港股名称词典成功")
	//stockus := &[]models.StockInfoUS{}
	//db.Dao.Model(&models.StockInfoUS{}).Where("trim(name) != ?", "").Find(stockus)
	//for _, stock := range *stockus {
	//	err := seg.AddToken(stock.Name, 500)
	//	if err != nil {
	//		logger.SugaredLogger.Errorf("添加%s失败:%s", stock.Name, err.Error())
	//	}
	//}
	tags := &[]models.Tags{}
	db.Dao.Model(&models.Tags{}).Where("type = ?", "subject").Find(tags)
	for _, tag := range *tags {
		if tag.Name == "" {
			continue
		}
		err := seg.AddToken(tag.Name, basefreq+100, "n")
		if err != nil {
			logger.SugaredLogger.Errorf("添加%s失败:%s", tag.Name, err.Error())
		} else {
			logger.SugaredLogger.Infof("添加tags词典[%s]成功", tag.Name)
		}
	}
	logger.SugaredLogger.Info("加载tags词典成功")
	seg.CalcToken()
	//加载用户自定义词典 先判断用户词典是否存在
	if fileutil.IsExist("data/dict/user.txt") {
		lines, err := fileutil.ReadFileByLine("data/dict/user.txt")
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
			return
		}
		for _, line := range lines {
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			k := strutil.SplitAndTrim(line, " ")
			if len(k) == 0 {
				continue
			}
			_, _, ok := seg.Find(k[0])
			switch len(k) {
			case 1:
				if ok {
					err = seg.ReAddToken(k[0], basefreq)
				} else {
					err = seg.AddToken(k[0], basefreq)
				}
			case 2:
				freq, _ := convertor.ToFloat(k[1])
				if ok {
					err = seg.ReAddToken(k[0], freq)
				} else {
					err = seg.AddToken(k[0], freq)
				}
			case 3:
				freq, _ := convertor.ToFloat(k[1])
				if ok {
					err = seg.ReAddToken(k[0], freq, k[2])
				} else {
					err = seg.AddToken(k[0], freq, k[2])
				}
			default:
				logger.SugaredLogger.Errorf("用户词典格式错误:%s", line)
			}
			logger.SugaredLogger.Infof("添加用户词典[%s]成功", line)
		}
		if err != nil {
			logger.SugaredLogger.Error(err.Error())
		} else {
			logger.SugaredLogger.Infof("加载用户词典成功")
		}
	} else {
		logger.SugaredLogger.Info("用户词典不存在")
	}
	seg.CalcToken()
}

// getWordWeight 获取词汇权重
func getWordWeight(word string) float64 {
	// 从分词器获取词汇权重

	freq, pos, ok := seg.Dictionary().Find([]byte(word))
	if ok {
		logger.SugaredLogger.Infof("获取%s的权重:%f,pos:%s,ok:%v", word, freq, pos, ok)
		return freq
	}
	return 0
}

// SortByWeightAndFrequency 按权重和频次排序词频结果
func SortByWeightAndFrequency(frequencies map[string]models.WordFreqWithWeight) []models.WordFreqWithWeight {
	// 将map转换为slice以便排序
	freqSlice := make([]models.WordFreqWithWeight, 0, len(frequencies))
	for _, freq := range frequencies {
		freqSlice = append(freqSlice, freq)
	}

	// 按权重*频次降序排列
	sort.Slice(freqSlice, func(i, j int) bool {
		return freqSlice[i].Weight*float64(freqSlice[i].Frequency) > freqSlice[j].Weight*float64(freqSlice[j].Frequency)
	})
	logger.SugaredLogger.Infof("排序后的结果:%v", freqSlice)

	return freqSlice
}

// FilterAndSortWords 过滤标点符号并按权重频次排序
func FilterAndSortWords(frequencies map[string]models.WordFreqWithWeight) []models.WordFreqWithWeight {
	// 先过滤标点符号和分隔符
	cleanFrequencies := FilterPunctuationAndSeparators(frequencies)

	// 再按权重和频次排序
	sortedFrequencies := SortByWeightAndFrequency(cleanFrequencies)

	return sortedFrequencies
}
func FilterPunctuationAndSeparators(frequencies map[string]models.WordFreqWithWeight) map[string]models.WordFreqWithWeight {
	filteredWords := make(map[string]models.WordFreqWithWeight)

	for word, freqInfo := range frequencies {
		// 过滤纯标点符号和分隔符
		if !isPunctuationOrSeparator(word) {
			filteredWords[word] = freqInfo
		}
	}
	return filteredWords
}

// isPunctuationOrSeparator 判断是否为标点符号或分隔符
func isPunctuationOrSeparator(word string) bool {
	// 空字符串
	if strings.TrimSpace(word) == "" {
		return true
	}

	// 检查是否全部由标点符号组成
	for _, r := range word {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// FilterWithRegex 使用正则表达式过滤标点和特殊字符
func FilterWithRegex(frequencies map[string]models.WordFreqWithWeight) map[string]models.WordFreqWithWeight {
	filteredWords := make(map[string]models.WordFreqWithWeight)

	// 匹配标点符号、特殊字符的正则表达式
	punctuationRegex := regexp.MustCompile(`^[[:punct:][:space:]]+$`)

	for word, freqInfo := range frequencies {
		// 过滤纯标点符号
		if !punctuationRegex.MatchString(word) && strings.TrimSpace(word) != "" {
			filteredWords[word] = freqInfo
		}
	}
	return filteredWords
}

// countWordFrequencyWithWeight 统计词频并包含权重信息
func countWordFrequencyWithWeight(text string) map[string]models.WordFreqWithWeight {
	words := splitWords(text)
	freqMap := make(map[string]models.WordFreqWithWeight)

	// 统计词频
	wordCount := make(map[string]int)
	for _, word := range words {
		wordCount[word]++
	}

	// 构建包含权重的结果
	for word, frequency := range wordCount {
		weight := getWordWeight(word)
		if weight >= basefreq {
			freqMap[word] = models.WordFreqWithWeight{
				Word:      word,
				Frequency: frequency,
				Weight:    weight,
				Score:     float64(frequency) * weight,
			}
		}

	}

	return freqMap
}

// AnalyzeSentimentWithFreqWeight 带权重词频统计的情感分析
func AnalyzeSentimentWithFreqWeight(text string) (models.SentimentResult, map[string]models.WordFreqWithWeight) {
	// 原有情感分析逻辑
	result := AnalyzeSentiment(text)

	// 带权重的词频统计
	frequencies := countWordFrequencyWithWeight(text)

	return result, frequencies
}

const (
	Positive models.SentimentType = iota
	Negative
	Neutral
)

// AnalyzeSentiment 判断文本的情感
func AnalyzeSentiment(text string) models.SentimentResult {
	// 初始化得分
	score := 0.0
	positiveCount := 0
	negativeCount := 0

	// 分词（简单按单个字符分割）
	words := splitWords(text)

	// 检查文本是否包含转折词，并分割成两部分
	var transitionIndex int
	var hasTransition bool
	for i, word := range words {
		if _, ok := transitionWords[word]; ok {
			transitionIndex = i
			hasTransition = true
			break
		}
	}

	// 处理有转折的文本
	if hasTransition {
		// 转折前的部分
		preTransitionWords := words[:transitionIndex]
		preScore, prePos, preNeg := calculateScore(preTransitionWords)

		// 转折后的部分，权重加倍
		postTransitionWords := words[transitionIndex+1:]
		postScore, postPos, postNeg := calculateScore(postTransitionWords)
		postScore *= 1.5 // 转折后的情感更重要

		score = preScore + postScore
		positiveCount = prePos + postPos
		negativeCount = preNeg + postNeg
	} else {
		// 没有转折的文本
		score, positiveCount, negativeCount = calculateScore(words)
	}

	// 确定情感类别
	var category models.SentimentType
	switch {
	case score > 1.0:
		category = Positive
	case score < -1.0:
		category = Negative
	default:
		category = Neutral
	}

	return models.SentimentResult{
		Score:         score,
		Category:      category,
		PositiveCount: positiveCount,
		NegativeCount: negativeCount,
		Description:   GetSentimentDescription(category),
	}
}

// 计算情感得分
func calculateScore(words []string) (float64, int, int) {
	score := 0.0
	positiveCount := 0
	negativeCount := 0

	// 遍历每个词，计算情感得分
	for i, word := range words {
		// 首先检查是否为程度副词
		degree, isDegree := degreeWords[word]

		// 检查是否为否定词
		_, isNegation := negationWords[word]

		// 检查是否为金融正面词
		if posScore, isPositive := positiveFinanceWords[word]; isPositive {
			// 检查前一个词是否为否定词或程度副词
			if i > 0 {
				prevWord := words[i-1]
				if _, isNeg := negationWords[prevWord]; isNeg {
					score -= posScore
					negativeCount++
					continue
				}

				if deg, isDeg := degreeWords[prevWord]; isDeg {
					score += posScore * deg
					positiveCount++
					continue
				}
			}

			score += posScore
			positiveCount++
			continue
		}

		// 检查是否为金融负面词
		if negScore, isNegative := negativeFinanceWords[word]; isNegative {
			// 检查前一个词是否为否定词或程度副词
			if i > 0 {
				prevWord := words[i-1]
				if _, isNeg := negationWords[prevWord]; isNeg {
					score += negScore
					positiveCount++
					continue
				}

				if deg, isDeg := degreeWords[prevWord]; isDeg {
					score -= negScore * deg
					negativeCount++
					continue
				}
			}

			score -= negScore
			negativeCount++
			continue
		}

		// 处理程度副词（如果后面跟着情感词）
		if isDegree && i+1 < len(words) {
			nextWord := words[i+1]

			if posScore, isPositive := positiveFinanceWords[nextWord]; isPositive {
				score += posScore * degree
				positiveCount++
				continue
			}

			if negScore, isNegative := negativeFinanceWords[nextWord]; isNegative {
				score -= negScore * degree
				negativeCount++
				continue
			}
		}

		// 处理否定词（如果后面跟着情感词）
		if isNegation && i+1 < len(words) {
			nextWord := words[i+1]

			if posScore, isPositive := positiveFinanceWords[nextWord]; isPositive {
				score -= posScore
				negativeCount++
				continue
			}

			if negScore, isNegative := negativeFinanceWords[nextWord]; isNegative {
				score += negScore
				positiveCount++
				continue
			}
		}
	}

	return score, positiveCount, negativeCount
}

// 简单的分词函数，考虑了中文和英文
func splitWords(text string) []string {
	return seg.Cut(text, true)
}

// GetSentimentDescription 获取情感类别的文本描述
func GetSentimentDescription(category models.SentimentType) string {
	switch category {
	case Positive:
		return "看涨"
	case Negative:
		return "看跌"
	case Neutral:
		return "中性"
	default:
		return "未知"
	}
}

func main() {
	// 从命令行读取输入
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("请输入要分析的股市相关文本（输入exit退出）：")

	for {
		fmt.Print("> ")
		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("读取输入时出错:", err)
			continue
		}

		// 去除换行符
		text = strings.TrimSpace(text)

		// 检查是否退出
		if text == "exit" {
			break
		}

		// 分析情感
		result := AnalyzeSentiment(text)

		// 输出结果
		fmt.Printf("情感分析结果: %s (得分: %.2f, 正面词:%d, 负面词:%d)\n",
			GetSentimentDescription(result.Category),
			result.Score,
			result.PositiveCount,
			result.NegativeCount)
	}
}

func SaveAnalyzeSentimentWithFreqWeight(frequencies []models.WordFreqWithWeight) {

	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].Frequency > frequencies[j].Frequency
	})
	wordAnalyzes := make([]models.WordAnalyze, 0)
	limit := 10
	if len(frequencies) < limit {
		limit = len(frequencies)
	}
	for _, freq := range frequencies[:limit] {
		wordAnalyze := models.WordAnalyze{
			WordFreqWithWeight: freq,
		}
		wordAnalyzes = append(wordAnalyzes, wordAnalyze)
	}
	if len(wordAnalyzes) > 0 {
		db.Dao.CreateInBatches(wordAnalyzes, 1000)
	}
}

func SaveStockSentimentAnalysis(result models.SentimentResult) {
	db.Dao.Create(&models.SentimentResultAnalyze{
		SentimentResult: result,
	})
}

func NewsAnalyze(text string, save bool) (models.SentimentResult, []models.WordFreqWithWeight) {
	if text == "" {
		telegraphs := NewMarketNewsApi().GetNews24HoursList("", 1000*10)
		messageText := strings.Builder{}
		for _, telegraph := range *telegraphs {
			messageText.WriteString(telegraph.Content + "\n")
		}
		text = messageText.String()
	}
	result, frequencies := AnalyzeSentimentWithFreqWeight(text)
	// 过滤标点符号和分隔符
	cleanFrequencies := FilterAndSortWords(frequencies)
	if save {
		go SaveAnalyzeSentimentWithFreqWeight(cleanFrequencies)
		go SaveStockSentimentAnalysis(result)
	}
	return result, cleanFrequencies
}
