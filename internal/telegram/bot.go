package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	tele "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"corpstore/internal/fileaccess"
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
	if msg.From != nil {
		if err := tb.ensureTelegramSession(ctx, msg.Chat.ID, msg.From); err != nil {
			log.Printf("telegram init session error: %v", err)
			tb.reply(msg.Chat.ID, "failed to init session")
			return
		}
	}

	if text != "" {
		if tb.handleAuthFlow(ctx, msg, text) {
			return
		}
	}

	// basic commands
	if strings.HasPrefix(text, "/start") {
		tb.replyWithKeyboard(msg.Chat.ID, "Welcome! Choose an action:", tb.mainKeyboard())
		return
	}
	if strings.HasPrefix(text, "/reg") || strings.HasPrefix(text, "/login") || strings.HasPrefix(text, "/logout") {
		tb.replyWithKeyboard(msg.Chat.ID, "Telegram login is automatic. Use the menu.", tb.mainKeyboard())
		return
	}

	// Handle document upload: requires session
	if msg.Document != nil || len(msg.Photo) > 0 {
		sess, ok := tb.sess.Get(msg.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.replyWithKeyboard(msg.Chat.ID, "Please open /start to initialize.", tb.mainKeyboard())
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
			log.Printf("telegram download error: %v", err)
			tb.reply(msg.Chat.ID, "failed to download file")
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
			log.Printf("telegram save error: %v", err)
			tb.reply(msg.Chat.ID, "failed to save file")
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
			tb.replyWithKeyboard(msg.Chat.ID, "Please open /start to initialize.", tb.mainKeyboard())
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
				log.Printf("telegram get file error: %v", err)
				tb.reply(msg.Chat.ID, "file not found")
			}
			return
		}
		tb.reply(msg.Chat.ID, "file id: "+meta.ID)
		d := tele.FileBytes{Name: meta.Filename, Bytes: b}
		msgCfg := tele.NewDocument(msg.Chat.ID, d)
		if _, err := tb.bot.Send(msgCfg); err != nil {
			log.Printf("telegram send doc err: %v", err)
			tb.reply(msg.Chat.ID, "failed to send file")
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
	if cb.From != nil {
		if err := tb.ensureTelegramSession(ctx, cb.Message.Chat.ID, cb.From); err != nil {
			log.Printf("telegram init session error: %v", err)
			tb.answerCallback(cb, "failed to init session")
			return
		}
	}
	data := strings.TrimSpace(cb.Data)
	if data == "" {
		return
	}
	switch data {
	case "files:list":
		tb.answerCallback(cb, "loading")
		tb.sendFilesList(ctx, cb.Message.Chat.ID)
		return
	case "files:shared":
		tb.answerCallback(cb, "loading")
		tb.sendSharedList(ctx, cb.Message.Chat.ID)
		return
	}
	if strings.HasPrefix(data, "fileown:") {
		id := strings.TrimPrefix(data, "fileown:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "no session")
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
				log.Printf("telegram fileown error: %v", err)
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
				tele.NewInlineKeyboardButtonData("Shared with", "filegrants:"+meta.ID),
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
	if strings.HasPrefix(data, "filegrants:") {
		id := strings.TrimPrefix(data, "filegrants:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "no session")
			return
		}
		sess.Temp.FileID = id
		tb.sess.Set(cb.Message.Chat.ID, sess)
		tb.answerCallback(cb, "loading")
		tb.sendGrantedUsers(ctx, cb.Message.Chat.ID)
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
				log.Printf("telegram download error: %v", err)
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
				log.Printf("telegram delete error: %v", err)
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
				log.Printf("telegram shared file error: %v", err)
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
			tb.answerCallback(cb, "no session")
			return
		}
		sess.Mode = ModeShareUsername
		sess.Temp.FileID = id
		if cb.From != nil {
			sess.Temp.OwnerFirstName = cb.From.FirstName
			sess.Temp.OwnerLastName = cb.From.LastName
			sess.Temp.OwnerUsername = cb.From.UserName
		}
		tb.sess.Set(cb.Message.Chat.ID, sess)
		tb.answerCallback(cb, "enter username")
		tb.reply(cb.Message.Chat.ID, "Share: enter username")
		return
	}
	if strings.HasPrefix(data, "rev:") {
		userID := strings.TrimPrefix(data, "rev:")
		sess, ok := tb.sess.Get(cb.Message.Chat.ID)
		if !ok || sess.UserID == "" {
			tb.answerCallback(cb, "no session")
			return
		}
		fileID := strings.TrimSpace(sess.Temp.FileID)
		if fileID == "" {
			tb.answerCallback(cb, "select a file first")
			return
		}
		if err := tb.files.RevokeShare(ctx, sess.UserID, fileID, userID); err != nil {
			log.Printf("telegram revoke error: %v", err)
			tb.answerCallback(cb, "revoke failed")
			return
		}
		tb.answerCallback(cb, "revoked")
		tb.sendGrantedUsers(ctx, cb.Message.Chat.ID)
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
	if text == "/cancel" || text == "/start" {
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		tb.sess.Set(msg.Chat.ID, sess)
		tb.replyWithKeyboard(msg.Chat.ID, "Canceled.", tb.mainKeyboard())
		return true
	}
	switch sess.Mode {
	case ModeShareUsername:
		username := strings.TrimSpace(text)
		fileID := strings.TrimSpace(sess.Temp.FileID)
		if username == "" || fileID == "" {
			tb.reply(msg.Chat.ID, "share failed: invalid username or file id")
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			sess.Temp.FileID = ""
			sess.Temp.OwnerFirstName = ""
			sess.Temp.OwnerLastName = ""
			sess.Temp.OwnerUsername = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		ownerTG := fileaccess.OwnerTGInfo{
			FirstName: sess.Temp.OwnerFirstName,
			LastName:  sess.Temp.OwnerLastName,
			Username:  sess.Temp.OwnerUsername,
		}
		if err := tb.files.ShareByUsername(ctx, sess.UserID, fileID, username, ownerTG); err != nil {
			switch err {
			case usecase.ErrUserNotFound:
				tb.reply(msg.Chat.ID, "user not found")
			case files.ErrNotFound:
				tb.reply(msg.Chat.ID, "file not found")
			case files.ErrForbidden:
				tb.reply(msg.Chat.ID, "forbidden")
			default:
				log.Printf("telegram share error: %v", err)
				tb.reply(msg.Chat.ID, "share failed")
			}
			sess.Mode = ModeNone
			sess.Temp.Username = ""
			sess.Temp.FileID = ""
			sess.Temp.OwnerFirstName = ""
			sess.Temp.OwnerLastName = ""
			sess.Temp.OwnerUsername = ""
			tb.sess.Set(msg.Chat.ID, sess)
			return true
		}
		sess.Mode = ModeNone
		sess.Temp.Username = ""
		sess.Temp.FileID = ""
		sess.Temp.OwnerFirstName = ""
		sess.Temp.OwnerLastName = ""
		sess.Temp.OwnerUsername = ""
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
		tb.replyWithKeyboard(chatID, "Please open /start to initialize.", tb.mainKeyboard())
		return
	}
	list, err := tb.files.ListByOwner(ctx, sess.UserID)
	if err != nil {
		log.Printf("telegram list files error: %v", err)
		tb.reply(chatID, "failed to list files")
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
		tb.reply(chatID, "failed to send list")
	}
}

func (tb *Bot) sendSharedList(ctx context.Context, chatID int64) {
	sess, ok := tb.sess.Get(chatID)
	if !ok || sess.UserID == "" {
		tb.replyWithKeyboard(chatID, "Please open /start to initialize.", tb.mainKeyboard())
		return
	}
	list, err := tb.files.ListShared(ctx, sess.UserID)
	if err != nil {
		log.Printf("telegram list shared error: %v", err)
		tb.reply(chatID, "failed to list shared files")
		return
	}
	if len(list) == 0 {
		tb.reply(chatID, "no shared files")
		return
	}
	rows := make([][]tele.InlineKeyboardButton, 0, len(list))
	for _, f := range list {
		owner := tb.ownerDisplay(f.OwnerTGFirstName, f.OwnerTGLastName, f.OwnerTGUsername, f.OwnerID)
		label := f.Filename
		if owner != "" {
			label = f.Filename + " · " + owner
		}
		btn := tele.NewInlineKeyboardButtonData(label, "filesh:"+f.ID)
		rows = append(rows, tele.NewInlineKeyboardRow(btn))
	}
	msgCfg := tele.NewMessage(chatID, "Shared files:")
	msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
	if _, err := tb.bot.Send(msgCfg); err != nil {
		log.Printf("telegram send list err: %v", err)
		tb.reply(chatID, "failed to send list")
	}
}

func (tb *Bot) sendGrantedUsers(ctx context.Context, chatID int64) {
	sess, ok := tb.sess.Get(chatID)
	if !ok || sess.UserID == "" {
		tb.replyWithKeyboard(chatID, "Please open /start to initialize.", tb.mainKeyboard())
		return
	}
	fileID := strings.TrimSpace(sess.Temp.FileID)
	if fileID == "" {
		tb.reply(chatID, "select a file first")
		return
	}
	users, err := tb.files.ListGrantedUsers(ctx, sess.UserID, fileID)
	if err != nil {
		log.Printf("telegram list granted error: %v", err)
		tb.reply(chatID, "failed to list shared users")
		return
	}
	if len(users) == 0 {
		tb.reply(chatID, "not shared with anyone")
		return
	}
	rows := make([][]tele.InlineKeyboardButton, 0, len(users))
	for _, u := range users {
		label := u.Username
		btn := tele.NewInlineKeyboardButtonData("Revoke: "+label, "rev:"+u.UserID)
		rows = append(rows, tele.NewInlineKeyboardRow(btn))
	}
	msgCfg := tele.NewMessage(chatID, "Shared with:")
	msgCfg.ReplyMarkup = tele.NewInlineKeyboardMarkup(rows...)
	if _, err := tb.bot.Send(msgCfg); err != nil {
		log.Printf("telegram send list err: %v", err)
		tb.reply(chatID, "failed to send list")
	}
}

func (tb *Bot) mainKeyboard() *tele.InlineKeyboardMarkup {
	rows := [][]tele.InlineKeyboardButton{
		tele.NewInlineKeyboardRow(
			tele.NewInlineKeyboardButtonData("My files", "files:list"),
			tele.NewInlineKeyboardButtonData("Shared files", "files:shared"),
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

func (tb *Bot) ensureTelegramSession(ctx context.Context, chatID int64, u *tele.User) error {
	if u == nil {
		return nil
	}
	id, err := tb.authUC.EnsureTelegramUser(ctx, u.ID, u.UserName)
	if err != nil {
		return err
	}
	sess, ok := tb.sess.Get(chatID)
	if !ok || sess.UserID == "" {
		tb.sess.Set(chatID, Session{UserID: id})
		return nil
	}
	if sess.UserID != id {
		sess.UserID = id
		tb.sess.Set(chatID, sess)
	}
	return nil
}

func (tb *Bot) ownerDisplay(first, last, username, ownerID string) string {
	name := strings.TrimSpace(first + " " + last)
	if username != "" {
		if name != "" {
			return name + " (@" + username + ")"
		}
		return "@" + username
	}
	if name != "" {
		return name
	}
	if ownerID != "" {
		return ownerID
	}
	return ""
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
