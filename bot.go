package botkit

import (
	"context"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/commander"
	"github.com/razzie/razcache"
)

var defaultOptions = BotOptions{
	apiEndpoint: tgbotapi.APIEndpoint,
	logger:      slog.Default(),
	timeout:     30,
}

type BotOption func(*BotOptions)

type BotOptions struct {
	apiEndpoint string
	redisDSN    string
	logger      *slog.Logger
	offset      int
	timeout     int
}

type Bot struct {
	BotOptions
	cache razcache.Cache
	api   *tgbotapi.BotAPI
	cmdr  commander.Commander
}

func WithAPIEndpoint(apiEndpoint string) BotOption {
	return func(bo *BotOptions) {
		bo.apiEndpoint = apiEndpoint
	}
}

func WithRedisDSN(redisDSN string) BotOption {
	return func(bo *BotOptions) {
		bo.redisDSN = redisDSN
	}
}

func WithLogger(logger *slog.Logger) BotOption {
	return func(bo *BotOptions) {
		bo.logger = logger
	}
}

func WithOffset(offset int) BotOption {
	return func(bo *BotOptions) {
		bo.offset = offset
	}
}

func WithTimeout(timeout int) BotOption {
	return func(bo *BotOptions) {
		bo.timeout = timeout
	}
}

func NewBot(token string, opts ...BotOption) (*Bot, error) {
	bot := &Bot{
		BotOptions: defaultOptions,
		cmdr:       *commander.NewCommander(),
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

func (bot *Bot) handleMessage(msg *tgbotapi.Message) {

}

func (bot *Bot) handleCommand(msg *tgbotapi.Message) {
	ctx := context.Background()
	ctx = ctxWithBot(ctx, bot)
	ctx = ctxWithMessage(ctx, msg)
	cmd := msg.Command()
	args := strings.Fields(msg.CommandArguments())
	_, err := bot.cmdr.Call(ctx, cmd, args)
	if err != nil {
		msg := tgbotapi.NewMessage(msg.Chat.ID, err.Error())
		//msg.ReplyToMessageID = msg.MessageID
		if _, err := bot.api.Send(msg); err != nil {
			bot.logger.Error("command %q returned error: %v", cmd, err)
		}
	}
}

func (bot *Bot) handleCallback(q *tgbotapi.CallbackQuery) {
	callback := tgbotapi.NewCallback(q.ID, q.Data)
	if _, err := bot.api.Request(callback); err != nil {
		bot.logger.Error("callback %q returned error: %v", q.Data, err)
	}
}

func (bot *Bot) SendMessage(chatID int64, text string, replyToMessageID int) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyToMessageID
	_, err := bot.api.Send(msg)
	return err
}

func (bot *Bot) Close() error {
	bot.api.StopReceivingUpdates()
	return nil
}
