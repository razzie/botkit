package botkit

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DialogHandler func(*Dialog) *Query

type dialogData struct {
	Name            string                  `json:"name"`
	LastQuery       string                  `json:"last_query"`
	Queries         map[string]Query        `json:"queries"`
	Responses       map[string]string       `json:"responses"`
	ChoiceResponses map[string]map[int]bool `json:"choice_responses"`
}

type Dialog struct {
	user *tgbotapi.User
	chat *tgbotapi.Chat
	data dialogData
}
