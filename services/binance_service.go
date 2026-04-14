package services

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"api-trade/models"
)

// Binance menyediakan beberapa mirror: api.binance.com, api1, api2, api3
// Jika satu diblokir, bisa coba mirror lain
const binanceBaseURL = "https://api3.binance.com"

// BinanceService menangani semua komunikasi dengan Binance public API
type BinanceService struct {
	client *http.Client
}

// NewBinanceService membuat instance baru BinanceService
func NewBinanceService() *BinanceService {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
		},
	}
	return &BinanceService{
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		},
	}
}

// doRequest membuat HTTP GET request dengan header yang proper
func (s *BinanceService) doRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Header agar tidak di-block Cloudflare / WAF
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	return s.client.Do(req)
}

// decodeJSON decode response body ke target, dengan cek content-type
func decodeJSON(resp *http.Response, target interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("gagal baca response: %w", err)
	}
	// Jika response bukan JSON (misal HTML dari Cloudflare block)
	if len(body) > 0 && body[0] == '<' {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return fmt.Errorf("Binance mengembalikan HTML bukan JSON (kemungkinan geo-block atau Cloudflare). Preview: %s", preview)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("gagal parse JSON: %w — body: %s", err, string(body[:min(len(body), 100)]))
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetTicker mengambil data harga 24hr untuk satu symbol
func (s *BinanceService) GetTicker(symbol string) (*models.TickerData, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/24hr?symbol=%s", binanceBaseURL, symbol)

	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request ke Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Binance error %d: %s", resp.StatusCode, string(body))
	}

	var ticker models.TickerData
	if err := decodeJSON(resp, &ticker); err != nil {
		return nil, fmt.Errorf("gagal parse response Binance: %w", err)
	}

	return &ticker, nil
}

// GetKlines mengambil data candlestick dari Binance
// interval: 1m, 5m, 15m, 30m, 1h, 4h, 1d, 1w
func (s *BinanceService) GetKlines(symbol, interval string, limit int) ([]models.KlineData, error) {
	if limit <= 0 {
		limit = 100
	}
	url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		binanceBaseURL, symbol, interval, limit)

	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request klines ke Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Binance klines error %d: %s", resp.StatusCode, string(body))
	}

	// Binance mengembalikan array of arrays
	var rawKlines [][]interface{}
	if err := decodeJSON(resp, &rawKlines); err != nil {
		return nil, fmt.Errorf("gagal parse klines: %w", err)
	}

	klines := make([]models.KlineData, 0, len(rawKlines))
	for _, raw := range rawKlines {
		if len(raw) < 7 {
			continue
		}

		kline := models.KlineData{}

		// Open time (index 0) - float64 dari JSON number
		if v, ok := raw[0].(float64); ok {
			kline.OpenTime = int64(v)
		}
		// Open (index 1) - string
		kline.Open = parseFloat(raw[1])
		// High (index 2)
		kline.High = parseFloat(raw[2])
		// Low (index 3)
		kline.Low = parseFloat(raw[3])
		// Close (index 4)
		kline.Close = parseFloat(raw[4])
		// Volume (index 5)
		kline.Volume = parseFloat(raw[5])
		// Close time (index 6)
		if v, ok := raw[6].(float64); ok {
			kline.CloseTime = int64(v)
		}

		klines = append(klines, kline)
	}

	return klines, nil
}

// GetTrending mengambil top movers dari semua pair USDT
func (s *BinanceService) GetTrending(limit int) ([]models.TrendingCoin, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/24hr", binanceBaseURL)

	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request trending ke Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Binance trending error %d: %s", resp.StatusCode, string(body))
	}

	var tickers []models.TickerData
	if err := decodeJSON(resp, &tickers); err != nil {
		return nil, fmt.Errorf("gagal parse trending: %w", err)
	}

	// Filter hanya pair USDT dan sort berdasarkan volume
	var result []models.TrendingCoin
	for _, t := range tickers {
		// Hanya ambil pair USDT
		if len(t.Symbol) < 4 || t.Symbol[len(t.Symbol)-4:] != "USDT" {
			continue
		}

		price, _ := strconv.ParseFloat(t.LastPrice, 64)
		change, _ := strconv.ParseFloat(t.PriceChangePercent, 64)
		volume, _ := strconv.ParseFloat(t.QuoteVolume, 64)

		// Filter volume minimal $1M
		if volume < 1_000_000 {
			continue
		}

		result = append(result, models.TrendingCoin{
			Symbol:    t.Symbol,
			Price:     price,
			Change24h: change,
			Volume:    volume,
		})
	}

	// Sort by absolute price change (top movers)
	sortTrendingByChange(result)

	// Ambil top N
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// parseFloat helper untuk parse string ke float64
func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case float64:
		return val
	}
	return 0
}

// sortTrendingByChange sort slice by absolute change descending (simple bubble sort)
func sortTrendingByChange(coins []models.TrendingCoin) {
	n := len(coins)
	abs := func(f float64) float64 {
		if f < 0 {
			return -f
		}
		return f
	}
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if abs(coins[j].Change24h) < abs(coins[j+1].Change24h) {
				coins[j], coins[j+1] = coins[j+1], coins[j]
			}
		}
	}
}
