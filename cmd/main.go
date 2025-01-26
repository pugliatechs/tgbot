package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Config holds configuration values
type Config struct {
	TelegramToken string
	OllamaHost    string
	OllamaModel   string
	LumaURL       string
	LogLevel      string
}

// loadConfig initializes and validates environment variables.
func loadConfig() (*Config, error) {
	cfg := &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		OllamaHost:    os.Getenv("OLLAMA_HOST"),
		OllamaModel:   os.Getenv("OLLAMA_MODEL"),
		LumaURL:       "https://api.lu.ma/calendar/get-items?calendar_api_id=cal-slXbDWpGDzDpbwS&period=future&pagination_limit=20",
		LogLevel:      os.Getenv("LOG_LEVEL"),
	}

	// Set defaults
	if cfg.OllamaHost == "" {
		cfg.OllamaHost = "http://localhost:11411"
	}
	if cfg.OllamaModel == "" {
		cfg.OllamaModel = "llama3.2:1b"
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
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

// isItalianName checks if firstName is likely Italian or not via Ollama.
func isItalianName(ctx context.Context, firstName string, cfg *Config) (bool, error) {
	slog.Debug("Checking if name is Italian", "name", firstName)

    prompt := fmt.Sprintf(
        "You are a name classifier. I will give you a first name, and you reply with exactly one word: either 'ITALIAN' if this name is likely from an Italian person, or 'FOREIGN' if it is not." +
            "The name is: \"%s\".\n\nAnswer:", firstName)

    payload := map[string]interface{}{
        "prompt":      prompt,
        "model":       cfg.OllamaModel,
        "temperature": 0.0,
        "top_p":       1.0,
    }

	if cfg.LogLevel == "debug" {
		slog.Debug("Sending request to Ollama", "payload", payload)
	}

    body, err := json.Marshal(payload)
    if err != nil {
        return false, err
    }

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.OllamaHost+"/api/generate", bytes.NewBuffer(body))
    if err != nil {
        return false, err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        errBody, _ := io.ReadAll(resp.Body)
        return false, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(errBody))
    }

    var sb strings.Builder
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        var chunk struct {
            Response string `json:"response"`
            Done     bool   `json:"done"`
        }
        if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
            return false, err
        }
        sb.WriteString(chunk.Response)
        if chunk.Done {
            break
        }
    }
    if err := scanner.Err(); err != nil {
        return false, err
    }

    fullText := strings.ToUpper(strings.TrimSpace(sb.String()))
    slog.Debug("Ollama classification result", "name", firstName, "raw", fullText)

    return fullText == "ITALIAN", nil
}

// generateResponseWithOllama generates a response using Ollama.
func generateResponseWithOllama(ctx context.Context, question, manifesto, events string, cfg *Config) (string, error) {
	slog.Debug("Generating Ollama response", "question", question)

	prompt := fmt.Sprintf(
		`You are an assistant for the PugliaTechs community.
Below is the community manifesto, followed by upcoming events, and then a user's question.
Use only the information from the manifesto and events to answer accurately.

Manifesto:
%s

Upcoming Events:
%s

Question:
%s`,
		manifesto, events, question,
	)

	payload := map[string]interface{}{
		"prompt": prompt,
		"model":  cfg.OllamaModel,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.OllamaHost+"/api/generate", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(errBody))
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return "", err
		}
		sb.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	result := sb.String()
	slog.Debug("Final Ollama response", "response", result)
	return result, nil
}

func getScheduledEvents(ctx context.Context, cfg *Config) (string, error) {
	slog.Debug("Fetching scheduled events", "url", cfg.LumaURL)

	req, err := http.NewRequestWithContext(ctx, "GET", cfg.LumaURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Luma API returned %d: %s", resp.StatusCode, string(errBody))
	}

	var response struct {
		Entries []struct {
			Event struct {
				Name      string `json:"name"`
				StartAt   string `json:"start_at"`
				Location  string `json:"geo_address_info.full_address"`
				EventLink string `json:"url"`
			} `json:"event"`
		} `json:"entries"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	if len(response.Entries) == 0 {
		return "No upcoming events found.", nil
	}

	var sb strings.Builder
	sb.WriteString("Upcoming events:\n")
	for _, entry := range response.Entries {
		event := entry.Event
		sb.WriteString(fmt.Sprintf("- %s at %s, on %s: %s\n", event.Name, event.Location, event.StartAt, event.EventLink))
	}

	return sb.String(), nil
}

// readManifestoFromFile reads the manifesto from a file.
func readManifestoFromFile(filePath string) (string, error) {
	slog.Debug("Reading manifesto file", "path", filePath)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func main() {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Failed to load configuration:", err)
		os.Exit(1)
	}

	// Configure logging
	configureLogger(cfg.LogLevel)

	// Initialize Telegram Bot
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		slog.Error("Failed to create bot", "error", err)
		os.Exit(1)
	}
	slog.Info("Bot authorized", "username", bot.Self.UserName)

	// Read Manifesto
	manifesto, err := readManifestoFromFile("manifesto.txt")
	if err != nil {
		slog.Error("Could not read manifesto", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	updates := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Ignore direct messages
		if update.Message.Chat.IsPrivate() {
			slog.Info("Ignoring direct message", "userID", update.Message.From.ID)
			continue
		}

        // Greet new members
        if len(update.Message.NewChatMembers) > 0 && update.Message.From.ID != bot.Self.ID {
            for _, newUser := range update.Message.NewChatMembers {
                likelyItalian, err := isItalianName(ctx, newUser.FirstName, cfg)
                if err != nil {
                    slog.Warn("Failed to classify name", "name", newUser.FirstName, "error", err)
                    likelyItalian = false
                }

                var welcomeMessage string
                if likelyItalian {
                    welcomeMessage = fmt.Sprintf(
                        "Ciao %s! Benvenutə nel gruppo PugliaTechs, il Global Tech Hub della Puglia. Condividiamo passione per business, innovazione e tecnologia.\n\n"+
                            "Alcuni link utili:\n"+
                            "• Manifesto: https://www.pugliatechs.com/manifesto\n"+
                            "• Eventi: https://lu.ma/pugliatechs\n"+
                            "• LinkedIn: https://www.linkedin.com/company/pugliatechs\n"+
                            "• Instagram: https://www.instagram.com/pugliatechs\n"+
                            "• YouTube: https://youtube.com/@pugliatechs\n\n"+
                            "Siamo felici di averti con noi!\n"+
                            "Scrivi una breve introduzione su di te.",
                        newUser.FirstName,
                    )
                } else {
                    welcomeMessage = fmt.Sprintf(
                        "Hello %s! Welcome to the PugliaTechs group, the Global Tech Hub of Puglia where we share a passion for business, innovation, and tech.\n\n"+
                            "Some useful links:\n"+
                            "• Manifesto: https://www.pugliatechs.com/manifesto\n"+
                            "• Upcoming events: https://lu.ma/pugliatechs\n"+
                            "• LinkedIn: https://www.linkedin.com/company/pugliatechs\n"+
                            "• Instagram: https://www.instagram.com/pugliatechs\n"+
                            "• YouTube: https://youtube.com/@pugliatechs\n\n"+
                            "Glad to have you on board!\n"+
                            "Write a quick intro about yourself.",
                        newUser.FirstName,
                    )
                }

                slog.Debug("Sending welcome", "likelyItalian", likelyItalian, "message", welcomeMessage)
                msg := tgbotapi.NewMessage(update.Message.Chat.ID, welcomeMessage)
                msg.DisableWebPagePreview = true
                if _, err := bot.Send(msg); err != nil {
                    slog.Warn("Failed to send welcome", "error", err, "user", newUser.FirstName)
                }
            }
            continue
        }

        // Respond only if the bot is mentioned in a group/supergroup
        if strings.Contains(update.Message.Text, fmt.Sprintf("@%s", bot.Self.UserName)) {
            cleanedText := strings.ReplaceAll(update.Message.Text, fmt.Sprintf("@%s", bot.Self.UserName), "")

			events, err := getScheduledEvents(ctx, cfg)
			if err != nil {
				slog.Warn("Failed to fetch events", "error", err)
				events = "No upcoming events found."
			}

            slog.Debug("Generating response for user", "user", update.Message.From.UserName)
            response, err := generateResponseWithOllama(ctx, cleanedText, manifesto, events, cfg)
            if err != nil {
                slog.Warn("Failed to generate response", "error", err)
                continue
            }

            msg := tgbotapi.NewMessage(update.Message.Chat.ID, response)
            msg.ReplyToMessageID = update.Message.MessageID
            msg.DisableWebPagePreview = true

            if _, err := bot.Send(msg); err != nil {
                slog.Warn("Failed to send response", "error", err, "user", update.Message.From.UserName)
            }
        }
    }
}
