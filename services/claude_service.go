package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"api-trade/config"
	"api-trade/models"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeService menangani komunikasi dengan Anthropic Claude
type ClaudeService struct {
	client anthropic.Client
	model  string
}

// NewClaudeService membuat instance baru ClaudeService
func NewClaudeService(cfg *config.Config) *ClaudeService {
	client := anthropic.NewClient(
		option.WithAPIKey(cfg.ClaudeKey),
	)
	return &ClaudeService{
		client: client,
		model:  "claude-sonnet-4-6",
	}
}

// AnalyzeV2 memvalidasi sinyal kuantitatif dengan Claude AI
func (s *ClaudeService) AnalyzeV2(
	symbol string,
	timeframe string,
	price float64,
	change24h float64,
	volume float64,
	indicators *models.IndicatorResult,
	patterns *models.PatternResult,
	quant *models.QuantScore,
) (*models.AIAnalysisResult, error) {

	systemPrompt := s.buildSystemPrompt(quant)
	userPrompt := s.buildEnhancedContext(symbol, timeframe, price, change24h, volume, indicators, patterns, quant)

	message, err := s.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(s.model),
		MaxTokens: int64(600),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Claude API error: %w", err)
	}

	if len(message.Content) == 0 {
		return nil, fmt.Errorf("Claude tidak memberikan response")
	}

	rawContent := message.Content[0].Text
	cleaned := stripMarkdownJSON(rawContent)

	var result models.AIAnalysisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("gagal parse response Claude: %s | error: %w", rawContent, err)
	}

	// Set model name
	result.Model = s.model

	// Normalisasi signal
	result.Signal = normalizeSignal(result.Signal)

	// Clamp confidence
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}

	// Clamp leverage
	if result.Leverage < 1 {
		result.Leverage = 1
	}
	if result.Leverage > 20 {
		result.Leverage = 20
	}

	return &result, nil
}

// buildSystemPrompt membuat system prompt yang menekankan peran validasi Claude
func (s *ClaudeService) buildSystemPrompt(quant *models.QuantScore) string {
	return fmt.Sprintf(`Kamu adalah ahli analisis teknikal crypto yang bertugas MEMVALIDASI sinyal kuantitatif, bukan men-generate ulang dari nol.

SISTEM KUANTITATIF TELAH MENGHITUNG:
- Total Score: %d/100
- Signal: %s
- Grade: %s

PERAN KAMU:
1. Periksa apakah indikator-indikator konsisten dengan sinyal kuantitatif di atas
2. Confidence kamu HARUS mendekati quant score (%d) kecuali ada kontradiksi teknikal yang SANGAT jelas
3. JANGAN return LONG jika quant signal adalah SHORT, dan sebaliknya, kecuali ada alasan spesifik
4. Jika sinyal quant adalah WAIT, konfirmasi WAIT kecuali ada setup yang sangat jelas
5. Berikan level entry, stop loss, dan take profit yang presisi berdasarkan indikator

ATURAN MANAJEMEN RISIKO:
- Stop Loss: maksimal 1.5-3%% dari entry, letakkan di luar level support/resistance terdekat
- TP1: Risk/Reward minimal 1:1.5
- TP2: Risk/Reward 1:2.5
- TP3: Risk/Reward 1:4.0
- Leverage: confidence >80%% → 10-20x, 65-80%% → 5-10x, <65%% → max 5x

Balas HANYA dengan JSON valid tanpa kata lain:
{
  "signal": "LONG" | "SHORT" | "WAIT",
  "confidence": <0-100, harus mendekati quant score %d>,
  "reasoning": "<analisis singkat 2-3 kalimat Bahasa Indonesia>",
  "entry": <harga entry>,
  "stop_loss": <harga stop loss>,
  "tp1": <take profit 1>,
  "tp2": <take profit 2>,
  "tp3": <take profit 3>,
  "leverage": <leverage rekomendasi>,
  "risk_reward": <rasio risk reward rata-rata>
}`, quant.Total, quant.Signal, quant.Grade, quant.Total, quant.Total)
}

// buildEnhancedContext memformat semua data teknikal + pattern + quant untuk Claude
func (s *ClaudeService) buildEnhancedContext(
	symbol, timeframe string,
	price, change24h, volume float64,
	ind *models.IndicatorResult,
	pat *models.PatternResult,
	quant *models.QuantScore,
) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Validasi analisis untuk %s (Timeframe: %s)\n\n", symbol, timeframe)

	fmt.Fprintf(&sb, "📊 DATA PASAR:\n")
	fmt.Fprintf(&sb, "- Harga: $%.8f\n", price)
	fmt.Fprintf(&sb, "- Perubahan 24h: %.2f%%\n", change24h)
	fmt.Fprintf(&sb, "- Volume 24h: $%.2f\n\n", volume)

	fmt.Fprintf(&sb, "📈 INDIKATOR TREND:\n")
	fmt.Fprintf(&sb, "- EMA9: $%.8f | EMA21: $%.8f\n", ind.EMA9, ind.EMA21)
	fmt.Fprintf(&sb, "- SMA20: $%.8f | SMA50: $%.8f\n", ind.SMA20, ind.SMA50)
	fmt.Fprintf(&sb, "- MACD: %.8f | Signal: %.8f | Hist: %.8f\n\n", ind.MACD, ind.MACDSignal, ind.MACDHist)

	fmt.Fprintf(&sb, "📉 INDIKATOR MOMENTUM:\n")
	fmt.Fprintf(&sb, "- RSI(14): %.2f\n", ind.RSI14)
	fmt.Fprintf(&sb, "- Stoch RSI %%K: %.2f | %%D: %.2f\n", ind.StochRSIK, ind.StochRSID)
	fmt.Fprintf(&sb, "- Williams %%R: %.2f\n\n", ind.WilliamsR)

	fmt.Fprintf(&sb, "📊 VOLATILITY & VOLUME:\n")
	fmt.Fprintf(&sb, "- ATR(14): %.8f\n", ind.ATR)
	fmt.Fprintf(&sb, "- BB Upper: $%.8f | Middle: $%.8f | Lower: $%.8f\n", ind.BBUpper, ind.BBMiddle, ind.BBLower)
	fmt.Fprintf(&sb, "- VWAP: $%.8f | OBV: %.4f\n\n", ind.VWAP, ind.OBV)

	if pat != nil {
		fmt.Fprintf(&sb, "🕯️ PATTERN RECOGNITION:\n")
		if len(pat.CandlePatterns) > 0 {
			fmt.Fprintf(&sb, "- Patterns: %s\n", strings.Join(pat.CandlePatterns, ", "))
		} else {
			fmt.Fprintf(&sb, "- Patterns: Tidak ada pattern signifikan\n")
		}
		fmt.Fprintf(&sb, "- Bias Pattern: %s\n", pat.PatternBias)
		if len(pat.SupportLevels) > 0 {
			fmt.Fprintf(&sb, "- Support: $%.8f", pat.SupportLevels[0])
			if len(pat.SupportLevels) > 1 {
				fmt.Fprintf(&sb, ", $%.8f", pat.SupportLevels[1])
			}
			fmt.Fprintf(&sb, "\n")
		}
		if len(pat.ResistanceLevels) > 0 {
			fmt.Fprintf(&sb, "- Resistance: $%.8f", pat.ResistanceLevels[0])
			if len(pat.ResistanceLevels) > 1 {
				fmt.Fprintf(&sb, ", $%.8f", pat.ResistanceLevels[1])
			}
			fmt.Fprintf(&sb, "\n")
		}
		fmt.Fprintf(&sb, "- Pivot: $%.8f | S1: $%.8f | R1: $%.8f\n\n", pat.PivotPoint, pat.S1, pat.R1)
		fmt.Fprintf(&sb, "- Fib 38.2%%: $%.8f | 50%%: $%.8f | 61.8%%: $%.8f\n\n", pat.Fib382, pat.Fib500, pat.Fib618)
	}

	if quant != nil {
		fmt.Fprintf(&sb, "🔢 SKOR KUANTITATIF:\n")
		fmt.Fprintf(&sb, "- Total: %d/100 (Grade: %s)\n", quant.Total, quant.Grade)
		fmt.Fprintf(&sb, "- Trend: %d/30 | Momentum: %d/25 | Volume: %d/20\n", quant.TrendScore, quant.MomentumScore, quant.VolumeScore)
		fmt.Fprintf(&sb, "- Volatility: %d/15 | Pattern: %d/10\n", quant.VolatilityScore, quant.PatternScore)
		fmt.Fprintf(&sb, "- Quant Signal: %s\n\n", quant.Signal)
	}

	fmt.Fprintf(&sb, "Berikan validasi dan rekomendasi trading futures:")

	return sb.String()
}

// normalizeSignal mengkonversi signal ke format standar LONG|SHORT|WAIT
func normalizeSignal(signal string) string {
	switch strings.ToUpper(signal) {
	case "LONG", "BUY":
		return "LONG"
	case "SHORT", "SELL":
		return "SHORT"
	default:
		return "WAIT"
	}
}
