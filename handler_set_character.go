package main

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func handlerSetCharacter(ctx context.Context, b *bot.Bot, update *models.Update) {
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

	chatStorage.SetPrompt(update.Message.Chat.ID, commandArgs)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Character prompt has been set. The bot will respond according to this character description.",
	})
}
