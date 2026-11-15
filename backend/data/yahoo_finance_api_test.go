package data

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestYahooSymbol(t *testing.T) {
	tests := map[string]string{
		"00700.HK": "0700.HK",
		"hk02513":  "2513.HK",
		"hk10000":  "10000.HK",
		"AAPL.US":  "AAPL",
		"gb_aapl":  "AAPL",
		"USO":      "USO",
		"HKD.US":   "HKD",
		"BRK-B.US": "BRK-B",
	}

	for input, want := range tests {
		got, err := yahooSymbol(input)
		if err != nil || got != want {
			t.Errorf("yahooSymbol(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "HK0", "HKABC", "AAPL/other", "000001"} {
		if _, err := yahooSymbol(input); err == nil {
			t.Errorf("yahooSymbol(%q) should fail", input)
		}
	}
}

func TestYahooFallbackScope(t *testing.T) {
	tests := []struct {
		code, klt, adjust string
		want              bool
	}{
		{"AAPL.US", "101", "none", true},
		{"00700.HK", "101", "", true},
		{"AAPL.US", "101", "qfq", false},
		{"AAPL.US", "101", "hfq", false},
		{"AAPL.US", "5", "none", false},
		{"600519.SH", "101", "none", false},
	}
	for _, tt := range tests {
		if got := supportsYahooDailyFallback(tt.code, tt.klt, tt.adjust); got != tt.want {
			t.Errorf("supportsYahooDailyFallback(%q, %q, %q) = %v", tt.code, tt.klt, tt.adjust, got)
		}
	}
}

func TestYahooLive(t *testing.T) {
	if os.Getenv("TEST_YAHOO_LIVE") != "1" {
		t.Skip("set TEST_YAHOO_LIVE=1 to test Yahoo's public endpoints")
	}
	api := &YahooFinanceAPI{client: resty.New().SetTimeout(15 * time.Second), baseURL: yahooFinanceBaseURL}
	for _, code := range []string{"AAPL.US", "00700.HK"} {
		rows, err := api.DailyKLine(code, 5, "")
		if err != nil || len(rows) == 0 || len(rows) > 5 {
			t.Errorf("%s daily data: %d rows, %v", code, len(rows), err)
		}
	}
	if _, err := api.StockNews("AAPL.US", 5); err != nil {
		t.Errorf("AAPL news: %v", err)
	}
}

func TestYahooDailyKLine(t *testing.T) {
	first := time.Date(2026, 1, 2, 14, 30, 0, 0, time.UTC).Unix()
	second := time.Date(2026, 1, 5, 14, 30, 0, 0, time.UTC).Unix()
	third := time.Date(2026, 1, 6, 14, 30, 0, 0, time.UTC).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v8/finance/chart/AAPL" || r.URL.Query().Get("interval") != "1d" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		_, _ = fmt.Fprintf(w, `{"chart":{"result":[{"meta":{"exchangeTimezoneName":"America/New_York"},"timestamp":[%d,%d,%d],"indicators":{"quote":[{"open":[10,11,null],"high":[12,13,14],"low":[9,10,11],"close":[11,12,13],"volume":[100,200,300]}]}}],"error":null}}`, first, second, third)
	}))
	defer server.Close()
	api := &YahooFinanceAPI{client: resty.New(), baseURL: server.URL}
	rows, err := api.DailyKLine("AAPL.US", 1, "20260105")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Day != "2026-01-05" || rows[0].Close != "12" || rows[0].Volume != "200" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if rows[0].ChangeValue != "1" || rows[0].Amount != "" || rows[0].TurnoverRate != "" {
		t.Fatalf("unexpected derived or unavailable fields: %+v", rows[0])
	}
	rows, err = api.DailyKLine("AAPL.US", 10, "20260102")
	if err != nil || len(rows) != 1 || rows[0].Day != "2026-01-02" {
		t.Fatalf("inclusive cutoff: %+v, %v", rows, err)
	}
	if _, err := api.DailyKLine("AAPL.US", 10, "20260230"); err == nil {
		t.Fatal("invalid date should fail")
	}
}

func TestYahooStockNews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/finance/search" || r.URL.Query().Get("q") != "AAPL" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"news":[{"title":"Apple","publisher":"Example","link":"https://example.com/1","providerPublishTime":123,"relatedTickers":["AAPL"]},{"title":"Unrelated","link":"https://example.com/2","relatedTickers":["META"]}]}`))
	}))
	defer server.Close()
	api := &YahooFinanceAPI{client: resty.New(), baseURL: server.URL}
	items, err := api.StockNews("AAPL.US", 10)
	if err != nil || len(items) != 1 || items[0].Title != "Apple" || items[0].PublishedAt != 123 {
		t.Fatalf("unexpected news: %+v, %v", items, err)
	}
	if _, err := api.StockNews("AAPL.US", 21); err == nil {
		t.Fatal("invalid limit should fail")
	}
}

func TestYahooErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		code int
	}{
		{"status", "rate limited", http.StatusTooManyRequests},
		{"malformed", "{", http.StatusOK},
		{"missing fields", `{}`, http.StatusOK},
		{"provider error", `{"chart":{"error":{"code":"Not Found","description":"No data"}}}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			api := &YahooFinanceAPI{client: resty.New(), baseURL: server.URL}
			_, err := api.DailyKLine("AAPL.US", 10, "")
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.name == "status" && !strings.Contains(err.Error(), "429") {
				t.Fatalf("missing status in error: %v", err)
			}
			if tc.name == "missing fields" {
				if _, err := api.StockNews("AAPL.US", 10); err == nil {
					t.Fatal("missing news field should fail")
				}
			}
		})
	}
}
