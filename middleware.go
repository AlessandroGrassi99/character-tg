package main

import (
	"context"
	"log"
	"math/rand"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// storeMessageMiddleware stores all incoming messages in the chat history
func storeMessageMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		// Store message if present
		if update.Message != nil {
			chatID := update.Message.Chat.ID
			chatStorage.StoreMessage(chatID, *update.Message)
		}
		
		// Continue to next middleware/handler
		next(ctx, b, update)
	}
}

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

// randomReplyMiddleware decides whether to process a message in group chats based on probability
// Note: Messages are already stored by storeMessageMiddleware before reaching this middleware
func randomReplyMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		// Skip middleware checks for non-message updates
		if update.Message == nil {
			next(ctx, b, update)
			return
		}

		// Always process messages in private chats
		if update.Message.Chat.Type == "private" {
			next(ctx, b, update)
			return
		}

		// Always process command messages
		if update.Message.Text != "" && strings.HasPrefix(update.Message.Text, "/") {
			next(ctx, b, update)
			return
		}

		// Always process messages that explicitly mention the bot
		me, err := b.GetMe(ctx)
		if err == nil && update.Message.Text != "" {
			botName := "@" + me.Username
			if strings.Contains(update.Message.Text, botName) || 
			   (update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.ID == me.ID) {
				next(ctx, b, update)
				return
			}
		}

		// Use probability for other messages in group chats
		if rand.Float64() <= appConfig.GroupReplyProbability {
			next(ctx, b, update)
			return
		}

		// If we decide not to reply, just log it and return
		// The message has already been stored by storeMessageMiddleware
		log.Printf("Randomly skipping reply to message in group chat %d", update.Message.Chat.ID)
	}
}