// internal/ollama/ollama.go
package ollama

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
)

// GenerateResponse interacts with Ollama to generate a response based on the input.
func GenerateResponse(ctx context.Context, input, ollamaHost, ollamaModel string) (string, error) {
	slog.Debug("Generating response with Ollama", "input", input, "model", ollamaModel)
	prompt := fmt.Sprintf("You are an assistant. Respond to: %s", input)

	payload := map[string]interface{}{
		"prompt": prompt,
		"model":  ollamaModel,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaHost+"/api/generate", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	slog.Debug("Sending request to Ollama", "url", ollamaHost, "payload", string(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		slog.Warn("Ollama API error", "statusCode", resp.StatusCode, "errorBody", string(errBody))
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(errBody))
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	response := sb.String()

	// Log the response in debug mode
	slog.Debug("Ollama response received", "response", response)
	return response, nil
}


