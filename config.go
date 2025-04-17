package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	GrokApiKey       string
	GeminiApiKey     string
	RedisAddr        string
	RedisPassword    string
	HttpServerPort   string
	AllowedChatIDs   []int64
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
	
	// Parse allowed chat IDs from environment variable
	allowedChatsStr := os.Getenv("ALLOWED_CHAT_IDS")
	if allowedChatsStr != "" {
		chatIDs, err := parseAllowedChatIDs(allowedChatsStr)
		if err != nil {
			log.Printf("warning: error parsing ALLOWED_CHAT_IDS: %v", err)
		} else {
			config.AllowedChatIDs = chatIDs
			log.Printf("Configured %d allowed chat IDs", len(chatIDs))
		}
	}

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

// parseAllowedChatIDs parses a comma-separated list of chat IDs
// Format example: "-1001234567890,123456789"
func parseAllowedChatIDs(input string) ([]int64, error) {
	if input == "" {
		return nil, nil
	}
	
	parts := strings.Split(input, ",")
	result := make([]int64, 0, len(parts))
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		var chatID int64
		n, err := fmt.Sscanf(part, "%d", &chatID)
		if err != nil || n != 1 {
			return nil, fmt.Errorf("invalid chat ID format: %s", part)
		}
		
		result = append(result, chatID)
	}
	
	return result, nil
}
