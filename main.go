package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var (
	chatStorage  *ChatStorage
	grokClient   openai.Client
	geminiClient openai.Client
)

// Prompts
var (
	//go:embed prompts/chat_message.txt
	promptChatMessage string
	//go:embed prompts/chat_overview.txt
	promptChatOverview string
)

func main() {
	// General Config
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
		return
	}

	// Initialize ai clients
	grokClient = openai.NewClient(
		option.WithBaseURL("https://api.x.ai/v1"),
		option.WithAPIKey(config.GrokApiKey),
	)

	geminiClient = openai.NewClient(
		option.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
		option.WithAPIKey(config.GeminiApiKey),
	)

	// Initialize Redis-based chat storage
	chatStorage = NewChatStorage(config)
	if err := chatStorage.Ping(); err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}

	// Bot Init
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	defer func() {
		if chatStorage == nil {
			return
		}
		if err := chatStorage.Close(); err != nil {
			log.Printf("error closing Redis connection: %v", err)
		}
	}()

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	b, err := bot.New(config.TelegramBotToken, opts...)
	if err != nil {
		log.Fatalf("failed to create bot instance: %v", err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "config", bot.MatchTypeCommand, setCharacter)
	b.RegisterHandler(bot.HandlerTypeMessageText, "init", bot.MatchTypeCommand, initChat)

	b.RegisterHandlerMatchFunc(matchJsonFiles, importChat)

	// health check server for Fly.io
	go startHealthCheckServer(&config)

	b.Start(ctx)
}

func startHealthCheckServer(config *Config) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	serverAddr := "0.0.0.0:" + config.HttpServerPort
	if err := http.ListenAndServe(serverAddr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("health check server error: %v", err)
	}
}

func matchJsonFiles(update *models.Update) bool {
	if update == nil || update.Message == nil {
		return false
	}
	isPrivate := update.Message.Chat.Type == "private"
	containsDocument := update.Message.Document != nil

	if !isPrivate || !containsDocument {
		return false
	}

	isJSONFile := filepath.Ext(update.Message.Document.FileName) == ".json"

	return isJSONFile
}
