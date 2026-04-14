package services

import (
	"fmt"
	"log"

	"api-trade/config"
	"api-trade/models"
)

// AIProvider adalah interface yang harus diimplementasikan oleh Claude dan GPT service
type AIProvider interface {
	AnalyzeV2(
		symbol, timeframe string,
		price, change24h, volume float64,
		indicators *models.IndicatorResult,
		patterns *models.PatternResult,
		quant *models.QuantScore,
	) (*models.AIAnalysisResult, error)
}

// AIService mengelola pemilihan AI provider (primary + fallback)
type AIService struct {
	primary     AIProvider
	fallback    AIProvider
	primaryName string
}

// NewAIService membuat instance baru AIService berdasarkan konfigurasi AI_MODEL
func NewAIService(cfg *config.Config) *AIService {
	svc := &AIService{}

	if cfg.AIModel == "gpt" {
		// GPT sebagai primary, Claude sebagai fallback
		svc.primaryName = "gpt"
		if cfg.OpenAIKey != "" {
			svc.primary = NewGPTService(cfg)
			log.Println("🤖 AI Primary: GPT-4o")
		} else {
			log.Println("⚠️  GPT dipilih sebagai primary tapi OPENAI_API_KEY kosong")
		}
		if cfg.ClaudeKey != "" {
			svc.fallback = NewClaudeService(cfg)
			log.Println("🔄 AI Fallback: Claude (claude-sonnet-4-6)")
		}
	} else {
		// Claude sebagai primary (default), GPT sebagai fallback
		svc.primaryName = "claude"
		if cfg.ClaudeKey != "" {
			svc.primary = NewClaudeService(cfg)
			log.Println("🤖 AI Primary: Claude (claude-sonnet-4-6)")
		} else {
			log.Println("⚠️  Claude dipilih sebagai primary tapi ANTHROPIC_API_KEY kosong")
		}
		if cfg.OpenAIKey != "" {
			svc.fallback = NewGPTService(cfg)
			log.Println("🔄 AI Fallback: GPT-4o")
		}
	}

	return svc
}

// Analyze menjalankan analisis AI dengan primary provider, fallback ke secondary jika gagal.
// Jika kedua AI tidak tersedia, return sentinel result berdasarkan quant signal.
func (svc *AIService) Analyze(
	symbol, timeframe string,
	price, change24h, volume float64,
	indicators *models.IndicatorResult,
	patterns *models.PatternResult,
	quant *models.QuantScore,
) (*models.AIAnalysisResult, error) {

	// Coba primary
	if svc.primary != nil {
		result, err := svc.primary.AnalyzeV2(symbol, timeframe, price, change24h, volume, indicators, patterns, quant)
		if err == nil {
			return result, nil
		}
		log.Printf("⚠️  Primary AI error: %v", err)

		// Coba fallback
		if svc.fallback != nil {
			log.Println("🔄 Mencoba fallback AI...")
			result, err = svc.fallback.AnalyzeV2(symbol, timeframe, price, change24h, volume, indicators, patterns, quant)
			if err == nil {
				return result, nil
			}
			log.Printf("⚠️  Fallback AI error: %v", err)
		}
	} else if svc.fallback != nil {
		// Primary tidak tersedia, langsung ke fallback
		result, err := svc.fallback.AnalyzeV2(symbol, timeframe, price, change24h, volume, indicators, patterns, quant)
		if err == nil {
			return result, nil
		}
		log.Printf("⚠️  Fallback AI error: %v", err)
	}

	// Kedua AI tidak tersedia: kembalikan error
	return nil, fmt.Errorf("semua AI provider tidak tersedia atau API key tidak dikonfigurasi")
}
