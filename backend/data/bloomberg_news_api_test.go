package data

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestBloombergMarketNews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel>
			<item><title>Earlier</title><link>https://www.bloomberg.com/news/1</link><pubDate>Mon, 16 Nov 2026 00:40:46 GMT</pubDate><description><![CDATA[<p>First &amp; second</p>]]></description></item>
			<item><title>Later</title><link>https://www.bloomberg.com/news/2</link><pubDate>Mon, 16 Nov 2026 01:40:46 +0000</pubDate></item>
			<item><title>Duplicate</title><link>https://www.bloomberg.com/news/2</link><pubDate>Mon, 16 Nov 2026 01:40:46 +0000</pubDate></item>
		</channel></rss>`))
	}))
	defer server.Close()
	api := &BloombergNewsAPI{client: resty.New(), feedURL: server.URL}
	items, err := api.GetMarketNews()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Title != "Later" || items[1].Description != "First & second" ||
		items[1].PublishedAt.Format(time.RFC3339) != "2026-11-16T00:40:46Z" {
		t.Fatalf("unexpected RSS result: %+v", items)
	}
}

func TestBloombergMarketNewsErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		code int
	}{
		{"http", "unavailable", http.StatusTooManyRequests},
		{"invalid XML", "bad", http.StatusOK},
		{"missing channel", `<rss/>`, http.StatusOK},
		{"invalid date", `<rss><channel><item><title>X</title><link>https://bloomberg.com/x</link><pubDate>bad</pubDate></item></channel></rss>`, http.StatusOK},
		{"unsafe link", `<rss><channel><item><title>X</title><link>https://bloomberg.com.evil.test/x</link><pubDate>Mon, 16 Nov 2026 00:40:46 GMT</pubDate></item></channel></rss>`, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.code)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			api := &BloombergNewsAPI{client: resty.New(), feedURL: server.URL}
			_, err := api.GetMarketNews()
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.name == "http" && !strings.Contains(err.Error(), "429") {
				t.Fatalf("missing HTTP status: %v", err)
			}
		})
	}
}

func TestBloombergMarketNewsForAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel>
			<item><title>First</title><link>https://www.bloomberg.com/news/1</link><pubDate>Mon, 16 Nov 2026 01:40:46 GMT</pubDate><description>Summary</description></item>
			<item><title>Second</title><link>https://www.bloomberg.com/news/2</link><pubDate>Mon, 16 Nov 2026 00:40:46 GMT</pubDate></item>
		</channel></rss>`))
	}))
	defer server.Close()
	api := &BloombergNewsAPI{client: resty.New(), feedURL: server.URL}
	result, err := api.MarketNewsForAI(1)
	if err != nil || !strings.Contains(result, "First") || !strings.Contains(result, "Summary") ||
		!strings.Contains(result, "https://www.bloomberg.com/news/1") ||
		!strings.Contains(result, "2026-11-16T01:40:46Z") || strings.Contains(result, "Second") {
		t.Fatalf("unexpected tool result: %s %v", result, err)
	}
	if _, err := api.MarketNewsForAI(21); err == nil {
		t.Fatal("invalid limit accepted")
	}
}

func TestBloombergMarketNewsLive(t *testing.T) {
	if os.Getenv("TEST_BLOOMBERG_LIVE") != "1" {
		t.Skip("set TEST_BLOOMBERG_LIVE=1 to test the RSS feed")
	}
	api := &BloombergNewsAPI{client: resty.New().SetTimeout(20 * time.Second), feedURL: bloombergMarketsRSS}
	items, err := api.GetMarketNews()
	if err != nil || len(items) == 0 {
		t.Fatalf("RSS feed: %d items, %v", len(items), err)
	}
	t.Logf("RSS returned %d articles, latest %s", len(items), items[0].PublishedAt.Format(time.RFC3339))
}
