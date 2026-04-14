package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"api-trade/models"
)

const (
	indodaxBaseURL           = "https://indodax.com"
	indodaxHistoryPath       = "/tradingview/history_v2"
	indodaxSummariesTTL      = 12 * time.Second
	indodaxDefaultTimeoutSec = 45
	// Harga kuotasi USDT/IDR di summaries (1 USDT = N Rupiah).
	indodaxUSDTIDRTickerID = "usdt_idr"
)

// IndodaxService mengambil data market dari Indodax; harga & notional volume di response disetara USDT
// memakai kurs pasangan usdt_idr (override opsional: env INDODAX_USDT_IDR).
// Dokumentasi: https://github.com/btcid/indodax-official-api-docs/blob/master/Public-RestAPI.md
type IndodaxService struct {
	client *http.Client

	sumMu     sync.Mutex
	sumBody   []byte
	sumExpiry time.Time
}

// NewIndodaxService membuat instance baru IndodaxService.
func NewIndodaxService() *IndodaxService {
	timeout := indodaxHTTPTimeout()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if timeout > 10*time.Second {
		transport.ResponseHeaderTimeout = timeout - 2*time.Second
	}
	return &IndodaxService{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

func indodaxHTTPTimeout() time.Duration {
	v := strings.TrimSpace(os.Getenv("INDODAX_HTTP_TIMEOUT_SEC"))
	if v == "" {
		return indodaxDefaultTimeoutSec * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 5 {
		return indodaxDefaultTimeoutSec * time.Second
	}
	if n > 120 {
		n = 120
	}
	return time.Duration(n) * time.Second
}

func isRetryableIndodaxNetworkErr(err error) bool {
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
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "EOF")
}

// --- responses ---

type indodaxSummaries struct {
	Tickers   map[string]json.RawMessage `json:"tickers"`
	Prices24h map[string]interface{}     `json:"prices_24h"`
}

type indodaxHistoryCandle struct {
	Time   int64   `json:"Time"`
	Open   float64 `json:"Open"`
	High   float64 `json:"High"`
	Low    float64 `json:"Low"`
	Close  float64 `json:"Close"`
	Volume string  `json:"Volume"`
}

// indodaxStringFromJSONValue mengonversi nilai JSON Indodax (string atau number) ke string numerik.
func indodaxStringFromJSONValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "1"
		}
		return "0"
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func indodaxTickerFieldString(fields map[string]interface{}, key string) string {
	return indodaxStringFromJSONValue(fields[key])
}

// indodaxDecodeTickerObject mem-parse satu objek ticker; angka bisa string atau number.
func indodaxDecodeTickerObject(raw json.RawMessage) (map[string]interface{}, error) {
	var fields map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func unmarshalIndodaxSummaries(body []byte) (indodaxSummaries, error) {
	var sum indodaxSummaries
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&sum); err != nil {
		return indodaxSummaries{}, err
	}
	return sum, nil
}

// indodaxIDRPerUSDT mengembalikan harga 1 USDT dalam IDR (angka besar, mis. 16500).
// Prioritas: env INDODAX_USDT_IDR, lalu ticker usdt_idr dari summaries.
func indodaxIDRPerUSDT(sum *indodaxSummaries) (float64, error) {
	if v := strings.TrimSpace(os.Getenv("INDODAX_USDT_IDR")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil && f > 0 {
			return f, nil
		}
	}
	raw, ok := sum.Tickers[indodaxUSDTIDRTickerID]
	if !ok || len(raw) == 0 {
		return 0, fmt.Errorf("kurs USDT/IDR tidak tersedia (pasangan %s)", indodaxUSDTIDRTickerID)
	}
	fields, err := indodaxDecodeTickerObject(raw)
	if err != nil {
		return 0, fmt.Errorf("parse ticker USDT/IDR: %w", err)
	}
	lastS := indodaxTickerFieldString(fields, "last")
	rate, err := strconv.ParseFloat(lastS, 64)
	if err != nil || rate <= 0 {
		return 0, fmt.Errorf("harga USDT/IDR tidak valid")
	}
	return rate, nil
}

func formatIndodaxUSDTString(f float64) string {
	s := strconv.FormatFloat(f, 'f', 10, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

// GetTicker mengambil ticker 24h; harga IDR dikonversi ke setara USDT (basis kurs usdt_idr).
func (s *IndodaxService) GetTicker(symbol string) (*models.TickerData, error) {
	pairID, chartSym, err := normalizeIndodaxSymbol(symbol)
	if err != nil {
		return nil, err
	}

	sum, err := s.fetchSummaries()
	if err != nil {
		return nil, err
	}

	raw, ok := sum.Tickers[pairID]
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("pair %s tidak ditemukan di Indodax", pairID)
	}

	fields, err := indodaxDecodeTickerObject(raw)
	if err != nil {
		return nil, fmt.Errorf("parse ticker indodax: %w", err)
	}

	last := indodaxTickerFieldString(fields, "last")
	high := indodaxTickerFieldString(fields, "high")
	low := indodaxTickerFieldString(fields, "low")
	volIDR := indodaxTickerFieldString(fields, "vol_idr")
	baseVol := indodaxBaseVolumeFromTicker(fields)

	lastF, err := strconv.ParseFloat(last, 64)
	if err != nil {
		return nil, fmt.Errorf("harga last tidak valid untuk %s", pairID)
	}

	key24 := strings.ReplaceAll(pairID, "_", "")
	var prevS string
	if sum.Prices24h != nil {
		prevS = indodaxStringFromJSONValue(sum.Prices24h[key24])
	}
	var changePct float64
	var changeAbs float64
	if prevS != "" {
		prevF, err := strconv.ParseFloat(prevS, 64)
		if err == nil && prevF != 0 {
			changeAbs = lastF - prevF
			changePct = (changeAbs / prevF) * 100
		}
	}

	rate, err := indodaxIDRPerUSDT(sum)
	if err != nil {
		return nil, err
	}

	highF, _ := strconv.ParseFloat(high, 64)
	lowF, _ := strconv.ParseFloat(low, 64)
	volIDRF, _ := strconv.ParseFloat(volIDR, 64)
	changeAbsUSDT := changeAbs / rate

	return &models.TickerData{
		Symbol:             chartSym,
		PriceChange:        formatIndodaxUSDTString(changeAbsUSDT),
		PriceChangePercent: fmt.Sprintf("%.4f", changePct),
		LastPrice:          formatIndodaxUSDTString(lastF / rate),
		HighPrice:          formatIndodaxUSDTString(highF / rate),
		LowPrice:           formatIndodaxUSDTString(lowF / rate),
		Volume:             baseVol,
		QuoteVolume:        formatIndodaxUSDTString(volIDRF / rate),
	}, nil
}

func indodaxBaseVolumeFromTicker(fields map[string]interface{}) string {
	for k, v := range fields {
		if k == "vol_idr" || !strings.HasPrefix(k, "vol_") {
			continue
		}
		s := indodaxStringFromJSONValue(v)
		if s != "" {
			return s
		}
	}
	return "0"
}

// GetKlines mengambil OHLCV dari endpoint TradingView history_v2.
func (s *IndodaxService) GetKlines(symbol, timeframe string, limit int) ([]models.KlineData, error) {
	_, chartSym, err := normalizeIndodaxSymbol(symbol)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	tfParam, tfSec := indodaxTF(timeframe)
	now := time.Now().Unix()
	// buffer supaya cukup candle meski pasar sepi / gap
	from := now - int64(limit+20)*tfSec
	if from < 0 {
		from = 0
	}

	q := url.Values{}
	q.Set("from", strconv.FormatInt(from, 10))
	q.Set("to", strconv.FormatInt(now, 10))
	q.Set("symbol", chartSym)
	q.Set("tf", tfParam)

	u := indodaxBaseURL + indodaxHistoryPath + "?" + q.Encode()
	body, err := s.get(u)
	if err != nil {
		return nil, err
	}

	var candles []indodaxHistoryCandle
	if err := json.Unmarshal(body, &candles); err != nil {
		return nil, fmt.Errorf("parse indodax history: %w", err)
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("tidak ada data kline untuk %s %s", chartSym, timeframe)
	}

	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Time < candles[j].Time
	})

	start := 0
	if len(candles) > limit {
		start = len(candles) - limit
	}
	candles = candles[start:]

	durMs := tfSec * 1000
	out := make([]models.KlineData, 0, len(candles))
	for _, c := range candles {
		vol, _ := strconv.ParseFloat(strings.TrimSpace(c.Volume), 64)
		openMs := c.Time * 1000
		out = append(out, models.KlineData{
			OpenTime:  openMs,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    vol,
			CloseTime: openMs + durMs,
		})
	}

	sum, err := s.fetchSummaries()
	if err != nil {
		return nil, err
	}
	rate, err := indodaxIDRPerUSDT(sum)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Open /= rate
		out[i].High /= rate
		out[i].Low /= rate
		out[i].Close /= rate
	}

	return out, nil
}

// GetTrending mengurutkan pair IDR berdasarkan vol_idr; harga & volume notional disetara USDT.
func (s *IndodaxService) GetTrending(limit int) ([]models.TrendingCoin, error) {
	sum, err := s.fetchSummaries()
	if err != nil {
		return nil, err
	}

	type row struct {
		pairID   string
		chartSym string
		volIDR   float64
		last     float64
		change   float64
	}
	var rows []row

	for pairID, raw := range sum.Tickers {
		if !strings.HasSuffix(pairID, "_idr") {
			continue
		}
		fields, err := indodaxDecodeTickerObject(raw)
		if err != nil {
			continue
		}
		lastS := indodaxTickerFieldString(fields, "last")
		volS := indodaxTickerFieldString(fields, "vol_idr")
		lastF, err1 := strconv.ParseFloat(lastS, 64)
		volF, err2 := strconv.ParseFloat(volS, 64)
		if err1 != nil || err2 != nil || volF <= 0 {
			continue
		}
		chartSym := indodaxPairIDToChartSymbol(pairID)
		key24 := strings.ReplaceAll(pairID, "_", "")
		var prevS string
		if sum.Prices24h != nil {
			prevS = indodaxStringFromJSONValue(sum.Prices24h[key24])
		}
		var ch float64
		if prevS != "" {
			if prevF, err := strconv.ParseFloat(prevS, 64); err == nil && prevF != 0 {
				ch = ((lastF - prevF) / prevF) * 100
			}
		}
		rows = append(rows, row{
			pairID:   pairID,
			chartSym: chartSym,
			volIDR:   volF,
			last:     lastF,
			change:   ch,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].volIDR > rows[j].volIDR
	})

	if limit <= 0 {
		limit = 20
	}
	if limit > len(rows) {
		limit = len(rows)
	}

	rate, err := indodaxIDRPerUSDT(sum)
	if err != nil {
		return nil, err
	}

	out := make([]models.TrendingCoin, 0, limit)
	for i := 0; i < limit; i++ {
		r := rows[i]
		out = append(out, models.TrendingCoin{
			Symbol:    r.chartSym,
			Price:     r.last / rate,
			Change24h: r.change,
			Volume:    r.volIDR / rate,
		})
	}
	return out, nil
}

func (s *IndodaxService) fetchSummaries() (*indodaxSummaries, error) {
	s.sumMu.Lock()
	defer s.sumMu.Unlock()
	if len(s.sumBody) > 0 && time.Now().Before(s.sumExpiry) {
		sum, err := unmarshalIndodaxSummaries(s.sumBody)
		if err == nil {
			return &sum, nil
		}
	}

	body, err := s.get(indodaxBaseURL + "/api/summaries")
	if err != nil {
		return nil, err
	}
	sum, err := unmarshalIndodaxSummaries(body)
	if err != nil {
		return nil, fmt.Errorf("parse indodax summaries: %w", err)
	}
	s.sumBody = append([]byte(nil), body...)
	s.sumExpiry = time.Now().Add(indodaxSummariesTTL)
	return &sum, nil
}

func (s *IndodaxService) get(urlStr string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
		}
		body, err := s.getOnce(urlStr)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !isRetryableIndodaxNetworkErr(err) || attempt == maxAttempts {
			return nil, fmt.Errorf("indodax request gagal: %w", err)
		}
	}
	return nil, fmt.Errorf("indodax request gagal: %w", lastErr)
}

func (s *IndodaxService) getOnce(urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; api-trade/1.0)")

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
		return nil, fmt.Errorf("indodax HTTP %d: %s", resp.StatusCode, string(body[:preview]))
	}
	return body, nil
}

// normalizeIndodaxSymbol mengonversi input pengguna ke pair_id (btc_idr) dan simbol chart (BTCIDR).
func normalizeIndodaxSymbol(symbol string) (pairID string, chartSymbol string, err error) {
	s := strings.TrimSpace(symbol)
	if s == "" {
		return "", "", fmt.Errorf("symbol kosong")
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "_idr") {
		pairID = low
		chartSymbol = indodaxPairIDToChartSymbol(pairID)
		return pairID, chartSymbol, nil
	}

	u := strings.ToUpper(s)
	u = strings.ReplaceAll(u, "/", "")
	u = strings.ReplaceAll(u, "-", "")

	if strings.HasSuffix(u, "USDT") {
		u = strings.TrimSuffix(u, "USDT")
	}
	if strings.HasSuffix(u, "IDR") && len(u) > 3 {
		base := u[:len(u)-3]
		pairID = strings.ToLower(base) + "_idr"
		return pairID, u, nil
	}

	pairID = strings.ToLower(u) + "_idr"
	chartSymbol = strings.ToUpper(u) + "IDR"
	return pairID, chartSymbol, nil
}

func indodaxPairIDToChartSymbol(pairID string) string {
	base := strings.TrimSuffix(strings.ToLower(pairID), "_idr")
	if base == "" {
		return strings.ToUpper(strings.ReplaceAll(pairID, "_", ""))
	}
	return strings.ToUpper(base) + "IDR"
}

// indodaxTF mengembalikan query tf dan durasi candle dalam detik.
func indodaxTF(tf string) (param string, seconds int64) {
	switch strings.ToLower(tf) {
	case "1m":
		return "1", 60
	case "3m", "5m":
		return "15", 15 * 60
	case "15m":
		return "15", 15 * 60
	case "30m":
		return "30", 30 * 60
	case "1h", "60m", "2h", "6h", "12h":
		return "60", 60 * 60
	case "4h":
		return "240", 240 * 60
	case "1d":
		return "1D", 86400
	case "3d":
		return "3D", 3 * 86400
	case "1w":
		return "1W", 7 * 86400
	default:
		return "60", 60 * 60
	}
}
