package botkit

import (
	"context"
	"fmt"
)

func SendMessage(ctx context.Context, format string, args ...any) error {
	bot := CtxGetBot(ctx)
	msg := CtxGetMessage(ctx)
	if bot == nil || msg == nil {
		return ErrInvalidContext
	}
	return bot.SendMessage(msg.Chat.ID, fmt.Sprintf(format, args...), 0)
}

func SendReply(ctx context.Context, format string, args ...any) error {
	bot := CtxGetBot(ctx)
	msg := CtxGetMessage(ctx)
	if bot == nil || msg == nil {
		return ErrInvalidContext
	}
	return bot.SendMessage(msg.Chat.ID, fmt.Sprintf(format, args...), msg.MessageID)
}
