package data

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
)

const bloombergMarketsRSS = "https://www.bloomberg.com/feeds/markets/news.rss"

type BloombergNewsItem struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
}

type BloombergNewsAPI struct {
	client  *resty.Client
	feedURL string
}

func NewBloombergNewsAPI() *BloombergNewsAPI {
	return &BloombergNewsAPI{
		client:  CreateGlobalProxiedHTTPClient(20 * time.Second),
		feedURL: bloombergMarketsRSS,
	}
}

// GetMarketNews fetches the latest items from Bloomberg's market RSS feed.
func (a *BloombergNewsAPI) GetMarketNews() ([]BloombergNewsItem, error) {
	resp, err := a.client.R().SetHeader("User-Agent", "Mozilla/5.0").
		SetHeader("Accept", "application/rss+xml, application/xml, text/xml").Get(a.feedURL)
	if err != nil {
		return nil, fmt.Errorf("获取彭博市场新闻失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("彭博市场新闻返回 HTTP %d", resp.StatusCode())
	}
	var feed struct {
		XMLName xml.Name `xml:"rss"`
		Channel *struct {
			Items []struct {
				Title       string `xml:"title"`
				Description string `xml:"description"`
				Link        string `xml:"link"`
				PubDate     string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(resp.Body(), &feed); err != nil {
		return nil, fmt.Errorf("解析彭博 RSS 失败: %w", err)
	}
	if feed.Channel == nil {
		return nil, fmt.Errorf("彭博 RSS 缺少频道数据")
	}
	items := make([]BloombergNewsItem, 0, len(feed.Channel.Items))
	seen := make(map[string]bool)
	for _, entry := range feed.Channel.Items {
		link, err := url.Parse(strings.TrimSpace(entry.Link))
		if err != nil || link.Scheme != "https" ||
			(link.Hostname() != "bloomberg.com" && !strings.HasSuffix(link.Hostname(), ".bloomberg.com")) ||
			link.User != nil {
			return nil, fmt.Errorf("彭博 RSS 包含无效原文链接")
		}
		date, err := mail.ParseDate(strings.TrimSpace(entry.PubDate))
		if err != nil {
			return nil, fmt.Errorf("彭博 RSS 发布时间无效: %w", err)
		}
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			return nil, fmt.Errorf("彭博 RSS 包含空标题")
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(entry.Description))
		if err != nil {
			return nil, fmt.Errorf("解析彭博 RSS 摘要失败: %w", err)
		}
		if seen[link.String()] {
			continue
		}
		seen[link.String()] = true
		items = append(items, BloombergNewsItem{
			Title: title, Description: strings.TrimSpace(doc.Text()),
			URL: link.String(), PublishedAt: date,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	return items, nil
}

// MarketNewsForAI returns a bounded current feed, not point-in-time history.
func (a *BloombergNewsAPI) MarketNewsForAI(limit int) (string, error) {
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 20 {
		return "", fmt.Errorf("新闻条数必须在 1 至 20 之间")
	}
	items, err := a.GetMarketNews()
	if err != nil {
		return "", err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	result, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("序列化彭博市场新闻失败: %w", err)
	}
	return "来源：Bloomberg Markets RSS；新闻为当前数据，不适用于历史时点回测，原文可能需要订阅。发布时间包含时区。\n" + string(result), nil
}
