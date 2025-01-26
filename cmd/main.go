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

// isItalianName checks if firstName is likely Italian or not via Ollama.
func isItalianName(ctx context.Context, firstName string) (bool, error) {
    slog.Debug("Checking if name is Italian", "name", firstName)

    ollamaHost := os.Getenv("OLLAMA_HOST")
    if ollamaHost == "" {
        ollamaHost = "http://localhost:11411"
    }
    ollamaModel := os.Getenv("OLLAMA_MODEL")
    if ollamaModel == "" {
        ollamaModel = "llama3.2:1b"
    }

    prompt := fmt.Sprintf(
        "You are a name classifier. I will give you a first name, and you reply with exactly one word: either 'ITALIAN' if this name is likely from an Italian person, or 'FOREIGN' if it is not." +
            "The name is: \"%s\".\n\nAnswer:", firstName)

    payload := map[string]interface{}{
        "prompt":      prompt,
        "model":       ollamaModel,
        "temperature": 0.0,
        "top_p":       1.0,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return false, err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", ollamaHost+"/api/generate", bytes.NewBuffer(body))
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

// generateResponseWithOllama answers a question using the manifesto and events.
func generateResponseWithOllama(ctx context.Context, question, manifesto, events string) (string, error) {
    slog.Debug("Generating Ollama response", "question", question)

    ollamaHost := os.Getenv("OLLAMA_HOST")
    if ollamaHost == "" {
        ollamaHost = "http://localhost:11411"
    }
    ollamaModel := os.Getenv("OLLAMA_MODEL")
    if ollamaModel == "" {
        ollamaModel = "llama3.2:1b"
    }

    prompt := fmt.Sprintf(
        `You are an assistant for the PugliaTechs community.
Below is the community manifesto, followed by upcoming events, and then a user's question.
Use only the information from the manifesto and events to answer accurately.
If someone greets you, respond politely.
Reject requests beyond the manifesto or events.

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
        "model":  ollamaModel,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        slog.Error("Failed to marshal request", "error", err)
        return "", err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", ollamaHost+"/api/generate", bytes.NewBuffer(body))
    if err != nil {
        slog.Error("Failed to create Ollama request", "error", err)
        return "", err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        slog.Error("Failed to contact Ollama", "error", err)
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        errBody, _ := io.ReadAll(resp.Body)
        slog.Error("Ollama returned non-OK status", "status", resp.StatusCode, "body", string(errBody))
        return "", fmt.Errorf("ollama returned %d", resp.StatusCode)
    }

    var sb strings.Builder
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        var chunk struct {
            Response string `json:"response"`
            Done     bool   `json:"done"`
        }
        if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
            slog.Error("Failed to unmarshal chunk", "error", err)
            return "", err
        }
        sb.WriteString(chunk.Response)
        if chunk.Done {
            break
        }
    }
    if err := scanner.Err(); err != nil {
        slog.Error("Reading Ollama response failed", "error", err)
        return "", err
    }

    result := sb.String()
    slog.Debug("Final Ollama response", "response", result)
    return result, nil
}

// readManifestoFromFile returns the manifesto text from a local file.
func readManifestoFromFile(filePath string) (string, error) {
    slog.Debug("Reading manifesto file", "path", filePath)

    content, err := os.ReadFile(filePath)
    if err != nil {
        slog.Error("Failed reading manifesto file", "error", err)
        return "", err
    }
    return strings.TrimSpace(string(content)), nil
}

// getScheduledEvents retrieves events data from the Luma API.
func getScheduledEvents(ctx context.Context, url string) (string, error) {
    slog.Debug("Fetching scheduled events", "url", url)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        slog.Error("Failed to get Luma events", "error", err)
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        slog.Error("Unexpected Luma status code", "status", resp.StatusCode)
        return "", fmt.Errorf("status: %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        slog.Error("Reading Luma response failed", "error", err)
        return "", err
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
    if err := json.Unmarshal(body, &response); err != nil {
        slog.Error("Failed to unmarshal Luma JSON", "error", err)
        return "", err
    }
    if len(response.Entries) == 0 {
        slog.Debug("No events found in Luma")
        return "No upcoming events found.", nil
    }

    var sb strings.Builder
    sb.WriteString("Upcoming events:\n")
    for _, entry := range response.Entries {
        e := entry.Event
        sb.WriteString(fmt.Sprintf("- %s at %s, on %s: https://lu.ma/%s\n", e.Name, e.Location, e.StartAt, e.EventLink))
    }
    result := sb.String()
    slog.Debug("Events found", "events", result)
    return result, nil
}

func main() {
    // Configure logging level from LOG_LEVEL env var (debug, info, warn, error).
    level := slog.LevelInfo
    switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
    case "debug":
        level = slog.LevelDebug
    case "warn":
        level = slog.LevelWarn
    case "error":
        level = slog.LevelError
    // default: info
    }

    // Create a global logger with text output.
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
    slog.SetDefault(logger)

    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    if token == "" {
        slog.Error("TELEGRAM_BOT_TOKEN is empty")
        os.Exit(1)
    }
    slog.Info("Starting bot")

    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        slog.Error("Failed to create bot", "error", err)
        os.Exit(1)
    }
    slog.Info("Bot authorized", "username", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)
    slog.Info("Listening for updates")

    manifesto, err := readManifestoFromFile("manifesto.txt")
    if err != nil {
        slog.Error("Could not read manifesto", "error", err)
        os.Exit(1)
    }

    lumaURL := "https://api.lu.ma/calendar/get-items?calendar_api_id=cal-slXbDWpGDzDpbwS&period=future&pagination_limit=20"
    ctx := context.Background()


    for update := range updates {
        if update.Message == nil {
            continue
        }

        // Log the received message for debugging
        slog.Debug("Received message",
            "userID", update.Message.From.ID,
            "username", update.Message.From.UserName,
            "text", update.Message.Text,
            "chatID", update.Message.Chat.ID,
            "msgID", update.Message.MessageID,
            "isPrivate", update.Message.Chat.IsPrivate(),
        )

        // Skip private messages (direct messages)
        if update.Message.Chat.IsPrivate() {
            slog.Info("Ignoring direct message", "userID", update.Message.From.ID, "username", update.Message.From.UserName)
            continue
        }

        // Greet new members
        if len(update.Message.NewChatMembers) > 0 && update.Message.From.ID != bot.Self.ID {
            for _, newUser := range update.Message.NewChatMembers {
                likelyItalian, err := isItalianName(ctx, newUser.FirstName)
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

            events, err := getScheduledEvents(ctx, lumaURL)
            if err != nil {
                slog.Warn("Failed to fetch events", "error", err)
                events = "No upcoming events found."
            }

            slog.Debug("Generating response for user", "user", update.Message.From.UserName)
            response, err := generateResponseWithOllama(ctx, cleanedText, manifesto, events)
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
