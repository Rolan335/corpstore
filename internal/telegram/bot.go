package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	tele "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"corpstore/internal/auth"
	"corpstore/internal/handlers"
)

// Bot wraps telegram bot and handlers to expose same functionality as HTTP endpoints.
type Bot struct {
	bot    *tele.BotAPI
	h      *handlers.Handler
	authSv *auth.Service
	store  handlers.Store
}

func NewBotFromEnv(h *handlers.Handler, a *auth.Service, storage handlers.Store) (*Bot, error) {
	_ = os.Setenv("TELEGRAM_TOKEN_LOADED", "1")
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}
	b, err := tele.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	b.Debug = false
	return &Bot{
		bot:    b,
		h:      h,
		authSv: a,
		store:  storage,
	}, nil
}

// StartPolling starts processing updates and maps commands to handlers.
func (tb *Bot) StartPolling(ctx context.Context) {
	u := tele.NewUpdate(0)
	u.Timeout = 60
	updates := tb.bot.GetUpdatesChan(u)
	for upd := range updates {
		if upd.Message == nil {
			continue
		}
		go tb.handleMessage(ctx, upd.Message)
	}
}

func (tb *Bot) handleMessage(ctx context.Context, msg *tele.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" && msg.Document == nil && len(msg.Photo) == 0 {
		return
	}

	// registration and login via text commands
	if strings.HasPrefix(text, "/reg") {
		// /reg username password
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			tb.reply(msg.Chat.ID, "usage: /reg <username> <password>")
			return
		}
		username := parts[1]
		password := parts[2]
		id, err := tb.authSv.Register(ctx, username, password)
		if err != nil {
			// try to detect duplicate username
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				tb.reply(msg.Chat.ID, "registration failed: username already exists")
				return
			}
			tb.reply(msg.Chat.ID, "registration failed: "+err.Error())
			return
		}
		tb.reply(msg.Chat.ID, "registered user id: "+id)
		return
	}

	if strings.HasPrefix(text, "/login") {
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			tb.reply(msg.Chat.ID, "usage: /login <username> <password>")
			return
		}
		username := parts[1]
		password := parts[2]
		tok, err := tb.authSv.Login(ctx, username, password)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				tb.reply(msg.Chat.ID, "login failed: invalid username or password")
				return
			}
			tb.reply(msg.Chat.ID, "login failed: "+err.Error())
			return
		}
		tb.reply(msg.Chat.ID, "token: "+tok)
		return
	}

	// Handle document upload: caption must contain token in form '/upload <token>' or 'token:<token>'
	if msg.Document != nil || len(msg.Photo) > 0 {
		caption := strings.TrimSpace(msg.Caption)
		var token string
		if strings.HasPrefix(caption, "/upload") {
			parts := strings.Fields(caption)
			if len(parts) >= 2 {
				token = parts[1]
			}
		} else if strings.Contains(strings.ToLower(caption), "token:") {
			parts := strings.SplitN(caption, "token:", 2)
			token = strings.TrimSpace(parts[1])
		}
		if token == "" {
			tb.reply(msg.Chat.ID, "To upload: send a file/document with caption '/upload <token>' where <token> is obtained from /login")
			return
		}

		claims, err := tb.authSv.ParseToken(ctx, token)
		if err != nil {
			// try to provide a friendlier message
			if strings.Contains(strings.ToLower(err.Error()), "expired") {
				tb.reply(msg.Chat.ID, "token expired, please /login again to get a new token")
				return
			}
			tb.reply(msg.Chat.ID, "invalid token: "+err.Error())
			return
		}
		owner := claims.Subject

		// pick file id and filename
		var fileID string
		var filename string
		if msg.Document != nil {
			fileID = msg.Document.FileID
			filename = msg.Document.FileName
		} else {
			// photo: take largest size
			p := msg.Photo[len(msg.Photo)-1]
			fileID = p.FileID
			filename = fileID + ".jpg"
		}

		data, err := tb.downloadFile(fileID)
		if err != nil {
			tb.reply(msg.Chat.ID, "failed to download file: "+err.Error())
			return
		}

		// size limit protection (20 MB)
		if len(data) > 20*1024*1024 {
			tb.reply(msg.Chat.ID, "file too large: maximum allowed size is 20MB")
			return
		}

		// save via storage
		id, err := tb.store.Save(ctx, data, filename, owner)
		if err != nil {
			// make error messages clearer for common failures
			le := strings.ToLower(err.Error())
			if strings.Contains(le, "permission") || strings.Contains(le, "denied") {
				tb.reply(msg.Chat.ID, "failed to save file: permission denied on storage")
				return
			}
			if strings.Contains(le, "disk") || strings.Contains(le, "no space") || strings.Contains(le, "no space left") {
				tb.reply(msg.Chat.ID, "failed to save file: insufficient disk space on server")
				return
			}
			tb.reply(msg.Chat.ID, "failed to save file: "+err.Error())
			return
		}
		tb.reply(msg.Chat.ID, "file saved with id: "+id)
		return
	}

	if strings.HasPrefix(text, "/get") {
		// /get <file-id> <token>
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			tb.reply(msg.Chat.ID, "usage: /get <file-id> <token>")
			return
		}
		id := parts[1]
		token := parts[2]
		// validate token and get owner
		claims, err := tb.authSv.ParseToken(ctx, token)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "expired") {
				tb.reply(msg.Chat.ID, "token expired, please /login again")
				return
			}
			tb.reply(msg.Chat.ID, "invalid token: "+err.Error())
			return
		}
		owner := claims.Subject
		// fetch metadata and file via handlers' storage
		filename, ownerID, err := tb.store.GetMetadata(ctx, id)
		if err != nil {
			// give clearer not found message
			if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no rows") {
				tb.reply(msg.Chat.ID, "file not found")
				return
			}
			tb.reply(msg.Chat.ID, "file metadata not found: "+err.Error())
			return
		}
		if ownerID != owner {
			tb.reply(msg.Chat.ID, "forbidden: owner mismatch")
			return
		}
		b, err := tb.store.Get(ctx, id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "no rows") {
				tb.reply(msg.Chat.ID, "file not found")
				return
			}
			tb.reply(msg.Chat.ID, "file not found: "+err.Error())
			return
		}
		// send as document
		d := tele.FileBytes{Name: filename, Bytes: b}
		msgCfg := tele.NewDocument(msg.Chat.ID, d)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send doc err: %v", err)
			tb.reply(msg.Chat.ID, "failed to send file: "+err.Error())
		}
		return
	}

	// default: show help
	tb.reply(msg.Chat.ID, "commands: /reg /login /get <file-id> <token> — to upload, send a file with caption '/upload <token>'")
}

func (tb *Bot) downloadFile(fileID string) ([]byte, error) {
	f, err := tb.bot.GetFile(tele.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}
	// construct download URL
	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", tb.bot.Token, f.FilePath)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("telegram file download status %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (tb *Bot) reply(chatID int64, text string) {
	m := tele.NewMessage(chatID, text)
	m.ParseMode = "Markdown"
	if _, err := tb.bot.Send(m); err != nil {
		log.Printf("telegram reply error: %v", err)
	}
}
