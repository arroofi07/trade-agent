package services

import (
	"fmt"
	"math"

	"api-trade/models"
)

// IndicatorService menghitung semua indikator teknikal dari data klines
type IndicatorService struct{}

// NewIndicatorService membuat instance baru IndicatorService
func NewIndicatorService() *IndicatorService {
	return &IndicatorService{}
}

// Calculate menghitung semua indikator sekaligus dari slice KlineData
func (s *IndicatorService) Calculate(klines []models.KlineData) (*models.IndicatorResult, error) {
	// StochRSI(14,14,3) butuh 14+14+3=31, ditambah buffer → set 60 sebagai safety margin
	const minRequired = 60
	if len(klines) < minRequired {
		return nil, fmt.Errorf("data tidak cukup: butuh minimal %d klines, dapat %d", minRequired, len(klines))
	}

	// Extract semua slice yang dibutuhkan
	n := len(klines)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Volume
	}

	result := &models.IndicatorResult{}

	// SMA 20 & 50
	result.SMA20 = s.SMA(closes, 20)
	result.SMA50 = s.SMA(closes, 50)

	// RSI 14
	rsi, err := s.RSI(closes, 14)
	if err != nil {
		return nil, err
	}
	result.RSI14 = rsi

	// MACD (12, 26, 9)
	macd, signal, hist, err := s.MACD(closes, 12, 26, 9)
	if err != nil {
		return nil, err
	}
	result.MACD = macd
	result.MACDSignal = signal
	result.MACDHist = hist

	// Bollinger Bands (20, 2)
	upper, middle, lower, err := s.BollingerBands(closes, 20, 2)
	if err != nil {
		return nil, err
	}
	result.BBUpper = upper
	result.BBMiddle = middle
	result.BBLower = lower

	// EMA 9 & 21 (short-term trend crossover)
	ema9slice, err := s.EMASlice(closes, 9)
	if err == nil {
		result.EMA9 = roundDecimal(ema9slice[len(ema9slice)-1])
	}
	ema21slice, err := s.EMASlice(closes, 21)
	if err == nil {
		result.EMA21 = roundDecimal(ema21slice[len(ema21slice)-1])
	}

	// ATR 14 (Average True Range)
	result.ATR = s.ATR(highs, lows, closes, 14)

	// Stochastic RSI (14, 14, 3, 3)
	rsiSlice, err := s.RSISlice(closes, 14)
	if err == nil {
		k14, d14 := s.StochRSI(rsiSlice, 14, 3, 3)
		result.StochRSIK = k14
		result.StochRSID = d14
	}

	// Williams %R (14)
	result.WilliamsR = s.WilliamsR(highs, lows, closes, 14)

	// VWAP (fallback ke lastClose jika volume=0, karena CoinGecko tidak sediakan per-candle volume)
	result.VWAP = s.VWAP(highs, lows, closes, volumes)

	// OBV (On-Balance Volume)
	result.OBV = s.OBV(closes, volumes)

	return result, nil
}

// =============================================
// SMA - Simple Moving Average
// =============================================

// SMA menghitung rata-rata sederhana N periode terakhir
func (s *IndicatorService) SMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}
	sum := 0.0
	start := len(data) - period
	for i := start; i < len(data); i++ {
		sum += data[i]
	}
	return roundDecimal(sum / float64(period))
}

// =============================================
// EMA - Exponential Moving Average
// =============================================

// EMASlice menghitung semua nilai EMA dari slice data
func (s *IndicatorService) EMASlice(data []float64, period int) ([]float64, error) {
	if len(data) < period {
		return nil, fmt.Errorf("data tidak cukup untuk EMA(%d)", period)
	}

	k := 2.0 / float64(period+1)
	emas := make([]float64, len(data))

	// Seed: SMA dari period pertama
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	emas[period-1] = sum / float64(period)

	// Hitung EMA dari periode ke-period+1 dst
	for i := period; i < len(data); i++ {
		emas[i] = data[i]*k + emas[i-1]*(1-k)
	}

	return emas, nil
}

// =============================================
// RSI - Relative Strength Index (Wilder's Smoothing)
// =============================================

// RSI menghitung RSI dengan periode tertentu (biasanya 14)
func (s *IndicatorService) RSI(data []float64, period int) (float64, error) {
	if len(data) < period+1 {
		return 0, fmt.Errorf("data tidak cukup untuk RSI(%d)", period)
	}

	// Hitung gain dan loss
	gains := make([]float64, len(data)-1)
	losses := make([]float64, len(data)-1)

	for i := 1; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			gains[i-1] = change
		} else {
			losses[i-1] = math.Abs(change)
		}
	}

	// SMA awal untuk periode pertama
	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Wilder's smoothing untuk sisa data
	for i := period; i < len(gains); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
	}

	if avgLoss == 0 {
		return 100, nil
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return roundDecimal(rsi), nil
}

// =============================================
// MACD - Moving Average Convergence Divergence
// =============================================

// MACD menghitung MACD line, signal line, dan histogram
// Standard: fastPeriod=12, slowPeriod=26, signalPeriod=9
func (s *IndicatorService) MACD(data []float64, fastPeriod, slowPeriod, signalPeriod int) (macdLine, signalLine, histogram float64, err error) {
	if len(data) < slowPeriod+signalPeriod {
		return 0, 0, 0, fmt.Errorf("data tidak cukup untuk MACD")
	}

	// Hitung EMA fast dan slow
	fastEMA, err := s.EMASlice(data, fastPeriod)
	if err != nil {
		return 0, 0, 0, err
	}
	slowEMA, err := s.EMASlice(data, slowPeriod)
	if err != nil {
		return 0, 0, 0, err
	}

	// Hitung MACD line = EMA(fast) - EMA(slow)
	// Hanya valid dari index slowPeriod-1 ke atas
	macdValues := make([]float64, len(data))
	for i := slowPeriod - 1; i < len(data); i++ {
		macdValues[i] = fastEMA[i] - slowEMA[i]
	}

	// Hitung Signal line = EMA(9) dari MACD line (hanya bagian valid)
	validMacd := macdValues[slowPeriod-1:]
	signalEMA, err := s.EMASlice(validMacd, signalPeriod)
	if err != nil {
		return 0, 0, 0, err
	}

	// Ambil nilai terakhir
	lastMACD := validMacd[len(validMacd)-1]
	lastSignal := signalEMA[len(signalEMA)-1]
	lastHist := lastMACD - lastSignal

	return roundDecimal(lastMACD), roundDecimal(lastSignal), roundDecimal(lastHist), nil
}

// =============================================
// Bollinger Bands
// =============================================

// BollingerBands menghitung upper, middle (SMA), dan lower band
func (s *IndicatorService) BollingerBands(data []float64, period int, stdDevMultiplier float64) (upper, middle, lower float64, err error) {
	if len(data) < period {
		return 0, 0, 0, fmt.Errorf("data tidak cukup untuk Bollinger Bands(%d)", period)
	}

	// Middle band = SMA(period)
	middle = s.SMA(data, period)

	// Standar deviasi dari period terakhir
	start := len(data) - period
	variance := 0.0
	for i := start; i < len(data); i++ {
		diff := data[i] - middle
		variance += diff * diff
	}
	stdDev := math.Sqrt(variance / float64(period))

	// Upper dan Lower band
	upper = roundDecimal(middle + stdDevMultiplier*stdDev)
	lower = roundDecimal(middle - stdDevMultiplier*stdDev)
	middle = roundDecimal(middle)

	return upper, middle, lower, nil
}

// roundDecimal membulatkan ke 4 desimal
func roundDecimal(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// =============================================
// RSISlice - RSI sebagai full slice (untuk StochRSI)
// =============================================

// RSISlice menghitung RSI dan mengembalikan seluruh slice nilai RSI.
// Index sebelum `period` bernilai 0 (belum valid).
func (s *IndicatorService) RSISlice(data []float64, period int) ([]float64, error) {
	if len(data) < period+1 {
		return nil, fmt.Errorf("data tidak cukup untuk RSISlice(%d)", period)
	}

	rsiValues := make([]float64, len(data))

	gains := make([]float64, len(data)-1)
	losses := make([]float64, len(data)-1)
	for i := 1; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			gains[i-1] = change
		} else {
			losses[i-1] = math.Abs(change)
		}
	}

	var avgGain, avgLoss float64
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	calcRSI := func(ag, al float64) float64 {
		if al == 0 {
			return 100
		}
		rs := ag / al
		return 100 - (100 / (1 + rs))
	}

	rsiValues[period] = calcRSI(avgGain, avgLoss)

	for i := period; i < len(gains); i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)
		if i+1 < len(rsiValues) {
			rsiValues[i+1] = calcRSI(avgGain, avgLoss)
		}
	}

	return rsiValues, nil
}

// =============================================
// ATR - Average True Range (Wilder's smoothing)
// =============================================

// ATR menghitung Average True Range dengan Wilder's smoothing
func (s *IndicatorService) ATR(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < period+1 {
		return 0
	}

	trValues := make([]float64, n-1)
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		tr := hl
		if hpc > tr {
			tr = hpc
		}
		if lpc > tr {
			tr = lpc
		}
		trValues[i-1] = tr
	}

	// Seed: SMA dari period pertama
	var atr float64
	for i := 0; i < period; i++ {
		atr += trValues[i]
	}
	atr /= float64(period)

	// Wilder's smoothing
	for i := period; i < len(trValues); i++ {
		atr = (atr*float64(period-1) + trValues[i]) / float64(period)
	}

	return roundDecimal(atr)
}

// =============================================
// StochRSI - Stochastic RSI (%K dan %D)
// =============================================

// StochRSI menghitung Stochastic RSI dari slice RSI.
// stochPeriod: periode lookup (14), smoothK: smoothing %K (3), smoothD: smoothing %D (3)
// Return: %K dan %D dalam range 0-100
func (s *IndicatorService) StochRSI(rsiValues []float64, rsiPeriod, stochPeriod, smoothD int) (k, d float64) {
	n := len(rsiValues)
	if n < rsiPeriod+stochPeriod {
		return 0, 0
	}

	rawStoch := make([]float64, n)
	for i := rsiPeriod + stochPeriod - 1; i < n; i++ {
		window := rsiValues[i-stochPeriod+1 : i+1]
		minRSI := window[0]
		maxRSI := window[0]
		for _, v := range window {
			if v < minRSI {
				minRSI = v
			}
			if v > maxRSI {
				maxRSI = v
			}
		}
		if maxRSI-minRSI == 0 {
			rawStoch[i] = 0
		} else {
			rawStoch[i] = (rsiValues[i] - minRSI) / (maxRSI - minRSI) * 100
		}
	}

	// %K = SMA(rawStoch, smoothK=3) menggunakan 3 nilai terakhir yang valid
	validStoch := rawStoch[rsiPeriod+stochPeriod-1:]
	if len(validStoch) < 3 {
		return 0, 0
	}

	// Ambil rata-rata 3 nilai terakhir untuk %K
	last3K := validStoch[len(validStoch)-3:]
	kSum := 0.0
	for _, v := range last3K {
		kSum += v
	}
	k = roundDecimal(kSum / 3.0)

	// %D = SMA dari 3 nilai %K terakhir (simplified: gunakan rata-rata window sebelumnya)
	if len(validStoch) >= 6 {
		prev3K := validStoch[len(validStoch)-6 : len(validStoch)-3]
		pSum := 0.0
		for _, v := range prev3K {
			pSum += v
		}
		d = roundDecimal(pSum / 3.0)
	} else {
		d = k
	}

	return k, d
}

// =============================================
// Williams %R
// =============================================

// WilliamsR menghitung Williams %R dalam range -100..0
// Overbought: dekat 0, Oversold: dekat -100
func (s *IndicatorService) WilliamsR(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < period {
		return -50
	}

	window := n - period
	highestHigh := highs[window]
	lowestLow := lows[window]
	for i := window + 1; i < n; i++ {
		if highs[i] > highestHigh {
			highestHigh = highs[i]
		}
		if lows[i] < lowestLow {
			lowestLow = lows[i]
		}
	}

	if highestHigh-lowestLow == 0 {
		return -50
	}

	wr := (highestHigh - closes[n-1]) / (highestHigh - lowestLow) * -100
	return roundDecimal(wr)
}

// =============================================
// VWAP - Volume Weighted Average Price
// =============================================

// VWAP menghitung Volume Weighted Average Price.
// CATATAN: CoinGecko OHLC endpoint tidak menyediakan per-candle volume,
// sehingga volumes slice akan berisi semua 0. Dalam kasus ini VWAP
// fallback ke harga close terakhir (netral).
func (s *IndicatorService) VWAP(highs, lows, closes, volumes []float64) float64 {
	n := len(closes)
	if n == 0 {
		return 0
	}

	var cumulTPV, cumulVol float64
	for i := 0; i < n; i++ {
		typicalPrice := (highs[i] + lows[i] + closes[i]) / 3.0
		cumulTPV += typicalPrice * volumes[i]
		cumulVol += volumes[i]
	}

	if cumulVol == 0 {
		// Fallback: tidak ada data volume (CoinGecko limitation)
		return roundDecimal(closes[n-1])
	}

	return roundDecimal(cumulTPV / cumulVol)
}

// =============================================
// OBV - On-Balance Volume
// =============================================

// OBV menghitung On-Balance Volume.
// Nilai positif = tekanan beli, negatif = tekanan jual.
// CATATAN: Akan bernilai 0 jika data volume tidak tersedia (CoinGecko).
func (s *IndicatorService) OBV(closes, volumes []float64) float64 {
	n := len(closes)
	if n < 2 {
		return 0
	}

	obv := 0.0
	for i := 1; i < n; i++ {
		if closes[i] > closes[i-1] {
			obv += volumes[i]
		} else if closes[i] < closes[i-1] {
			obv -= volumes[i]
		}
		// jika equal: OBV tidak berubah
	}

	return roundDecimal(obv)
}
