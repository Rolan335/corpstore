package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	tele "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"corpstore/internal/auth"
	"corpstore/internal/files"
	"corpstore/internal/usecase"
)

// Bot wraps telegram bot and handlers to expose same functionality as HTTP endpoints.
type Bot struct {
	bot    *tele.BotAPI
	authUC *usecase.Auth
	files  *usecase.Files
	sess   SessionStore
}

func NewBot(bot *tele.BotAPI, authUC *usecase.Auth, filesUC *usecase.Files, sess SessionStore) *Bot {
	bot.Debug = false
	return &Bot{
		bot:    bot,
		authUC: authUC,
		files:  filesUC,
		sess:   sess,
	}
}

// StartPolling starts processing updates and maps commands to handlers.
func (tb *Bot) StartPolling(ctx context.Context) {
	u := tele.NewUpdate(0)
	u.Timeout = 60
	updates := tb.bot.GetUpdatesChan(u)
	for upd := range updates {
		if upd.CallbackQuery != nil {
			go tb.handleCallback(ctx, upd.CallbackQuery)
			continue
		}
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

	if text != "" {
		if tb.handleAuthFlow(ctx, msg, text) {
			return
		}
	}

	// registration and login via text commands
	if strings.HasPrefix(text, "/start") {
		tb.replyWithKeyboard(msg.Chat.ID, "Welcome! Choose an action:", tb.mainKeyboard())
		return
	}
	if strings.HasPrefix(text, "/logout") {
		tb.sess.Clear(msg.Chat.ID)
		tb.replyWithKeyboard(msg.Chat.ID, "Logged out.", tb.mainKeyboard())
		return
	}
	if strings.HasPrefix(text, "/reg") {
		if tb.isLoggedIn(msg.Chat.ID) {
			tb.replyWithKeyboard(msg.Chat.ID, "You are already logged in.", tb.mainKeyboard())
			return
		}
		// /reg username password
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			tb.replyWithKeyboard(msg.Chat.ID, "Use the buttons to register.", tb.authKeyboard())
			return
		}
		username := parts[1]
		password := parts[2]
		id, err := tb.authUC.Register(ctx, username, password)
		if err != nil {
			// try to detect duplicate username
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				tb.reply(msg.Chat.ID, "registration failed: username already exists")
				return
			}
			tb.reply(msg.Chat.ID, "registration failed: "+err.Error())
			return
		}
		tb.sess.Set(msg.Chat.ID, Session{UserID: id})
		tb.reply(msg.Chat.ID, "registered user id: "+id)
		return
	}

	if strings.HasPrefix(text, "/login") {
		if tb.isLoggedIn(msg.Chat.ID) {
			tb.replyWithKeyboard(msg.Chat.ID, "You are already logged in.", tb.mainKeyboard())
			return
		}
		parts := strings.SplitN(text, " ", 3)
		if len(parts) < 3 {
			tb.replyWithKeyboard(msg.Chat.ID, "Use the buttons to login.", tb.authKeyboard())
			return
		}
		username := parts[1]
		password := parts[2]
		tok, err := tb.authUC.Login(ctx, username, password)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				tb.reply(msg.Chat.ID, "login failed: invalid username or password")
				return
			}
			tb.reply(msg.Chat.ID, "login failed: "+err.Error())
			return
		}
		claims, err := tb.authUC.ParseToken(ctx, tok)
		if err == nil && claims != nil && claims.Subject != "" {
			tb.sess.Set(msg.Chat.ID, Session{UserID: claims.Subject})
		}
		tb.reply(msg.Chat.ID, "token: "+tok)
		return
	}

	// Handle document upload: requires session
	if msg.Document != nil || len(msg.Photo) > 0 {
		sess, ok := tb.sess.Get(msg.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.replyWithKeyboard(msg.Chat.ID, "Please log in to upload files.", tb.authKeyboard())
			return
		}
		owner := sess.UserID

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

		// save via usecase
		id, err := tb.files.SaveRaw(ctx, owner, filename, data)
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
		tb.reply(msg.Chat.ID, "file saved\nname: "+filename+"\nid: "+id)
		return
	}

	if strings.HasPrefix(text, "/getfiles") || strings.HasPrefix(text, "/files") {
		tb.sendFilesList(ctx, msg.Chat.ID)
		return
	}

	if strings.HasPrefix(text, "/get") {
		if !tb.isLoggedIn(msg.Chat.ID) {
			tb.replyWithKeyboard(msg.Chat.ID, "Please log in first.", tb.authKeyboard())
			return
		}
		// /get <file-id> [token]
		parts := strings.Fields(text)
		if len(parts) < 2 {
			tb.reply(msg.Chat.ID, "usage: /get <file-id>")
			return
		}
		id := parts[1]
		sess, _ := tb.sess.Get(msg.Chat.ID)
		owner := sess.UserID

		meta, b, err := tb.files.GetForOwner(ctx, owner, id)
		if err != nil {
			switch err {
			case files.ErrForbidden:
				tb.reply(msg.Chat.ID, "forbidden: owner mismatch")
			case files.ErrNotFound:
				tb.reply(msg.Chat.ID, "file not found")
			default:
				tb.reply(msg.Chat.ID, "file not found: "+err.Error())
			}
			return
		}
		tb.reply(msg.Chat.ID, "file id: "+meta.ID)
		d := tele.FileBytes{Name: meta.Filename, Bytes: b}
		msgCfg := tele.NewDocument(msg.Chat.ID, d)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send doc err: %v", err)
			tb.reply(msg.Chat.ID, "failed to send file: "+err.Error())
		}
		return
	}

	// default: show help
	tb.replyWithKeyboard(msg.Chat.ID, "Choose an action:", tb.mainKeyboard())
}

func (tb *Bot) handleCallback(ctx context.Context, cb *tele.CallbackQuery) {
	if cb == nil || cb.Message == nil {
		return
	}
	data := strings.TrimSpace(cb.Data)
	if data == "" {
		return
	}
	switch data {
	case "auth:register":
		if tb.isLoggedIn(cb.Message.Chat.ID) {
			tb.answerCallback(cb, "already logged in")
			return
		}
		tb.setSessionMode(cb.Message.Chat.ID, ModeRegisterUsername)
		tb.answerCallback(cb, "enter username")
		tb.reply(cb.Message.Chat.ID, "Registration: enter username")
		return
	case "auth:login":
		if tb.isLoggedIn(cb.Message.Chat.ID) {
			tb.answerCallback(cb, "already logged in")
			return
		}
		tb.setSessionMode(cb.Message.Chat.ID, ModeLoginUsername)
		tb.answerCallback(cb, "enter username")
		tb.reply(cb.Message.Chat.ID, "Login: enter username")
		return
	case "files:list":
		tb.answerCallback(cb, "loading")
		tb.sendFilesList(ctx, cb.Message.Chat.ID)
		return
	case "files:shared":
		tb.answerCallback(cb, "loading")
		tb.sendSharedList(ctx, cb.Message.Chat.ID)
		return
	case "auth:logout":
		tb.sess.Clear(cb.Message.Chat.ID)
		tb.answerCallback(cb, "logged out")
		tb.replyWithKeyboard(cb.Message.Chat.ID, "Logged out.", tb.mainKeyboard())
		return
	}
	if strings.HasPrefix(data, "fileown:") {
		id := strings.TrimPrefix(data, "fileown:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "please /login first")
			return
		}
		meta, _, err := tb.files.GetForOwner(ctx, sess.UserID, id)
		if err != nil {
			switch err {
			case files.ErrForbidden:
				tb.answerCallback(cb, "forbidden")
			case files.ErrNotFound:
				tb.answerCallback(cb, "file not found")
			default:
				tb.answerCallback(cb, "failed to get file")
			}
			return
		}
		rows := [][]tele.InlineKeyboardButton{
			tele.NewInlineKeyboardRow(
				tele.NewInlineKeyboardButtonData("Download", "filedl:"+meta.ID),
				tele.NewInlineKeyboardButtonData("Delete", "filedel:"+meta.ID),
			),
			tele.NewInlineKeyboardRow(
				tele.NewInlineKeyboardButtonData("Share", "fileshare:"+meta.ID),
			),
		}
		msgCfg := tele.NewMessage(cb.Message.Chat.ID, "file:\nname: "+meta.Filename+"\nid: "+meta.ID)
		msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send file actions err: %v", err)
			tb.answerCallback(cb, "failed to show actions")
			return
		}
		tb.answerCallback(cb, "select action")
		return
	}
	if strings.HasPrefix(data, "filedl:") {
		id := strings.TrimPrefix(data, "filedl:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "please /login first")
			return
		}
		meta, b, err := tb.files.GetForOwner(ctx, sess.UserID, id)
		if err != nil {
			switch err {
			case files.ErrForbidden:
				tb.answerCallback(cb, "forbidden")
			case files.ErrNotFound:
				tb.answerCallback(cb, "file not found")
			default:
				tb.answerCallback(cb, "failed to get file")
			}
			return
		}
		d := tele.FileBytes{Name: meta.Filename, Bytes: b}
		msgCfg := tele.NewDocument(cb.Message.Chat.ID, d)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send doc err: %v", err)
			tb.answerCallback(cb, "failed to send file")
			return
		}
		tb.answerCallback(cb, "sending file")
		return
	}
	if strings.HasPrefix(data, "filedel:") {
		id := strings.TrimPrefix(data, "filedel:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "please /login first")
			return
		}
		meta, err := tb.files.DeleteForOwner(ctx, sess.UserID, id)
		if err != nil {
			switch err {
			case files.ErrForbidden:
				tb.answerCallback(cb, "forbidden")
			case files.ErrNotFound:
				tb.answerCallback(cb, "file not found")
			default:
				tb.answerCallback(cb, "failed to delete file")
			}
			return
		}
		tb.answerCallback(cb, "deleted")
		tb.reply(cb.Message.Chat.ID, "deleted:\nname: "+meta.Filename+"\nid: "+meta.ID)
		return
	}
	if strings.HasPrefix(data, "filesh:") {
		id := strings.TrimPrefix(data, "filesh:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "please /login first")
			return
		}
		meta, _, err := tb.files.GetForUser(ctx, sess.UserID, id)
		if err != nil {
			switch err {
			case files.ErrForbidden:
				tb.answerCallback(cb, "forbidden")
			case files.ErrNotFound:
				tb.answerCallback(cb, "file not found")
			default:
				tb.answerCallback(cb, "failed to get file")
			}
			return
		}
		rows := [][]tele.InlineKeyboardButton{
			tele.NewInlineKeyboardRow(
				tele.NewInlineKeyboardButtonData("Download", "filedl:"+meta.ID),
			),
		}
		msgCfg := tele.NewMessage(cb.Message.Chat.ID, "shared file:\nname: "+meta.Filename+"\nid: "+meta.ID)
		msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send file actions err: %v", err)
			tb.answerCallback(cb, "failed to show actions")
			return
		}
		tb.answerCallback(cb, "select action")
		return
	}
	if strings.HasPrefix(data, "fileshare:") {
		id := strings.TrimPrefix(data, "fileshare:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "please /login first")
			return
		}
		sess.Mode = ModeShareUsername
		sess.Temp.FileID = id
		tb.sess.Set(cb.Message.Chat.ID, sess)
		tb.answerCallback(cb, "enter username")
		tb.reply(cb.Message.Chat.ID, "Share: enter username")
		return
	}
}

func (tb *Bot) answerCallback(cb *tele.CallbackQuery, text string) {
	cfg := tele.NewCallback(cb.ID, text)
	if _, err := tb.bot.Request(cfg); err != nil {
		log.Printf("telegram callback answer err: %v", err)
	}
}

func (tb *Bot) handleAuthFlow(ctx context.Context, msg *tele.Message, text string) bool {
	sess, ok := tb.sess.Get(msg.Chat.ID)
	if !ok || sess.Mode == ModeNone {
		return false
	}
	if sess.UserID != "" && sess.Mode != ModeShareUsername {
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "You are already logged in.", tb.mainKeyboard())
		return true
	}
	if text == "/cancel" || text == "/start" {
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "Canceled.", tb.mainKeyboard())
		return true
	}
	switch sess.Mode {
	case ModeRegisterUsername:
		sess.Mode = ModeRegisterPassword
		sess.Temp.Username = text
		tb.sess.Set(msg.Chat.ID, sess)
		tb.reply(msg.Chat.ID, "Registration: enter password")
		return true
	case ModeRegisterPassword:
		username := strings.TrimSpace(sess.Temp.Username)
		password := text
		id, err := tb.authUC.Register(ctx, username, password)
		if err != nil {
			tb.reply(msg.Chat.ID, "registration failed: "+err.Error())
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		sess.UserID = id
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "Registered. You can upload files now.", tb.mainKeyboard())
		return true
	case ModeLoginUsername:
		sess.Mode = ModeLoginPassword
		sess.Temp.Username = text
		tb.sess.Set(msg.Chat.ID, sess)
		tb.reply(msg.Chat.ID, "Login: enter password")
		return true
	case ModeLoginPassword:
		username := strings.TrimSpace(sess.Temp.Username)
		password := text
		tok, err := tb.authUC.Login(ctx, username, password)
		if err != nil {
			tb.reply(msg.Chat.ID, "login failed: "+err.Error())
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		claims, err := tb.authUC.ParseToken(ctx, tok)
		if err == nil && claims != nil && claims.Subject != "" {
			sess.UserID = claims.Subject
		}
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "Logged in. You can upload files now.", tb.mainKeyboard())
		return true
	case ModeShareUsername:
		username := strings.TrimSpace(text)
		fileID := strings.TrimSpace(sess.Temp.FileID)
		if username == "" || fileID == "" {
			tb.reply(msg.Chat.ID, "share failed: invalid username or file id")
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			sess.Temp.FileID = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		if err := tb.files.ShareByUsername(ctx, sess.UserID, fileID, username); err != nil {
			tb.reply(msg.Chat.ID, "share failed: "+err.Error())
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			sess.Temp.FileID = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "Shared.", tb.mainKeyboard())
		return true
	default:
		return false
	}
}

func (tb *Bot) setSessionMode(chatID int64, mode SessionMode) {
	sess, ok := tb.sess.Get(chatID)
	if !ok {
		tb.sess.Set(chatID, Session{Mode: mode})
		return
	}
	sess.Mode = mode
	tb.sess.Set(chatID, sess)
}

func (tb *Bot) sendFilesList(ctx context.Context, chatID int64) {
	sess, ok := tb.sess.Get(chatID)
	if !ok || sess.UserID == "" {
		tb.replyWithKeyboard(chatID, "Please log in to see files.", tb.authKeyboard())
		return
	}
	list, err := tb.files.ListByOwner(ctx, sess.UserID)
	if err != nil {
		tb.reply(chatID, "failed to list files: "+err.Error())
		return
	}
	if len(list) == 0 {
		tb.reply(chatID, "no files")
		return
	}
	rows := make([][]tele.InlineKeyboardButton, 0, len(list))
	for _, f := range list {
		btn := tele.NewInlineKeyboardButtonData(f.Filename, "fileown:"+f.ID)
		rows = append(rows, tele.NewInlineKeyboardRow(btn))
	}
	msgCfg := tele.NewMessage(chatID, "Your files:")
	msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
	if _, err := tb.bot.Send(msgCfg); err != nil {
		log.Printf("telegram send list err: %v", err)
		tb.reply(chatID, "failed to send list: "+err.Error())
	}
}

func (tb *Bot) sendSharedList(ctx context.Context, chatID int64) {
	sess, ok := tb.sess.Get(chatID)
	if !ok || sess.UserID == "" {
		tb.replyWithKeyboard(chatID, "Please log in to see shared files.", tb.authKeyboard())
		return
	}
	list, err := tb.files.ListShared(ctx, sess.UserID)
	if err != nil {
		tb.reply(chatID, "failed to list shared files: "+err.Error())
		return
	}
	if len(list) == 0 {
		tb.reply(chatID, "no shared files")
		return
	}
	rows := make([][]tele.InlineKeyboardButton, 0, len(list))
	for _, f := range list {
		btn := tele.NewInlineKeyboardButtonData(f.Filename, "filesh:"+f.ID)
		rows = append(rows, tele.NewInlineKeyboardRow(btn))
	}
	msgCfg := tele.NewMessage(chatID, "Shared files:")
	msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
	if _, err := tb.bot.Send(msgCfg); err != nil {
		log.Printf("telegram send list err: %v", err)
		tb.reply(chatID, "failed to send list: "+err.Error())
	}
}

func (tb *Bot) mainKeyboard() *tele.InlineKeyboardMarkup {
	rows := [][]tele.InlineKeyboardButton{
		tele.NewInlineKeyboardRow(
			tele.NewInlineKeyboardButtonData("My files", "files:list"),
			tele.NewInlineKeyboardButtonData("Shared files", "files:shared"),
		),
		tele.NewInlineKeyboardRow(
			tele.NewInlineKeyboardButtonData("Logout", "auth:logout"),
		),
		tele.NewInlineKeyboardRow(
			tele.NewInlineKeyboardButtonData("Login", "auth:login"),
			tele.NewInlineKeyboardButtonData("Register", "auth:register"),
		),
	}
	m := tele.NewInlineKeyboardMarkup(rows...)
	return &m
}

func (tb *Bot) authKeyboard() *tele.InlineKeyboardMarkup {
	rows := [][]tele.InlineKeyboardButton{
		tele.NewInlineKeyboardRow(
			tele.NewInlineKeyboardButtonData("Login", "auth:login"),
			tele.NewInlineKeyboardButtonData("Register", "auth:register"),
		),
	}
	m := tele.NewInlineKeyboardMarkup(rows...)
	return &m
}

func (tb *Bot) replyWithKeyboard(chatID int64, text string, kb *tele.InlineKeyboardMarkup) {
	msg := tele.NewMessage(chatID, text)
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	if _, err := tb.bot.Send(msg); err != nil {
		log.Printf("telegram reply error: %v", err)
	}
}

func (tb *Bot) isLoggedIn(chatID int64) bool {
	sess, ok := tb.sess.Get(chatID)
	return ok && sess.UserID != ""
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
