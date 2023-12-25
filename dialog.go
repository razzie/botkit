package botkit

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DialogHandler func(context.Context, *Dialog) *Query

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

func (dlg *Dialog) LastQuery() *Query {
	if q, ok := dlg.data.Queries[dlg.data.LastQuery]; ok {
		return &q
	}
	return nil
}

func (dlg *Dialog) UserResponse() string {
	return dlg.data.Responses[dlg.data.LastQuery]
}

func (dlg *Dialog) UserChoices() (results []int) {
	choices := dlg.data.ChoiceResponses[dlg.data.LastQuery]
	for choice, isSet := range choices {
		if isSet {
			results = append(results, choice)
		}
	}
	return
}

func (dlg *Dialog) setLastQuery(q Query) {
	dlg.data.LastQuery = q.Name
	if dlg.data.Queries == nil {
		dlg.data.Queries = map[string]Query{q.Name: q}
		return
	}
	dlg.data.Queries[q.Name] = q
}

func (dlg *Dialog) setUserResponse(response string) {
	if dlg.data.Responses == nil {
		dlg.data.Responses = map[string]string{dlg.data.LastQuery: response}
		return
	}
	dlg.data.Responses[dlg.data.LastQuery] = response
}

func (dlg *Dialog) flipUserChoice(choice int) {
	if dlg.data.ChoiceResponses == nil {
		dlg.data.ChoiceResponses = make(map[string]map[int]bool)
	}
	choices := dlg.data.ChoiceResponses[dlg.data.LastQuery]
	if choices == nil {
		choices = map[int]bool{choice: true}
		dlg.data.ChoiceResponses[dlg.data.LastQuery] = choices
		return
	}
	choices[choice] = !choices[choice]
}
