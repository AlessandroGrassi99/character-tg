package main

import (
	"context"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// allowListMiddleware is a middleware that ensures only allowed chats can use the bot
func allowListMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		// Skip middleware checks for non-message updates
		if update.Message == nil {
			next(ctx, b, update)
			return
		}

		chatID := update.Message.Chat.ID

		// If no allow list is configured, allow all chats
		if len(appConfig.AllowedChatIDs) == 0 {
			next(ctx, b, update)
			return
		}

		// Check if the chat ID is in the allowed list
		for _, id := range appConfig.AllowedChatIDs {
			if id == chatID {
				next(ctx, b, update)
				return
			}
		}

		// Log the rejection
		chatName := update.Message.Chat.Title
		if chatName == "" {
			chatName = strings.TrimSpace(update.Message.Chat.FirstName + " " + update.Message.Chat.LastName)
		}
		log.Printf("Rejecting message from unauthorized chat %d (%s)", chatID, chatName)

		// Notify the user that the chat is not authorized
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Sorry, this bot is not available in this chat.",
		})
	}
}