package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config menyimpan semua konfigurasi aplikasi
type Config struct {
	Port        string
	OpenAIKey   string
	OpenAIModel string
	ClaudeKey   string // ANTHROPIC_API_KEY
	AIModel     string // AI_MODEL: "claude" | "gpt", default "claude"
	DatabaseURL string
	SupabaseURL string
	SupabaseKey string
}

// Load membaca konfigurasi dari file .env
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Tidak ada file .env, menggunakan environment variable system")
	}

	cfg := &Config{
		Port:        getEnv("PORT", "3000"),
		OpenAIKey:   getEnv("OPENAI_API_KEY", ""),
		OpenAIModel: getEnv("OPENAI_MODEL", "gpt-4o"),
		ClaudeKey:   getEnv("ANTHROPIC_API_KEY", ""),
		AIModel:     getEnv("AI_MODEL", "claude"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		SupabaseURL: getEnv("SUPABASE_URL", ""),
		SupabaseKey: getEnv("SUPABASE_KEY", ""),
	}

	// Validasi config kritis
	if cfg.AIModel == "claude" && cfg.ClaudeKey == "" {
		log.Println("⚠️  ANTHROPIC_API_KEY belum diset! Fitur analisis Claude tidak akan berfungsi.")
	}
	if cfg.AIModel == "gpt" && cfg.OpenAIKey == "" {
		log.Println("⚠️  OPENAI_API_KEY belum diset! Fitur analisis GPT tidak akan berfungsi.")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("❌ DATABASE_URL wajib diisi!")
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
