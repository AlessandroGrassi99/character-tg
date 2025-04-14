package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	GrokApiKey       string
	GeminiApiKey     string
	RedisAddr        string
	RedisPassword    string
	HttpServerPort   string
}

func loadConfig() (Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("error loading .env file: %v", err)
	}

	var config Config
	config.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	config.GrokApiKey = os.Getenv("GROK_API_KEY")
	config.GeminiApiKey = os.Getenv("GEMINI_API_KEY")
	config.RedisAddr = getEnv("REDIS_ADDR", "localhost:6379")
	config.RedisPassword = os.Getenv("REDIS_PASSWORD")
	config.HttpServerPort = getEnv("PORT", "8080")

	if config.TelegramBotToken == "" {
		return config, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable not set")
	}
	if config.GrokApiKey == "" {
		return config, fmt.Errorf("GROK_API_KEY environment variable not set")
	}
	if config.GeminiApiKey == "" {
		return config, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	if config.RedisAddr == "" {
		return config, fmt.Errorf("REDIS_ADDR environment variable not set")
	}
	if config.RedisPassword == "" {
		return config, fmt.Errorf("REDIS_PASSWORD environment variable not set")
	}
	return config, nil
}

// getEnv retrieves the value of the environment variable named by the key.
// If the variable is not present, returns the fallback value.
func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}
