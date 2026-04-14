package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"api-trade/models"
)

const bybitBaseURL = "https://api.bybit.com"

// BybitService mengambil data market dari Bybit.
// Keunggulan vs CoinGecko: menyediakan volume per-candle (OHLCV lengkap),
// rate limit lebih tinggi (120 req/menit), data real-time dari exchange.
type BybitService struct {
	client *http.Client
}

// NewBybitService membuat instance baru BybitService
func NewBybitService() *BybitService {
	timeout := bybitHTTPTimeout()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Batasi tunggu header agar tidak menggantung di koneksi jelek (sisanya untuk baca body)
	if timeout > 10*time.Second {
		transport.ResponseHeaderTimeout = timeout - 2*time.Second
	}
	return &BybitService{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// bybitHTTPTimeout durasi total per request (dial + TLS + header + body).
// Default 45s — jaringan lambat/ISP sering gagal di 15s. Atur BYBIT_HTTP_TIMEOUT_SEC (5–120).
func bybitHTTPTimeout() time.Duration {
	const defaultSec = 45
	v := strings.TrimSpace(os.Getenv("BYBIT_HTTP_TIMEOUT_SEC"))
	if v == "" {
		return defaultSec * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 5 {
		return defaultSec * time.Second
	}
	if n > 120 {
		n = 120
	}
	return time.Duration(n) * time.Second
}

func isRetryableBybitNetworkErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// os.ErrDeadlineExceeded (Go 1.15+)
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	// pesan dari net/http untuk kegagalan sementara
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "EOF")
}

// =============================================
// Response structs (internal)
// =============================================

type bybitResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

type bybitTickerResult struct {
	Category string        `json:"category"`
	List     []bybitTicker `json:"list"`
}

type bybitTicker struct {
	Symbol        string `json:"symbol"`
	LastPrice     string `json:"lastPrice"`
	HighPrice24h  string `json:"highPrice24h"`
	LowPrice24h   string `json:"lowPrice24h"`
	Volume24h     string `json:"volume24h"`
	Turnover24h   string `json:"turnover24h"` // Volume dalam USDT
	Price24hPcnt  string `json:"price24hPcnt"`
}

type bybitKlineResult struct {
	Symbol   string     `json:"symbol"`
	Category string     `json:"category"`
	List     [][]string `json:"list"`
	// Setiap elemen: [startTime, open, high, low, close, volume, turnover]
}

// =============================================
// GetTicker — harga realtime dari Bybit
// =============================================

func (s *BybitService) GetTicker(symbol string) (*models.TickerData, error) {
	sym := normalizeBybitSymbol(symbol)
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s", bybitBaseURL, sym)

	body, err := s.get(url)
	if err != nil {
		return nil, err
	}

	var resp bybitResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse bybit response: %w", err)
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit error %d: %s", resp.RetCode, resp.RetMsg)
	}

	var result bybitTickerResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse bybit ticker: %w", err)
	}
	if len(result.List) == 0 {
		return nil, fmt.Errorf("symbol %s tidak ditemukan di Bybit", sym)
	}

	t := result.List[0]

	// Bybit price24hPcnt adalah desimal (0.0112 = 1.12%), konversi ke persen
	pctDecimal, _ := strconv.ParseFloat(t.Price24hPcnt, 64)
	pctPercent := fmt.Sprintf("%.4f", pctDecimal*100)

	// Hitung PriceChange dari lastPrice dan pcnt
	lastPriceF, _ := strconv.ParseFloat(t.LastPrice, 64)
	changeAmt := lastPriceF * pctDecimal
	priceChange := fmt.Sprintf("%.8f", changeAmt)

	return &models.TickerData{
		Symbol:             t.Symbol,
		PriceChange:        priceChange,
		PriceChangePercent: pctPercent,
		LastPrice:          t.LastPrice,
		HighPrice:          t.HighPrice24h,
		LowPrice:           t.LowPrice24h,
		Volume:             t.Volume24h,   // base asset volume (e.g., BTC)
		QuoteVolume:        t.Turnover24h, // USDT volume — dipakai di handler
	}, nil
}

// =============================================
// GetKlines — OHLCV candlestick data dari Bybit
// =============================================

func (s *BybitService) GetKlines(symbol, timeframe string, limit int) ([]models.KlineData, error) {
	sym := normalizeBybitSymbol(symbol)
	interval := bybitInterval(timeframe)

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	url := fmt.Sprintf("%s/v5/market/kline?category=linear&symbol=%s&interval=%s&limit=%d",
		bybitBaseURL, sym, interval, limit)

	body, err := s.get(url)
	if err != nil {
		return nil, err
	}

	var resp bybitResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse bybit response: %w", err)
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit error %d: %s", resp.RetCode, resp.RetMsg)
	}

	var result bybitKlineResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse bybit klines: %w", err)
	}

	if len(result.List) == 0 {
		return nil, fmt.Errorf("tidak ada data kline untuk %s %s", sym, timeframe)
	}

	// Bybit mengembalikan data TERBARU di index 0 (reverse order)
	// Kita harus reverse agar indeks 0 = candle TERLAMA
	klines := make([]models.KlineData, 0, len(result.List))
	for i := len(result.List) - 1; i >= 0; i-- {
		row := result.List[i]
		if len(row) < 6 {
			continue
		}
		// [startTime, open, high, low, close, volume, turnover]
		openTime, _ := strconv.ParseInt(row[0], 10, 64)
		open, _ := strconv.ParseFloat(row[1], 64)
		high, _ := strconv.ParseFloat(row[2], 64)
		low, _ := strconv.ParseFloat(row[3], 64)
		close, _ := strconv.ParseFloat(row[4], 64)
		volume, _ := strconv.ParseFloat(row[5], 64)

		klines = append(klines, models.KlineData{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: openTime + bybitIntervalMs(timeframe),
		})
	}

	return klines, nil
}

// =============================================
// GetTrending — top crypto by volume dari Bybit
// =============================================

func (s *BybitService) GetTrending(limit int) ([]models.TrendingCoin, error) {
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear", bybitBaseURL)

	body, err := s.get(url)
	if err != nil {
		return nil, err
	}

	var resp bybitResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse bybit response: %w", err)
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit error %d: %s", resp.RetCode, resp.RetMsg)
	}

	var result bybitTickerResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse bybit tickers: %w", err)
	}

	// Filter hanya pasangan USDT perpetual
	type tickerWithVol struct {
		t   bybitTicker
		vol float64
	}
	var usdt []tickerWithVol
	for _, t := range result.List {
		if !strings.HasSuffix(t.Symbol, "USDT") {
			continue
		}
		vol, _ := strconv.ParseFloat(t.Turnover24h, 64)
		usdt = append(usdt, tickerWithVol{t: t, vol: vol})
	}

	// Sort descending by volume (turnover USDT)
	sort.Slice(usdt, func(i, j int) bool {
		return usdt[i].vol > usdt[j].vol
	})

	if limit <= 0 {
		limit = 20
	}
	if limit > len(usdt) {
		limit = len(usdt)
	}

	coins := make([]models.TrendingCoin, 0, limit)
	for _, item := range usdt[:limit] {
		t := item.t
		price, _ := strconv.ParseFloat(t.LastPrice, 64)
		pctDecimal, _ := strconv.ParseFloat(t.Price24hPcnt, 64)
		coins = append(coins, models.TrendingCoin{
			Symbol:    t.Symbol,
			Price:     price,
			Change24h: pctDecimal * 100, // konversi ke persen
			Volume:    item.vol,
		})
	}

	return coins, nil
}

// =============================================
// Helpers
// =============================================

// get melakukan HTTP GET dengan beberapa percobaan ulang untuk error jaringan sementara.
func (s *BybitService) get(url string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
		}
		body, err := s.getOnce(url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isRetryableBybitNetworkErr(err) || attempt == maxAttempts {
			return nil, fmt.Errorf("bybit request gagal: %w", err)
		}
	}
	return nil, fmt.Errorf("bybit request gagal: %w", lastErr)
}

func (s *BybitService) getOnce(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; crypto-trader/2.0)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		preview := len(body)
		if preview > 200 {
			preview = 200
		}
		return nil, fmt.Errorf("bybit HTTP %d: %s", resp.StatusCode, string(body[:preview]))
	}

	return body, nil
}

// normalizeBybitSymbol memastikan format BTCUSDT (uppercase, hapus / atau -)
func normalizeBybitSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "-", "")
	// Jika tidak ada suffix, tambahkan USDT
	if !strings.Contains(s, "USDT") && !strings.Contains(s, "BTC") && !strings.Contains(s, "ETH") {
		s = s + "USDT"
	}
	return s
}

// bybitInterval mengkonversi timeframe standar ke format Bybit
// Bybit: 1,3,5,15,30,60,120,240,360,720,D,M,W
func bybitInterval(tf string) string {
	switch strings.ToLower(tf) {
	case "1m":
		return "1"
	case "3m":
		return "3"
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "1h", "60m":
		return "60"
	case "2h":
		return "120"
	case "4h":
		return "240"
	case "6h":
		return "360"
	case "12h":
		return "720"
	case "1d":
		return "D"
	case "1w":
		return "W"
	case "1mo":
		return "M"
	default:
		return "60" // default 1h
	}
}

// bybitIntervalMs mengembalikan durasi interval dalam milliseconds (untuk CloseTime)
func bybitIntervalMs(tf string) int64 {
	switch strings.ToLower(tf) {
	case "1m":
		return 60_000
	case "3m":
		return 3 * 60_000
	case "5m":
		return 5 * 60_000
	case "15m":
		return 15 * 60_000
	case "30m":
		return 30 * 60_000
	case "1h":
		return 60 * 60_000
	case "2h":
		return 2 * 60 * 60_000
	case "4h":
		return 4 * 60 * 60_000
	case "6h":
		return 6 * 60 * 60_000
	case "12h":
		return 12 * 60 * 60_000
	case "1d":
		return 24 * 60 * 60_000
	case "1w":
		return 7 * 24 * 60 * 60_000
	default:
		return 60 * 60_000
	}
}

