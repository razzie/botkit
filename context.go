package botkit

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ctxKey string

var (
	ctxBot     ctxKey = "ctxBot"
	ctxMessage ctxKey = "ctxMessage"
)

func ctxWithBot(ctx context.Context, bot *Bot) context.Context {
	return context.WithValue(ctx, ctxBot, bot)
}

func CtxGetBot(ctx context.Context) *Bot {
	if bot, ok := ctx.Value(ctxBot).(*Bot); ok {
		return bot
	}
	return nil
}

func ctxWithMessage(ctx context.Context, msg *tgbotapi.Message) context.Context {
	return context.WithValue(ctx, ctxMessage, msg)
}

func CtxGetMessage(ctx context.Context) *tgbotapi.Message {
	if msg, ok := ctx.Value(ctxMessage).(*tgbotapi.Message); ok {
		return msg
	}
	return nil
}
