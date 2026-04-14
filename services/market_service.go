package services

import "api-trade/models"

// MarketService adalah interface yang diimplementasikan oleh provider exchange / agregator.
// Gunakan interface ini di handler agar mudah swap provider.
type MarketService interface {
	GetTicker(symbol string) (*models.TickerData, error)
	GetKlines(symbol, interval string, limit int) ([]models.KlineData, error)
	GetTrending(limit int) ([]models.TrendingCoin, error)
}

// Compile-time check: pastikan semua service mengimplementasikan interface
var _ MarketService = (*BinanceService)(nil)
var _ MarketService = (*CoinGeckoService)(nil)
var _ MarketService = (*BybitService)(nil)
var _ MarketService = (*IndodaxService)(nil)
