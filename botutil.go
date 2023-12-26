package botkit

import (
	"context"
	"fmt"
	"time"
)

func SendMessage(ctx context.Context, format string, args ...any) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.SendMessage(ctx, fmt.Sprintf(format, args...), false)
}

func SendReply(ctx context.Context, format string, args ...any) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.SendMessage(ctx, fmt.Sprintf(format, args...), true)
}

func StartDialog(ctx context.Context, name string) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.StartDialog(ctx, name)
}

func SetUserData(ctx context.Context, key, value string, ttl time.Duration) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.SetUserData(ctx, key, value, ttl)
}

func GetUserData(ctx context.Context, key string) (string, error) {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return "", ErrInvalidContext
	}
	return bot.GetUserData(ctx, key)
}
