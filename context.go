package botkit

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ctxKey string

var (
	ErrInvalidContext = fmt.Errorf("invalid context")

	ctxBot     ctxKey = "ctxBot"
	ctxUserID  ctxKey = "ctxUser"
	ctxChatID  ctxKey = "ctxChat"
	ctxReplyID ctxKey = "ctxReplyID"
	ctxDialog  ctxKey = "ctxDialog"
)

func newContextWithUserAndChat(bot *Bot, userID, chatID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxBot, bot)
	ctx = context.WithValue(ctx, ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxChatID, chatID)
	return ctx
}

func newDialogContext(bot *Bot, dlg *Dialog) context.Context {
	ctx := newContextWithUserAndChat(bot, dlg.userID, dlg.chatID)
	ctx = context.WithValue(ctx, ctxDialog, dlg)
	return ctx
}

func newContextWithMessage(bot *Bot, msg *tgbotapi.Message) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxBot, bot)
	ctx = context.WithValue(ctx, ctxUserID, msg.From.ID)
	ctx = context.WithValue(ctx, ctxChatID, msg.Chat.ID)
	ctx = context.WithValue(ctx, ctxReplyID, msg.MessageID)
	return ctx
}

func CtxGetBot(ctx context.Context) *Bot {
	if bot, ok := ctx.Value(ctxBot).(*Bot); ok {
		return bot
	}
	return nil
}

func CtxGetUserAndChat(ctx context.Context) (int64, int64, bool) {
	userID, ok1 := ctx.Value(ctxUserID).(int64)
	chatID, ok2 := ctx.Value(ctxChatID).(int64)
	return userID, chatID, ok1 && ok2
}

func CtxGetReplyID(ctx context.Context) (int, bool) {
	replyID, ok := ctx.Value(ctxReplyID).(int)
	return replyID, ok
}

func ctxGetDialog(ctx context.Context) *Dialog {
	if dlg, ok := ctx.Value(ctxDialog).(*Dialog); ok {
		return dlg
	}
	return nil
}
