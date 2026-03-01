package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL    string
	RedisURL       string
	TelegramToken  string
	BaseURL        string // public base URL for redirect tracking, e.g. https://mybot.example.com
	MiniAppURL     string // GitHub Pages URL for the Telegram Mini App settings page
	MinWhaleAmount float64
}

func Load() *Config {
	minAmount, _ := strconv.ParseFloat(os.Getenv("MIN_WHALE_AMOUNT"), 64)
	if minAmount == 0 {
		minAmount = 100 // absolute floor; per-user settings do the real filtering
	}

	return &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		RedisURL:       os.Getenv("REDIS_URL"),
		TelegramToken:  os.Getenv("TELEGRAM_TOKEN"),
		BaseURL:        os.Getenv("BOT_BASE_URL"),
		MiniAppURL:     os.Getenv("MINI_APP_URL"),
		MinWhaleAmount: minAmount,
	}
}
