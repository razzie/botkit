package botkit

import (
	"context"
	"io"
	"log/slog"

	"github.com/razzie/commander"
)

var cmdContextResolver = commander.ContextResolverFunc(resolveContext)

type CommandResponse func(*Context) error

func StartDialog(name string) CommandResponse {
	return func(ctx *Context) error {
		return ctx.StartDialog(name)
	}
}

func SendMessage(format string, args ...any) CommandResponse {
	return func(ctx *Context) error {
		return ctx.SendMessage(format, args...)
	}
}

func SendReply(format string, args ...any) CommandResponse {
	return func(ctx *Context) error {
		return ctx.SendReply(format, args...)
	}
}

func SendMedia(media ...Media) CommandResponse {
	return func(ctx *Context) error {
		return ctx.SendMedia(media...)
	}
}

func ReplyMedia(media ...Media) CommandResponse {
	return func(ctx *Context) error {
		return ctx.ReplyMedia(media...)
	}
}

func SendSticker(stickerSet string, num int) CommandResponse {
	return func(ctx *Context) error {
		return ctx.SendSticker(stickerSet, num)
	}
}

func ReplySticker(stickerSet string, num int) CommandResponse {
	return func(ctx *Context) error {
		return ctx.ReplySticker(stickerSet, num)
	}
}

func UploadFile(name string, r io.Reader) CommandResponse {
	return func(ctx *Context) error {
		return ctx.UploadFile(name, r)
	}
}

func UploadFileFromURL(url string) CommandResponse {
	return func(ctx *Context) error {
		return ctx.UploadFileFromURL(url)
	}
}

func (cresp CommandResponse) handle(ctx *Context) {
	if cresp == nil {
		return
	}
	if err := cresp(ctx); err != nil {
		ctx.bot.logger.Error("command response failed", slog.Any("err", err))
	}
}

func handleCommandResponses(ctx *Context, resps []any) {
	for _, resp := range resps {
		switch resp := resp.(type) {
		case CommandResponse:
			resp.handle(ctx)
		case []CommandResponse:
			for _, resp := range resp {
				resp.handle(ctx)
			}
		}
	}
}

func resolveContext(ctx context.Context) (*Context, error) {
	return ctx.(*Context), nil
}
