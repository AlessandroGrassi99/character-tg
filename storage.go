package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/redis/go-redis/v9"
)

type ChatState struct {
	Messages []ChatMessage
	Prompt   string
	Summary  string
}

// TextEntityRef represents a formatted text entity (e.g., bold, link)
type TextEntityRef struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Href string `json:"href,omitempty"`
}

type ChatMessage struct {
	// Message metadata
	ID       int    `json:"id"`
	FromUser string `json:"from_user,omitempty"`
	FromID   int64  `json:"from_id,omitempty"`
	Date     int64  `json:"date"`
	EditDate int64  `json:"edit_date,omitempty"`

	// Content
	Text string `json:"text,omitempty"`

	// Reply information
	ReplyToID int `json:"reply_to_id,omitempty"`

	// Media types
	MediaType string `json:"media_type,omitempty"`
	File      string `json:"file,omitempty"`
	Caption   string `json:"caption,omitempty"`

	// Original message references
	IsFromBot bool `json:"is_from_bot,omitempty"`

	// Preserve original formats for export compatibility
	OriginalEntities []TextEntityRef `json:"entities,omitempty"`
}

type ChatStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewChatStorage(config Config) *ChatStorage {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       0,
	})

	return &ChatStorage{
		client: rdb,
		ctx:    context.Background(),
	}
}

const defaultPrompt = ""

// Chat keys in Redis
func (cs *ChatStorage) getChatKey(chatID int64) string {
	return fmt.Sprintf("chat:%d", chatID)
}

func (cs *ChatStorage) getPromptKey(chatID int64) string {
	return fmt.Sprintf("chat:%d:prompt", chatID)
}

func (cs *ChatStorage) getSummaryKey(chatID int64) string {
	return fmt.Sprintf("chat:%d:summary", chatID)
}

// FromTelegramMessage converts a Telegram models.Message to our internal ChatMessage
func FromTelegramMessage(msg models.Message) ChatMessage {
	chatMsg := ChatMessage{
		ID:        msg.ID,
		Date:      int64(msg.Date),
		EditDate:  int64(msg.EditDate),
		Text:      msg.Text,
		IsFromBot: msg.From != nil && msg.From.IsBot,
	}

	// Add sender information if available
	if msg.From != nil {
		name := msg.From.FirstName
		if msg.From.LastName != "" {
			name += " " + msg.From.LastName
		}
		chatMsg.FromUser = name
		chatMsg.FromID = msg.From.ID
	}

	// Handle reply
	if msg.ReplyToMessage != nil {
		chatMsg.ReplyToID = msg.ReplyToMessage.ID
	}

	// Handle caption for media messages
	if msg.Caption != "" {
		chatMsg.Caption = msg.Caption
	}

	// Handle media types
	if len(msg.Photo) > 0 {
		chatMsg.MediaType = "photo"
	} else if msg.Video != nil {
		chatMsg.MediaType = "video"
	} else if msg.Audio != nil {
		chatMsg.MediaType = "audio"
	} else if msg.Document != nil {
		chatMsg.MediaType = "document"
		if msg.Document.FileName != "" {
			chatMsg.File = msg.Document.FileName
		}
	} else if msg.Sticker != nil {
		chatMsg.MediaType = "sticker"
	}

	// Handle text entities
	if len(msg.Entities) > 0 {
		chatMsg.OriginalEntities = make([]TextEntityRef, 0, len(msg.Entities))
		for _, entity := range msg.Entities {
			entityRef := TextEntityRef{
				Type: string(entity.Type),
			}

			// Extract text for the entity
			if len(msg.Text) >= entity.Offset+entity.Length {
				entityRef.Text = msg.Text[entity.Offset : entity.Offset+entity.Length]
			}

			// Handle URLs
			if entity.Type == "text_link" {
				entityRef.Href = entity.URL
			}

			chatMsg.OriginalEntities = append(chatMsg.OriginalEntities, entityRef)
		}
	}

	return chatMsg
}

// FromExportMessage converts a ChatExportMessage to our internal ChatMessage
func FromExportMessage(msg ChatExportMessage) ChatMessage {
	chatMsg := ChatMessage{
		ID:        msg.ID,
		FromUser:  msg.From,
		Text:      getTextContent(msg.Text),
		Date:      msg.DateUnixtime,
		ReplyToID: msg.ReplyToMessageID,
	}

	// Extract FromID from the format "user123456"
	if len(msg.FromID) > 4 {
		// Try to extract numeric part after "user" prefix
		numStr := msg.FromID[4:]
		// Don't handle conversion error, zero value is fine if conversion fails
		fromID, _ := parseInt64(numStr)
		chatMsg.FromID = fromID
	}

	// Handle edited
	if msg.EditedUnixtime > 0 {
		chatMsg.EditDate = msg.EditedUnixtime
	}

	// Handle media types
	if msg.MediaType != "" {
		chatMsg.MediaType = msg.MediaType
		if msg.FileName != "" {
			chatMsg.File = msg.FileName
		}
	}

	// Convert text entities
	if len(msg.TextEntities) > 0 {
		chatMsg.OriginalEntities = make([]TextEntityRef, 0, len(msg.TextEntities))
		for _, entity := range msg.TextEntities {
			entityRef := TextEntityRef{
				Type: entity.Type,
				Text: entity.Text,
				Href: entity.Href,
			}
			chatMsg.OriginalEntities = append(chatMsg.OriginalEntities, entityRef)
		}
	}

	return chatMsg
}

// Helper to extract text content from the flexible text field
func getTextContent(text any) string {
	if text == nil {
		return ""
	}

	// Try as string first
	if strText, ok := text.(string); ok {
		return strText
	}

	// Try to extract from array of text entities
	if entities, ok := text.([]any); ok {
		var result string
		for _, entity := range entities {
			if entityMap, ok := entity.(map[string]any); ok {
				if textStr, ok := entityMap["text"].(string); ok {
					result += textStr
				}
			}
		}
		return result
	}

	return ""
}

// Helper function to parse int64 from string
func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// ConvertExportMessages converts an array of ChatExportMessage to an array of ChatMessage
func ConvertExportMessages(exportMessages []ChatExportMessage) []ChatMessage {
	result := make([]ChatMessage, 0, len(exportMessages))
	for _, message := range exportMessages {
		result = append(result, FromExportMessage(message))
	}
	return result
}

func (cs *ChatStorage) ImportChat(chatID int64, messages []ChatMessage) error {
	// Sort messages by ID before storing
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].ID < messages[j].ID
	})

	// Serialize messages to JSON
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	// Store in Redis with a transaction
	pipe := cs.client.Pipeline()
	pipe.Set(cs.ctx, cs.getChatKey(chatID), messagesJSON, 0)
	pipe.Set(cs.ctx, cs.getPromptKey(chatID), defaultPrompt, 0)
	pipe.Set(cs.ctx, cs.getSummaryKey(chatID), "", 0)
	_, err = pipe.Exec(cs.ctx)
	if err != nil {
		return fmt.Errorf("failed to store chat in Redis: %w", err)
	}

	return nil
}

func (cs *ChatStorage) StoreMessage(chatID int64, message models.Message) error {
	// Convert Telegram message to our ChatMessage format
	chatMessage := FromTelegramMessage(message)

	// Get existing messages
	chatState, found, err := cs.getChatStateInternal(chatID)
	if err != nil {
		return fmt.Errorf("failed to retrieve chat state: %w", err)
	}

	if !found {
		chatState = ChatState{
			Messages: []ChatMessage{},
			Prompt:   defaultPrompt,
			Summary:  "",
		}
	}

	// Add the new message
	chatState.Messages = append(chatState.Messages, chatMessage)

	// Sort messages by ID to maintain chronological order
	sort.Slice(chatState.Messages, func(i, j int) bool {
		return chatState.Messages[i].ID < chatState.Messages[j].ID
	})

	// Serialize and store messages
	messagesJSON, err := json.Marshal(chatState.Messages)
	if err != nil {
		return fmt.Errorf("failed to marshal messages: %w", err)
	}

	return cs.client.Set(cs.ctx, cs.getChatKey(chatID), messagesJSON, 0).Err()
}

// Internal method to get chat state
func (cs *ChatStorage) getChatStateInternal(chatID int64) (ChatState, bool, error) {
	pipe := cs.client.Pipeline()

	// Get all components of the chat state in one round trip
	messagesCmd := pipe.Get(cs.ctx, cs.getChatKey(chatID))
	promptCmd := pipe.Get(cs.ctx, cs.getPromptKey(chatID))
	summaryCmd := pipe.Get(cs.ctx, cs.getSummaryKey(chatID))

	_, err := pipe.Exec(cs.ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return ChatState{}, false, fmt.Errorf("failed to execute Redis pipeline: %w", err)
	}

	// Initialize chat state
	chatState := ChatState{
		Messages: []ChatMessage{},
		Prompt:   defaultPrompt,
		Summary:  "",
	}

	// Check if at least messages exist
	messagesJSON, err := messagesCmd.Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return chatState, false, nil
		}
		return chatState, false, fmt.Errorf("failed to get messages: %w", err)
	}

	// Unmarshal messages
	if err := json.Unmarshal([]byte(messagesJSON), &chatState.Messages); err != nil {
		return chatState, false, fmt.Errorf("failed to unmarshal messages: %w", err)
	}

	// Get prompt if available
	promptStr, err := promptCmd.Result()
	if err == nil {
		chatState.Prompt = promptStr
	}

	// Get summary if available
	summaryStr, err := summaryCmd.Result()
	if err == nil {
		chatState.Summary = summaryStr
	}

	return chatState, true, nil
}

func (cs *ChatStorage) GetChatState(chatID int64) (ChatState, bool) {
	chatState, found, err := cs.getChatStateInternal(chatID)
	if err != nil {
		// Log error but return empty state
		fmt.Printf("Error getting chat state: %v\n", err)
		return ChatState{}, false
	}

	return chatState, found
}

func (cs *ChatStorage) SetPrompt(chatID int64, prompt string) error {
	return cs.client.Set(cs.ctx, cs.getPromptKey(chatID), prompt, 0).Err()
}

func (cs *ChatStorage) SetSummary(chatID int64, summary string) error {
	return cs.client.Set(cs.ctx, cs.getSummaryKey(chatID), summary, 0).Err()
}

// Check if Redis connection is healthy
func (cs *ChatStorage) Ping() error {
	ctx, cancel := context.WithTimeout(cs.ctx, 5*time.Second)
	defer cancel()

	return cs.client.Ping(ctx).Err()
}

// Clean up resources
func (cs *ChatStorage) Close() error {
	return cs.client.Close()
}
