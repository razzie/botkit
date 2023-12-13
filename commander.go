package main

import (
	"context"
	"fmt"
	"reflect"
)

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	stringType  = reflect.TypeOf((*string)(nil)).Elem()
)

type Commander struct {
	cmds map[string]Command
}

func NewCommander() *Commander {
	return &Commander{
		cmds: make(map[string]Command),
	}
}

func (cmdr *Commander) RegisterCommand(cmd string, callback any) error {
	c, err := NewCommand(callback)
	if err != nil {
		return err
	}
	cmdr.cmds[cmd] = *c
	return nil
}

func (cmdr *Commander) UnregisterCommand(cmd string) {
	delete(cmdr.cmds, cmd)
}

func (cmdr *Commander) Call(ctx context.Context, cmd string, args []string) ([]any, error) {
	c, ok := cmdr.cmds[cmd]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	return c.Call(ctx, args)
}

type Command struct {
	fn                   reflect.Value
	fnType               reflect.Type
	numNonCtxInputs      int
	inputHandlers        []commandInputHandler
	isVariadic           bool
	variadicInputHandler commandInputHandler
}

func NewCommand(callback any) (*Command, error) {
	fn := reflect.ValueOf(callback)
	if fn.Kind() != reflect.Func {
		return nil, fmt.Errorf("not a function")
	}

	fnType := fn.Type()
	isVariadic := fnType.IsVariadic()
	numInputs := fnType.NumIn()

	numNonCtxInputs := numInputs
	inputHandlers := make([]commandInputHandler, numInputs)
	for i := range inputHandlers {
		inputType := fnType.In(i)
		if isVariadic && i == numInputs-1 {
			inputType = inputType.Elem()
		}
		switch inputType {
		case contextType:
			numNonCtxInputs--
			inputHandlers[i] = cmdHandleCtx
		case stringType:
			inputHandlers[i] = cmdHandleStringArg
		default:
			inputHandlers[i] = cmdHandleAnyArg(inputType)
		}
	}
	var variadicInputHandler commandInputHandler
	if isVariadic {
		variadicInputHandler = inputHandlers[len(inputHandlers)-1]
		inputHandlers = inputHandlers[:len(inputHandlers)-1]
	}

	return &Command{
		fn:                   fn,
		fnType:               fnType,
		numNonCtxInputs:      numNonCtxInputs,
		inputHandlers:        inputHandlers,
		isVariadic:           isVariadic,
		variadicInputHandler: variadicInputHandler,
	}, nil
}

func (cmd *Command) checkArgs(args []string) error {
	if cmd.isVariadic && len(args) < cmd.numNonCtxInputs-1 {
		return fmt.Errorf("expected at least %d argument(s), got %d", cmd.numNonCtxInputs-1, len(args))
	} else if !cmd.isVariadic && len(args) != cmd.numNonCtxInputs {
		return fmt.Errorf("expected %d argument(s), got %d", cmd.numNonCtxInputs, len(args))
	}
	return nil
}

func (cmd *Command) Call(ctx context.Context, args []string) ([]any, error) {
	if err := cmd.checkArgs(args); err != nil {
		return nil, err
	}
	cc := &commandCall{
		cmd:  cmd,
		ctx:  ctx,
		args: args,
	}
	return cc.run()
}

type commandCall struct {
	cmd  *Command
	ctx  context.Context
	args []string
}

func (cc *commandCall) popArg() string {
	arg := cc.args[0]
	cc.args = cc.args[1:]
	return arg
}

func (cc *commandCall) run() (outputs []any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	var inputs []reflect.Value
	for _, h := range cc.cmd.inputHandlers {
		in, err := h(cc)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in)
	}
	for len(cc.args) > 0 {
		in, err := cc.cmd.variadicInputHandler(cc)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, in)
	}

	outputs = make([]any, 0, len(cc.args))
	for _, out := range cc.cmd.fn.Call(inputs) {
		outputs = append(outputs, out.Interface())
	}

	return
}

type commandInputHandler func(*commandCall) (reflect.Value, error)

func cmdHandleCtx(cc *commandCall) (reflect.Value, error) {
	return reflect.ValueOf(cc.ctx), nil
}

func cmdHandleStringArg(cc *commandCall) (reflect.Value, error) {
	return reflect.ValueOf(cc.popArg()), nil
}

func cmdHandleAnyArg(targetType reflect.Type) commandInputHandler {
	return func(cc *commandCall) (v reflect.Value, err error) {
		arg := cc.popArg()
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("failed to convert arg %q to %s: %v", arg, targetType.Name(), r)
			}
		}()
		result := reflect.New(targetType)
		_, err = fmt.Sscan(arg, result.Interface())
		if err != nil {
			err = fmt.Errorf("failed to convert arg %q to %s: %v", arg, targetType.Name(), err)
		}
		return reflect.Indirect(result), err
	}
}
