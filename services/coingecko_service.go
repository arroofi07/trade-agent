package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"api-trade/models"
)

const coinGeckoBaseURL = "https://api.coingecko.com/api/v3"

// =============================================
// Coin List Cache — dynamic mapping dari CoinGecko
// =============================================

// cgCoinEntry adalah entry dari /coins/list
type cgCoinEntry struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

// coinCache menyimpan mapping symbol → CoinGecko ID secara global
var coinCache struct {
	sync.RWMutex
	data     map[string]string // key: uppercase symbol, value: coingecko id
	loadedAt time.Time
}

// CoinGeckoService menggantikan BinanceService dengan method yang identik
type CoinGeckoService struct {
	client *http.Client
}

// NewCoinGeckoService membuat instance baru dan langsung load coin list
func NewCoinGeckoService() *CoinGeckoService {
	svc := &CoinGeckoService{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}

	// Load coin list saat startup (async agar server tidak delay)
	go func() {
		if err := svc.loadCoinList(); err != nil {
			log.Printf("⚠️  Gagal load CoinGecko coin list: %v", err)
			log.Println("   Menggunakan fallback mapping (coin populer saja)")
			loadFallbackCoins()
		} else {
			log.Printf("✅ CoinGecko coin list berhasil dimuat (%d coin)", len(coinCache.data))
		}
	}()

	return svc
}

// loadCoinList fetch semua coin dari CoinGecko dan cache ke memory
func (s *CoinGeckoService) loadCoinList() error {
	url := fmt.Sprintf("%s/coins/list", coinGeckoBaseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AI-Trading-Bot/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal request coin list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return fmt.Errorf("rate limit, coba lagi nanti")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CoinGecko coin list error %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var coins []cgCoinEntry
	if err := json.Unmarshal(body, &coins); err != nil {
		return err
	}

	// Build mapping: UPPERCASE_SYMBOL → id
	// Prioritaskan coin dengan volume/rank lebih tinggi untuk symbol yang ambigu
	// Karena /coins/list tidak ada rank, kita pakai priority list untuk coin populer
	priority := map[string]string{
		"BTC":   "bitcoin",
		"ETH":   "ethereum",
		"BNB":   "binancecoin",
		"SOL":   "solana",
		"XRP":   "ripple",
		"ADA":   "cardano",
		"DOGE":  "dogecoin",
		"AVAX":  "avalanche-2",
		"DOT":   "polkadot",
		"MATIC": "matic-network",
		"LINK":  "chainlink",
		"UNI":   "uniswap",
		"LTC":   "litecoin",
		"ATOM":  "cosmos",
		"XLM":   "stellar",
		"NEAR":  "near",
		"TRX":   "tron",
		"FTM":   "fantom",
		"SAND":  "the-sandbox",
		"MANA":  "decentraland",
		"AXS":   "axie-infinity",
		"APT":   "aptos",
		"ARB":   "arbitrum",
		"OP":    "optimism",
		"INJ":   "injective-protocol",
		"SUI":   "sui",
		"SEI":   "sei-network",
		"PEPE":  "pepe",
		"SHIB":  "shiba-inu",
		"FLOKI": "floki",
		"TON":   "the-open-network",
		"WIF":   "dogwifcoin",
		"BONK":  "bonk",
		"JUP":   "jupiter-exchange-solana",
		"PYTH":  "pyth-network",
		"JTO":   "jito-governance-token",
		"RNDR":  "render-token",
		"GRT":   "the-graph",
		"LDO":   "lido-dao",
		"MKR":   "maker",
		"AAVE":  "aave",
		"SNX":   "havven",
		"CRV":   "curve-dao-token",
		"1INCH": "1inch",
		"COMP":  "compound-governance-token",
		"ENS":   "ethereum-name-service",
		"IMX":   "immutable-x",
		"ALGO":  "algorand",
		"VET":   "vechain",
		"ICP":   "internet-computer",
		"HBAR":  "hedera-hashgraph",
		"FIL":   "filecoin",
		"ETC":   "ethereum-classic",
		"EGLD":  "elrond-erd-2",
		"THETA": "theta-token",
		"FLOW":  "flow",
		"XTZ":   "tezos",
		"NEO":   "neo",
		"ZEC":   "zcash",
		"DASH":  "dash",
		"BCH":   "bitcoin-cash",
		"BSV":   "bitcoin-cash-sv",
	}

	newData := make(map[string]string, len(coins))

	// Pertama masukkan semua coin dari API
	for _, c := range coins {
		sym := strings.ToUpper(c.Symbol)
		// Hanya set jika belum ada (ambil yang pertama muncul = umumnya lebih populer)
		if _, exists := newData[sym]; !exists {
			newData[sym] = c.ID
		}
	}

	// Override dengan priority untuk coin populer (pastikan tidak salah mapping)
	for sym, id := range priority {
		newData[sym] = id
	}

	coinCache.Lock()
	coinCache.data = newData
	coinCache.loadedAt = time.Now()
	coinCache.Unlock()

	return nil
}

// loadFallbackCoins isi cache dengan coin populer jika API gagal
func loadFallbackCoins() {
	fallback := map[string]string{
		"BTC": "bitcoin", "ETH": "ethereum", "BNB": "binancecoin",
		"SOL": "solana", "XRP": "ripple", "ADA": "cardano",
		"DOGE": "dogecoin", "AVAX": "avalanche-2", "DOT": "polkadot",
		"MATIC": "matic-network", "LINK": "chainlink", "UNI": "uniswap",
		"LTC": "litecoin", "ATOM": "cosmos", "XLM": "stellar",
		"NEAR": "near", "TRX": "tron", "FTM": "fantom",
		"SAND": "the-sandbox", "MANA": "decentraland", "SHIB": "shiba-inu",
		"APT": "aptos", "ARB": "arbitrum", "OP": "optimism",
		"INJ": "injective-protocol", "SUI": "sui", "PEPE": "pepe",
		"TON": "the-open-network", "WIF": "dogwifcoin", "BONK": "bonk",
	}
	coinCache.Lock()
	coinCache.data = fallback
	coinCache.loadedAt = time.Now()
	coinCache.Unlock()
}

// ResolveCoinID mendapatkan CoinGecko ID dari simbol Binance (e.g. BTCUSDT → "bitcoin")
func ResolveCoinID(symbol string) (string, error) {
	base := extractBase(symbol)

	coinCache.RLock()
	data := coinCache.data
	coinCache.RUnlock()

	if data == nil {
		return "", fmt.Errorf("coin list belum dimuat, tunggu beberapa detik lalu coba lagi")
	}

	id, ok := data[base]
	if !ok {
		return "", fmt.Errorf("coin '%s' tidak ditemukan (dari symbol: %s). Pastikan symbol benar, contoh: BTCUSDT, ETHUSDT", base, symbol)
	}
	return id, nil
}

// extractBase mengekstrak base coin dari Binance symbol (BTCUSDT → BTC)
func extractBase(symbol string) string {
	symbol = strings.ToUpper(symbol)
	for _, quote := range []string{"USDT", "BUSD", "USDC", "TUSD", "DAI", "BTC", "ETH", "BNB"} {
		if strings.HasSuffix(symbol, quote) {
			base := strings.TrimSuffix(symbol, quote)
			if base != "" {
				return base
			}
		}
	}
	return symbol
}

// =============================================
// HTTP helpers
// =============================================

func (s *CoinGeckoService) doRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AI-Trading-Bot/1.0)")
	req.Header.Set("Accept", "application/json")
	return s.client.Do(req)
}

func cgDecodeJSON(resp *http.Response, target interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("gagal baca response: %w", err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		preview := string(body)
		if len(preview) > 150 {
			preview = preview[:150]
		}
		return fmt.Errorf("gagal parse JSON: %w — body: %s", err, preview)
	}
	return nil
}

func checkRateLimit(resp *http.Response) error {
	if resp.StatusCode == 429 {
		return fmt.Errorf("CoinGecko rate limit tercapai (maks ~30 req/menit), tunggu sebentar lalu coba lagi")
	}
	return nil
}

// =============================================
// CoinGecko response structs (internal)
// =============================================

type cgMarketData struct {
	ID                       string  `json:"id"`
	Symbol                   string  `json:"symbol"`
	CurrentPrice             float64 `json:"current_price"`
	PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
	High24h                  float64 `json:"high_24h"`
	Low24h                   float64 `json:"low_24h"`
	TotalVolume              float64 `json:"total_volume"`
}

// =============================================
// GetTicker
// =============================================

func (s *CoinGeckoService) GetTicker(symbol string) (*models.TickerData, error) {
	coinID, err := ResolveCoinID(symbol)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/coins/markets?vs_currency=usd&ids=%s&sparkline=false", coinGeckoBaseURL, coinID)
	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request ke CoinGecko: %w", err)
	}
	defer resp.Body.Close()

	if err := checkRateLimit(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CoinGecko error %d: %s", resp.StatusCode, string(body))
	}

	var markets []cgMarketData
	if err := cgDecodeJSON(resp, &markets); err != nil {
		return nil, err
	}
	if len(markets) == 0 {
		return nil, fmt.Errorf("data tidak ditemukan di CoinGecko untuk %s", symbol)
	}

	m := markets[0]
	ticker := &models.TickerData{
		Symbol:             strings.ToUpper(symbol),
		LastPrice:          strconv.FormatFloat(m.CurrentPrice, 'f', 8, 64),
		PriceChangePercent: strconv.FormatFloat(m.PriceChangePercentage24h, 'f', 4, 64),
		HighPrice:          strconv.FormatFloat(m.High24h, 'f', 8, 64),
		LowPrice:           strconv.FormatFloat(m.Low24h, 'f', 8, 64),
		QuoteVolume:        strconv.FormatFloat(m.TotalVolume, 'f', 2, 64),
		Volume:             strconv.FormatFloat(m.TotalVolume/m.CurrentPrice, 'f', 4, 64),
	}
	return ticker, nil
}

// =============================================
// GetKlines
// =============================================

func (s *CoinGeckoService) GetKlines(symbol, interval string, limit int) ([]models.KlineData, error) {
	coinID, err := ResolveCoinID(symbol)
	if err != nil {
		return nil, err
	}

	days := intervalToDays(interval)
	url := fmt.Sprintf("%s/coins/%s/ohlc?vs_currency=usd&days=%d", coinGeckoBaseURL, coinID, days)
	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request klines ke CoinGecko: %w", err)
	}
	defer resp.Body.Close()

	if err := checkRateLimit(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CoinGecko OHLC error %d: %s", resp.StatusCode, string(body))
	}

	var raw [][]float64
	if err := cgDecodeJSON(resp, &raw); err != nil {
		return nil, err
	}

	klines := make([]models.KlineData, 0, len(raw))
	for _, r := range raw {
		if len(r) < 5 {
			continue
		}
		klines = append(klines, models.KlineData{
			OpenTime:  int64(r[0]),
			Open:      r[1],
			High:      r[2],
			Low:       r[3],
			Close:     r[4],
			Volume:    0,
			CloseTime: int64(r[0]) + intervalToMs(interval),
		})
	}

	if limit > 0 && len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}

	return klines, nil
}

// =============================================
// GetTrending
// =============================================

func (s *CoinGeckoService) GetTrending(limit int) ([]models.TrendingCoin, error) {
	if limit <= 0 {
		limit = 20
	}
	perPage := limit
	if perPage > 100 {
		perPage = 100
	}

	url := fmt.Sprintf("%s/coins/markets?vs_currency=usd&order=volume_desc&per_page=%d&page=1&sparkline=false",
		coinGeckoBaseURL, perPage)
	resp, err := s.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("gagal request trending ke CoinGecko: %w", err)
	}
	defer resp.Body.Close()

	if err := checkRateLimit(resp); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CoinGecko trending error %d: %s", resp.StatusCode, string(body))
	}

	var markets []cgMarketData
	if err := cgDecodeJSON(resp, &markets); err != nil {
		return nil, err
	}

	result := make([]models.TrendingCoin, 0, len(markets))
	for _, m := range markets {
		result = append(result, models.TrendingCoin{
			Symbol:    strings.ToUpper(m.Symbol) + "USDT",
			Price:     m.CurrentPrice,
			Change24h: m.PriceChangePercentage24h,
			Volume:    m.TotalVolume,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		absI, absJ := result[i].Change24h, result[j].Change24h
		if absI < 0 {
			absI = -absI
		}
		if absJ < 0 {
			absJ = -absJ
		}
		return absI > absJ
	})

	return result, nil
}

// =============================================
// Helpers
// =============================================

func intervalToDays(interval string) int {
	switch interval {
	case "1m":
		return 1 // ~1440 candles
	case "5m":
		return 3 // ~864 candles
	case "15m":
		return 7 // ~672 candles
	case "30m":
		return 14 // ~672 candles
	case "1h", "2h":
		return 14 // ~336 candles — cukup untuk semua indikator
	case "4h":
		return 30 // ~180 candles
	case "1d":
		return 90 // ~90 candles
	case "1w":
		return 365 // ~52 candles
	default:
		return 14
	}
}

func intervalToMs(interval string) int64 {
	switch interval {
	case "1m":
		return 60000
	case "5m":
		return 300000
	case "15m":
		return 900000
	case "30m":
		return 1800000
	case "1h":
		return 3600000
	case "4h":
		return 14400000
	case "1d":
		return 86400000
	case "1w":
		return 604800000
	default:
		return 3600000
	}
}
