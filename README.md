# PugliaTechs Telegram Bot

PugliaTechs Telegram Bot is a group assistant designed to welcome new members, classify names using Ollama AI, and ensure group management is smooth and efficient. The bot also includes a health endpoint for monitoring its status.

---

## Features

- **Welcome Messages**: Sends personalized welcome messages to new members, available in Italian or English, based on name classification.
- **Name Classification**: Uses the Ollama API to determine if a name is likely Italian.
- **Health Endpoint**: Exposes an HTTP endpoint to monitor the bot's health and Telegram connectivity.
- **Robust Logging**: Includes debug-level logging for tracing issues and understanding bot behavior.

---

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/pugliatechs/tgbot.git
   cd tgbot
   ```

2. Set up your environment variables:

   ```bash
   export TELEGRAM_BOT_TOKEN=your-telegram-bot-token
   export OLLAMA_HOST=http://localhost:11411
   export OLLAMA_MODEL=llama3.2:1b
   export LOG_LEVEL=debug
   export HTTP_PORT=8080
   ```

3. Install dependencies:

   ```bash
   go mod tidy
   ```

4. Run the bot:

   ```bash
   go run main.go
   ```

---

## Configuration

| Environment Variable   | Description                                  | Default Value            |
|------------------------|----------------------------------------------|--------------------------|
| `TELEGRAM_BOT_TOKEN`   | Your Telegram bot token (required).          | `N/A`                    |
| `OLLAMA_HOST`          | Host URL for the Ollama API.                 | `http://localhost:11411` |
| `OLLAMA_MODEL`         | The model used for name classification.      | `llama3.2:1b`            |
| `LOG_LEVEL`            | Logging level (`debug`, `info`, `warn`).     | `info`                   |
| `HTTP_PORT`            | Port for the health endpoint.                | `8080`                   |

---

## Features in Detail

### 1. Welcome Messages
The bot sends personalized welcome messages when new members join a group.

- **Italian Names**: If the member's name is classified as Italian, the bot sends a message in Italian.
- **Non-Italian Names**: For non-Italian names, the bot sends a message in English.

Example welcome messages:

**For Italian Names:**
```
Ciao Giovanni! Benvenutə nel gruppo PugliaTechs...
```

**For Non-Italian Names:**
```
Hello John! Welcome to the PugliaTechs group...
```

---

### 2. Name Classification with Ollama
The bot uses the **Ollama API** to classify names based on their origin. It sends a prompt like:

```plaintext
You are a name classifier. I will give you a first name, and you reply with exactly one word: either 'ITALIAN' if this name is likely from an Italian person, or 'FOREIGN' if it is not. The name is: "Giovanni".
```

If the response contains `"ITALIAN"`, the bot identifies the name as Italian.

---

### 3. Health Endpoint
The bot includes an HTTP health check endpoint:

- **URL**: `/health`
- **Response**:
  - `200 OK`: Telegram bot is connected.
  - `500 Internal Server Error`: Telegram bot is not connected.

Start the HTTP server alongside the bot:

```plaintext
INFO Starting HTTP server port=8080
```

Test the health endpoint:

```bash
curl http://localhost:8080/health
```

---

### 4. Logging
The bot uses `slog` for structured logging. Enable debug-level logging by setting `LOG_LEVEL=debug`. Example logs:

- Debugging classification results:
  ```plaintext
  DEBUG Checking if name is Italian name="Giovanni Mario Rossi"
  DEBUG Ollama classification result name="Giovanni Mario Rossi" raw="ITALIAN (HIGH CONFIDENCE)"
  DEBUG Name classification result firstName="Giovanni Mario Rossi" likelyItalian=true
  ```

- Health endpoint logs:
  ```plaintext
  DEBUG Health check passed
  ```

---

## Project Structure

```
.
├── cmd
│   └── main.go            # Entry point for the bot
├── internal
│   ├── telegram           # Telegram-related functionality
│   ├── welcome            # Welcome message logic
│   └── ollama             # Name classification logic
├── go.mod                 # Module dependencies
└── README.md     
```

---

## Development

1. To add a new feature or fix a bug, create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Submit a pull request for review.

---

## Contributing

Contributions are welcome! Please follow these steps:
1. Fork the repository.
2. Create a new branch.
3. Submit a pull request with detailed explanations of your changes.

---

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.

---

