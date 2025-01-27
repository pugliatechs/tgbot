// main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"tgbot/internal/telegram"
	"tgbot/internal/welcome"
)

// Config holds configuration values
type Config struct {
    TelegramToken string
    OllamaHost    string
    OllamaModel   string
    LumaURL       string
    LogLevel      string
    HttpPort      string
    Version       string
}

// loadConfig initializes and validates environment variables.
func loadConfig() (*Config, error) {
    cfg := &Config{
        TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
        OllamaHost:    os.Getenv("OLLAMA_HOST"),
        OllamaModel:   os.Getenv("OLLAMA_MODEL"),
        LumaURL:       "https://api.lu.ma/calendar/get-items?calendar_api_id=cal-slXbDWpGDzDpbwS&period=future&pagination_limit=20",
        LogLevel:      os.Getenv("LOG_LEVEL"),
        HttpPort:      os.Getenv("HTTP_PORT"),
        Version:       "1.0.2",
	}

	if cfg.OllamaHost == "" {
		cfg.OllamaHost = "http://localhost:11411"
	}
	if cfg.OllamaModel == "" {
		cfg.OllamaModel = "llama3.2:1b"
	}
	if cfg.TelegramToken == "" {
		slog.Error("TELEGRAM_BOT_TOKEN is required")
		return nil, nil
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	return cfg, nil
}

// configureLogger sets up the logger based on the log level from the config.
func configureLogger(level string) {
	logLevel := slog.LevelInfo
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
}

func main() {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	slog.Debug("Configuration loaded successfully", "config", cfg)

	// Configure logging
	configureLogger(cfg.LogLevel)
	slog.Debug("Logger configured", "level", cfg.LogLevel)

	// Initialize Telegram Bot
	ctx := context.Background()
	go func() {
		err = telegram.StartBot(ctx, cfg.TelegramToken, cfg.Version, func(ctx context.Context, firstName string, chatID int64) {
			slog.Debug("New member detected", "firstName", firstName, "chatID", chatID)
			welcome.HandleNewMember(ctx, firstName, chatID, cfg.OllamaHost, cfg.OllamaModel)
		})
		if err != nil {
			slog.Error("Failed to start Telegram bot", "error", err)
			os.Exit(1)
		}
	}()

	// Start HTTP server for health endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if telegram.IsConnected() {
			slog.Debug("Health check passed")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			slog.Warn("Health check failed: Telegram bot is disconnected")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("ERROR: Telegram bot is disconnected"))
		}
	})

	port := cfg.HttpPort
	slog.Info("Starting HTTP server", "port", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		slog.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}
}

