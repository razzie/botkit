package botkit

import (
	"io"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Media interface {
	toChattable(chatID int64, replyID int) tgbotapi.Chattable
	toInputMedia() any
}

type MediaSource interface {
	toRequestFileData() tgbotapi.RequestFileData
}

type wrapperMediaSource struct {
	data tgbotapi.RequestFileData
}

type BaseMedia struct {
	File    MediaSource
	Thumb   MediaSource
	Caption string
}

type Photo struct {
	BaseMedia
}

type Video struct {
	BaseMedia
	Duration          int
	SupportsStreaming bool
}

type Audio struct {
	BaseMedia
	Duration  int
	Performer string
	Title     string
}

func FileSource(name string, r io.Reader) MediaSource {
	return &wrapperMediaSource{data: tgbotapi.FileReader{Name: name, Reader: r}}
}

func URLSource(url string) MediaSource {
	return &wrapperMediaSource{data: tgbotapi.FileURL(url)}
}

func NewPhoto(file MediaSource) *Photo {
	return &Photo{
		BaseMedia: BaseMedia{File: file},
	}
}

func (photo *Photo) toChattable(chatID int64, replyID int) tgbotapi.Chattable {
	cfg := tgbotapi.NewPhoto(chatID, photo.File.toRequestFileData())
	if photo.Thumb != nil {
		cfg.Thumb = photo.Thumb.toRequestFileData()
	}
	cfg.Caption = photo.Caption
	cfg.ReplyToMessageID = replyID
	return cfg
}

func (photo *Photo) toInputMedia() any {
	cfg := tgbotapi.NewInputMediaPhoto(photo.File.toRequestFileData())
	cfg.Caption = photo.Caption
	return cfg
}

func NewVideo(file MediaSource) *Video {
	return &Video{
		BaseMedia: BaseMedia{File: file},
	}
}

func (video *Video) toChattable(chatID int64, replyID int) tgbotapi.Chattable {
	cfg := tgbotapi.NewVideo(chatID, video.File.toRequestFileData())
	if video.Thumb != nil {
		cfg.Thumb = video.Thumb.toRequestFileData()
	}
	cfg.Caption = video.Caption
	cfg.ReplyToMessageID = replyID
	cfg.Duration = video.Duration
	cfg.SupportsStreaming = video.SupportsStreaming
	return cfg
}

func (video *Video) toInputMedia() any {
	cfg := tgbotapi.NewInputMediaVideo(video.File.toRequestFileData())
	if video.Thumb != nil {
		cfg.Thumb = video.Thumb.toRequestFileData()
	}
	cfg.Caption = video.Caption
	cfg.Duration = video.Duration
	cfg.SupportsStreaming = video.SupportsStreaming
	return cfg
}

func NewAudio(file MediaSource) *Audio {
	return &Audio{
		BaseMedia: BaseMedia{File: file},
	}
}

func (audio *Audio) toChattable(chatID int64, replyID int) tgbotapi.Chattable {
	cfg := tgbotapi.NewAudio(chatID, audio.File.toRequestFileData())
	if audio.Thumb != nil {
		cfg.Thumb = audio.Thumb.toRequestFileData()
	}
	cfg.Caption = audio.Caption
	cfg.ReplyToMessageID = replyID
	cfg.Duration = audio.Duration
	cfg.Performer = audio.Performer
	cfg.Title = audio.Title
	return cfg
}

func (audio *Audio) toInputMedia() any {
	cfg := tgbotapi.NewInputMediaAudio(audio.File.toRequestFileData())
	if audio.Thumb != nil {
		cfg.Thumb = audio.Thumb.toRequestFileData()
	}
	cfg.Caption = audio.Caption
	cfg.Duration = audio.Duration
	cfg.Performer = audio.Performer
	cfg.Title = audio.Title
	return cfg
}

func (w wrapperMediaSource) toRequestFileData() tgbotapi.RequestFileData {
	return w.data
}
