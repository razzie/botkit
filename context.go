package botkit

import (
	"context"
	"fmt"
	"io"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/razcache"
)

type Context struct {
	context.Context
	bot         *Bot
	userID      int64
	chatID      int64
	replyID     int
	dlg         *Dialog
	taggedUsers []int64
}

func resolveContext(ctx context.Context) (*Context, error) {
	return ctx.(*Context), nil
}

func newContext(bot *Bot, msg *tgbotapi.Message) *Context {
	return &Context{
		bot:         bot,
		userID:      msg.From.ID,
		chatID:      msg.Chat.ID,
		replyID:     msg.MessageID,
		taggedUsers: getTaggedUsers(msg),
	}
}

func (ctx *Context) SendMessage(format string, args ...any) error {
	return ctx.bot.sendMessage(ctx.chatID, fmt.Sprintf(format, args...), 0)
}

func (ctx *Context) SendReply(format string, args ...any) error {
	return ctx.bot.sendMessage(ctx.chatID, fmt.Sprintf(format, args...), ctx.replyID)
}

func (ctx *Context) SendMedia(media ...Media) error {
	return ctx.bot.sendMedia(ctx.chatID, 0, media...)
}

func (ctx *Context) ReplyMedia(media ...Media) error {
	return ctx.bot.sendMedia(ctx.chatID, ctx.replyID, media...)
}

func (ctx *Context) SendSticker(stickerSet string, num int) error {
	return ctx.bot.sendSticker(ctx.chatID, stickerSet, num, 0)
}

func (ctx *Context) ReplySticker(stickerSet string, num int) error {
	return ctx.bot.sendSticker(ctx.chatID, stickerSet, num, ctx.replyID)
}

func (ctx *Context) StartDialog(name string) error {
	return ctx.bot.startDialog(ctx, name)
}

func (ctx *Context) GetChatCache() (razcache.Cache, error) {
	return ctx.bot.getChatCache(ctx.chatID)
}

func (ctx *Context) GetUserCache() (razcache.Cache, error) {
	return ctx.bot.getUserCache(ctx.chatID, ctx.chatID)
}

func (ctx *Context) GetTaggedUserCache(num int) (razcache.Cache, error) {
	if num < 0 || num >= len(ctx.taggedUsers) {
		return nil, fmt.Errorf("num %d out of range (%d tagged users)", num, len(ctx.taggedUsers))
	}
	return ctx.bot.getUserCache(ctx.taggedUsers[num], ctx.chatID)
}

func (ctx *Context) GetTaggedUserCount() int {
	return len(ctx.taggedUsers)
}

func (ctx *Context) UploadFile(name string, r io.Reader) error {
	return ctx.bot.uploadFile(ctx.chatID, name, r)
}

func (ctx *Context) UploadFileFromURL(url string) error {
	return ctx.bot.uploadFileFromURL(ctx.chatID, url)
}

func (ctx *Context) DownloadFile(fileID string) (io.ReadCloser, error) {
	return ctx.bot.downloadFile(fileID)
}
