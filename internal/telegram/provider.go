package telegram

import (
	"fmt"
	"os"

	tele "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"corpstore/internal/usecase"
)

// Config holds telegram bot configuration.
type Config struct {
	Token string
}

func ConfigFromEnv() (Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}
	return Config{Token: token}, nil
}

// Provider wires telegram bot.
type Provider struct {
	Bot *Bot
}

func NewProvider(cfg Config, authUC *usecase.Auth, filesUC *usecase.Files) (*Provider, error) {
	bot, err := tele.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, err
	}
	sessions := NewInMemorySessionStore()
	return &Provider{Bot: NewBot(bot, authUC, filesUC, sessions)}, nil
}
