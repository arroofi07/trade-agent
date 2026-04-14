package services

import (
	"api-trade/models"
)

// QuantService menghitung skor kuantitatif matematis untuk sinyal trading
type QuantService struct{}

// NewQuantService membuat instance baru QuantService
func NewQuantService() *QuantService {
	return &QuantService{}
}

// Score menghitung skor kuantitatif dari indikator dan pattern (0-100)
func (s *QuantService) Score(
	ind *models.IndicatorResult,
	pat *models.PatternResult,
	price float64,
) *models.QuantScore {
	result := &models.QuantScore{}

	result.TrendScore = s.scoreTrend(ind, price)
	result.MomentumScore = s.scoreMomentum(ind)
	result.VolumeScore = s.scoreVolume(ind, price)
	result.VolatilityScore = s.scoreVolatility(ind)
	result.PatternScore = s.scorePattern(pat)

	total := result.TrendScore + result.MomentumScore + result.VolumeScore +
		result.VolatilityScore + result.PatternScore
	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}
	result.Total = total

	// Signal berdasarkan threshold
	if total > 65 && result.TrendScore >= 15 {
		result.Signal = "LONG"
	} else if total < 35 && result.TrendScore <= 8 {
		result.Signal = "SHORT"
	} else {
		result.Signal = "WAIT"
	}

	// Grade
	result.Grade = s.calcGrade(total)

	return result
}

// scoreTrend menghitung skor trend (0-30)
// Komponen: EMA alignment (10) + SMA alignment (10) + MACD direction (10)
func (s *QuantService) scoreTrend(ind *models.IndicatorResult, price float64) int {
	score := 0

	// EMA9/21 alignment (0-10)
	if ind.EMA9 > 0 && ind.EMA21 > 0 {
		if price > ind.EMA9 && ind.EMA9 > ind.EMA21 {
			score += 10 // strong uptrend
		} else if price > ind.EMA9 && ind.EMA9 <= ind.EMA21 {
			score += 5 // weak uptrend
		} else if price < ind.EMA9 && ind.EMA9 > ind.EMA21 {
			score += 3 // price below EMA9 but EMA trending up
		}
		// price < EMA9 && EMA9 < EMA21 = downtrend = 0 pts
	} else {
		score += 5 // no EMA data, neutral
	}

	// SMA20/50 alignment (0-10)
	if ind.SMA20 > 0 && ind.SMA50 > 0 {
		if price > ind.SMA20 && ind.SMA20 > ind.SMA50 {
			score += 10 // strong uptrend
		} else if price > ind.SMA20 && ind.SMA20 <= ind.SMA50 {
			score += 5 // weak uptrend
		} else if price < ind.SMA20 && ind.SMA20 > ind.SMA50 {
			score += 3 // price below SMA20 but MA trending up
		}
		// full downtrend = 0 pts
	} else {
		score += 5
	}

	// MACD direction (0-10)
	if ind.MACDHist > 0 {
		if ind.MACD > ind.MACDSignal {
			score += 10 // bullish histogram + crossover
		} else {
			score += 6 // positive histogram, fading
		}
	} else if ind.MACDHist == 0 {
		score += 4 // neutral
	} else {
		// negative histogram
		if ind.MACD > ind.MACDSignal {
			score += 3 // bearish hist but MACD recovering
		}
		// full bearish = 0 pts
	}

	if score > 30 {
		score = 30
	}
	return score
}

// scoreMomentum menghitung skor momentum (0-25)
// Komponen: RSI (10) + StochRSI (10) + Williams %R (5)
func (s *QuantService) scoreMomentum(ind *models.IndicatorResult) int {
	score := 0

	// RSI zone (0-10)
	rsi := ind.RSI14
	switch {
	case rsi >= 55 && rsi < 70:
		score += 10 // momentum bullish tapi belum overbought
	case rsi >= 50 && rsi < 55:
		score += 8 // momentum positif
	case rsi >= 40 && rsi < 50:
		score += 5 // neutral
	case rsi >= 30 && rsi < 40:
		score += 6 // recovery zone (potensial reversal)
	case rsi >= 70:
		score += 3 // overbought, hati-hati
	default: // < 30
		score += 1 // oversold (sell pressure)
	}

	// StochRSI %K vs %D (0-10)
	k := ind.StochRSIK
	d := ind.StochRSID
	if k > 0 || d > 0 {
		if k > d && k < 80 {
			score += 10 // bullish cross, not overbought
		} else if k > d && k >= 80 {
			score += 5 // bullish tapi overbought
		} else if k <= d && k > 20 {
			score += 3 // bearish cross, not oversold
		} else { // k <= 20
			score += 5 // oversold → potensial reversal long
		}
	} else {
		score += 5 // no data, neutral
	}

	// Williams %R (0-5)
	wr := ind.WilliamsR
	switch {
	case wr >= -20:
		score += 1 // overbought zone
	case wr >= -50:
		score += 5 // bullish momentum zone
	case wr >= -80:
		score += 3 // neutral-weak
	default: // <= -80
		score += 2 // oversold (potensial reversal long)
	}

	if score > 25 {
		score = 25
	}
	return score
}

// scoreVolume menghitung skor volume (0-20)
// CATATAN: CoinGecko tidak sediakan per-candle volume, sehingga OBV dan VWAP
// sering bernilai 0/lastClose. Dalam kondisi ini berikan skor neutral (10).
func (s *QuantService) scoreVolume(ind *models.IndicatorResult, price float64) int {
	score := 0

	// Deteksi apakah ada data volume
	noVolumeData := ind.OBV == 0 && ind.VWAP == price

	if noVolumeData {
		// CoinGecko tidak sediakan volume per-candle: berikan skor neutral
		return 10
	}

	// OBV direction (0-12)
	if ind.OBV > 0 {
		score += 12 // net buying pressure
	} else if ind.OBV == 0 {
		score += 6 // neutral
	}
	// OBV < 0: net selling pressure = 0 pts

	// Price vs VWAP (0-8)
	if ind.VWAP > 0 && price > ind.VWAP {
		score += 8 // bullish above institutional reference
	} else if ind.VWAP > 0 {
		score += 0 // bearish below VWAP
	} else {
		score += 4 // no VWAP data
	}

	if score > 20 {
		score = 20
	}
	return score
}

// scoreVolatility menghitung skor volatility (0-15)
// Komponen: ATR% dari price (8) + BB squeeze (7)
func (s *QuantService) scoreVolatility(ind *models.IndicatorResult) int {
	score := 0

	// ATR sebagai % dari price (0-8): semakin rendah = lebih baik untuk entry
	if ind.ATR > 0 && ind.BBMiddle > 0 {
		atrPct := ind.ATR / ind.BBMiddle * 100
		switch {
		case atrPct < 0.5:
			score += 8 // sangat tight
		case atrPct < 1.0:
			score += 6 // volatility normal
		case atrPct < 2.0:
			score += 4 // elevated
		case atrPct < 3.0:
			score += 2 // high volatility
		default:
			score += 0 // very high, risky
		}
	} else {
		score += 4 // no ATR data, neutral
	}

	// BB width / squeeze (0-7): semakin sempit = potensial breakout
	if ind.BBUpper > 0 && ind.BBLower > 0 && ind.BBMiddle > 0 {
		bbWidth := (ind.BBUpper - ind.BBLower) / ind.BBMiddle * 100
		switch {
		case bbWidth < 2.0:
			score += 7 // squeeze ketat, breakout imminent
		case bbWidth < 4.0:
			score += 5 // moderate
		case bbWidth < 6.0:
			score += 3 // lebar
		default:
			score += 1 // sangat lebar, choppy
		}
	} else {
		score += 3 // no BB data
	}

	if score > 15 {
		score = 15
	}
	return score
}

// scorePattern menghitung skor pattern (0-10)
func (s *QuantService) scorePattern(pat *models.PatternResult) int {
	if pat == nil || len(pat.CandlePatterns) == 0 {
		return 5 // neutral jika tidak ada pattern
	}

	strongBullish := map[string]bool{"BullishEngulfing": true, "MorningStar": true}
	strongBearish := map[string]bool{"BearishEngulfing": true, "EveningStar": true}
	weakBullish := map[string]bool{"Hammer": true, "InvertedHammer": true}
	weakBearish := map[string]bool{"ShootingStar": true}

	bullPts, bearPts := 0, 0
	for _, p := range pat.CandlePatterns {
		if strongBullish[p] {
			bullPts += 4
		} else if strongBearish[p] {
			bearPts += 4
		} else if weakBullish[p] {
			bullPts += 2
		} else if weakBearish[p] {
			bearPts += 2
		}
		// Doji = neutral, tidak menambah skor
	}

	if bullPts > bearPts {
		if bullPts > 10 {
			bullPts = 10
		}
		return bullPts
	} else if bearPts > bullPts {
		return 0 // penalize long bias jika ada pattern bearish
	}

	// Mixed atau tidak ada pattern kuat
	return 5
}

// calcGrade menentukan grade dari total skor
func (s *QuantService) calcGrade(total int) string {
	switch {
	case total >= 85:
		return "A+"
	case total >= 75:
		return "A"
	case total >= 65:
		return "B+"
	case total >= 55:
		return "B"
	case total >= 40:
		return "C"
	default:
		return "D"
	}
}
