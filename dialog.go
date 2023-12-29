package botkit

import (
	"context"
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
	userID int64
	chatID int64
	data   dialogData
}

func (dlg *Dialog) Query(queryName string) *Query {
	if q, ok := dlg.data.Queries[queryName]; ok {
		return &q
	}
	return nil
}

func (dlg *Dialog) LastQuery() *Query {
	return dlg.Query(dlg.data.LastQuery)
}

func (dlg *Dialog) LastQueryName() string {
	return dlg.data.LastQuery
}

func (dlg *Dialog) UserResponse(queryName string) (string, bool) {
	if q, ok := dlg.data.Queries[queryName]; !ok || !q.Kind.HasTextResponse() {
		return "", false
	}
	resp, ok := dlg.data.Responses[queryName]
	return resp, ok
}

func (dlg *Dialog) LastUserResponse() (string, bool) {
	return dlg.UserResponse(dlg.data.LastQuery)
}

func (dlg *Dialog) UserChoices(queryName string) (results []int, ok bool) {
	if q, ok := dlg.data.Queries[queryName]; !ok || !q.Kind.HasChoiceResponse() {
		return nil, false
	}
	choices, ok := dlg.data.ChoiceResponses[queryName]
	for choice, isSet := range choices {
		if isSet {
			results = append(results, choice)
		}
	}
	return results, ok
}

func (dlg *Dialog) LastUserChoices() ([]int, bool) {
	return dlg.UserChoices(dlg.data.LastQuery)
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
