package botkit

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	TextInputQuery QueryKind = iota
	SingleChoiceQuery
	MultiChoiceQuery
)

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
		Kind: TextInputQuery,
		Text: text,
	}
}

func NewSingleChoiseQuery(name, text string, choices ...string) *Query {
	return &Query{
		Kind:    SingleChoiceQuery,
		Text:    text,
		Choices: choices,
	}
}

func NewMultiChoiseQuery(name, text string, choices ...string) *Query {
	return &Query{
		Kind:    MultiChoiceQuery,
		Text:    text,
		Choices: choices,
	}
}

func (q *Query) toMessage() *tgbotapi.MessageConfig {
	switch q.Kind {
	default:
		return nil
	}
}
