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

func TestPublicMacroWorldBank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/country/CHN/indicator/FP.CPI.TOTL.ZG" || r.URL.Query().Get("per_page") != "2" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		_, _ = w.Write([]byte(`[{"lastupdated":"2026-10-01"},[{"date":"2025","value":0},{"date":"2024","value":null}]]`))
	}))
	defer server.Close()
	api := &PublicMacroAPI{client: resty.New(), worldBankURL: server.URL}
	out, err := api.WorldBankIndicator("chn", "inflation", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"世界银行", "2026-10-01", "2025 | 0.0000", "2024 | 暂无数据", "FP.CPI.TOTL.ZG"} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q in %s", expected, out)
		}
	}
}

func TestPublicMacroTreasury(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("filter") != "country_currency_desc:eq:China-Renminbi" || q.Get("sort") != "-record_date" || q.Get("page[size]") != "5" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[{"record_date":"2026-09-30","exchange_rate":"6.705"}]}`))
	}))
	defer server.Close()
	api := &PublicMacroAPI{client: resty.New(), treasuryURL: server.URL}
	out, err := api.TreasuryExchangeRates("cny", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"USD/CNY", "季度", "不是实时", "2026-09-30", "6.705", "fiscaldata.treasury.gov"} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q in %s", expected, out)
		}
	}
}

func TestPublicMacroLive(t *testing.T) {
	if os.Getenv("GO_STOCK_RUN_PUBLIC_MACRO_LIVE") != "1" {
		t.Skip("set GO_STOCK_RUN_PUBLIC_MACRO_LIVE=1 to query public APIs")
	}
	api := &PublicMacroAPI{client: CreateProxiedHTTPClient(20*time.Second, os.Getenv("HTTPS_PROXY")), worldBankURL: worldBankURL, treasuryURL: treasuryURL}
	for _, tc := range []struct {
		name string
		call func() (string, error)
	}{
		{"world bank", func() (string, error) { return api.WorldBankIndicator("CHN", "inflation", 2) }},
		{"treasury", func() (string, error) { return api.TreasuryExchangeRates("CNY", 2) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if out, err := tc.call(); err != nil || len(out) == 0 {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

func TestPublicMacroErrors(t *testing.T) {
	status := http.StatusOK
	body := `[]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	api := &PublicMacroAPI{client: resty.New(), worldBankURL: server.URL, treasuryURL: server.URL}
	for _, tc := range []struct {
		name string
		call func() (string, error)
	}{
		{"invalid country", func() (string, error) { return api.WorldBankIndicator("../", "inflation", 1) }},
		{"invalid indicator", func() (string, error) { return api.WorldBankIndicator("CHN", "bad", 1) }},
		{"invalid limit", func() (string, error) { return api.TreasuryExchangeRates("CNY", 31) }},
		{"invalid currency", func() (string, error) { return api.TreasuryExchangeRates("bad", 1) }},
		{"malformed world bank", func() (string, error) { return api.WorldBankIndicator("CHN", "inflation", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.call(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	status = http.StatusTooManyRequests
	if _, err := api.TreasuryExchangeRates("CNY", 1); err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected HTTP error, got %v", err)
	}
	status, body = http.StatusOK, `{"data":[]}`
	if _, err := api.TreasuryExchangeRates("CNY", 1); err == nil {
		t.Fatal("expected no data error")
	}
}
