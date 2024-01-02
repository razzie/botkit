package botkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/commander"
	"github.com/razzie/razcache"
)

type Bot struct {
	BotOptions
	token string
	cache razcache.Cache
	api   *tgbotapi.BotAPI
}

func NewBot(token string, opts ...BotOption) (*Bot, error) {
	bot := &Bot{
		BotOptions: defaultOptions,
		token:      token,
	}
	for _, opt := range opts {
		opt(&bot.BotOptions)
	}

	var err error
	bot.api, err = tgbotapi.NewBotAPIWithAPIEndpoint(token, bot.apiEndpoint)
	if err != nil {
		return nil, err
	}

	if len(bot.redisDSN) > 0 {
		bot.cache, err = razcache.NewRedisCache(bot.redisDSN)
		if err != nil {
			return nil, err
		}
	} else {
		bot.cache = razcache.NewInMemCache()
	}

	return bot, nil
}

func (bot *Bot) Run() {
	updateConfig := tgbotapi.NewUpdate(bot.offset)
	updateConfig.Timeout = bot.timeout

	for update := range bot.api.GetUpdatesChan(updateConfig) {
		if update.Message != nil {
			if update.Message.IsCommand() {
				bot.handleCommand(update.Message)
			} else {
				bot.handleMessage(update.Message)
			}
		}
		if update.CallbackQuery != nil {
			bot.handleCallback(update.CallbackQuery)
		}
	}
}

func (bot *Bot) SendMessage(ctx context.Context, text string, reply bool) error {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	msg := tgbotapi.NewMessage(ctxMsg.Chat.ID, text)
	if reply {
		msg.ReplyToMessageID = ctxMsg.MessageID
	}
	_, err := bot.api.Send(msg)
	return err
}

func (bot *Bot) StartDialog(ctx context.Context, name string) error {
	h := bot.dialogs[name]
	if h == nil {
		return fmt.Errorf("unknown dialog: %s", name)
	}
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	dlg := &Dialog{
		userID: ctxMsg.From.ID,
		chatID: ctxMsg.Chat.ID,
		data: dialogData{
			Name:      name,
			Username:  ctxMsg.From.UserName,
			IsPrivate: ctxMsg.Chat.IsPrivate(),
		},
	}
	if q := h(ctx, dlg); q != nil {
		bot.handleQuery(dlg, q)
		bot.saveDialog(dlg)
	}
	return nil
}

func (bot *Bot) SetUserData(ctx context.Context, key, data string, ttl time.Duration) error {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	return bot.cache.Set(fmt.Sprintf("userdata:%d:%d:%s", ctxMsg.From.ID, ctxMsg.Chat.ID, key), data, ttl)
}

func (bot *Bot) GetUserData(ctx context.Context, key string) (string, error) {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return "", ErrInvalidContext
	}
	return bot.cache.Get(fmt.Sprintf("userdata:%d:%d:%s", ctxMsg.From.ID, ctxMsg.Chat.ID, key))
}

func (bot *Bot) SetChatData(ctx context.Context, key, data string, ttl time.Duration) error {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	return bot.cache.Set(fmt.Sprintf("chatdata:%d:%s", ctxMsg.Chat.ID, key), data, ttl)
}

func (bot *Bot) GetChatData(ctx context.Context, key string) (string, error) {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return "", ErrInvalidContext
	}
	return bot.cache.Get(fmt.Sprintf("chatdata:%d:%s", ctxMsg.Chat.ID, key))
}

func (bot *Bot) UploadFile(ctx context.Context, name string, r io.Reader) error {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	doc := tgbotapi.NewDocument(ctxMsg.Chat.ID, tgbotapi.FileReader{
		Name:   name,
		Reader: r,
	})
	_, err := bot.api.Send(doc)
	return err
}

func (bot *Bot) UploadFileFromURL(ctx context.Context, url string) error {
	ctxMsg := CtxGetMessage(ctx)
	if ctxMsg == nil {
		return ErrInvalidContext
	}
	doc := tgbotapi.NewDocument(ctxMsg.Chat.ID, tgbotapi.FileURL(url))
	_, err := bot.api.Send(doc)
	return err
}

func (bot *Bot) DownloadFile(fileID string) (io.ReadCloser, error) {
	file, err := bot.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(bot.fileEndpoint, bot.token, file.FilePath)
	return newLazyDownloader(url), nil
}

func (bot *Bot) Close() error {
	bot.api.StopReceivingUpdates()
	return nil
}

func (bot *Bot) send(c tgbotapi.Chattable) (int, bool) {
	resp, err := bot.api.Send(c)
	if err != nil {
		bot.logger.Error("failed to send message", slog.Any("err", err))
		return 0, false
	}
	return resp.MessageID, true
}

func (bot *Bot) callCommand(cmd string, ctx context.Context, args []string) error {
	if c, ok := bot.commands[cmd]; ok {
		_, err := c.Call(ctx, args)
		return err
	}
	return &commander.UnknownCommandError{Cmd: cmd}
}

func (bot *Bot) getDialog(userID, chatID int64) *Dialog {
	dataJson, err := bot.cache.Get(fmt.Sprintf("dialog:%d:%d", userID, chatID))
	if err != nil {
		bot.logger.Debug("dialog not found",
			slog.Any("userID", userID),
			slog.Any("chatID", chatID),
			slog.Any("err", err))
		return nil
	}
	dlg := &Dialog{
		userID: userID,
		chatID: chatID,
	}
	err = json.Unmarshal([]byte(dataJson), &dlg.data)
	if err != nil {
		bot.logger.Error("failed to unmarshal dialog", slog.Any("err", err))
		return nil
	}
	return dlg
}

func (bot *Bot) saveDialog(dlg *Dialog) {
	dataJson, err := json.Marshal(dlg.data)
	if err != nil {
		bot.logger.Error("failed to marshal dialog", slog.Any("err", err))
		return
	}
	err = bot.cache.Set(fmt.Sprintf("dialog:%d:%d", dlg.userID, dlg.chatID), string(dataJson), bot.dialogTTL)
	if err != nil {
		bot.logger.Error("failed to save dialog", slog.Any("err", err))
		return
	}
}

func (bot *Bot) deleteDialog(dlg *Dialog) {
	err := bot.cache.Del(fmt.Sprintf("dialog:%d:%d", dlg.userID, dlg.chatID))
	if err != nil {
		bot.logger.Error("failed to delete dialog", slog.Any("err", err))
	}
}

func (bot *Bot) handleDialog(userID, chatID int64, input any) bool {
	dlg := bot.getDialog(userID, chatID)
	if dlg == nil {
		return false
	}

	h := bot.dialogs[dlg.data.Name]
	if h == nil {
		bot.logger.Error("dialog not handled", slog.String("name", dlg.data.Name))
		bot.deleteDialog(dlg)
		return true
	}

	lastQuery := dlg.LastQuery()
	if lastQuery == nil {
		bot.logger.Error("last query missing", slog.String("name", dlg.data.Name))
		bot.deleteDialog(dlg)
		return true
	}

	ctx := context.Background()
	ctx = ctxWithBot(ctx, bot)

	switch input := input.(type) {
	case *tgbotapi.CallbackQuery:
		ctx = ctxWithMessage(ctx, input.Message)
		if choice, isDone, ok := lastQuery.getDataFromCallback(input); ok {
			if !isDone {
				dlg.flipUserChoice(choice)
				editMsg := tgbotapi.NewEditMessageText(
					input.Message.Chat.ID,
					input.Message.MessageID,
					input.Message.Text,
				)
				if kbm, ok := lastQuery.getReplyMarkup(dlg).(tgbotapi.InlineKeyboardMarkup); ok {
					editMsg.ReplyMarkup = &kbm
				}
				bot.send(editMsg)
				bot.saveDialog(dlg)
				return true
			}
		} else {
			return true
		}
	case *tgbotapi.Message:
		if !input.Chat.IsPrivate() &&
			(input.ReplyToMessage == nil || input.ReplyToMessage.MessageID != lastQuery.MessageID) {
			return false
		}
		ctx = ctxWithMessage(ctx, input)
		dlg.setUserResponse(input.Text)
	default:
		bot.logger.Error("unhandled dialog input", slog.Any("input", input))
	}

	defer func() {
		if r := recover(); r != nil {
			bot.logger.Error("dialog panic", slog.String("name", dlg.data.Name), slog.Any("panic", r))
			bot.deleteDialog(dlg)
		}
	}()
	if q := h(ctx, dlg); q != nil {
		if q.Kind == RetryQueryKind {
			return true
		}
		bot.handleQuery(dlg, q)
		bot.saveDialog(dlg)
	} else {
		bot.deleteDialog(dlg)
	}
	return true
}

func (bot *Bot) handleCommand(msg *tgbotapi.Message) {
	ctx := context.Background()
	ctx = ctxWithBot(ctx, bot)
	ctx = ctxWithMessage(ctx, msg)
	cmd := msg.Command()
	args := strings.Fields(msg.CommandArguments())
	if err := bot.callCommand(cmd, ctx, args); err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, err.Error())
		if !msg.Chat.IsPrivate() {
			reply.ReplyToMessageID = msg.MessageID
		}
		bot.send(reply)
	}
}

func (bot *Bot) handleMessage(msg *tgbotapi.Message) {
	if !bot.handleDialog(msg.From.ID, msg.Chat.ID, msg) && bot.defaultMsgHandler != nil {
		ctx := context.Background()
		ctx = ctxWithBot(ctx, bot)
		ctx = ctxWithMessage(ctx, msg)
		err := bot.defaultMsgHandler(ctx)
		if err != nil {
			bot.logger.Error("default message handler error", slog.Any("err", err))
		}
	}
}

func (bot *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(q.ID, "")
	if q.Message == nil || !bot.handleDialog(q.From.ID, q.Message.Chat.ID, q) {
		callback.Text = "Input not handled"
	}
	if _, err := bot.api.Request(callback); err != nil {
		bot.logger.Error("callback returned error", slog.String("data", q.Data), slog.Any("err", err))
	}
}

func (bot *Bot) handleQuery(dlg *Dialog, q *Query) {
	msg, err := q.toMessage(dlg)
	if err != nil {
		bot.logger.Error("failed to convert query to message", slog.Any("err", err))
		return
	}
	if msgID, ok := bot.send(msg); ok {
		q.MessageID = msgID
		dlg.setLastQuery(*q)
	}
}
