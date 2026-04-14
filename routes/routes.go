package routes

import (
	"api-trade/config"
	"api-trade/handlers"
	"api-trade/services"

	"github.com/gofiber/fiber/v3"
)

// Setup mendaftarkan semua route ke instance Fiber
func Setup(app *fiber.App, cfg *config.Config) {
	// Inisialisasi semua service
	// Indodax: data market spot IDR (summaries + tradingview history_v2, ~180 req/menit publik)
	marketSvc := services.NewIndodaxService()
	indicatorSvc := services.NewIndicatorService()
	gptSvc := services.NewGPTService(cfg)
	// V2 services
	patternSvc := services.NewPatternService()
	quantSvc := services.NewQuantService()
	aiSvc := services.NewAIService(cfg)

	// Inisialisasi handler
	cryptoHandler := handlers.NewCryptoHandler(marketSvc, indicatorSvc, gptSvc, patternSvc, quantSvc, aiSvc)

	// ==========================================
	// Health Check
	// ==========================================
	app.Get("/health", cryptoHandler.Health)
	// Kompatibilitas klien yang memanggil /trending tanpa prefix /api
	app.Get("/trending", cryptoHandler.GetTrending)

	// ==========================================
	// API Routes
	// ==========================================
	api := app.Group("/api")
	// Alias di /api (tanpa /crypto) — daftarkan di sini agar jelas dan konsisten
	api.Get("/analyze-v2/:symbol", cryptoHandler.AnalyzeV2)
	api.Get("/price/:symbol", cryptoHandler.GetPrice)
	api.Get("/trending", cryptoHandler.GetTrending)
	api.Get("/analyze/:symbol", cryptoHandler.Analyze)

	crypto := api.Group("/crypto")

	// GET /api/crypto/analyze/:symbol?timeframe=1h
	// Analisis teknikal lengkap dengan AI (v1, backward compatible)
	crypto.Get("/analyze/:symbol", cryptoHandler.Analyze)

	// GET /api/crypto/analyze-v2/:symbol?timeframe=1h
	// Analisis v2: patterns + quant scoring + MTF + Claude AI
	crypto.Get("/analyze-v2/:symbol", cryptoHandler.AnalyzeV2)

	// GET /api/crypto/price/:symbol
	// Harga realtime
	crypto.Get("/price/:symbol", cryptoHandler.GetPrice)

	// GET /api/crypto/klines/:symbol?timeframe=1h&limit=100
	// Data candlestick
	crypto.Get("/klines/:symbol", cryptoHandler.GetKlines)

	// GET /api/crypto/indicators/:symbol?timeframe=1h
	// Semua indikator teknikal (tanpa AI)
	crypto.Get("/indicators/:symbol", cryptoHandler.GetIndicators)

	// GET /api/crypto/trending?limit=20
	// Pair IDR teratas menurut volume (IDR)
	crypto.Get("/trending", cryptoHandler.GetTrending)

	// GET /api/crypto/history/:symbol?limit=20
	// History analisis dari Supabase
	crypto.Get("/history/:symbol", cryptoHandler.GetHistory)
}
