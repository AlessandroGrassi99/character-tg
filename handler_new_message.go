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
)

func handlerNewMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID

	if update.Message.Text == "" { // Ignore non-text messages for history/counting
		log.Printf("Ignoring non-text message (e.g., sticker, photo) in chat %d", chatID)
		return
	}

	// Store the full message object
	chatStorage.StoreMessage(1171388254, *update.Message)

	resp, err := grokClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(buildChatMessages(ctx, b, 1171388254, 1000)),
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

func buildChatMessages(ctx context.Context, b *bot.Bot, chatID int64, limit int) string {
	chatState, ok := chatStorage.GetChatState(chatID)
	if !ok {
		return "Just reply: No prompt provided"
	}

	finalPrompt := promptChatMessage
	finalPrompt = strings.Replace(finalPrompt, "{{PROMPT}}", chatState.Prompt, 1)
	finalPrompt = strings.Replace(finalPrompt, "{{OVERVIEW}}", chatState.Summary, 1)

	me, err := b.GetMe(ctx)
	if err != nil {
		log.Printf("error getting information about the bot: %v", err)
	} else {
		botName := fmt.Sprintf("%s %s: @%s", me.FirstName, me.LastName, me.Username)
		finalPrompt = strings.ReplaceAll(finalPrompt, "{{BOT_NAME}}", botName)
	}

	// Last 20 messages for {{LAST_MESSAGES}}
	startIndex := max(len(chatState.Messages)-limit, 0)
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
