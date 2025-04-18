# Character-TG

A Telegram bot that uses AI models to simulate characters in chats.

## Overview

Character-TG allows you to set up a Telegram bot that acts as a character using AI models. The bot can:
- Interact naturally in conversations based on a defined personality
- Process and respond to messages using Grok and Gemini AI models
- Import chat history to provide context for responses
- Remember conversation details through Redis-based storage

## Features

- Character customization through `/config` command
- Conversation initialization with `/init` command
- Support for importing chat history from JSON files
- Context-aware responses based on chat history
- Configurable behavior in group chats

## Requirements

- Go 1.24+
- Redis server
- API keys for Grok and Gemini
- Telegram Bot Token

## Configuration

The bot is configured through environment variables:

```
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
GROK_API_KEY=your_grok_api_key
GEMINI_API_KEY=your_gemini_api_key
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=your_redis_password
GROUP_REPLY_PROBABILITY=1.0
ALLOWED_CHAT_IDS=123456789,-1001234567890
```

## Deployment

The project includes a Dockerfile and Fly.io configuration for easy deployment.

### Local Development

1. Clone the repository
2. Create a `.env` file with the required environment variables
3. Run `go build && ./character-tg`

### Docker Deployment

```bash
docker build -t character-tg .
docker run -p 8080:8080 --env-file .env character-tg
```

### Fly.io Deployment

```bash
fly launch
fly secrets set TELEGRAM_BOT_TOKEN=your_token
fly secrets set GROK_API_KEY=your_key
fly secrets set GEMINI_API_KEY=your_key
fly secrets set REDIS_PASSWORD=your_password
fly deploy
```

## Usage

1. Start a chat with your bot
2. Use `/config` to set a character prompt
3. Use `/init` to generate a chat overview
4. Start chatting with the bot
