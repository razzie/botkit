package main

import (
	"fmt"
	"reflect"
	"strings"
)

type Commander struct {
	cmds map[string]func(string) ([]any, error)
}

func NewCommander() *Commander {
	return &Commander{
		cmds: make(map[string]func(string) ([]any, error)),
	}
}

func (cmdr *Commander) RegisterCommand(cmd string, fn reflect.Value) error {
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("not a function")
	}
	cmdr.cmds[cmd] = func(line string) (results []any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		args := strings.Fields(line)
		return callFunction(fn, args)
	}
	return nil
}

func (cmdr *Commander) Call(cmd string, args string) ([]any, error) {
	fn, ok := cmdr.cmds[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	return fn(args)
}

func callFunction(fn reflect.Value, args []string) (results []any, err error) {
	fnType := fn.Type()
	if fnType.NumIn() != len(args) {
		err = fmt.Errorf("expected %d arguments, got %d", fnType.NumIn(), len(args))
		return
	}

	inputs := make([]reflect.Value, len(args))
	for i, arg := range args {
		paramType := fnType.In(i)
		val, convErr := convertToType(arg, paramType)
		if convErr != nil {
			err = fmt.Errorf("error converting argument %d: %s", i, convErr)
			return
		}
		inputs[i] = val
	}

	results = make([]any, 0, fnType.NumOut())
	for _, r := range fn.Call(inputs) {
		results = append(results, r.Interface())
	}

	if len(results) > 0 {
		last := results[len(results)-1]
		if e, ok := last.(error); ok {
			err = e
		}
	}

	return
}

func convertToType(value string, targetType reflect.Type) (reflect.Value, error) {
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(value), nil
	default:
		result := reflect.New(targetType)
		_, err := fmt.Sscan(value, &result)
		return reflect.ValueOf(result), err
	}
}
