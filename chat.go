package botkit

import (
	"fmt"
	"io"

	"github.com/razzie/razcache"
)

type Chat struct {
	bot    *Bot
	chatID int64
}

func newChat(bot *Bot, chatID int64) Chat {
	return Chat{
		bot:    bot,
		chatID: chatID,
	}
}

func (chat Chat) SendMessage(format string, args ...any) error {
	return chat.bot.sendMessage(chat.chatID, fmt.Sprintf(format, args...), 0)
}

func (chat Chat) SendMedia(media ...Media) error {
	return chat.bot.sendMedia(chat.chatID, 0, media...)
}

func (chat Chat) SendSticker(stickerSet string, num int) error {
	return chat.bot.sendSticker(chat.chatID, stickerSet, num, 0)
}

func (chat Chat) UploadFile(name string, r io.Reader) error {
	return chat.bot.uploadFile(chat.chatID, name, r)
}

func (chat Chat) UploadFileFromURL(url string) error {
	return chat.bot.uploadFileFromURL(chat.chatID, url)
}

func (chat Chat) GetCache() (razcache.Cache, error) {
	return chat.bot.getChatCache(chat.chatID)
}

func (chat Chat) GetID() int64 {
	return chat.chatID
}
