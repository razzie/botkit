package botkit

import (
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

func (bot *Bot) Close() error {
	bot.api.StopReceivingUpdates()
	return nil
}

func (bot *Bot) GetChat(chatID int64) Chat {
	return newChat(bot, chatID)
}

func (bot *Bot) getChatCache(chatID int64) (razcache.Cache, error) {
	return bot.cache.SubCache(fmt.Sprintf("chatdata:%d:", chatID)), nil
}

func (bot *Bot) getUserCache(userID, chatID int64) (razcache.Cache, error) {
	return bot.cache.SubCache(fmt.Sprintf("userdata:%d:%d:", userID, chatID)), nil
}

func (bot *Bot) getChatData(chatID int64) (tgbotapi.Chat, error) {
	return bot.api.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: chatID}})
}

func (bot *Bot) getUsernameFromUserID(userID int64) (string, error) {
	chat, err := bot.getChatData(userID)
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

func (bot *Bot) sendMessage(chatID int64, text string, replyID int) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyID
	_, err := bot.api.Send(msg)
	return err
}

func (bot *Bot) sendMedia(chatID int64, replyID int, media ...Media) error {
	if len(media) == 0 {
		return nil
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

func (bot *Bot) sendSticker(chatID int64, stickerSet string, num int, replyID int) error {
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
	sticker.ReplyToMessageID = replyID
	_, err = bot.api.Send(sticker)
	return err
}

func (bot *Bot) uploadFile(chatID int64, name string, r io.Reader) error {
	if rc, ok := r.(io.ReadCloser); ok {
		// tgbotapi might or might not close the reader, so let's do it only here
		r = &wrappedReader{Reader: r}
		defer rc.Close()
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileReader{
		Name:   name,
		Reader: r,
	})
	_, err := bot.api.Send(doc)
	return err
}

func (bot *Bot) uploadFileFromURL(chatID int64, url string) error {
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

func (bot *Bot) startDialog(ctx *Context, name string) error {
	h := bot.dialogs[name]
	if h == nil {
		return fmt.Errorf("unknown dialog: %s", name)
	}
	chat, err := bot.getChatData(ctx.chatID)
	if err != nil {
		return err
	}
	username, _ := bot.getUsernameFromUserID(ctx.userID)
	dlg := &Dialog{
		userID: ctx.userID,
		chatID: ctx.chatID,
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

func (bot *Bot) getDialog(userID, chatID int64) *Dialog {
	dataJson, err := bot.cache.Get(fmt.Sprintf("dialog:%d:%d", userID, chatID))
	if err != nil {
		if err != razcache.ErrNotFound {
			bot.logger.Error("dialog not found",
				slog.Any("userID", userID),
				slog.Any("chatID", chatID),
				slog.Any("err", err))
		}
		return nil
	}
	dlg := &Dialog{
		userID: userID,
		chatID: chatID,
	}
	err = json.Unmarshal([]byte(dataJson), &dlg.data)
	if err != nil {
		bot.logger.Error("failed to unmarshal dialog", slogDialog(dlg), slog.Any("err", err))
		return nil
	}
	if dlg.handler = bot.dialogs[dlg.data.Name]; dlg.handler == nil {
		bot.logger.Error("missing handler for dialog", slogDialog(dlg))
		return nil
	}
	return dlg
}

func (bot *Bot) saveDialog(dlg *Dialog) {
	dataJson, err := json.Marshal(dlg.data)
	if err != nil {
		bot.logger.Error("failed to marshal dialog", slogDialog(dlg), slog.Any("err", err))
		return
	}
	err = bot.cache.Set(fmt.Sprintf("dialog:%d:%d", dlg.userID, dlg.chatID), string(dataJson), bot.dialogTTL)
	if err != nil {
		bot.logger.Error("failed to save dialog", slogDialog(dlg), slog.Any("err", err))
		return
	}
}

func (bot *Bot) deleteDialog(dlg *Dialog) {
	err := bot.cache.Del(fmt.Sprintf("dialog:%d:%d", dlg.userID, dlg.chatID))
	if err != nil {
		bot.logger.Error("failed to delete dialog", slogDialog(dlg), slog.Any("err", err))
	}
}

func (bot *Bot) handleDialogInput(ctx *Context, dlg *Dialog, kind dialogInputKind, data string) bool {
	defer func() {
		if r := recover(); r != nil {
			bot.logger.Error("dialog panic", slogContext(ctx), slogDialog(dlg), slog.Any("panic", r))
			bot.deleteDialog(dlg)
		}
	}()

	ctx.dlg = dlg
	updates, isDone, err := dlg.handleInput(ctx, kind, data)
	if err != nil {
		if err == errInvalidDialogInput {
			return false
		}
		bot.logger.Error("dialog error", slogDialog(dlg), slog.Any("err", err))
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

func (bot *Bot) callCommand(cmd string, ctx *Context, args []string) error {
	if c, ok := bot.commands[cmd]; ok {
		resps, err := c.Call(ctx, args)
		handleCommandResponses(ctx, resps)
		return err
	}
	return &commander.UnknownCommandError{Cmd: cmd}
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
			bot.logger.Error("default message handler error", slogMessage(msg), slog.Any("err", err))
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
		bot.logger.Error("callback returned error", slogCallbackQuery(q), slog.Any("err", err))
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
