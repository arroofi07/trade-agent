package services

import (
	"math"
	"sort"

	"api-trade/models"
)

// PatternService mendeteksi pola candlestick dan level harga penting
type PatternService struct{}

// NewPatternService membuat instance baru PatternService
func NewPatternService() *PatternService {
	return &PatternService{}
}

// Analyze menjalankan semua analisis pattern dari slice KlineData
func (s *PatternService) Analyze(klines []models.KlineData) *models.PatternResult {
	result := &models.PatternResult{
		CandlePatterns:   []string{},
		SupportLevels:    []float64{},
		ResistanceLevels: []float64{},
	}

	n := len(klines)
	if n < 3 {
		result.PatternBias = "NEUTRAL"
		return result
	}

	last := klines[n-1]
	currentPrice := last.Close

	// Noise filter: skip jika candle terlalu kecil (range < 0.1% dari harga)
	minRange := currentPrice * 0.001

	// Deteksi pola candlestick dari 3 candle terakhir
	s.detectPatterns(result, klines, minRange)

	// Support & Resistance dari 50 candle terakhir
	lookback := 50
	if n < lookback {
		lookback = n
	}
	recent := klines[n-lookback:]
	s.findSupportResistance(result, recent, currentPrice)

	// Pivot Points (dari candle terakhir)
	s.calcPivotPoints(result, last)

	// Fibonacci dari swing high/low dalam 50 candle terakhir
	s.calcFibonacci(result, recent)

	return result
}

// =============================================
// CANDLESTICK PATTERN DETECTORS
// =============================================

func (s *PatternService) detectPatterns(result *models.PatternResult, klines []models.KlineData, minRange float64) {
	n := len(klines)
	last := klines[n-1]
	prev := klines[n-2]

	var bullPts, bearPts int

	// 1-candle patterns pada candle terakhir
	if last.High-last.Low >= minRange {
		if s.isDoji(last) {
			result.CandlePatterns = append(result.CandlePatterns, "Doji")
		}
		if s.isHammer(last) {
			result.CandlePatterns = append(result.CandlePatterns, "Hammer")
			bullPts += 2
		}
		if s.isInvertedHammer(last) {
			result.CandlePatterns = append(result.CandlePatterns, "InvertedHammer")
			bullPts += 2
		}
		if s.isShootingStar(last) {
			result.CandlePatterns = append(result.CandlePatterns, "ShootingStar")
			bearPts += 2
		}
	}

	// 2-candle patterns
	if prev.High-prev.Low >= minRange && last.High-last.Low >= minRange {
		if s.isBullishEngulfing(prev, last) {
			result.CandlePatterns = append(result.CandlePatterns, "BullishEngulfing")
			bullPts += 4
		}
		if s.isBearishEngulfing(prev, last) {
			result.CandlePatterns = append(result.CandlePatterns, "BearishEngulfing")
			bearPts += 4
		}
	}

	// 3-candle patterns
	if n >= 3 {
		prev2 := klines[n-3]
		if prev2.High-prev2.Low >= minRange {
			if s.isMorningStar(prev2, prev, last) {
				result.CandlePatterns = append(result.CandlePatterns, "MorningStar")
				bullPts += 4
			}
			if s.isEveningStar(prev2, prev, last) {
				result.CandlePatterns = append(result.CandlePatterns, "EveningStar")
				bearPts += 4
			}
		}
	}

	// Tentukan bias
	if bullPts > bearPts && bullPts > 0 {
		result.PatternBias = "BULLISH"
	} else if bearPts > bullPts && bearPts > 0 {
		result.PatternBias = "BEARISH"
	} else {
		result.PatternBias = "NEUTRAL"
	}
}

// isDoji: body < 10% dari total range
func (s *PatternService) isDoji(k models.KlineData) bool {
	body := math.Abs(k.Close - k.Open)
	totalRange := k.High - k.Low
	if totalRange == 0 {
		return false
	}
	return body/totalRange < 0.10
}

// isHammer: lower shadow >= 2x body, upper shadow <= 0.5x body
func (s *PatternService) isHammer(k models.KlineData) bool {
	body := math.Abs(k.Close - k.Open)
	if body == 0 {
		return false
	}
	lowerShadow := math.Min(k.Open, k.Close) - k.Low
	upperShadow := k.High - math.Max(k.Open, k.Close)
	return lowerShadow >= 2*body && upperShadow <= 0.5*body
}

// isInvertedHammer: upper shadow >= 2x body, lower shadow <= 0.5x body
func (s *PatternService) isInvertedHammer(k models.KlineData) bool {
	body := math.Abs(k.Close - k.Open)
	if body == 0 {
		return false
	}
	upperShadow := k.High - math.Max(k.Open, k.Close)
	lowerShadow := math.Min(k.Open, k.Close) - k.Low
	return upperShadow >= 2*body && lowerShadow <= 0.5*body
}

// isShootingStar: sama dengan inverted hammer tapi close < open (bearish)
func (s *PatternService) isShootingStar(k models.KlineData) bool {
	return s.isInvertedHammer(k) && k.Close < k.Open
}

// isBullishEngulfing: prev bearish, curr bullish & body curr mencakup body prev
func (s *PatternService) isBullishEngulfing(prev, curr models.KlineData) bool {
	prevBearish := prev.Close < prev.Open
	currBullish := curr.Close > curr.Open
	engulfs := curr.Open <= prev.Close && curr.Close >= prev.Open
	return prevBearish && currBullish && engulfs
}

// isBearishEngulfing: prev bullish, curr bearish & body curr mencakup body prev
func (s *PatternService) isBearishEngulfing(prev, curr models.KlineData) bool {
	prevBullish := prev.Close > prev.Open
	currBearish := curr.Close < curr.Open
	engulfs := curr.Open >= prev.Close && curr.Close <= prev.Open
	return prevBullish && currBearish && engulfs
}

// isMorningStar: a bearish, b badan kecil, c bullish memasuki body a
func (s *PatternService) isMorningStar(a, b, c models.KlineData) bool {
	aBody := a.Open - a.Close // positif jika bearish
	bBody := math.Abs(b.Close - b.Open)
	cBody := c.Close - c.Open // positif jika bullish

	if aBody <= 0 || cBody <= 0 {
		return false
	}
	// b badan kecil (<30% dari a) dan c menutup di atas pertengahan body a
	midA := a.Open - aBody*0.5
	return bBody < aBody*0.3 && c.Close > midA
}

// isEveningStar: kebalikan morning star (a bullish, b kecil, c bearish)
func (s *PatternService) isEveningStar(a, b, c models.KlineData) bool {
	aBody := a.Close - a.Open // positif jika bullish
	bBody := math.Abs(b.Close - b.Open)
	cBody := c.Open - c.Close // positif jika bearish

	if aBody <= 0 || cBody <= 0 {
		return false
	}
	midA := a.Open + aBody*0.5
	return bBody < aBody*0.3 && c.Close < midA
}

// =============================================
// SUPPORT & RESISTANCE
// =============================================

// findSupportResistance mendeteksi level support dan resistance dari local minima/maxima
func (s *PatternService) findSupportResistance(result *models.PatternResult, klines []models.KlineData, currentPrice float64) {
	n := len(klines)
	if n < 3 {
		return
	}

	var supports, resistances []float64

	// Local minima = support, local maxima = resistance
	for i := 1; i < n-1; i++ {
		// Local minimum
		if klines[i].Low <= klines[i-1].Low && klines[i].Low <= klines[i+1].Low {
			supports = append(supports, klines[i].Low)
		}
		// Local maximum
		if klines[i].High >= klines[i-1].High && klines[i].High >= klines[i+1].High {
			resistances = append(resistances, klines[i].High)
		}
	}

	// Cluster levels dalam 0.5% dan ambil 3 terdekat ke currentPrice
	result.SupportLevels = s.clusterAndSort(supports, 0.005, currentPrice, 3, false)
	result.ResistanceLevels = s.clusterAndSort(resistances, 0.005, currentPrice, 3, true)
}

// clusterAndSort mengelompokkan level yang berdekatan dan mengambil top N terdekat
func (s *PatternService) clusterAndSort(levels []float64, tolerance float64, currentPrice float64, topN int, above bool) []float64 {
	if len(levels) == 0 {
		return []float64{}
	}

	sort.Float64s(levels)

	// Cluster levels yang berdekatan
	var clustered []float64
	i := 0
	for i < len(levels) {
		cluster := []float64{levels[i]}
		j := i + 1
		for j < len(levels) && levels[i] > 0 && (levels[j]-levels[i])/levels[i] <= tolerance {
			cluster = append(cluster, levels[j])
			j++
		}
		// Rata-rata cluster
		sum := 0.0
		for _, v := range cluster {
			sum += v
		}
		clustered = append(clustered, sum/float64(len(cluster)))
		i = j
	}

	// Filter: support di bawah currentPrice, resistance di atas
	var filtered []float64
	for _, v := range clustered {
		if above && v > currentPrice {
			filtered = append(filtered, v)
		} else if !above && v < currentPrice {
			filtered = append(filtered, v)
		}
	}

	// Sort by proximity to currentPrice
	sort.Slice(filtered, func(i, j int) bool {
		di := math.Abs(filtered[i] - currentPrice)
		dj := math.Abs(filtered[j] - currentPrice)
		return di < dj
	})

	if len(filtered) > topN {
		filtered = filtered[:topN]
	}

	// Round semua values
	for i, v := range filtered {
		filtered[i] = roundDecimal(v)
	}

	return filtered
}

// =============================================
// PIVOT POINTS
// =============================================

// calcPivotPoints menghitung Classic Floor Trader pivot points dari candle terakhir
func (s *PatternService) calcPivotPoints(result *models.PatternResult, k models.KlineData) {
	h := k.High
	l := k.Low
	c := k.Close

	pp := (h + l + c) / 3.0
	r1 := 2*pp - l
	r2 := pp + (h - l)
	s1 := 2*pp - h
	s2 := pp - (h - l)

	result.PivotPoint = roundDecimal(pp)
	result.R1 = roundDecimal(r1)
	result.R2 = roundDecimal(r2)
	result.S1 = roundDecimal(s1)
	result.S2 = roundDecimal(s2)
}

// =============================================
// FIBONACCI RETRACEMENT
// =============================================

// calcFibonacci menghitung level Fibonacci dari swing high/low
func (s *PatternService) calcFibonacci(result *models.PatternResult, klines []models.KlineData) {
	if len(klines) == 0 {
		return
	}

	swingHigh := klines[0].High
	swingLow := klines[0].Low

	for _, k := range klines {
		if k.High > swingHigh {
			swingHigh = k.High
		}
		if k.Low < swingLow {
			swingLow = k.Low
		}
	}

	diff := swingHigh - swingLow
	if diff == 0 {
		return
	}

	result.Fib236 = roundDecimal(swingHigh - diff*0.236)
	result.Fib382 = roundDecimal(swingHigh - diff*0.382)
	result.Fib500 = roundDecimal(swingHigh - diff*0.500)
	result.Fib618 = roundDecimal(swingHigh - diff*0.618)
	result.Fib786 = roundDecimal(swingHigh - diff*0.786)
}
