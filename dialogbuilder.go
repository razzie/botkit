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

type dialogStepHandler func(response any) error

func NewDialogBuilder() *DialogBuilder {
	return new(DialogBuilder)
}

func (db *DialogBuilder) AddTextInputQuery(text string, validator func(resp string) error) *DialogBuilder {
	id := len(db.steps)
	h := func(resp any) error {
		return validator(resp.(string))
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

func (db *DialogBuilder) AddSingleChoiceQuery(text string, validator func(choice int) error, choices ...string) *DialogBuilder {
	id := len(db.steps)
	h := func(resp any) error {
		return validator(resp.([]int)[0])
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	step := dialogStep{
		id:      id,
		query:   NewSingleChoiceQuery(getQueryNameFromDialogStepID(id), text, choices...),
		handler: h,
	}
	db.steps = append(db.steps, step)
	return db
}

func (db *DialogBuilder) AddMultiChoiceQuery(text string, validator func(choices []int) error, choices ...string) *DialogBuilder {
	id := len(db.steps)
	h := func(resp any) error {
		return validator(resp.([]int))
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	step := dialogStep{
		id:      id,
		query:   NewMultiChoiceQuery(getQueryNameFromDialogStepID(id), text, choices...),
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
		q := dlg.LastQuery()
		if q == nil {
			return db.steps[0].query
		}

		id, _ := getDialogStepIDFromQueryName(q.Name)
		resp := db.steps[id].getUserResponse(dlg)
		handler := db.steps[id].handler
		if err := handler(resp); err != nil {
			SendMessage(ctx, "%v", err)
			return RetryQuery
		}

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

		return db.steps[id].query
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

func dummyDialogStepHandler(resp any) error {
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
