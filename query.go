package botkit

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	TextInputQueryKind QueryKind = iota
	SingleChoiceQueryKind
	MultiChoiceQueryKind
	FileInputQueryKind
	RetryQueryKind
)

var RetryQuery = &Query{Kind: RetryQueryKind}

type QueryKind int

type Query struct {
	Name      string    `json:"name"`
	Kind      QueryKind `json:"kind"`
	Text      string    `json:"text"`
	Choices   []string  `json:"choices,omitempty"`
	MessageID int       `json:"message_id,omitempty"`
}

func NewTextInputQuery(name, text string) *Query {
	return &Query{
		Name: name,
		Kind: TextInputQueryKind,
		Text: text,
	}
}

func NewSingleChoiceQuery(name, text string, choices ...string) *Query {
	return &Query{
		Name:    name,
		Kind:    SingleChoiceQueryKind,
		Text:    text,
		Choices: choices,
	}
}

func NewMultiChoiceQuery(name, text string, choices ...string) *Query {
	return &Query{
		Name:    name,
		Kind:    MultiChoiceQueryKind,
		Text:    text,
		Choices: choices,
	}
}

func NewFileInputQuery(name, text string) *Query {
	return &Query{
		Name: name,
		Kind: FileInputQueryKind,
		Text: text,
	}
}

func (qk QueryKind) HasTextResponse() bool {
	switch qk {
	case TextInputQueryKind, FileInputQueryKind:
		return true
	default:
		return false
	}
}

func (qk QueryKind) HasChoiceResponse() bool {
	switch qk {
	case SingleChoiceQueryKind, MultiChoiceQueryKind:
		return true
	default:
		return false
	}
}

func (q *Query) toChattable(dlg *Dialog) tgbotapi.Chattable {
	msgText := q.getMessageText(dlg)
	msg := tgbotapi.NewMessage(dlg.chatID, msgText)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	msg.ReplyMarkup = q.getReplyMarkup(dlg)
	return &msg
}

func (q *Query) setMessageID(messageID int) {
	q.MessageID = messageID
}

func (q *Query) getMessageText(dlg *Dialog) string {
	msgText := q.Text
	if !dlg.isPrivate() {
		msgText = fmt.Sprintf("[%s](tg://user?id=%d) %s", dlg.data.Username, dlg.userID, msgText)
	}
	return msgText
}

func (q *Query) getReplyMarkup(dlg *Dialog) any {
	switch q.Kind {
	case SingleChoiceQueryKind:
		buttons := make([][]tgbotapi.InlineKeyboardButton, 0, len(q.Choices))
		for i, choice := range q.Choices {
			btnText := choice
			btnData := strconv.Itoa(i) + ":" + q.Name
			buttons = append(buttons, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData(btnText, btnData)})
		}
		return tgbotapi.NewInlineKeyboardMarkup(buttons...)

	case MultiChoiceQueryKind:
		dlgChoices := dlg.data.ChoiceResponses[q.Name]
		buttons := make([][]tgbotapi.InlineKeyboardButton, 0, len(q.Choices)+1)
		for i, choice := range q.Choices {
			btnText := choice
			if dlgChoices[i] {
				btnText = "☒ " + btnText
			} else {
				btnText = "☐ " + btnText
			}
			btnData := strconv.Itoa(i) + ":" + q.Name
			buttons = append(buttons, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData(btnText, btnData)})
		}
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData("OK", "done:"+q.Name)})
		return tgbotapi.NewInlineKeyboardMarkup(buttons...)

	case TextInputQueryKind:
		if dlg.isPrivate() {
			return nil
		}
		return tgbotapi.ForceReply{
			ForceReply: true,
			Selective:  true,
		}

	default:
		return nil
	}
}

func (q *Query) getChoiceFromCallbackData(data string) (choice int, isDone, ok bool) {
	if !strings.Contains(data, ":") {
		return -1, false, false
	}
	parts := strings.SplitN(data, ":", 2)
	data, queryName := parts[0], parts[1]
	if queryName != q.Name {
		return -1, false, false
	}
	if data == "done" {
		return -1, true, true
	}
	if choice, err := strconv.Atoi(data); err == nil {
		return choice, false, true
	}
	return -1, false, false
}
