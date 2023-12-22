package botkit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/commander"
	"github.com/razzie/razcache"
)

type Bot struct {
	BotOptions
	cache razcache.Cache
	api   *tgbotapi.BotAPI
}

func NewBot(token string, opts ...BotOption) (*Bot, error) {
	bot := &Bot{
		BotOptions: defaultOptions,
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
		user: ctxMsg.From,
		chat: ctxMsg.Chat,
	}
	if q := h(dlg); q != nil {
		bot.handleQuery(dlg, q)
		bot.saveDialog(dlg)
	}
	return nil
}

func (bot *Bot) Close() error {
	bot.api.StopReceivingUpdates()
	return nil
}

func (bot *Bot) callCommand(cmd string, ctx context.Context, args []string) error {
	if c, ok := bot.commands[cmd]; ok {
		_, err := c.Call(ctx, args)
		return err
	}
	return &commander.UnknownCommandError{Cmd: cmd}
}

func (bot *Bot) getDialog(user *tgbotapi.User, chat *tgbotapi.Chat) *Dialog {
	dataJson, err := bot.cache.Get(fmt.Sprintf("dialog:%d:%d", user.ID, chat.ID))
	if err != nil {
		bot.logger.Error("dialog not found", slog.Any("err", err))
		return nil
	}
	dlg := &Dialog{
		user: user,
		chat: chat,
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
	err = bot.cache.Set(fmt.Sprintf("dialog:%d:%d", dlg.user.ID, dlg.chat.ID), string(dataJson), bot.dialogTTL)
	if err != nil {
		bot.logger.Error("failed to save dialog", slog.Any("err", err))
		return
	}
}

func (bot *Bot) handleDialog(user *tgbotapi.User, chat *tgbotapi.Chat, response string) bool {
	dlg := bot.getDialog(user, chat)
	if dlg == nil {
		return false
	}
	h := bot.dialogs[dlg.data.Name]
	if h == nil {
		bot.logger.Error("dialog not handled", slog.String("name", dlg.data.Name))
		return false
	}
	defer bot.saveDialog(dlg)
	if q, ok := dlg.data.Queries[dlg.data.LastQuery]; ok {
		switch q.Kind {
		case SingleChoiceQuery:
			choice, _ := strconv.Atoi(response)
			dlg.data.ChoiceResponses[q.Name][choice] = true
		case MultiChoiceQuery:
			if response != "done" {
				choice, _ := strconv.Atoi(response)
				dlg.data.ChoiceResponses[q.Name][choice] = !dlg.data.ChoiceResponses[q.Name][choice]

				// update buttons !!!

				return true
			}
		}
	}
	if q := h(dlg); q != nil {
		bot.handleQuery(dlg, q)
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
		msg := tgbotapi.NewMessage(msg.Chat.ID, err.Error())
		//msg.ReplyToMessageID = msg.MessageID
		if _, err := bot.api.Send(msg); err != nil {
			bot.logger.Error("command returned error", slog.String("cmd", cmd), slog.Any("err", err))
		}
	}
}

func (bot *Bot) handleMessage(msg *tgbotapi.Message) {
	if !bot.handleDialog(msg.From, msg.Chat, msg.Text) && bot.defaultMsgHandler != nil {
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
	if bot.handleDialog(q.From, q.Message.Chat, q.Data) {
		callback := tgbotapi.NewCallback(q.ID, q.Data)
		if _, err := bot.api.Request(callback); err != nil {
			bot.logger.Error("callback returned error", slog.String("data", q.Data), slog.Any("err", err))
		}
	}
}

func (bot *Bot) handleQuery(dlg *Dialog, q *Query) {
	msg := q.toMessage()
	if msg == nil {
		bot.logger.Error("nothing sent")
		return
	}
	resp, err := bot.api.Send(msg)
	if err != nil {
		bot.logger.Error("failed to send message", slog.Any("err", err))
		return
	}
	q.MessageID = resp.MessageID
	dlg.data.LastQuery = q.Name
	dlg.data.Queries[q.Name] = *q
}
