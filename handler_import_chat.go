package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleFileImport processes incoming files and imports chat history
func handlerImportChat(ctx context.Context, b *bot.Bot, update *models.Update) {
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
