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

func NewSingleChoiseQuery(name, text string, choices ...string) *Query {
	return &Query{
		Name:    name,
		Kind:    SingleChoiceQueryKind,
		Text:    text,
		Choices: choices,
	}
}

func NewMultiChoiseQuery(name, text string, choices ...string) *Query {
	return &Query{
		Name:    name,
		Kind:    MultiChoiceQueryKind,
		Text:    text,
		Choices: choices,
	}
}

func (qk QueryKind) HasTextResponse() bool {
	switch qk {
	case TextInputQueryKind:
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

func (q *Query) toMessage(dlg *Dialog) (*tgbotapi.MessageConfig, error) {
	switch q.Kind {
	case TextInputQueryKind, SingleChoiceQueryKind, MultiChoiceQueryKind:
		msg := tgbotapi.NewMessage(dlg.chatID, q.Text)
		if kbm := q.getInlineKeybordMarkup(dlg); kbm != nil {
			msg.ReplyMarkup = *kbm
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unsupported/unknown query kind: %v", q.Kind)
	}
}

func (q *Query) getInlineKeybordMarkup(dlg *Dialog) *tgbotapi.InlineKeyboardMarkup {
	switch q.Kind {
	case SingleChoiceQueryKind:
		buttons := make([][]tgbotapi.InlineKeyboardButton, 0, len(q.Choices))
		for i, choice := range q.Choices {
			btnText := choice
			btnData := strconv.Itoa(i) + ":" + q.Name
			buttons = append(buttons, []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData(btnText, btnData)})
		}
		markup := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		return &markup

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
		markup := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		return &markup

	default:
		return nil
	}
}

func (q *Query) getDataFromCallback(cq *tgbotapi.CallbackQuery) (choice int, isDone, ok bool) {
	if !strings.Contains(cq.Data, ":") {
		return -1, false, false
	}
	parts := strings.SplitN(cq.Data, ":", 2)
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
