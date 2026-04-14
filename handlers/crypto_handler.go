package handlers

import (
	"math"
	"strconv"
	"strings"
	"time"

	"api-trade/database"
	"api-trade/models"
	"api-trade/services"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// CryptoHandler menyimpan semua dependencies untuk handler crypto
type CryptoHandler struct {
	market    services.MarketService // bisa Binance atau CoinGecko
	indicator *services.IndicatorService
	gpt       *services.GPTService
	// V2 services
	pattern *services.PatternService
	quant   *services.QuantService
	ai      *services.AIService
	db      *gorm.DB
}

// NewCryptoHandler membuat instance handler baru
func NewCryptoHandler(
	market services.MarketService,
	indicator *services.IndicatorService,
	gpt *services.GPTService,
	pattern *services.PatternService,
	quant *services.QuantService,
	ai *services.AIService,
) *CryptoHandler {
	return &CryptoHandler{
		market:    market,
		indicator: indicator,
		gpt:       gpt,
		pattern:   pattern,
		quant:     quant,
		ai:        ai,
		db:        database.DB,
	}
}

// =============================================
// GET /api/crypto/analyze/:symbol
// =============================================

// Analyze melakukan analisis teknikal lengkap menggunakan AI
func (h *CryptoHandler) Analyze(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	timeframe := c.Query("timeframe", "1h")

	// 1. Ambil harga realtime
	ticker, err := h.market.GetTicker(symbol)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "MARKET_ERROR",
			Message: err.Error(),
		})
	}

	// 2. Ambil klines untuk kalkulasi indikator
	klines, err := h.market.GetKlines(symbol, timeframe, 100)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "KLINES_ERROR",
			Message: err.Error(),
		})
	}

	// 3. Hitung indikator teknikal
	indicators, err := h.indicator.Calculate(klines)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(models.ErrorResponse{
			Error:   "INDICATOR_ERROR",
			Message: err.Error(),
		})
	}

	// Parse harga dari ticker
	price, _ := strconv.ParseFloat(ticker.LastPrice, 64)
	change, _ := strconv.ParseFloat(ticker.PriceChangePercent, 64)
	high, _ := strconv.ParseFloat(ticker.HighPrice, 64)
	low, _ := strconv.ParseFloat(ticker.LowPrice, 64)
	volume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)

	// 4. Kirim ke GPT untuk analisis futures
	gptResult, err := h.gpt.AnalyzeCrypto(symbol, timeframe, price, change, volume, indicators)
	if err != nil {
		// Jika GPT gagal, kembalikan data teknikal tanpa sinyal AI
		return c.Status(fiber.StatusOK).JSON(models.AnalysisResponse{
			Symbol:     symbol,
			Timeframe:  timeframe,
			Price:      price,
			Change24h:  change,
			High24h:    high,
			Low24h:     low,
			Volume:     volume,
			Indicators: *indicators,
			Signal:     "WAIT",
			Confidence: 0,
			Reasoning:  "GPT tidak tersedia: " + err.Error(),
			AnalyzedAt: time.Now(),
		})
	}

	// 5. Simpan history ke Supabase
	history := models.AnalysisHistory{
		Symbol:     symbol,
		Timeframe:  timeframe,
		Price:      price,
		Change24h:  change,
		Volume:     volume,
		Signal:     gptResult.Signal,
		Confidence: gptResult.Confidence,
		Reasoning:  gptResult.Reasoning,
		RSI14:      indicators.RSI14,
		SMA20:      indicators.SMA20,
		SMA50:      indicators.SMA50,
		MACD:       indicators.MACD,
		MACDSignal: indicators.MACDSignal,
		BBUpper:    indicators.BBUpper,
		BBLower:    indicators.BBLower,
	}
	go func() {
		h.db.Create(&history)
	}()

	// 6. Kembalikan response futures lengkap
	return c.JSON(models.AnalysisResponse{
		Symbol:     symbol,
		Timeframe:  timeframe,
		Price:      price,
		Change24h:  change,
		High24h:    high,
		Low24h:     low,
		Volume:     volume,
		Indicators: *indicators,
		Signal:     gptResult.Signal,
		Confidence: gptResult.Confidence,
		Reasoning:  gptResult.Reasoning,
		Entry:      gptResult.Entry,
		StopLoss:   gptResult.StopLoss,
		TP1:        gptResult.TP1,
		TP2:        gptResult.TP2,
		TP3:        gptResult.TP3,
		Leverage:   gptResult.Leverage,
		RiskReward: gptResult.RiskReward,
		AnalyzedAt: time.Now(),
	})
}

// =============================================
// GET /api/crypto/price/:symbol
// =============================================

// GetPrice mengambil harga realtime dari Binance
func (h *CryptoHandler) GetPrice(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))

	ticker, err := h.market.GetTicker(symbol)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "MARKET_ERROR",
			Message: err.Error(),
		})
	}

	price, _ := strconv.ParseFloat(ticker.LastPrice, 64)
	change, _ := strconv.ParseFloat(ticker.PriceChangePercent, 64)
	high, _ := strconv.ParseFloat(ticker.HighPrice, 64)
	low, _ := strconv.ParseFloat(ticker.LowPrice, 64)
	volume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)

	return c.JSON(models.PriceResponse{
		Symbol:    ticker.Symbol,
		Price:     price,
		Change24h: change,
		High24h:   high,
		Low24h:    low,
		Volume:    volume,
	})
}

// =============================================
// GET /api/crypto/klines/:symbol
// =============================================

// GetKlines mengambil data candlestick dari Binance
func (h *CryptoHandler) GetKlines(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	timeframe := c.Query("timeframe", "1h")
	limitStr := c.Query("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	klines, err := h.market.GetKlines(symbol, timeframe, limit)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "KLINES_ERROR",
			Message: err.Error(),
		})
	}

	return c.JSON(models.KlineResponse{
		Symbol:    symbol,
		Timeframe: timeframe,
		Klines:    klines,
	})
}

// =============================================
// GET /api/crypto/indicators/:symbol
// =============================================

// GetIndicators menghitung dan mengembalikan semua indikator teknikal
func (h *CryptoHandler) GetIndicators(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	timeframe := c.Query("timeframe", "1h")

	klines, err := h.market.GetKlines(symbol, timeframe, 100)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "KLINES_ERROR",
			Message: err.Error(),
		})
	}

	indicators, err := h.indicator.Calculate(klines)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(models.ErrorResponse{
			Error:   "INDICATOR_ERROR",
			Message: err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"symbol":     symbol,
		"timeframe":  timeframe,
		"indicators": indicators,
	})
}

// =============================================
// GET /api/crypto/trending
// =============================================

// GetTrending mengambil top 20 crypto movers
func (h *CryptoHandler) GetTrending(c fiber.Ctx) error {
	limitStr := c.Query("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	coins, err := h.market.GetTrending(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   "TRENDING_ERROR",
			Message: err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"count": len(coins),
		"data":  coins,
	})
}

// =============================================
// GET /api/crypto/history/:symbol
// =============================================

// GetHistory mengambil history analisis dari Supabase
func (h *CryptoHandler) GetHistory(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	limitStr := c.Query("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit > 100 {
		limit = 100
	}

	var histories []models.AnalysisHistory
	result := h.db.
		Where("symbol = ?", symbol).
		Order("created_at DESC").
		Limit(limit).
		Find(&histories)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   "DB_ERROR",
			Message: result.Error.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"symbol": symbol,
		"count":  len(histories),
		"data":   histories,
	})
}

// =============================================
// GET /health
// =============================================

// Health mengembalikan status server
func (h *CryptoHandler) Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "AI Crypto Analysis API",
		"version": "2.0.0",
		"time":    time.Now(),
	})
}

// =============================================
// GET /api/crypto/analyze-v2/:symbol
// =============================================

// AnalyzeV2 melakukan analisis lengkap: indicators + patterns + quant scoring + MTF + AI validation
func (h *CryptoHandler) AnalyzeV2(c fiber.Ctx) error {
	symbol := strings.ToUpper(c.Params("symbol"))
	timeframe := c.Query("timeframe", "1h")

	// 1. Ambil harga realtime
	ticker, err := h.market.GetTicker(symbol)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "MARKET_ERROR",
			Message: err.Error(),
		})
	}

	price, _ := strconv.ParseFloat(ticker.LastPrice, 64)
	change, _ := strconv.ParseFloat(ticker.PriceChangePercent, 64)
	high, _ := strconv.ParseFloat(ticker.HighPrice, 64)
	low, _ := strconv.ParseFloat(ticker.LowPrice, 64)
	volume, _ := strconv.ParseFloat(ticker.QuoteVolume, 64)

	// 2. Ambil klines untuk timeframe utama (100 candle)
	klines, err := h.market.GetKlines(symbol, timeframe, 100)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "KLINES_ERROR",
			Message: err.Error(),
		})
	}

	// 3. Hitung enhanced indicators
	indicators, err := h.indicator.Calculate(klines)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(models.ErrorResponse{
			Error:   "INDICATOR_ERROR",
			Message: err.Error(),
		})
	}

	// 4. Pattern recognition
	patterns := h.pattern.Analyze(klines)

	// 5. Quantitative scoring
	quantScore := h.quant.Score(indicators, patterns, price)

	// 6. Multi-timeframe analysis (15m & 4h concurrent)
	mtf := h.calcMTF(symbol, timeframe, *indicators, *quantScore)

	// 7. AI validation (Claude primary, GPT fallback)
	aiResult, err := h.ai.Analyze(symbol, timeframe, price, change, volume, indicators, patterns, quantScore)
	if err != nil {
		// AI tidak tersedia: gunakan quant signal sebagai fallback
		aiResult = &models.AIAnalysisResult{
			Model:      "unavailable",
			Signal:     quantScore.Signal,
			Confidence: quantScore.Total,
			Reasoning:  "AI tidak tersedia, menggunakan sinyal kuantitatif. Error: " + err.Error(),
			Entry:      price,
			StopLoss:   calcDefaultSL(price, quantScore.Signal),
			TP1:        calcDefaultTP(price, quantScore.Signal, 1.5),
			TP2:        calcDefaultTP(price, quantScore.Signal, 2.5),
			TP3:        calcDefaultTP(price, quantScore.Signal, 4.0),
			Leverage:   calcDefaultLeverage(quantScore.Total),
			RiskReward: 2.5,
		}
	}

	// 8. Hitung sinyal final (quant 60% + AI 40%)
	finalSignal, finalConfidence := computeFinalSignal(quantScore, aiResult, mtf)

	// 8.1 Penalti confidence jika kualitas data volume lemah (umum di provider tanpa volume per-candle)
	finalConfidence = applyDataQualityPenalty(finalConfidence, indicators, price)

	// 8.2 Hard risk gate untuk mencegah entry di kondisi market yang buruk
	if ok, reason := passesRiskGate(finalSignal, aiResult, indicators, price, volume); !ok {
		finalSignal = "WAIT"
		if finalConfidence > 45 {
			finalConfidence = 45
		}
		if reason != "" {
			aiResult.Reasoning = strings.TrimSpace(aiResult.Reasoning + " | RiskGate: " + reason)
		}
	}

	// 9. Simpan ke database secara async
	go h.saveHistoryV2(symbol, timeframe, price, change, volume, indicators, patterns, quantScore, aiResult, finalSignal, finalConfidence)

	// 10. Return response lengkap
	return c.JSON(models.AnalysisResponseV2{
		Symbol:          symbol,
		Timeframe:       timeframe,
		Price:           price,
		Change24h:       change,
		High24h:         high,
		Low24h:          low,
		Volume:          volume,
		Indicators:      *indicators,
		Patterns:        *patterns,
		QuantScore:      *quantScore,
		MTF:             mtf,
		AIAnalysis:      *aiResult,
		FinalSignal:     finalSignal,
		FinalConfidence: finalConfidence,
		AnalyzedAt:      time.Now(),
	})
}

// calcMTF menjalankan analisis multi-timeframe (15m & 4h) secara concurrent
func (h *CryptoHandler) calcMTF(symbol, primaryTF string, primaryIndicators models.IndicatorResult, primaryQuant models.QuantScore) models.MTFAnalysis {
	type tfResult struct {
		tf    string
		ind   *models.IndicatorResult
		quant *models.QuantScore
		err   error
	}

	// Fetch timeframe selain primary secara concurrent
	otherTFs := []string{}
	for _, tf := range []string{"15m", "4h"} {
		if tf != primaryTF {
			otherTFs = append(otherTFs, tf)
		}
	}

	ch := make(chan tfResult, len(otherTFs))
	for _, tf := range otherTFs {
		go func(t string) {
			kl, err := h.market.GetKlines(symbol, t, 100)
			if err != nil {
				ch <- tfResult{tf: t, err: err}
				return
			}
			ind, err := h.indicator.Calculate(kl)
			if err != nil {
				ch <- tfResult{tf: t, err: err}
				return
			}
			pat := h.pattern.Analyze(kl)
			// Gunakan close candle terakhir agar scoring MTF merepresentasikan harga aktual timeframe terkait.
			estimatedPrice := 0.0
			if len(kl) > 0 {
				estimatedPrice = kl[len(kl)-1].Close
			}
			if estimatedPrice <= 0 {
				estimatedPrice = primaryIndicators.SMA20
			}
			if estimatedPrice <= 0 {
				estimatedPrice = ind.SMA20
			}
			q := h.quant.Score(ind, pat, estimatedPrice)
			ch <- tfResult{tf: t, ind: ind, quant: q}
		}(tf)
	}

	results := map[string]tfResult{}
	for i := 0; i < len(otherTFs); i++ {
		r := <-ch
		results[r.tf] = r
	}

	mtf := models.MTFAnalysis{}

	// Set primary timeframe data
	primaryTA := models.TimeframeAnalysis{
		Timeframe:  primaryTF,
		Signal:     primaryQuant.Signal,
		QuantScore: primaryQuant.Total,
		Indicators: primaryIndicators,
	}

	// Tentukan slot berdasarkan primary timeframe
	switch primaryTF {
	case "15m":
		mtf.TF15m = primaryTA
	case "4h":
		mtf.TF4h = primaryTA
	default: // 1h (default)
		mtf.TF1h = primaryTA
	}

	// Fill hasil concurrent fetch
	for tf, r := range results {
		var ta models.TimeframeAnalysis
		if r.err != nil || r.ind == nil {
			ta = models.TimeframeAnalysis{
				Timeframe:  tf,
				Signal:     "WAIT",
				QuantScore: 50,
			}
		} else {
			ta = models.TimeframeAnalysis{
				Timeframe:  tf,
				Signal:     r.quant.Signal,
				QuantScore: r.quant.Total,
				Indicators: *r.ind,
			}
		}
		switch tf {
		case "15m":
			mtf.TF15m = ta
		case "4h":
			mtf.TF4h = ta
		}
	}

	// Jika primary adalah 1h dan TF15m/4h belum diisi (karena primary bukan keduanya)
	// Fill default jika masih kosong
	if mtf.TF15m.Timeframe == "" {
		mtf.TF15m = models.TimeframeAnalysis{Timeframe: "15m", Signal: "WAIT", QuantScore: 50}
	}
	if mtf.TF1h.Timeframe == "" {
		mtf.TF1h = models.TimeframeAnalysis{Timeframe: "1h", Signal: "WAIT", QuantScore: 50}
	}
	if mtf.TF4h.Timeframe == "" {
		mtf.TF4h = models.TimeframeAnalysis{Timeframe: "4h", Signal: "WAIT", QuantScore: 50}
	}

	// Hitung confluence
	signals := []string{mtf.TF15m.Signal, mtf.TF1h.Signal, mtf.TF4h.Signal}
	longCount, shortCount := 0, 0
	for _, sig := range signals {
		if sig == "LONG" {
			longCount++
		} else if sig == "SHORT" {
			shortCount++
		}
	}

	mtf.ConfluenceCount = longCount
	if shortCount > longCount {
		mtf.ConfluenceCount = shortCount
	}

	switch {
	case longCount == 3:
		mtf.Confluence = "STRONG_LONG"
	case longCount == 2:
		mtf.Confluence = "LONG"
	case shortCount == 3:
		mtf.Confluence = "STRONG_SHORT"
	case shortCount == 2:
		mtf.Confluence = "SHORT"
	default:
		mtf.Confluence = "NEUTRAL"
	}

	return mtf
}

// computeFinalSignal menggabungkan sinyal kuantitatif dan AI (quant 60% + AI 40%)
func computeFinalSignal(quant *models.QuantScore, ai *models.AIAnalysisResult, mtf models.MTFAnalysis) (string, int) {
	finalConfidence := int(float64(quant.Total)*0.6 + float64(ai.Confidence)*0.4)

	// Tentukan sinyal final
	var finalSignal string
	if ai.Signal == quant.Signal {
		// AI dan quant sepakat
		finalSignal = quant.Signal
	} else if ai.Confidence < 60 || ai.Model == "unavailable" {
		// AI tidak cukup yakin untuk override
		finalSignal = quant.Signal
	} else {
		// Kontradiksi sejati antara quant dan AI
		finalSignal = "WAIT"
		finalConfidence = finalConfidence / 2
	}

	// Downgrade confidence jika MTF confluence lemah
	if mtf.ConfluenceCount <= 1 && finalConfidence > 60 {
		finalConfidence = 60
	}

	if finalConfidence < 0 {
		finalConfidence = 0
	}
	if finalConfidence > 100 {
		finalConfidence = 100
	}

	return finalSignal, finalConfidence
}

func applyDataQualityPenalty(confidence int, ind *models.IndicatorResult, price float64) int {
	if ind == nil {
		return confidence
	}
	// OBV=0 dan VWAP==price sering menandakan provider tidak punya volume per-candle.
	if ind.OBV == 0 && ind.VWAP == price {
		confidence -= 15
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}

func passesRiskGate(signal string, ai *models.AIAnalysisResult, ind *models.IndicatorResult, price, volume float64) (bool, string) {
	if signal == "WAIT" {
		return true, ""
	}
	if volume > 0 && volume < 250000 {
		return false, "volume rendah (<$250k)"
	}
	if ind != nil && price > 0 && ind.ATR > 0 {
		atrPct := (ind.ATR / price) * 100
		if atrPct > 4.5 {
			return false, "volatilitas terlalu tinggi (ATR>4.5%)"
		}
		if atrPct < 0.1 {
			return false, "volatilitas terlalu rendah (ATR<0.1%)"
		}
	}
	rr := extractRiskReward(ai, signal)
	if rr > 0 && rr < 1.3 {
		return false, "risk-reward < 1.3"
	}
	return true, ""
}

func extractRiskReward(ai *models.AIAnalysisResult, signal string) float64 {
	if ai == nil {
		return 0
	}
	if ai.RiskReward > 0 {
		return ai.RiskReward
	}
	if ai.Entry <= 0 || ai.StopLoss <= 0 || ai.TP1 <= 0 {
		return 0
	}
	risk := math.Abs(ai.Entry - ai.StopLoss)
	if risk == 0 {
		return 0
	}
	reward := math.Abs(ai.TP1 - ai.Entry)
	if reward == 0 {
		return 0
	}
	if signal == "SHORT" && ai.TP1 > ai.Entry {
		return 0
	}
	if signal == "LONG" && ai.TP1 < ai.Entry {
		return 0
	}
	return reward / risk
}

// Helper: default stop loss jika AI tidak tersedia
func calcDefaultSL(price float64, signal string) float64 {
	if signal == "LONG" {
		return roundTo4(price * 0.98) // 2% below
	}
	return roundTo4(price * 1.02) // 2% above
}

// Helper: default take profit dengan ratio tertentu
func calcDefaultTP(price float64, signal string, rrRatio float64) float64 {
	risk := price * 0.02 // 2% risk
	if signal == "LONG" {
		return roundTo4(price + risk*rrRatio)
	}
	return roundTo4(price - risk*rrRatio)
}

// Helper: default leverage berdasarkan confidence
func calcDefaultLeverage(confidence int) int {
	switch {
	case confidence >= 80:
		return 10
	case confidence >= 65:
		return 7
	default:
		return 3
	}
}

func roundTo4(v float64) float64 {
	var m float64 = 10000
	return float64(int(v*m+0.5)) / m
}

// saveHistoryV2 menyimpan record analisis v2 ke Supabase secara async
func (h *CryptoHandler) saveHistoryV2(
	symbol, timeframe string,
	price, change, volume float64,
	ind *models.IndicatorResult,
	pat *models.PatternResult,
	quant *models.QuantScore,
	ai *models.AIAnalysisResult,
	finalSignal string,
	finalConfidence int,
) {
	record := models.AnalysisHistoryV2{
		Symbol:      symbol,
		Timeframe:   timeframe,
		Price:       price,
		Change24h:   change,
		Volume:      volume,
		Signal:      finalSignal,
		Confidence:  finalConfidence,
		QuantTotal:  quant.Total,
		QuantSignal: quant.Signal,
		AIModel:     ai.Model,
		Reasoning:   ai.Reasoning,
		RSI14:       ind.RSI14,
		EMA9:        ind.EMA9,
		EMA21:       ind.EMA21,
		ATR:         ind.ATR,
		OBV:         ind.OBV,
		PatternBias: pat.PatternBias,
	}
	h.db.Create(&record)
}
