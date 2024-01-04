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
	Name      string                `json:"name"`
	Username  string                `json:"username"`
	IsPrivate bool                  `json:"is_private"`
	LastQuery string                `json:"last_query"`
	Queries   map[string]*queryData `json:"queries"`
}

type queryData struct {
	Query        *Query       `json:"query"`
	UserResponse string       `json:"user_response"`
	UserChoices  map[int]bool `json:"user_choices"`
	ReplyID      int          `json:"reply_id"`
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
	if q := dlg.data.Queries[queryName]; q != nil {
		return q.Query
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
	q := dlg.data.Queries[queryName]
	if q == nil || !q.Query.Kind.HasTextResponse() {
		return "", false
	}
	return q.UserResponse, true
}

func (dlg *Dialog) LastUserResponse() (string, bool) {
	return dlg.UserResponse(dlg.data.LastQuery)
}

func (dlg *Dialog) UserChoices(queryName string) (results []int, ok bool) {
	q := dlg.data.Queries[queryName]
	if q == nil || !q.Query.Kind.HasChoiceResponse() {
		return nil, false
	}
	for choice, isSet := range q.UserChoices {
		if isSet {
			results = append(results, choice)
		}
	}
	return results, ok
}

func (dlg *Dialog) LastUserChoices() ([]int, bool) {
	return dlg.UserChoices(dlg.data.LastQuery)
}

func (dlg *Dialog) handleInput(ctx context.Context, kind dialogInputKind, data string, replyID int) (updates []dialogMessage, isDone bool, err error) {
	last := dlg.getQueryData(dlg.data.LastQuery)
	if last == nil {
		return nil, true, fmt.Errorf("missing last query of dialog %q", dlg.data.Name)
	}

	switch kind {
	case dialogInputCallback:
		if last.Query.Kind != SingleChoiceQueryKind && last.Query.Kind != MultiChoiceQueryKind {
			return nil, false, errInvalidDialogInput
		}
		if choice, isDone, ok := last.Query.getChoiceFromCallbackData(data); ok {
			last.ReplyID = last.Query.MessageID
			if !isDone {
				last.UserChoices[choice] = !last.UserChoices[choice]
				if kbm, ok := last.Query.getReplyMarkup(dlg).(tgbotapi.InlineKeyboardMarkup); ok {
					update := tgbotapi.NewEditMessageText(
						dlg.chatID,
						last.Query.MessageID,
						last.Query.getMessageText(dlg),
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
		if last.Query.Kind != TextInputQueryKind {
			return nil, false, errInvalidDialogInput
		}
		last.UserResponse = data
		last.ReplyID = replyID

	case dialogInputFile:
		if last.Query.Kind != FileInputQueryKind {
			return nil, false, errInvalidDialogInput
		}
		last.UserResponse = data
		last.ReplyID = replyID

	default:
		return nil, false, errInvalidDialogInput
	}

	handlerUpdates, isDone := dlg.runHandler(ctx)
	updates = append(updates, handlerUpdates...)
	return updates, isDone, nil
}

func (dlg *Dialog) runHandler(ctx context.Context) (updates []dialogMessage, isDone bool) {
	if q := dlg.handler(ctx, dlg); q != nil {
		if q.Kind == RetryQueryKind {
			return updates, false
		}
		updates = append(updates, q)
		dlg.setLastQuery(q)
		return updates, false
	}
	return updates, true
}

func (dlg *Dialog) getQueryData(queryName string) *queryData {
	if dlg.data.Queries == nil {
		qdata := &queryData{UserChoices: make(map[int]bool)}
		dlg.data.Queries = map[string]*queryData{queryName: qdata}
		return qdata
	}
	if qdata := dlg.data.Queries[queryName]; qdata != nil {
		return qdata
	}
	qdata := &queryData{UserChoices: make(map[int]bool)}
	dlg.data.Queries[queryName] = qdata
	return qdata
}

func (dlg *Dialog) setLastQuery(q *Query) {
	dlg.data.LastQuery = q.Name
	dlg.getQueryData(q.Name).Query = q
}

func (dlg *Dialog) isPrivate() bool {
	return dlg.data.IsPrivate
}

func newMessageFromChattable(c tgbotapi.Chattable) dialogMessage {
	return &wrapperDialogMessage{c: c}
}

func (m *wrapperDialogMessage) toChattable(*Dialog) tgbotapi.Chattable {
	return m.c
}

func (*wrapperDialogMessage) setMessageID(int) {
}
