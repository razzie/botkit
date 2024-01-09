package botkit

import (
	"fmt"
	"io"
	"strconv"
)

type DialogBuilder struct {
	steps     []dialogStep
	finalizer dialogFinalizer
}

type dialogStep struct {
	id      int
	query   *Query
	handler dialogStepHandler
}

type dialogStepHandler func(response any) error
type dialogFinalizer func(ctx *Context, responses []any)

func NewDialogBuilder() *DialogBuilder {
	return new(DialogBuilder)
}

func (db *DialogBuilder) AddTextInputQuery(text string, validator func(resp string) error) *DialogBuilder {
	h := func(resp any) error {
		return validator(resp.(string))
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	db.addStep(TextInputQueryKind, text, h)
	return db
}

func (db *DialogBuilder) AddSingleChoiceQuery(text string, validator func(choice int) error, choices ...string) *DialogBuilder {
	h := func(resp any) error {
		return validator(resp.([]int)[0])
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	db.addStep(SingleChoiceQueryKind, text, h).Choices = choices
	return db
}

func (db *DialogBuilder) AddMultiChoiceQuery(text string, validator func(choices []int) error, choices ...string) *DialogBuilder {
	h := func(resp any) error {
		return validator(resp.([]int))
	}
	if validator == nil {
		h = dummyDialogStepHandler
	}
	db.addStep(MultiChoiceQueryKind, text, h).Choices = choices
	return db
}

func (db *DialogBuilder) AddFileInputQuery(text string, validator func(io.Reader) error) *DialogBuilder {
	h := func(resp any) error {
		reader := resp.(io.ReadCloser)
		defer reader.Close()
		return validator(reader)
	}
	if validator == nil {
		h = func(resp any) error {
			reader := resp.(io.ReadCloser)
			reader.Close()
			return nil
		}
	}
	db.addStep(FileInputQueryKind, text, h)
	return db
}

func (db *DialogBuilder) SetFinalizer(finalizer func(ctx *Context, responses []any)) *DialogBuilder {
	db.finalizer = finalizer
	return db
}

func (db *DialogBuilder) Build() DialogHandler {
	if len(db.steps) == 0 {
		return nil
	}

	return func(ctx *Context, dlg *Dialog) *Query {
		q := dlg.LastQuery()
		if q == nil {
			return db.steps[0].query
		}

		id, _ := getDialogStepIDFromQueryName(q.Name)
		resp := db.steps[id].getUserResponse(ctx, dlg)
		handler := db.steps[id].handler
		if err := handler(resp); err != nil {
			ctx.SendReply("%v", err)
			return RetryQuery
		}

		id++
		if id >= len(db.steps) {
			if db.finalizer != nil {
				resps := make([]any, len(db.steps))
				for i := range resps {
					resps[i] = db.steps[i].getUserResponse(ctx, dlg)
				}
				db.finalizer(ctx, resps)
			}
			return nil
		}

		return db.steps[id].query
	}
}

func (db *DialogBuilder) addStep(kind QueryKind, text string, handler dialogStepHandler) *Query {
	id := len(db.steps)
	query := &Query{
		Name: getQueryNameFromDialogStepID(id),
		Kind: kind,
		Text: text,
	}
	step := dialogStep{
		id:      id,
		query:   query,
		handler: handler,
	}
	db.steps = append(db.steps, step)
	return query
}

func (ds *dialogStep) getUserResponse(ctx *Context, dlg *Dialog) any {
	switch ds.query.Kind {
	case TextInputQueryKind:
		resp, _ := dlg.UserResponse(ds.query.Name)
		return resp
	case SingleChoiceQueryKind:
		resp, _ := dlg.UserChoices(ds.query.Name)
		return resp[0]
	case MultiChoiceQueryKind:
		resp, _ := dlg.UserChoices(ds.query.Name)
		return resp
	case FileInputQueryKind:
		resp, _ := dlg.UserResponse(ds.query.Name)
		file, _ := ctx.DownloadFile(resp)
		return file
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
