// internal/telegram/telegram.go
package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var bot *tgbotapi.BotAPI

// StartBot initializes the Telegram bot and processes updates.
func StartBot(ctx context.Context, token string, version string, handleNewMember func(ctx context.Context, firstName string, chatID int64)) error {
	slog.Info("Starting Telegram bot", "version", version)
	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}
	slog.Debug("Bot authorized successfully", "username", bot.Self.UserName)

	updates := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))
	slog.Debug("Listening for updates")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		slog.Debug("Processing update", "updateID", update.UpdateID, "chatID", update.Message.Chat.ID)

		if len(update.Message.NewChatMembers) > 0 {
			slog.Debug("New members joined", "membersCount", len(update.Message.NewChatMembers))
			for _, newUser := range update.Message.NewChatMembers {
				handleNewMember(ctx, newUser.FirstName, update.Message.Chat.ID)
			}
		}
	}

	return nil
}

// SendMessage sends a message to a specific chat in Telegram.
func SendMessage(chatID int64, message string) error {
	slog.Debug("Sending message", "chatID", chatID, "message", message)
	msg := tgbotapi.NewMessage(chatID, message)
	msg.DisableWebPagePreview = true
	_, err := bot.Send(msg)
	if err != nil {
		slog.Error("Failed to send message", "chatID", chatID, "error", err)
	}
	return err
}
