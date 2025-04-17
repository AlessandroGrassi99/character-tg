package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/openai/openai-go"
)

func handlerInitChat(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Only process in private chats to prevent spamming group chats
	if update.Message.Chat.Type != "private" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "This command can only be used in private chats",
		})
		return
	}

	// chatID := update.Message.Chat.ID
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Analyzing chat history... This might take a moment.",
	})

	// Get chat state
	chatState, ok := chatStorage.GetChatState(update.Message.Chat.ID)
	if !ok || len(chatState.Messages) == 0 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No chat history found. Please import a chat first.",
		})
		return
	}

	startIndex := max(len(chatState.Messages)-7000, 0)

	// Convert messages to JSON for the prompt
	messagesJSON, err := json.Marshal(chatState.Messages[startIndex:])
	if err != nil {
		log.Printf("Error marshaling messages to JSON: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error processing chat history",
		})
		return
	}
	prompt := strings.ReplaceAll(promptChatOverview, "{{CHAT_HISTORY}}", string(messagesJSON))

	log.Printf("Prompt: %v\n", prompt)

	// Call Gemini API for analysis
	resp, err := geminiClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: "gemini-2.5-pro-exp-03-25", // Adjust model as needed
	})
	if err != nil {
		log.Printf("Error calling Gemini API: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error analyzing chat: " + err.Error(),
		})
		return
	}

	if len(resp.Choices) == 0 {
		log.Printf("Error: Empty choices array in Gemini response")
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error: Received empty response from Gemini",
		})
		return
	}

	analysisText := resp.Choices[0].Message.Content
	log.Printf("Analysis text: %s", analysisText)

	// Store the summary in the chat state
	chatStorage.SetSummary(update.Message.Chat.ID, analysisText)

	// Save analysis to a temporary file
	tmpFile, err := os.CreateTemp("", "analysis-*.txt")
	if err != nil {
		log.Printf("Error creating temp file: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error creating analysis file: " + err.Error(),
		})
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	if _, err := tmpFile.WriteString(analysisText); err != nil {
		log.Printf("Error writing to temp file: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error writing analysis to file: " + err.Error(),
		})
		return
	}
	tmpFile.Close()

	// Read the file for upload
	fileData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		log.Printf("Error reading temp file: %v", err)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error reading analysis file: " + err.Error(),
		})
		return
	}

	// Send the file
	b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID: update.Message.Chat.ID,
		Document: &models.InputFileUpload{
			Filename: "chat_analysis.txt",
			Data:     bytes.NewReader(fileData),
		},
		Caption: "📊 Chat Analysis",
	})
}
