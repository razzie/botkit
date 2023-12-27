package botkit

import (
	"context"
	"fmt"
	"strconv"
)

type DialogBuilder struct {
	steps     []dialogStep
	finalizer func(ctx context.Context, responses []any)
}

type dialogStep struct {
	id      int
	query   *Query
	handler dialogStepHandler
}

type dialogStepHandler func(responses []any) error

func NewDialogBuilder() *DialogBuilder {
	return new(DialogBuilder)
}

func (db *DialogBuilder) AddTextInputQuery(text string, validator func(resp string, prev []any) error) *DialogBuilder {
	id := len(db.steps)
	h := func(resps []any) error {
		return validator(resps[0].(string), resps[:len(resps)-1])
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	step := dialogStep{
		id:      id,
		query:   NewTextInputQuery(getQueryNameFromDialogStepID(id), text),
		handler: h,
	}
	db.steps = append(db.steps, step)
	return db
}

func (db *DialogBuilder) AddSingleChoiceQuery(text string, validator func(resp int, prev []any) error, choices ...string) *DialogBuilder {
	id := len(db.steps)
	h := func(resps []any) error {
		return validator(resps[0].([]int)[0], resps[:len(resps)-1])
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	step := dialogStep{
		id:      id,
		query:   NewSingleChoiseQuery(getQueryNameFromDialogStepID(id), text, choices...),
		handler: h,
	}
	db.steps = append(db.steps, step)
	return db
}

func (db *DialogBuilder) AddMultiChoiceQuery(text string, validator func(resp []int, prev []any) error, choices ...string) *DialogBuilder {
	id := len(db.steps)
	h := func(resps []any) error {
		return validator(resps[0].([]int), resps[:len(resps)-1])
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	step := dialogStep{
		id:      id,
		query:   NewMultiChoiseQuery(getQueryNameFromDialogStepID(id), text, choices...),
		handler: h,
	}
	db.steps = append(db.steps, step)
	return db
}

func (db *DialogBuilder) SetFinalizer(finalizer func(ctx context.Context, responses []any)) *DialogBuilder {
	db.finalizer = finalizer
	return db
}

func (db *DialogBuilder) Build() DialogHandler {
	if len(db.steps) == 0 {
		return nil
	}
	return func(ctx context.Context, dlg *Dialog) *Query {
		if q := dlg.LastQuery(); q != nil {
			id, _ := getDialogStepIDFromQueryName(q.Name)
			handler := db.steps[id].handler
			id++
			if id >= len(db.steps) {
				if db.finalizer != nil {
					resps := make([]any, len(db.steps))
					for i := range resps {
						resps[i] = db.steps[i].getUserResponse(dlg)
					}
					db.finalizer(ctx, resps)
				}
				return nil
			}
			resps := make([]any, id)
			for i := range resps {
				resps[i] = db.steps[i].getUserResponse(dlg)
			}
			if err := handler(resps); err != nil {
				SendMessage(ctx, "%v", err)
				return RetryQuery
			}
			return db.steps[id].query
		} else {
			return db.steps[0].query
		}
	}
}

func (ds *dialogStep) getUserResponse(dlg *Dialog) any {
	switch ds.query.Kind {
	case TextInputQueryKind:
		resp, _ := dlg.UserResponse(ds.query.Name)
		return resp
	case SingleChoiceQueryKind, MultiChoiceQueryKind:
		resp, _ := dlg.UserChoices(ds.query.Name)
		return resp
	default:
		return nil
	}
}

func dummyDialogStepHandler(resps []any) error {
	return nil
}

func getQueryNameFromDialogStepID(id int) string {
	return "Q" + strconv.Itoa(id)
}

func getDialogStepIDFromQueryName(queryName string) (int, bool) {
	var id int
	if _, err := fmt.Sscanf(queryName, "Q%d", &id); err != nil {
		return 0, false
	}
	return id, true
}
