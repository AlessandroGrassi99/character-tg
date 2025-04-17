package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// NewMessageResponse represents the JSON structure returned by the AI model
type NewMessageResponse struct {
	ConversationAnalysis string `json:"conversation_analysis"`
	ResponseMessage      string `json:"response_message"`
}

// handlerNewMessage processes incoming text messages and replies using the AI model
func handlerNewMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	if update.Message.Text == "" {
		log.Printf("Ignoring non-text message in chat %d", chatID)
		return
	}

	chatStorage.StoreMessage(chatID, *update.Message)
	prompt := buildChatMessages(ctx, b, chatID, 1000)

	req := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model:           "grok-3-mini-beta",
		ReasoningEffort: "low",
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		},
	}

	resp, err := grokClient.Chat.Completions.New(ctx, req)
	if err != nil {
		log.Printf("Error calling llm model")
		return
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		log.Printf("Llm client returned empty response")
		return
	}

	var result NewMessageResponse
	raw := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		fallback := fmt.Sprintf("Sorry, I had trouble formatting my response. Raw output: %s", raw)
		if len(fallback) > 4000 {
			fallback = fallback[:4000] + "..."
		}
		sendChatMessage(ctx, b, chatID, fallback)
		return
	}

	if msg := strings.TrimSpace(result.ResponseMessage); msg != "" {
		sendChatMessage(ctx, b, chatID, msg)
	}
}

// buildChatMessages constructs the prompt using stored chat history and templates
func buildChatMessages(ctx context.Context, b *bot.Bot, chatID int64, limit int) string {
	state, ok := chatStorage.GetChatState(chatID)
	if !ok {
		log.Printf("Chat state not found for %d", chatID)
		return `{"error":"state missing","response_preparation":"","response_message":""}`
	}

	prompt := promptChatMessage
	prompt = strings.Replace(prompt, "{{PROMPT}}", state.Prompt, 1)
	prompt = strings.Replace(prompt, "{{OVERVIEW}}", state.Summary, 1)

	me, err := b.GetMe(ctx)
	botName := "@Bot"
	if err == nil {
		botName = "@" + me.Username
	} else {
		log.Printf("Error getting bot info: %v", err)
	}
	prompt = strings.ReplaceAll(prompt, "{{BOT_NAME}}", botName)

	total := len(state.Messages)
	start := max(total-limit, 0)
	last := max(start, total-20)

	if data, err := json.Marshal(state.Messages[last:]); err == nil {
		prompt = strings.Replace(prompt, "{{LAST_MESSAGES}}", string(data), 1)
	} else {
		log.Printf("Error marshaling last messages: %v", err)
		prompt = strings.Replace(prompt, "{{LAST_MESSAGES}}", "[]", 1)
	}

	if data, err := json.Marshal(state.Messages[start:last]); err == nil {
		prompt = strings.Replace(prompt, "{{CHAT_HISTORY}}", string(data), 1)
	} else {
		log.Printf("Error marshaling history messages: %v", err)
		prompt = strings.Replace(prompt, "{{CHAT_HISTORY}}", "[]", 1)
	}

	log.Printf("Final prompt: %.200s...", prompt)
	return prompt
}

// sendChatMessage sends a message and stores it in chat history
func sendChatMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	params := &bot.SendMessageParams{ChatID: chatID, Text: text}
	if msg, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("Error sending message: %v", err)
	} else {
		chatStorage.StoreMessage(chatID, *msg)
	}
}
