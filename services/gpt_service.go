package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"api-trade/config"
	"api-trade/models"

	openai "github.com/sashabaranov/go-openai"
)

// GPTService menangani komunikasi dengan OpenAI GPT
type GPTService struct {
	client *openai.Client
	model  string
}

// NewGPTService membuat instance baru GPTService
func NewGPTService(cfg *config.Config) *GPTService {
	client := openai.NewClient(cfg.OpenAIKey)
	return &GPTService{
		client: client,
		model:  cfg.OpenAIModel,
	}
}

// AnalyzeCrypto mengirim data teknikal ke GPT dan mendapatkan sinyal trading
func (s *GPTService) AnalyzeCrypto(
	symbol string,
	timeframe string,
	price float64,
	change24h float64,
	volume float64,
	indicators *models.IndicatorResult,
) (*models.GPTAnalysisResult, error) {

	// Buat deskripsi kondisi pasar berdasarkan indikator
	marketContext := s.buildMarketContext(symbol, timeframe, price, change24h, volume, indicators)

	systemPrompt := `Kamu adalah trader crypto profesional yang spesialis di FUTURES TRADING dengan win rate tinggi.
Tugasmu adalah menganalisis data teknikal dan memberikan sinyal futures yang objektif, sangat akurat, dan actionable.

ATURAN ANALISIS FUTURES:
- Tentukan trend utama dari SMA20 dan SMA50.
- Gunakan RSI untuk mencari momentum overbought/oversold.
- Gunakan MACD untuk konfirmasi pembalikan arah atau kelanjutan trend.
- Gunakan area Bollinger Bands (Upper/Lower/Middle) sebagai acuan dinamis untuk titik Resistance dan Support terdekat.
- Jika data saling bertentangan (misal trend naik tapi RSI overbought parah), berikan sinyal WAIT.

ATURAN MANAJEMEN RISIKO (WAJIB DIIKUTI):
- Entry: Tentukan harga entry terbaik (bisa saat ini atau tunggu koreksi/pullback ke area support/resistance terdekat).
- Stop Loss (SL): Letakkan SL di area yang masuk akal secara teknikal (misalnya di luar Bollinger Bands atau di bawah moving average). Maksimal risiko 1-3% dari titik entry harga nyata.
- Take Profit (TP):
  - TP1: Hitung dengan rasio Risk/Reward minimal 1:1.5
  - TP2: Hitung dengan rasio Risk/Reward 1:2.5
  - TP3: Hitung dengan rasio Risk/Reward 1:4.0 (Riding the trend)
- Leverage: Sesuaikan kelayakan setup. 
  - Sinyal pasti (Confidence > 80%): 10x - 20x
  - Sinyal standar (Confidence 65-80%): 5x - 10x
  - Sinyal spekulasi (< 65%): Jangan paksa, beri sinyal WAIT.

Balas HANYA dengan JSON valid tanpa ada kata lain. Formulanya adalah:
{
  "signal": "LONG" | "SHORT" | "WAIT",
  "confidence": <angka 0-100>,
  "reasoning": "<analisis teknikal singkat dalam Bahasa Indonesia, max 3 kalimat>",
  "entry": <harga entry yang direkomendasikan>,
  "stop_loss": <harga stop loss>,
  "tp1": <take profit 1>,
  "tp2": <take profit 2>,
  "tp3": <take profit 3>,
  "leverage": <leverage rekomendasi, angka bulat>,
  "risk_reward": <rasio risk reward rata-rata, contoh: 2.5>
}`

	userPrompt := marketContext

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: s.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			MaxTokens:   500,
			Temperature: 0.1, // Temperatur yang sangat rendah untuk analitik objektif dan konsisten
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("GPT tidak memberikan response")
	}

	rawContent := resp.Choices[0].Message.Content

	// Strip markdown code block jika GPT membungkus dengan ```json ... ```
	cleaned := stripMarkdownJSON(rawContent)

	// Parse JSON response dari GPT
	var result models.GPTAnalysisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("gagal parse response GPT: %s | error: %w", rawContent, err)
	}

	// Validasi signal futures
	switch result.Signal {
	case "LONG", "SHORT", "WAIT":
		// valid
	default:
		// Backward compat jika GPT masih balas BUY/SELL/HOLD
		switch result.Signal {
		case "BUY":
			result.Signal = "LONG"
		case "SELL":
			result.Signal = "SHORT"
		default:
			result.Signal = "WAIT"
		}
	}

	// Clamp confidence ke 0-100
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}

	// Clamp leverage ke 1-20
	if result.Leverage < 1 {
		result.Leverage = 1
	}
	if result.Leverage > 20 {
		result.Leverage = 20
	}

	return &result, nil
}

// AnalyzeV2 memvalidasi sinyal kuantitatif dengan GPT-4o (enhanced signature untuk AI interface)
func (s *GPTService) AnalyzeV2(
	symbol string,
	timeframe string,
	price float64,
	change24h float64,
	volume float64,
	indicators *models.IndicatorResult,
	patterns *models.PatternResult,
	quant *models.QuantScore,
) (*models.AIAnalysisResult, error) {

	systemPrompt := fmt.Sprintf(`Kamu adalah trader crypto profesional yang bertugas MEMVALIDASI sinyal kuantitatif.

SISTEM KUANTITATIF TELAH MENGHITUNG:
- Total Score: %d/100, Signal: %s, Grade: %s

PERAN KAMU: Validasi sinyal di atas, confidence harus mendekati %d kecuali ada kontradiksi jelas.
JANGAN ubah signal LONG→SHORT atau SHORT→LONG tanpa alasan teknikal sangat kuat.

Aturan manajemen risiko: SL maks 1.5-3%% dari entry. TP1 RR 1:1.5, TP2 RR 1:2.5, TP3 RR 1:4.0.
Leverage: >80%% confidence=10-20x, 65-80%%=5-10x, <65%%=maks 5x.

Balas HANYA dengan JSON valid:
{"signal":"LONG"|"SHORT"|"WAIT","confidence":<0-100>,"reasoning":"<2-3 kalimat>","entry":<harga>,"stop_loss":<harga>,"tp1":<harga>,"tp2":<harga>,"tp3":<harga>,"leverage":<angka>,"risk_reward":<rasio>}`,
		quant.Total, quant.Signal, quant.Grade, quant.Total)

	// Build context dengan data enhanced
	var sb strings.Builder
	fmt.Fprintf(&sb, "Analisis %s (%s) | Harga: $%.8f | 24h: %.2f%%\n", symbol, timeframe, price, change24h)
	fmt.Fprintf(&sb, "EMA9: %.8f, EMA21: %.8f, SMA20: %.8f, SMA50: %.8f\n", indicators.EMA9, indicators.EMA21, indicators.SMA20, indicators.SMA50)
	fmt.Fprintf(&sb, "RSI: %.2f, StochRSI K/D: %.2f/%.2f, Williams%%R: %.2f\n", indicators.RSI14, indicators.StochRSIK, indicators.StochRSID, indicators.WilliamsR)
	fmt.Fprintf(&sb, "MACD: %.8f, Signal: %.8f, Hist: %.8f\n", indicators.MACD, indicators.MACDSignal, indicators.MACDHist)
	fmt.Fprintf(&sb, "ATR: %.8f, BB: %.8f/%.8f/%.8f\n", indicators.ATR, indicators.BBUpper, indicators.BBMiddle, indicators.BBLower)
	if patterns != nil {
		if len(patterns.CandlePatterns) > 0 {
			fmt.Fprintf(&sb, "Patterns: %s (Bias: %s)\n", strings.Join(patterns.CandlePatterns, ","), patterns.PatternBias)
		}
		fmt.Fprintf(&sb, "Pivot: %.8f, S1: %.8f, R1: %.8f\n", patterns.PivotPoint, patterns.S1, patterns.R1)
	}
	fmt.Fprintf(&sb, "Quant Score: %d/100 (%s) - Signal: %s\nValidasi sinyal ini:", quant.Total, quant.Grade, quant.Signal)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: s.model,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: sb.String()},
			},
			MaxTokens:   500,
			Temperature: 0.1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("GPT tidak memberikan response")
	}

	rawContent := resp.Choices[0].Message.Content
	cleaned := stripMarkdownJSON(rawContent)

	var result models.AIAnalysisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("gagal parse response GPT v2: %s | error: %w", rawContent, err)
	}

	result.Model = s.model
	result.Signal = normalizeSignal(result.Signal)
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}
	if result.Leverage < 1 {
		result.Leverage = 1
	}
	if result.Leverage > 20 {
		result.Leverage = 20
	}

	return &result, nil
}

// buildMarketContext memformat semua data teknikal menjadi teks untuk GPT
func (s *GPTService) buildMarketContext(
	symbol, timeframe string,
	price, change24h, volume float64,
	ind *models.IndicatorResult,
) string {
	return fmt.Sprintf(`Analisis teknikal untuk %s (Timeframe: %s):

📊 DATA PASAR:
- Harga Saat Ini: $%.8f
- Perubahan 24h: %.2f%%
- Volume 24h: $%.2f

📈 INDIKATOR TEKNIKAL:
- RSI (14): %.2f
- SMA (20): $%.8f
- SMA (50): $%.8f
- MACD Line: %.8f
- MACD Signal: %.8f
- MACD Histogram: %.8f
- Bollinger Upper: $%.8f
- Bollinger Middle: $%.8f
- Bollinger Lower: $%.8f

📍 POSISI HARGA:
- Harga vs SMA20: %s
- Harga vs SMA50: %s
- Posisi di Bollinger Band: %s

Berikan analisis teknikal dan rekomendasi trading:`,
		symbol, timeframe,
		price, change24h, volume,
		ind.RSI14,
		ind.SMA20, ind.SMA50,
		ind.MACD, ind.MACDSignal, ind.MACDHist,
		ind.BBUpper, ind.BBMiddle, ind.BBLower,
		pricePosition(price, ind.SMA20, "SMA20"),
		pricePosition(price, ind.SMA50, "SMA50"),
		bollingerPosition(price, ind.BBUpper, ind.BBLower, ind.BBMiddle),
	)
}

// pricePosition mendeskripsikan posisi harga relatif terhadap moving average
func pricePosition(price, ma float64, name string) string {
	if price > ma {
		pct := ((price - ma) / ma) * 100
		return fmt.Sprintf("Di ATAS %s (+%.2f%%)", name, pct)
	}
	pct := ((ma - price) / ma) * 100
	return fmt.Sprintf("Di BAWAH %s (-%.2f%%)", name, pct)
}

// bollingerPosition mendeskripsikan posisi harga di dalam Bollinger Bands
func bollingerPosition(price, upper, lower, middle float64) string {
	bandwidth := upper - lower
	if bandwidth == 0 {
		return "Di tengah Bollinger Band"
	}
	position := (price - lower) / bandwidth * 100
	switch {
	case position >= 90:
		return fmt.Sprintf("Sangat dekat UPPER band (%.1f%%)", position)
	case position >= 70:
		return fmt.Sprintf("Dekat upper band (%.1f%%)", position)
	case position >= 30:
		return fmt.Sprintf("Di tengah band (%.1f%%)", position)
	case position >= 10:
		return fmt.Sprintf("Dekat lower band (%.1f%%)", position)
	default:
		return fmt.Sprintf("Sangat dekat LOWER band (%.1f%%)", position)
	}
}

// stripMarkdownJSON membersihkan markdown code block dari response GPT.
// GPT kadang mengembalikan ```json { ... } ``` meskipun diminta plain JSON.
func stripMarkdownJSON(s string) string {
	s = strings.TrimSpace(s)

	// Hapus opening ```json atau ``` (dengan atau tanpa newline)
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*")
	s = re.ReplaceAllString(s, "")

	// Hapus closing ```
	re2 := regexp.MustCompile("(?s)\\s*```$")
	s = re2.ReplaceAllString(s, "")

	return strings.TrimSpace(s)
}

