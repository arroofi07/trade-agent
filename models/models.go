package models

import (
	"time"
)

// =============================================
// BINANCE DATA MODELS
// =============================================

// KlineData merepresentasikan satu candlestick dari Binance
type KlineData struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// TickerData merepresentasikan data 24hr ticker dari Binance
type TickerData struct {
	Symbol             string  `json:"symbol"`
	PriceChange        string  `json:"priceChange"`
	PriceChangePercent string  `json:"priceChangePercent"`
	LastPrice          string  `json:"lastPrice"`
	HighPrice          string  `json:"highPrice"`
	LowPrice           string  `json:"lowPrice"`
	Volume             string  `json:"volume"`
	QuoteVolume        string  `json:"quoteVolume"`
}

// =============================================
// INDICATOR MODELS
// =============================================

// IndicatorResult menyimpan hasil kalkulasi semua indikator teknikal
type IndicatorResult struct {
	// Existing indicators
	SMA20      float64 `json:"sma20"`
	SMA50      float64 `json:"sma50"`
	RSI14      float64 `json:"rsi14"`
	MACD       float64 `json:"macd"`
	MACDSignal float64 `json:"macd_signal"`
	MACDHist   float64 `json:"macd_histogram"`
	BBUpper    float64 `json:"bb_upper"`
	BBMiddle   float64 `json:"bb_middle"`
	BBLower    float64 `json:"bb_lower"`
	// Enhanced indicators
	EMA9      float64 `json:"ema9"`
	EMA21     float64 `json:"ema21"`
	StochRSIK float64 `json:"stoch_rsi_k"` // Stochastic RSI %K
	StochRSID float64 `json:"stoch_rsi_d"` // Stochastic RSI %D (signal)
	WilliamsR float64 `json:"williams_r"`  // Williams %R (-100..0)
	ATR       float64 `json:"atr"`         // Average True Range
	VWAP      float64 `json:"vwap"`        // Volume Weighted Average Price
	OBV       float64 `json:"obv"`         // On-Balance Volume
}

// =============================================
// RESPONSE MODELS
// =============================================

// AnalysisResponse adalah response utama endpoint /analyze
type AnalysisResponse struct {
	Symbol     string          `json:"symbol"`
	Timeframe  string          `json:"timeframe"`
	Price      float64         `json:"price"`
	Change24h  float64         `json:"change_24h_percent"`
	High24h    float64         `json:"high_24h"`
	Low24h     float64         `json:"low_24h"`
	Volume     float64         `json:"volume"`
	Indicators IndicatorResult `json:"indicators"`
	// Futures Signal
	Signal      string  `json:"signal"`      // LONG | SHORT | WAIT
	Confidence  int     `json:"confidence"`  // 0-100
	Reasoning   string  `json:"reasoning"`
	Entry       float64 `json:"entry"`       // Rekomendasi harga entry
	StopLoss    float64 `json:"stop_loss"`   // Stop loss
	TP1         float64 `json:"tp1"`         // Take profit 1 (konservatif)
	TP2         float64 `json:"tp2"`         // Take profit 2 (moderat)
	TP3         float64 `json:"tp3"`         // Take profit 3 (agresif)
	Leverage    int     `json:"leverage"`    // Leverage rekomendasi (max 20x)
	RiskReward  float64 `json:"risk_reward"` // Rasio risk/reward
	AnalyzedAt time.Time       `json:"analyzed_at"`
}

// PriceResponse adalah response untuk endpoint /price
type PriceResponse struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change_24h_percent"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume    float64 `json:"volume"`
}

// TrendingCoin adalah data untuk endpoint /trending
type TrendingCoin struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change_24h_percent"`
	Volume    float64 `json:"volume_usd"`
}

// KlineResponse adalah response untuk endpoint /klines
type KlineResponse struct {
	Symbol    string      `json:"symbol"`
	Timeframe string      `json:"timeframe"`
	Klines    []KlineData `json:"klines"`
}

// =============================================
// DATABASE MODEL (GORM)
// =============================================

// AnalysisHistory menyimpan history analisis ke Supabase
type AnalysisHistory struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Symbol     string    `json:"symbol" gorm:"not null;index;size:20"`
	Timeframe  string    `json:"timeframe" gorm:"not null;size:10"`
	Price      float64   `json:"price" gorm:"not null"`
	Change24h  float64   `json:"change_24h"`
	Volume     float64   `json:"volume"`
	Signal     string    `json:"signal" gorm:"not null;size:10"` // BUY, SELL, HOLD
	Confidence int       `json:"confidence"`
	Reasoning  string    `json:"reasoning" gorm:"type:text"`
	// Indikator
	RSI14      float64   `json:"rsi14"`
	SMA20      float64   `json:"sma20"`
	SMA50      float64   `json:"sma50"`
	MACD       float64   `json:"macd"`
	MACDSignal float64   `json:"macd_signal"`
	BBUpper    float64   `json:"bb_upper"`
	BBLower    float64   `json:"bb_lower"`
	// Timestamps
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName override nama tabel Supabase
func (AnalysisHistory) TableName() string {
	return "analysis_history"
}

// =============================================
// V2 MODELS - Pattern Recognition
// =============================================

// PatternResult menyimpan hasil analisis candlestick pattern dan level harga
type PatternResult struct {
	CandlePatterns   []string  `json:"candle_patterns"`   // e.g. ["BullishEngulfing", "Hammer"]
	PatternBias      string    `json:"pattern_bias"`      // BULLISH | BEARISH | NEUTRAL
	SupportLevels    []float64 `json:"support_levels"`    // level support terdekat
	ResistanceLevels []float64 `json:"resistance_levels"` // level resistance terdekat
	// Pivot Points (Classic Floor Trader)
	PivotPoint float64 `json:"pivot"`
	R1         float64 `json:"r1"`
	R2         float64 `json:"r2"`
	S1         float64 `json:"s1"`
	S2         float64 `json:"s2"`
	// Fibonacci Retracement
	Fib236 float64 `json:"fibonacci_236"`
	Fib382 float64 `json:"fibonacci_382"`
	Fib500 float64 `json:"fibonacci_500"`
	Fib618 float64 `json:"fibonacci_618"`
	Fib786 float64 `json:"fibonacci_786"`
}

// =============================================
// V2 MODELS - Quantitative Scoring
// =============================================

// QuantScore menyimpan skor kuantitatif matematis 0-100
type QuantScore struct {
	Total           int    `json:"total"`            // 0-100
	TrendScore      int    `json:"trend_score"`      // 0-30
	MomentumScore   int    `json:"momentum_score"`   // 0-25
	VolumeScore     int    `json:"volume_score"`     // 0-20
	VolatilityScore int    `json:"volatility_score"` // 0-15
	PatternScore    int    `json:"pattern_score"`    // 0-10
	Signal          string `json:"signal"`           // LONG | SHORT | WAIT
	Grade           string `json:"grade"`            // A+ | A | B+ | B | C | D
}

// =============================================
// V2 MODELS - Multi-Timeframe Analysis
// =============================================

// TimeframeAnalysis menyimpan hasil analisis satu timeframe
type TimeframeAnalysis struct {
	Timeframe  string          `json:"timeframe"`
	Signal     string          `json:"signal"`
	QuantScore int             `json:"quant_score"`
	Indicators IndicatorResult `json:"indicators"`
}

// MTFAnalysis menyimpan konfluensi analisis multi-timeframe
type MTFAnalysis struct {
	TF15m           TimeframeAnalysis `json:"timeframe_15m"`
	TF1h            TimeframeAnalysis `json:"timeframe_1h"`
	TF4h            TimeframeAnalysis `json:"timeframe_4h"`
	Confluence      string            `json:"confluence"`       // STRONG_LONG | LONG | NEUTRAL | SHORT | STRONG_SHORT
	ConfluenceCount int               `json:"confluence_count"` // 0-3 timeframes yang setuju
}

// =============================================
// V2 MODELS - AI Analysis
// =============================================

// AIAnalysisResult adalah hasil analisis AI (Claude atau GPT-4o)
type AIAnalysisResult struct {
	Model      string  `json:"model"`       // "claude-sonnet-4-6" atau "gpt-4o"
	Signal     string  `json:"signal"`      // LONG | SHORT | WAIT
	Confidence int     `json:"confidence"`  // 0-100
	Reasoning  string  `json:"reasoning"`
	Entry      float64 `json:"entry"`
	StopLoss   float64 `json:"stop_loss"`
	TP1        float64 `json:"tp1"`
	TP2        float64 `json:"tp2"`
	TP3        float64 `json:"tp3"`
	Leverage   int     `json:"leverage"`
	RiskReward float64 `json:"risk_reward"`
}

// =============================================
// V2 MODELS - Full Response
// =============================================

// AnalysisResponseV2 adalah response endpoint /api/crypto/analyze-v2/:symbol
type AnalysisResponseV2 struct {
	Symbol    string  `json:"symbol"`
	Timeframe string  `json:"timeframe"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change_24h_percent"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume    float64 `json:"volume"`
	// Analysis layers
	Indicators      IndicatorResult  `json:"indicators"`
	Patterns        PatternResult    `json:"patterns"`
	QuantScore      QuantScore       `json:"quant_score"`
	MTF             MTFAnalysis      `json:"mtf_analysis"`
	AIAnalysis      AIAnalysisResult `json:"ai_analysis"`
	FinalSignal     string           `json:"final_signal"`
	FinalConfidence int              `json:"final_confidence"`
	AnalyzedAt      time.Time        `json:"analyzed_at"`
}

// =============================================
// V2 DATABASE MODEL (GORM)
// =============================================

// AnalysisHistoryV2 menyimpan history analisis v2 ke Supabase
type AnalysisHistoryV2 struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Symbol      string    `json:"symbol" gorm:"not null;index;size:20"`
	Timeframe   string    `json:"timeframe" gorm:"not null;size:10"`
	Price       float64   `json:"price" gorm:"not null"`
	Change24h   float64   `json:"change_24h"`
	Volume      float64   `json:"volume"`
	Signal      string    `json:"signal" gorm:"not null;size:10"`
	Confidence  int       `json:"confidence"`
	QuantTotal  int       `json:"quant_total"`
	QuantSignal string    `json:"quant_signal" gorm:"size:10"`
	AIModel     string    `json:"ai_model" gorm:"size:30"`
	Reasoning   string    `json:"reasoning" gorm:"type:text"`
	Confluence  string    `json:"confluence" gorm:"size:20"`
	RSI14       float64   `json:"rsi14"`
	EMA9        float64   `json:"ema9"`
	EMA21       float64   `json:"ema21"`
	ATR         float64   `json:"atr"`
	OBV         float64   `json:"obv"`
	PatternBias string    `json:"pattern_bias" gorm:"size:10"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName override nama tabel untuk v2
func (AnalysisHistoryV2) TableName() string {
	return "analysis_history_v2"
}

// =============================================
// GPT INTERNAL MODELS





// =============================================

// GPTAnalysisResult adalah hasil parsing response GPT untuk futures trading
type GPTAnalysisResult struct {
	Signal     string  `json:"signal"`      // LONG | SHORT | WAIT
	Confidence int     `json:"confidence"`  // 0-100
	Reasoning  string  `json:"reasoning"`
	Entry      float64 `json:"entry"`       // Harga entry rekomendasi
	StopLoss   float64 `json:"stop_loss"`   // Stop loss
	TP1        float64 `json:"tp1"`         // Take profit 1
	TP2        float64 `json:"tp2"`         // Take profit 2
	TP3        float64 `json:"tp3"`         // Take profit 3
	Leverage   int     `json:"leverage"`    // Leverage rekomendasi
	RiskReward float64 `json:"risk_reward"` // Risk/Reward ratio
}

// ErrorResponse untuk response error API
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
