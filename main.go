package main

import (
	"context"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/razzie/commander"
)

type ctxKey string

var ctxMessage ctxKey = "ctxMessage"

func injectMessage(ctx context.Context, msg *tgbotapi.Message) context.Context {
	return context.WithValue(ctx, ctxMessage, msg)
}

func getMessage(ctx context.Context) *tgbotapi.Message {
	if msg, ok := ctx.Value(ctxMessage).(*tgbotapi.Message); ok {
		return msg
	}
	return nil
}

func main() {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(err)
	}

	send := func(ctx context.Context, msg string) {
		if _, err := bot.Send(tgbotapi.NewMessage(getMessage(ctx).Chat.ID, msg)); err != nil {
			panic(err)
		}
	}

	add := func(ctx context.Context, nums ...int) {
		sum := 0
		for _, n := range nums {
			sum += n
		}
		if _, err := bot.Send(tgbotapi.NewMessage(getMessage(ctx).Chat.ID, strconv.Itoa(sum))); err != nil {
			panic(err)
		}
	}

	cmdr := commander.NewCommander()
	cmdr.RegisterCommand("send", send)
	cmdr.RegisterCommand("add", add)

	numericKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1", "1"),
			tgbotapi.NewInlineKeyboardButtonData("2", "2"),
			tgbotapi.NewInlineKeyboardButtonData("3", "3"),
		),
	)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {

		if update.Message != nil {
			if update.Message.IsCommand() {
				ctx := injectMessage(context.Background(), update.Message)
				args := strings.Fields(update.Message.CommandArguments())
				_, err := cmdr.Call(ctx, update.Message.Command(), args)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
					msg.ReplyToMessageID = update.Message.MessageID
					if _, err := bot.Send(msg); err != nil {
						panic(err)
					}
				}

			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
				msg.ReplyToMessageID = update.Message.MessageID
				msg.ReplyMarkup = numericKeyboard
				if _, err := bot.Send(msg); err != nil {
					panic(err)
				}
			}
		}

		if update.CallbackQuery != nil {
			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
			if _, err := bot.Request(callback); err != nil {
				panic(err)
			}

			editMsg := tgbotapi.NewEditMessageText(
				update.CallbackQuery.Message.Chat.ID,
				update.CallbackQuery.Message.MessageID,
				update.CallbackQuery.Message.Text,
			)
			editMsg.ReplyMarkup = nil
			if _, err := bot.Send(editMsg); err != nil {
				panic(err)
			}

			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Data)
			if _, err := bot.Send(msg); err != nil {
				panic(err)
			}
		}

	}
}
