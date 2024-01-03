package botkit

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
	Queries         map[string]Query        `json:"queries"`
	Responses       map[string]string       `json:"responses"`
	ChoiceResponses map[string]map[int]bool `json:"choice_responses"`
}

type dialogMessage interface {
	toChattable(dlg *Dialog) tgbotapi.Chattable
	setMessageID(int)
}

type wrapperDialogMessage struct {
	c tgbotapi.Chattable
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

func (dlg *Dialog) handleInput(ctx context.Context, input any) (updates []dialogMessage, isDone bool, err error) {
	lastQuery := dlg.LastQuery()
	if lastQuery == nil {
		return nil, true, fmt.Errorf("missing last query of dialog %q", dlg.data.Name)
	}

	switch input := input.(type) {
	case *tgbotapi.CallbackQuery:
		ctx = ctxWithMessage(ctx, input.Message)
		if choice, isDone, ok := lastQuery.getChoiceFromCallbackData(input.Data); ok {
			if !isDone {
				dlg.flipUserChoice(choice)
				if kbm, ok := lastQuery.getReplyMarkup(dlg).(tgbotapi.InlineKeyboardMarkup); ok {
					update := tgbotapi.NewEditMessageText(
						input.Message.Chat.ID,
						input.Message.MessageID,
						input.Message.Text,
					)
					update.ReplyMarkup = &kbm
					updates = append(updates, newMessageFromChattable(update))
				}
				return updates, false, nil
			}
		} else {
			return nil, false, nil
		}
	case *tgbotapi.Message:
		if !input.Chat.IsPrivate() &&
			(input.ReplyToMessage == nil || input.ReplyToMessage.MessageID != lastQuery.MessageID) {
			return nil, false, nil
		}
		ctx = ctxWithMessage(ctx, input)
		dlg.setUserResponse(input.Text)
	default:
		return nil, false, fmt.Errorf("unhandled dialog input: %v", input)
	}

	if q := dlg.handler(ctx, dlg); q != nil {
		if q.Kind == RetryQueryKind {
			return updates, false, nil
		}
		updates = append(updates, q)
		dlg.setLastQuery(*q)
		return updates, false, nil
	}
	return updates, true, nil
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

func newMessageFromChattable(c tgbotapi.Chattable) dialogMessage {
	return &wrapperDialogMessage{c: c}
}

func (m *wrapperDialogMessage) toChattable(*Dialog) tgbotapi.Chattable {
	return m.c
}

func (*wrapperDialogMessage) setMessageID(int) {
}
