## AI Trading API - Makefile

APP_NAME = api-trade
MAIN     = main.go
PORT     = 3000

.PHONY: run dev build tidy install clean help

## help: Tampilkan daftar perintah
help:
	@echo ""
	@echo "  AI Crypto Trading API - Commands"
	@echo "  ================================="
	@echo "  make run      → Jalankan server"
	@echo "  make dev      → Hot reload (Air via go run, tanpa install ke PATH)"
	@echo "  make build    → Build binary"
	@echo "  make install  → Install semua dependencies"
	@echo "  make tidy     → go mod tidy"
	@echo "  make clean    → Hapus binary"
	@echo ""

## install: Install semua dependencies
install:
	go get github.com/joho/godotenv
	go get github.com/sashabaranov/go-openai
	go get gorm.io/gorm
	go get gorm.io/driver/postgres
	go get github.com/gofiber/fiber/v3/middleware/cors
	go get github.com/gofiber/fiber/v3/middleware/logger
	go get github.com/gofiber/fiber/v3/middleware/recover
	go mod tidy
	@echo "✅ Semua dependency berhasil diinstall!"

## tidy: Jalankan go mod tidy
tidy:
	go mod tidy

## run: Jalankan server
run:
	go run $(MAIN)

## dev: Hot reload — Air dijalankan lewat go run (tetap di root project ini)
dev:
	go run github.com/air-verse/air@latest

## build: Build binary
build:
	go build -o $(APP_NAME).exe $(MAIN)
	@echo "✅ Build berhasil: $(APP_NAME).exe"

## clean: Hapus binary
clean:
	rm -f $(APP_NAME).exe
	@echo "🗑️  Binary dihapus"

## test-health: Test endpoint health
test-health:
	curl -s http://localhost:$(PORT)/health | python -m json.tool

## test-price: Test endpoint price (BTCUSDT)
test-price:
	curl -s http://localhost:$(PORT)/api/crypto/price/BTCUSDT | python -m json.tool

## test-analyze: Test endpoint analyze (BTCUSDT, 1h)
test-analyze:
	curl -s "http://localhost:$(PORT)/api/crypto/analyze/BTCUSDT?timeframe=1h" | python -m json.tool

## test-trending: Test endpoint trending
test-trending:
	curl -s "http://localhost:$(PORT)/api/crypto/trending?limit=10" | python -m json.tool
