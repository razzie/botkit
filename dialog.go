package botkit

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	dialogInputText dialogInputKind = iota
	dialogInputCallback
	dialogInputFile
)

var errInvalidDialogInput = fmt.Errorf("invalid dialog input")

type DialogHandler func(context.Context, *Dialog) *Query

type Dialog struct {
	userID  int64
	chatID  int64
	data    dialogData
	handler DialogHandler
}

type dialogData struct {
	Name            string                  `json:"name"`
	Username        string                  `json:"username"`
	IsPrivate       bool                    `json:"is_private"`
	LastQuery       string                  `json:"last_query"`
	Queries         map[string]*Query       `json:"queries"`
	Responses       map[string]string       `json:"responses"`
	ChoiceResponses map[string]map[int]bool `json:"choice_responses"`
}

type dialogInputKind int

type dialogMessage interface {
	toChattable(dlg *Dialog) tgbotapi.Chattable
	setMessageID(int)
}

type wrapperDialogMessage struct {
	c tgbotapi.Chattable
}

func (dlg *Dialog) Query(queryName string) *Query {
	return dlg.data.Queries[queryName]
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

func (dlg *Dialog) handleInput(ctx context.Context, kind dialogInputKind, data string) (updates []dialogMessage, isDone bool, err error) {
	lastQuery := dlg.LastQuery()
	if lastQuery == nil {
		return nil, true, fmt.Errorf("missing last query of dialog %q", dlg.data.Name)
	}

	switch kind {
	case dialogInputCallback:
		if lastQuery.Kind != SingleChoiceQueryKind && lastQuery.Kind != MultiChoiceQueryKind {
			return nil, false, errInvalidDialogInput
		}
		if choice, isDone, ok := lastQuery.getChoiceFromCallbackData(data); ok {
			if !isDone {
				dlg.flipUserChoice(choice)
				if kbm, ok := lastQuery.getReplyMarkup(dlg).(tgbotapi.InlineKeyboardMarkup); ok {
					update := tgbotapi.NewEditMessageText(
						dlg.chatID,
						lastQuery.MessageID,
						lastQuery.getMessageText(dlg),
					)
					update.ReplyMarkup = &kbm
					update.ParseMode = tgbotapi.ModeMarkdownV2
					updates = append(updates, newMessageFromChattable(update))
				}
				return updates, false, nil
			}
		} else {
			return nil, false, nil
		}

	case dialogInputText:
		if lastQuery.Kind != TextInputQueryKind {
			return nil, false, errInvalidDialogInput
		}
		dlg.setUserResponse(data)

	case dialogInputFile:
		if lastQuery.Kind != FileInputQueryKind {
			return nil, false, errInvalidDialogInput
		}
		dlg.setUserResponse(data)

	default:
		return nil, false, errInvalidDialogInput
	}

	if q := dlg.handler(ctx, dlg); q != nil {
		if q.Kind == RetryQueryKind {
			return updates, false, nil
		}
		updates = append(updates, q)
		dlg.setLastQuery(q)
		return updates, false, nil
	}
	return updates, true, nil
}

func (dlg *Dialog) setLastQuery(q *Query) {
	dlg.data.LastQuery = q.Name
	if dlg.data.Queries == nil {
		dlg.data.Queries = map[string]*Query{q.Name: q}
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

func (dlg *Dialog) isPrivate() bool {
	return dlg.data.IsPrivate
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

func newMessageFromChattable(c tgbotapi.Chattable) dialogMessage {
	return &wrapperDialogMessage{c: c}
}

func (m *wrapperDialogMessage) toChattable(*Dialog) tgbotapi.Chattable {
	return m.c
}

func (*wrapperDialogMessage) setMessageID(int) {
}
