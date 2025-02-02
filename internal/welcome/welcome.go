// internal/welcome/welcome.go
package welcome

import (
    "bytes"
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "strings"

    "tgbot/internal/telegram"
)

// HandleNewMember sends a welcome message to a new member.
func HandleNewMembers(ctx context.Context, names []string, chatID int64, ollamaHost, ollamaModel string) {
    slog.Debug("Handling new members", "names", names, "chatID", chatID)

    // Define welcome messages at the beginning
    englishMessage := fmt.Sprintf(
        "Hello %s! Welcome to the PugliaTechs group, the Global Tech Hub of Puglia where we share a passion for business, innovation, and tech.\n\n"+
            "We’re a bilingual group: feel free to write in Italian or English (or use Telegram’s translation feature if needed).\n"+
            "Some useful links:\n"+
            "• Manifesto: https://www.pugliatechs.com/manifesto\n"+
            "• Upcoming events: https://lu.ma/pugliatechs\n"+
            "• LinkedIn: https://www.linkedin.com/company/pugliatechs\n"+
            "• Instagram: https://www.instagram.com/pugliatechs\n"+
            "• YouTube: https://youtube.com/@pugliatechs\n"+
            "• GitHub: https://github.com/pugliatechs\n\n"+
            "Glad to have you on board!\n"+
            "Write a quick intro about yourselves.",
        strings.Join(names, ", "),
    )

    italianMessage := fmt.Sprintf(
        "Ciao %s! Benvenutə nel gruppo PugliaTechs, il Global Tech Hub della Puglia. Condividiamo passione per business, innovazione e tecnologia.\n\n"+
            "Siamo un gruppo bilingue: puoi scrivere in italiano o in inglese (o usare la funzione di traduzione di Telegram se necessario).\n"+
            "Alcuni link utili:\n"+
            "• Manifesto: https://www.pugliatechs.com/manifesto\n"+
            "• Eventi: https://lu.ma/pugliatechs\n"+
            "• LinkedIn: https://www.linkedin.com/company/pugliatechs\n"+
            "• Instagram: https://www.instagram.com/pugliatechs\n"+
            "• YouTube: https://youtube.com/@pugliatechs\n"+
            "• GitHub: https://github.com/pugliatechs\n\n"+
            "Siamo felici di averti con noi!\n"+
            "Scrivi una breve introduzione su di te.",
        strings.Join(names, ", "),
    )

    // Case 1: Multiple members join -> Use English message directly
    if len(names) > 1 {
        if err := telegram.SendMessage(chatID, englishMessage); err != nil {
            slog.Warn("Failed to send welcome message", "error", err, "names", names)
        } else {
            slog.Debug("Welcome message sent successfully", "chatID", chatID, "names", names)
        }
        return
    }

    // Case 2: Single member joins -> Perform classification
    firstName := names[0]
    likelyItalian, err := isItalianName(ctx, firstName, ollamaHost, ollamaModel)
    if err != nil {
        slog.Warn("Failed to classify name", "name", firstName, "error", err)
        likelyItalian = false
    }
    slog.Debug("Name classification result", "name", firstName, "likelyItalian", likelyItalian)

    // Send the appropriate message
    if likelyItalian {
        if err := telegram.SendMessage(chatID, italianMessage); err != nil {
            slog.Warn("Failed to send welcome message", "error", err, "name", firstName)
        } else {
            slog.Debug("Welcome message sent successfully", "chatID", chatID, "name", firstName)
        }
    } else {
        if err := telegram.SendMessage(chatID, englishMessage); err != nil {
            slog.Warn("Failed to send welcome message", "error", err, "name", firstName)
        } else {
            slog.Debug("Welcome message sent successfully", "chatID", chatID, "name", firstName)
        }
    }
}

// isItalianName determines if a name is likely Italian using Ollama API.
func isItalianName(ctx context.Context, firstName, ollamaHost, ollamaModel string) (bool, error) {
    slog.Debug("Checking if name is Italian", "name", firstName)

    // Create the prompt for Ollama
    prompt := fmt.Sprintf(
        "You are a name classifier. I will give you a first name, and you reply with exactly one word: either 'ITALIAN' if this name is likely from an Italian person, or 'FOREIGN' if it is not. The name is: \"%s\".\n\nAnswer:", firstName)

    payload := map[string]interface{}{
        "prompt":      prompt,
        "model":       ollamaModel,
        "temperature": 0.0,
        "top_p":       1.0,
    }

    // Marshal payload
    body, err := json.Marshal(payload)
    if err != nil {
        return false, err
    }

    // Send request to Ollama
    req, err := http.NewRequestWithContext(ctx, "POST", ollamaHost+"/api/generate", bytes.NewBuffer(body))
    if err != nil {
        return false, err
    }
    req.Header.Set("Content-Type", "application/json")
    slog.Debug("Sending request to Ollama", "url", ollamaHost, "payload", string(body))

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()

    // Handle response status code
    if resp.StatusCode != http.StatusOK {
        errBody, _ := io.ReadAll(resp.Body)
        return false, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(errBody))
    }

    // Process response chunks
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

    // Trim and normalize the response text
    fullText := strings.TrimSpace(sb.String())
    fullTextUpper := strings.ToUpper(fullText)

    // Log raw bytes for debugging
    slog.Debug("Ollama classification result (raw bytes)", "name", firstName, "rawBytes", []byte(fullText))
    slog.Debug("Ollama classification result (processed)", "name", firstName, "raw", fullTextUpper)

    // Use "contains" for classification check
    isItalian := strings.Contains(fullTextUpper, "ITALIAN")

    return isItalian, nil
}
