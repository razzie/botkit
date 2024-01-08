package botkit

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ctxKey string

var (
	ErrInvalidContext = fmt.Errorf("invalid context")

	ctxBot         ctxKey = "ctxBot"
	ctxUserID      ctxKey = "ctxUser"
	ctxChatID      ctxKey = "ctxChat"
	ctxReplyID     ctxKey = "ctxReplyID"
	ctxDialog      ctxKey = "ctxDialog"
	ctxTaggedUsers ctxKey = "ctxTaggedUsers"
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
	ctx := newContextWithUserAndChat(bot, msg.From.ID, msg.Chat.ID)
	ctx = context.WithValue(ctx, ctxReplyID, msg.MessageID)
	ctx = context.WithValue(ctx, ctxTaggedUsers, getTaggedUsers(msg))
	return ctx
}

func CtxGetBot(ctx context.Context) *Bot {
	if bot, ok := ctx.Value(ctxBot).(*Bot); ok {
		return bot
	}
	return nil
}

func CtxGetChat(ctx context.Context) (int64, bool) {
	chatID, ok := ctx.Value(ctxChatID).(int64)
	return chatID, ok
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

func CtxGetTaggedUsers(ctx context.Context) ([]int64, bool) {
	users, ok := ctx.Value(ctxTaggedUsers).([]int64)
	return users, ok
}

func CtxGetTaggedUserCount(ctx context.Context) (int, bool) {
	users, ok := ctx.Value(ctxTaggedUsers).([]int64)
	return len(users), ok
}

func ctxGetDialog(ctx context.Context) *Dialog {
	if dlg, ok := ctx.Value(ctxDialog).(*Dialog); ok {
		return dlg
	}
	return nil
}

func getTaggedUsers(msg *tgbotapi.Message) (users []int64) {
	for _, entity := range msg.Entities {
		if entity.User != nil {
			users = append(users, entity.User.ID)
		}
	}
	return
}
