package database

import (
	"log"

	"api-trade/config"
	"api-trade/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB adalah instance database global
var DB *gorm.DB

// Connect membuka koneksi ke Supabase PostgreSQL dan melakukan auto-migrate
func Connect(cfg *config.Config) {
	var err error

	DB, err = gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
		PreferSimpleProtocol: true, // Diperlukan untuk Supabase pooler (transaksi) agar tidak menggunakan prepared statements
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Gagal koneksi ke database: %v", err)
	}

	// Auto migrate tabel
	if err := DB.AutoMigrate(
		&models.AnalysisHistory{},   // v1
		&models.AnalysisHistoryV2{}, // v2: pattern + quant + AI
	); err != nil {
		log.Fatalf("❌ Gagal migrate database: %v", err)
	}

	// Test ping
	sqlDB, _ := DB.DB()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Database ping gagal: %v", err)
	}

	log.Println("✅ Koneksi Supabase berhasil, tabel ter-migrate!")
}
