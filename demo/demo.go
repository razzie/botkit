package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/razzie/botkit"
)

func main() {
	var token string
	flag.StringVar(&token, "token", "", "Telegram bot token")
	flag.Parse()

	if len(token) == 0 {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter token: ")
		token, _ = reader.ReadString('\n')
	}

	dlg := botkit.NewDialogBuilder().
		AddMultiChoiceQuery("Pick your favorite", func(resp []int, prev []any) error {
			if len(resp) < 1 {
				return fmt.Errorf("pick at least one")
			}
			return nil
		}, "Apple", "Orange", "Banana", "Grapes", "Melon").
		AddTextInputQuery("Why?", nil).
		SetFinalizer(func(ctx context.Context, responses []any) {
			botkit.SendMessage(ctx, "responses: %v", responses)
		}).
		Build()

	bot, err := botkit.NewBot(token,
		botkit.WithCommand("hello", cmdHelloWorld),
		botkit.WithCommand("startdlg", cmdStartDialog),
		botkit.WithDialog("dlg", dlg))
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		bot.Close()
	}()

	bot.Run()
}

func cmdHelloWorld(ctx context.Context) {
	botkit.SendMessage(ctx, "Hello World!")
}

func cmdStartDialog(ctx context.Context) {
	botkit.StartDialog(ctx, "dlg")
}
