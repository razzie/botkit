package botkit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/razzie/razcache"
)

type lazyDownloader struct {
	reader io.ReadCloser
	init   func() (io.ReadCloser, error)
}

func newLazyDownloader(url string) *lazyDownloader {
	init := func() (io.ReadCloser, error) {
		resp, err := http.Get(url)
		if err != nil {
			return nil, errors.Unwrap(err) // try not to leak url with the bot token
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%s", resp.Status)
		}
		return resp.Body, nil
	}
	return &lazyDownloader{init: init}
}

func (dl *lazyDownloader) Read(p []byte) (int, error) {
	if dl.reader == nil {
		reader, err := dl.init()
		if err != nil {
			return 0, err
		}
		dl.reader = reader
	}
	return dl.reader.Read(p)
}

func (dl *lazyDownloader) Close() error {
	return dl.reader.Close()
}

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

func GetUserCache(ctx context.Context) (razcache.Cache, error) {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return nil, ErrInvalidContext
	}
	return bot.GetUserCache(ctx)
}

func GetChatCache(ctx context.Context) (razcache.Cache, error) {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return nil, ErrInvalidContext
	}
	return bot.GetChatCache(ctx)
}

func UploadFile(ctx context.Context, name string, r io.Reader) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.UploadFile(ctx, name, r)
}

func UploadFileFromURL(ctx context.Context, url string) error {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return ErrInvalidContext
	}
	return bot.UploadFileFromURL(ctx, url)
}

func DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	bot := CtxGetBot(ctx)
	if bot == nil {
		return nil, ErrInvalidContext
	}
	return bot.DownloadFile(fileID)
}
