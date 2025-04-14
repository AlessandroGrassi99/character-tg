package main

import (
	"encoding/json"
	"log"
	"os"
)

// ChatExport define the chat history structure of the json-format file from a telegram client.
type ChatExport struct {
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	ID       int64               `json:"id"` // Using int64 for potentially large IDs
	Messages []ChatExportMessage `json:"messages"`
}

// ChatExportMessage represents a single message object within the chat export.
type ChatExportMessage struct {
	ID                int          `json:"id"`
	Type              string       `json:"type"`
	Date              string       `json:"date"`                 // Could parse to time.Time if needed
	DateUnixtime      int64        `json:"date_unixtime,string"` // JSON value is string, parse as number
	From              string       `json:"from,omitempty"`       // Use omitempty as it might be missing in some system messages
	FromID            string       `json:"from_id,omitempty"`    // Use omitempty
	Text              any          `json:"text"`                 // Can be string or []TextEntity, use interface{} for flexibility
	TextEntities      []TextEntity `json:"text_entities"`
	File              string       `json:"file,omitempty"`
	FileSize          int64        `json:"file_size,omitempty"` // Using int64 for potentially large sizes
	Thumbnail         string       `json:"thumbnail,omitempty"`
	ThumbnailFileSize int          `json:"thumbnail_file_size,omitempty"`
	MediaType         string       `json:"media_type,omitempty"`
	MimeType          string       `json:"mime_type,omitempty"`
	DurationSeconds   int          `json:"duration_seconds,omitempty"`
	Width             int          `json:"width,omitempty"`
	Height            int          `json:"height,omitempty"`
	FileName          string       `json:"file_name,omitempty"`
	Photo             string       `json:"photo,omitempty"`
	PhotoFileSize     int          `json:"photo_file_size,omitempty"`
	ReplyToMessageID  int          `json:"reply_to_message_id,omitempty"`
	Edited            string       `json:"edited,omitempty"`                 // Could parse to time.Time
	EditedUnixtime    int64        `json:"edited_unixtime,string,omitempty"` // JSON value is string, parse as number
	ForwardedFrom     string       `json:"forwarded_from,omitempty"`
	Reactions         []Reaction   `json:"reactions,omitempty"`
}

// TextEntity represents formatting or special entities within the message text.
type TextEntity struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Href string `json:"href,omitempty"` // For text_link type
}

// Reaction represents an emoji reaction to a message.
type Reaction struct {
	Type   string          `json:"type"` // e.g., "emoji"
	Count  int             `json:"count"`
	Emoji  string          `json:"emoji"`
	Recent []RecentReactor `json:"recent"`
}

// RecentReactor represents a user who recently added a specific reaction.
type RecentReactor struct {
	From   string `json:"from"`
	FromID string `json:"from_id"`
	Date   string `json:"date"` // Could parse to time.Time
}

func NewChatExport(filePath string) (ChatExport, error) {
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("error reading JSON file %s: %v", filePath, err)
		return ChatExport{}, err
	}

	var chatData ChatExport

	err = json.Unmarshal(jsonData, &chatData)
	if err != nil {
		log.Printf("error unmarshaling JSON: %v", err)
		return ChatExport{}, err
	}

	return chatData, nil
}
