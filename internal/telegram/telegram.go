// internal/telegram/telegram.go
package telegram

import (
    "context"
    "fmt"
    "log/slog"
    "sync"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
    bot          *tgbotapi.BotAPI
    connected    bool
    statusMutex  sync.RWMutex // Protects the `connected` variable
)

// StartBot initializes the Telegram bot and processes updates.
func StartBot(ctx context.Context, token string, version string, handleNewMembers func(ctx context.Context, names []string, chatID int64)) error {
    slog.Info("Starting Telegram bot", "version", version)
    var err error
    bot, err = tgbotapi.NewBotAPI(token)
    if err != nil {
        setConnectionStatus(false)
        return fmt.Errorf("failed to create Telegram bot: %w", err)
    }
    setConnectionStatus(true)
    slog.Debug("Bot authorized successfully", "username", bot.Self.UserName)

    go func() {
        updates := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))
        slog.Debug("Listening for updates")
        for update := range updates {
            if update.Message == nil {
                continue
            }

            // Process new chat members in a batch
            if len(update.Message.NewChatMembers) > 0 {
                var newMembers []string
                for _, newUser := range update.Message.NewChatMembers {
                    // Skip processing if the bot is the one joining
                    if newUser.ID == bot.Self.ID {
                        slog.Debug("Bot joined a group, ignoring", "chatID", update.Message.Chat.ID)
                        continue
                    }

                    newMembers = append(newMembers, newUser.FirstName)
                }

                // Send a single welcome message for all new members
                if len(newMembers) > 0 {
                     handleNewMembers(ctx, newMembers, update.Message.Chat.ID)
                }
            }
        }
    }()

    return nil
}

// setConnectionStatus updates the connected status of the bot.
func setConnectionStatus(status bool) {
    statusMutex.Lock()
    defer statusMutex.Unlock()
    connected = status
}

// IsConnected returns the current connection status of the bot.
func IsConnected() bool {
    statusMutex.RLock()
    defer statusMutex.RUnlock()
    return connected
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
