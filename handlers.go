package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/openai/openai-go"
)

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	if update.Message.Text == "" { // Ignore non-text messages for history/counting
		log.Printf("Ignoring non-text message (e.g., sticker, photo) in chat %d", chatID)
		return
	}

	// Store the full message object
	chatStorage.StoreMessage(1171388254, *update.Message)

	resp, err := grokClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(buildChatMessages(1171388254, 1000)),
		},
		Model:           "grok-3-mini-beta",
		ReasoningEffort: "low",
	})
	if err != nil {
		panic(err)
	}

	responseText := resp.Choices[0].Message.Content

	// Remove message planning section from the response
	cleanedResponse := responseText
	if planningStart := strings.Index(responseText, "<response_preparation>"); planningStart != -1 {
		if planningEnd := strings.Index(responseText, "</response_preparation>"); planningEnd != -1 {
			log.Printf("Found planning end at position %d", planningEnd)

			// Check if there's content before the planning section
			var beforePlanning string
			if planningStart > 0 {
				beforePlanning = strings.TrimSpace(responseText[:planningStart])
			}

			// Check if there's content after the planning section
			var afterPlanning string
			if planningEnd+len("</response_preparation>") < len(responseText) {
				afterPlanning = strings.TrimSpace(responseText[planningEnd+len("</response_preparation>"):])
			}

			// Decide what to use as the cleaned response
			if afterPlanning != "" {
				// If there's content after planning, use that
				cleanedResponse = afterPlanning
			} else if beforePlanning != "" {
				// If there's only content before planning, use that
				cleanedResponse = beforePlanning
			}
		}
	}

	// Create new message with cleaned response
	messageToSend := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   cleanedResponse,
	}

	sentMsg, err := b.SendMessage(ctx, messageToSend)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	chatStorage.StoreMessage(1171388254, *sentMsg)
}

func initChat(ctx context.Context, b *bot.Bot, update *models.Update) {
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
	chatState, ok := chatStorage.GetChatState(1171388254)
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

	me, err := b.GetMe(ctx)
	if err != nil {
		log.Printf("error getting information about the bot: %v", err)
		return
	}
	botName := fmt.Sprintf("%s %s: @%s", me.FirstName, me.LastName, me.Username)

	prompt = strings.ReplaceAll(prompt, "{{BOT_INFO}}", botName)

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
	chatStorage.SetSummary(1171388254, analysisText)

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

func setCharacter(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message.Chat.Type != "private" {
		return
	}

	commandFullText := update.Message.Text
	commandArgs := ""

	parts := strings.Fields(commandFullText) // Split by whitespace
	if len(parts) > 1 {
		commandPart := parts[0]
		commandEndIndex := len(commandPart)
		commandArgs = strings.TrimSpace(commandFullText[commandEndIndex:])
	}

	// No command args provided
	if commandArgs == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Usage: /config <character description>\nPlease provide a character description to set as the prompt.",
		})
		return
	}

	chatStorage.SetPrompt(1171388254, commandArgs)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Character prompt has been set. The bot will respond according to this character description.",
	})
}

// handleFileImport processes incoming files and imports chat history from JSON files
func importChat(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Only process files in private chats
	if update.Message.Chat.Type != "private" {
		return
	}

	userID := update.Message.From.ID
	document := update.Message.Document
	if document == nil {
		log.Printf("no document in message %d from chat %d", update.Message.ID, userID)
		return
	}

	// Log information about received file
	log.Printf("received file: %s (ID: %s, MIME: %s, Size: %d bytes) from chat %d",
		document.FileName, document.FileID, document.MimeType, document.FileSize, userID)

	// Use system temporary directory
	tmpDir := os.TempDir()

	// Create a unique filename with chatID and timestamp
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	safeFilename := filepath.Clean(document.FileName)
	filePath := filepath.Join(tmpDir, strconv.FormatInt(userID, 10)+"_"+timestamp+"_"+safeFilename)

	// Get file information to download it
	fileInfo, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: document.FileID,
	})
	if err != nil {
		log.Printf("error getting file info: %v", err)
		return
	}

	// Get the download link for the file
	downloadURL := b.FileDownloadLink(fileInfo)

	// Download the file using HTTP
	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Printf("error downloading file: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("error downloading file: HTTP status code %d", resp.StatusCode)
		return
	}

	// Read file data from response
	fileData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error reading file data: %v", err)
		return
	}

	// Save the file locally
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		log.Printf("error saving file: %v", err)
		return
	}

	// Check if it's a JSON file (potential chat history)
	if filepath.Ext(safeFilename) != ".json" {
		log.Printf("not a JSON file, skipping import: %v", filePath)
		return
	}
	chatExport, err := NewChatExport(filePath)
	if err != nil {
		log.Printf("error reading file: %v", err)
		return
	}

	// Convert and import messages to the chat storage
	convertedMessages := ConvertExportMessages(chatExport.Messages)
	chatStorage.ImportChat(chatExport.ID, convertedMessages)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: userID,
		Text:   "Successfully imported chat export with " + strconv.Itoa(len(chatExport.Messages)) + " messages from chat " + chatExport.Name,
	})
}

func buildChatMessages(chatID int64, limit int) string {
	chatState, ok := chatStorage.GetChatState(chatID)
	if !ok {
		return "Just reply: No prompt provided"
	}

	finalPrompt := promptChatMessage
	finalPrompt = strings.Replace(finalPrompt, "{{PROMPT}}", chatState.Prompt, 1)
	finalPrompt = strings.Replace(finalPrompt, "{{OVERVIEW}}", chatState.Summary, 1)

	startIndex := max(len(chatState.Messages)-limit, 0)

	// Last 20 messages for {{LAST_MESSAGES}}
	lastMsgCount := 20
	lastMsgStartIndex := max(len(chatState.Messages)-lastMsgCount, startIndex)
	lastMessagesJSON, err := json.Marshal(chatState.Messages[lastMsgStartIndex:])
	if err != nil {
		log.Printf("Error marshaling last messages to JSON: %v", err)
		return "Error: Could not process recent messages"
	}

	// All previous messages for {{CHAT_HISTORY}}
	historyJSON, err := json.Marshal(chatState.Messages[startIndex:lastMsgStartIndex])
	if err != nil {
		log.Printf("Error marshaling history messages to JSON: %v", err)
		return "Error: Could not process chat history"
	}

	finalPrompt = strings.Replace(finalPrompt, "{{CHAT_HISTORY}}", string(historyJSON), 1)
	finalPrompt = strings.Replace(finalPrompt, "{{LAST_MESSAGES}}", string(lastMessagesJSON), 1)

	log.Printf("Prompt: %v\n", finalPrompt)
	return finalPrompt
}
