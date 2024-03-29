package botkit

import (
	"context"
	"log/slog"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/commander"
)

var defaultOptions = BotOptions{
	apiEndpoint:  tgbotapi.APIEndpoint,
	fileEndpoint: tgbotapi.FileEndpoint,
	logger:       slog.Default(),
	timeout:      30,
	dialogTTL:    time.Hour * 24,
}

type BotOption func(*BotOptions)

type BotOptions struct {
	apiEndpoint       string
	fileEndpoint      string
	redisDSN          string
	logger            *slog.Logger
	offset            int
	timeout           int
	commands          map[string]*commander.Command
	dialogs           map[string]DialogHandler
	dialogTTL         time.Duration
	defaultMsgHandler func(context.Context, string) error
}

func WithAPIEndpoint(apiEndpoint string) BotOption {
	return func(bo *BotOptions) {
		if !strings.HasPrefix(apiEndpoint, "http://") && !strings.HasPrefix(apiEndpoint, "https://") {
			apiEndpoint = "http://" + apiEndpoint
		}
		if !strings.HasSuffix(apiEndpoint, "/") {
			apiEndpoint += "/"
		}
		bo.apiEndpoint = apiEndpoint + "bot%s/%s"
		bo.fileEndpoint = apiEndpoint + "file/bot%s/%s"
	}
}

func WithRedisDSN(redisDSN string) BotOption {
	return func(bo *BotOptions) {
		bo.redisDSN = redisDSN
	}
}

func WithLogger(logger *slog.Logger) BotOption {
	return func(bo *BotOptions) {
		bo.logger = logger
	}
}

func WithOffset(offset int) BotOption {
	return func(bo *BotOptions) {
		bo.offset = offset
	}
}

func WithTimeout(timeout int) BotOption {
	return func(bo *BotOptions) {
		bo.timeout = timeout
	}
}

func WithCommand(cmd string, callback any) BotOption {
	return func(bo *BotOptions) {
		if bo.commands == nil {
			bo.commands = make(map[string]*commander.Command)
		}
		if c, err := commander.NewCommand(callback, cmdContextResolver); err != nil {
			bo.logger.Error("failed to create command %s: %v", cmd, err)
		} else {
			bo.commands[cmd] = c
		}
	}
}

func WithDialog(name string, h DialogHandler) BotOption {
	return func(bo *BotOptions) {
		if bo.dialogs == nil {
			bo.dialogs = make(map[string]DialogHandler)
		}
		bo.dialogs[name] = h
	}
}

func WithDialogTTL(ttl time.Duration) BotOption {
	return func(bo *BotOptions) {
		bo.dialogTTL = ttl
	}
}

func WithDefaultMessageHandler(h func(context.Context, string) error) BotOption {
	return func(bo *BotOptions) {
		bo.defaultMsgHandler = h
	}
}
