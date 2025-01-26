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
func HandleNewMember(ctx context.Context, firstName string, chatID int64, ollamaHost, ollamaModel string) {
	slog.Debug("Handling new member", "firstName", firstName, "chatID", chatID)
	likelyItalian, err := isItalianName(ctx, firstName, ollamaHost, ollamaModel)
	if err != nil {
		slog.Warn("Failed to classify name", "name", firstName, "error", err)
		likelyItalian = false
	} else {
		slog.Debug("Name classification result", "firstName", firstName, "likelyItalian", likelyItalian)
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
			firstName,
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
			firstName,
		)
	}

	if err := telegram.SendMessage(chatID, welcomeMessage); err != nil {
		slog.Warn("Failed to send welcome message", "error", err, "user", firstName)
	} else {
		slog.Debug("Welcome message sent successfully", "chatID", chatID, "firstName", firstName)
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
