package botkit

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	if dl.reader == nil {
		return nil
	}
	err := dl.reader.Close()
	dl.reader = nil
	return err
}

func getTaggedUsers(msg *tgbotapi.Message) (users []int64) {
	for _, entity := range msg.Entities {
		if entity.User != nil {
			users = append(users, entity.User.ID)
		}
	}
	return
}

func getFileIDsFromMessage(msg *tgbotapi.Message) (ids []string) {
	if len(msg.Photo) > 0 {
		ids = append(ids, msg.Photo[0].FileID)
	}
	if msg.Video != nil {
		ids = append(ids, msg.Video.FileID)
	}
	if msg.VideoNote != nil {
		ids = append(ids, msg.VideoNote.FileID)
	}
	if msg.Audio != nil {
		ids = append(ids, msg.Audio.FileID)
	}
	if msg.Voice != nil {
		ids = append(ids, msg.Voice.FileID)
	}
	if msg.Sticker != nil {
		ids = append(ids, msg.Sticker.FileID)
	}
	if msg.Document != nil {
		ids = append(ids, msg.Document.FileID)
	}
	return
}

type wrapperDialogMessage struct {
	c tgbotapi.Chattable
}

func newMessageFromChattable(c tgbotapi.Chattable) dialogMessage {
	return &wrapperDialogMessage{c: c}
}

func (m *wrapperDialogMessage) toChattable(*Dialog) tgbotapi.Chattable {
	return m.c
}

func (*wrapperDialogMessage) setMessageID(int) {
}
