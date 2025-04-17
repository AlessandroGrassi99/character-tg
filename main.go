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
	appConfig    Config
)

// Prompts
var (
	//go:embed prompts/chat_message.txt
	promptChatMessage string
	//go:embed prompts/chat_overview.txt
	promptChatOverview string
)

func main() {
	// rand.Seed is deprecated in Go 1.20+, but we don't need to explicitly initialize 
	// the random number generator in newer Go versions
	
	// General Config
	var err error
	appConfig, err = loadConfig()
	if err != nil {
		log.Fatalf("error loading configuration: %v", err)
		return
	}

	// Initialize ai clients
	grokClient = openai.NewClient(
		option.WithBaseURL("https://api.x.ai/v1"),
		option.WithAPIKey(appConfig.GrokApiKey),
	)

	geminiClient = openai.NewClient(
		option.WithBaseURL("https://generativelanguage.googleapis.com/v1beta/openai/"),
		option.WithAPIKey(appConfig.GeminiApiKey),
	)

	// Initialize Redis-based chat storage
	chatStorage = NewChatStorage(appConfig)
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
		bot.WithMiddlewares(allowListMiddleware, storeMessageMiddleware, randomReplyMiddleware),
		bot.WithDefaultHandler(handlerNewMessage),
	}

	b, err := bot.New(appConfig.TelegramBotToken, opts...)
	if err != nil {
		log.Fatalf("failed to create bot instance: %v", err)
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "config", bot.MatchTypeCommand, handlerSetCharacter)
	b.RegisterHandler(bot.HandlerTypeMessageText, "init", bot.MatchTypeCommand, handlerInitChat)

	b.RegisterHandlerMatchFunc(matchJsonFiles, handlerImportChat)

	// health check server for Fly.io
	go startHealthCheckServer(&appConfig)

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
	containsDocument := update.Message.Document != nil

	if !containsDocument {
		return false
	}

	isJSONFile := filepath.Ext(update.Message.Document.FileName) == ".json"

	return isJSONFile
}
