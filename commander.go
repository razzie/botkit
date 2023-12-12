package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

type Command func(context.Context, string) ([]any, error)

type Commander struct {
	cmds map[string]Command
}

func NewCommander() *Commander {
	return &Commander{
		cmds: make(map[string]Command),
	}
}

func (cmdr *Commander) RegisterCommand(cmd string, callback any) error {
	fn := reflect.ValueOf(callback)
	if fn.Kind() != reflect.Func {
		return fmt.Errorf("not a function")
	}
	cmdr.cmds[cmd] = func(ctx context.Context, line string) (results []any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		args := strings.Fields(line)
		return callFunction(ctx, fn, args)
	}
	return nil
}

func (cmdr *Commander) Call(ctx context.Context, cmd string, args string) ([]any, error) {
	fn, ok := cmdr.cmds[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	return fn(ctx, args)
}

func callFunction(ctx context.Context, fn reflect.Value, args []string) ([]any, error) {
	fnType := fn.Type()
	inputs, err := convertInputs(ctx, fnType, args)
	if err != nil {
		return nil, err
	}
	outputs := make([]any, 0, fnType.NumOut())
	for _, o := range fn.Call(inputs) {
		outputs = append(outputs, o.Interface())
	}
	if len(outputs) > 0 {
		last := outputs[len(outputs)-1]
		if e, ok := last.(error); ok {
			err = e
		}
	}
	return outputs, err
}

func convertInputs(ctx context.Context, fnType reflect.Type, args []string) ([]reflect.Value, error) {
	numIn := fnType.NumIn()
	if numIn == 0 {
		return nil, nil
	}
	inputs := make([]reflect.Value, numIn)
	skipped := 0

	if fnType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		inputs[0] = reflect.ValueOf(ctx)
		skipped++
	}
	if numIn-skipped != len(args) {
		return nil, fmt.Errorf("expected %d argument(s), got %d", numIn-skipped, len(args))
	}

	for i, arg := range args {
		paramType := fnType.In(i + skipped)
		val, convErr := convertToType(arg, paramType)
		if convErr != nil {
			return nil, fmt.Errorf("error converting argument %d: %s", i, convErr)
		}
		inputs[i+skipped] = val
	}

	// TODO: variadic support

	return inputs, nil
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
