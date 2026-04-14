package main

import (
	"log"

	"api-trade/config"
	"api-trade/database"
	"api-trade/routes"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
	// Load config dari .env
	cfg := config.Load()

	// Koneksi ke database Supabase
	database.Connect(cfg)

	// Setup Fiber
	app := fiber.New(fiber.Config{
		AppName: "AI Crypto Analysis API v1.0",
	})

	// Middleware global
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
	}))
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Setup semua route
	routes.Setup(app, cfg)

	log.Printf("🚀 AI Crypto Analysis API berjalan di http://localhost:%s", cfg.Port)
	log.Fatal(app.Listen(":"+cfg.Port, fiber.ListenConfig{
		EnablePrintRoutes: true,
	}))
}
