package botkit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
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
	rand  rand.Rand
}

func NewBot(token string, opts ...BotOption) (*Bot, error) {
	bot := &Bot{
		BotOptions: defaultOptions,
		token:      token,
		rand:       *rand.New(rand.NewSource(time.Now().Unix())),
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
		if msg := update.Message; msg != nil {
			if update.Message.IsCommand() {
				bot.handleCommand(msg)
			} else if len(msg.Text) > 0 {
				bot.handleMessage(msg)
			} else if fileIDs := getFileIDsFromMessage(msg); len(fileIDs) > 0 {
				for _, fileID := range fileIDs {
					bot.handleFile(msg, fileID)
				}
			}
		}
		if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
			bot.handleCallback(update.CallbackQuery)
		}
	}
}

func (bot *Bot) SendMessage(ctx context.Context, text string, reply bool) error {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if reply {
		msg.ReplyToMessageID = bot.getReplyIDFromCtx(ctx)
	}
	_, err := bot.api.Send(msg)
	return err
}

func (bot *Bot) SendMedia(ctx context.Context, reply bool, media ...Media) error {
	if len(media) == 0 {
		return nil
	}

	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	replyID := 0
	if reply {
		replyID = bot.getReplyIDFromCtx(ctx)
	}

	if len(media) == 1 {
		msg := media[0].toChattable(chatID, replyID)
		_, err := bot.api.Send(msg)
		return err

	}

	files := make([]any, len(media))
	for i, media := range media {
		files[i] = media.toInputMedia()
	}
	group := tgbotapi.NewMediaGroup(chatID, files)
	group.ReplyToMessageID = replyID
	_, err := bot.api.SendMediaGroup(group)
	return err
}

func (bot *Bot) SendSticker(ctx context.Context, stickerSet string, num int, reply bool) error {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	stickers, err := bot.api.GetStickerSet(tgbotapi.GetStickerSetConfig{Name: stickerSet})
	if err != nil {
		return err
	}
	stickerCount := len(stickers.Stickers)
	if stickerCount == 0 {
		return fmt.Errorf("no stickers in set %q", stickerSet)
	}
	if num >= stickerCount {
		return fmt.Errorf("sticker number %d out of range (0-%d)", num, stickerCount-1)
	}
	if num < 0 {
		num = bot.rand.Intn(stickerCount)
	}
	sticker := tgbotapi.NewSticker(chatID, tgbotapi.FileID(stickers.Stickers[num].FileID))
	_, err = bot.api.Send(sticker)
	return err
}

func (bot *Bot) StartDialog(ctx context.Context, name string) error {
	h := bot.dialogs[name]
	if h == nil {
		return fmt.Errorf("unknown dialog: %s", name)
	}
	userID, chatID, ok := CtxGetUserAndChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	chat, err := bot.getChat(chatID)
	if err != nil {
		return err
	}
	username, _ := bot.getUsernameFromUserID(userID)
	dlg := &Dialog{
		userID: userID,
		chatID: chatID,
		data: dialogData{
			Name:      name,
			Username:  username,
			IsPrivate: chat.IsPrivate(),
		},
		handler: h,
	}
	updates, isDone := dlg.runHandler(ctx)
	for _, update := range updates {
		bot.sendDialogMessage(dlg, update)
	}
	if !isDone {
		bot.saveDialog(dlg)
	}
	return nil
}

func (bot *Bot) GetUserCache(ctx context.Context) (razcache.Cache, error) {
	userID, chatID, ok := CtxGetUserAndChat(ctx)
	if !ok {
		return nil, ErrInvalidContext
	}
	return bot.cache.SubCache(fmt.Sprintf("userdata:%d:%d:", userID, chatID)), nil
}

func (bot *Bot) GetTaggedUserCache(ctx context.Context, num int) (razcache.Cache, error) {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return nil, ErrInvalidContext
	}
	users, ok := CtxGetTaggedUsers(ctx)
	if !ok {
		return nil, ErrInvalidContext
	}
	if num < 0 || num >= len(users) {
		return nil, fmt.Errorf("num %d out of range (%d tagged users)", num, len(users))
	}
	return bot.cache.SubCache(fmt.Sprintf("userdata:%d:%d:", users[num], chatID)), nil
}

func (bot *Bot) GetChatCache(ctx context.Context) (razcache.Cache, error) {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return nil, ErrInvalidContext
	}
	return bot.cache.SubCache(fmt.Sprintf("chatdata:%d:", chatID)), nil
}

func (bot *Bot) UploadFile(ctx context.Context, name string, r io.Reader) error {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileReader{
		Name:   name,
		Reader: r,
	})
	_, err := bot.api.Send(doc)
	return err
}

func (bot *Bot) UploadFileFromURL(ctx context.Context, url string) error {
	chatID, ok := CtxGetChat(ctx)
	if !ok {
		return ErrInvalidContext
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileURL(url))
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

func (bot *Bot) sendDialogMessage(dlg *Dialog, msg dialogMessage) {
	c := msg.toChattable(dlg)
	msgID, ok := bot.send(c)
	if ok {
		msg.setMessageID(msgID)
	}
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
	if dlg.handler = bot.dialogs[dlg.data.Name]; dlg.handler == nil {
		bot.logger.Error("missing handler for dialog", slog.Any("dlg", dlg.data.Name))
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

func (bot *Bot) handleDialogInput(ctx context.Context, dlg *Dialog, kind dialogInputKind, data string) bool {
	defer func() {
		if r := recover(); r != nil {
			bot.logger.Error("dialog panic", slog.String("name", dlg.data.Name), slog.Any("panic", r))
			bot.deleteDialog(dlg)
		}
	}()

	ctx = ctxWithDialog(ctx, dlg)
	updates, isDone, err := dlg.handleInput(ctx, kind, data)
	if err != nil {
		if err == errInvalidDialogInput {
			return false
		}
		bot.logger.Error("dialog error", slog.String("dlg", dlg.data.Name), slog.Any("err", err))
	}
	for _, update := range updates {
		bot.sendDialogMessage(dlg, update)
	}
	if isDone {
		bot.deleteDialog(dlg)
	} else {
		bot.saveDialog(dlg)
	}
	return true
}

func (bot *Bot) handleCommand(msg *tgbotapi.Message) {
	ctx := newContext(bot, msg)
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
	if dlg := bot.getDialog(msg.From.ID, msg.Chat.ID); dlg != nil {
		if !dlg.isPrivate() {
			q := dlg.LastQuery()
			if q == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.MessageID != q.MessageID {
				goto fallback
			}
		}
		ctx := newContext(bot, msg)
		if bot.handleDialogInput(ctx, dlg, dialogInputText, msg.Text) {
			return
		}
	}
fallback:
	if bot.defaultMsgHandler != nil {
		ctx := newContext(bot, msg)
		err := bot.defaultMsgHandler(ctx, msg.Text)
		if err != nil {
			bot.logger.Error("default message handler error", slog.Any("err", err))
		}
	}
}

func (bot *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(q.ID, "Input not handled")
	if dlg := bot.getDialog(q.From.ID, q.Message.Chat.ID); dlg != nil {
		ctx := newContext(bot, q.Message)
		if bot.handleDialogInput(ctx, dlg, dialogInputCallback, q.Data) {
			callback.Text = ""
		}
	}
	if _, err := bot.api.Request(callback); err != nil {
		bot.logger.Error("callback returned error", slog.String("data", q.Data), slog.Any("err", err))
	}
}

func (bot *Bot) handleFile(msg *tgbotapi.Message, fileID string) {
	if dlg := bot.getDialog(msg.From.ID, msg.Chat.ID); dlg != nil {
		if !dlg.isPrivate() {
			q := dlg.LastQuery()
			if q == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.MessageID != q.MessageID {
				return
			}
		}
		ctx := newContext(bot, msg)
		bot.handleDialogInput(ctx, dlg, dialogInputFile, fileID)
	}
}

func (bot *Bot) getReplyIDFromCtx(ctx context.Context) int {
	if replyID, ok := CtxGetReplyID(ctx); ok {
		return replyID
	}
	if dlg := ctxGetDialog(ctx); dlg != nil {
		if q := dlg.getQueryData(dlg.data.LastQuery); q != nil {
			if q.ReplyID != 0 {
				return q.ReplyID
			}
			return q.Query.MessageID
		}
	}
	return 0
}

func (bot *Bot) getChat(chatID int64) (tgbotapi.Chat, error) {
	return bot.api.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}})
}

func (bot *Bot) getUsernameFromUserID(userID int64) (string, error) {
	chat, err := bot.getChat(userID)
	if err != nil {
		return fmt.Sprintf("user:%d", userID), err
	}
	if len(chat.UserName) > 0 {
		return chat.UserName, nil
	} else if len(chat.FirstName) > 0 {
		return chat.FirstName, nil
	} else if len(chat.LastName) > 0 {
		return chat.LastName, nil
	}
	return fmt.Sprintf("user:%d", userID), nil
}

func getFileIDsFromMessage(msg *tgbotapi.Message) (ids []string) {
	if len(msg.Photo) > 0 {
		ids = append(ids, msg.Photo[0].FileID)
	}
	if msg.Video != nil {
		ids = append(ids, msg.Video.FileID)
	}
	if msg.VideoNote != nil {
		ids = append(ids, msg.VideoNote.FileID)
	}
	if msg.Audio != nil {
		ids = append(ids, msg.Audio.FileID)
	}
	if msg.Voice != nil {
		ids = append(ids, msg.Voice.FileID)
	}
	if msg.Sticker != nil {
		ids = append(ids, msg.Sticker.FileID)
	}
	if msg.Document != nil {
		ids = append(ids, msg.Document.FileID)
	}
	return
}
