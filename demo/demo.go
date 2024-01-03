package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
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
		AddMultiChoiceQuery("Pick your favorite", func(choices []int) error {
			if len(choices) < 1 {
				return fmt.Errorf("pick at least one")
			}
			return nil
		}, "Apple", "Orange", "Banana", "Grapes", "Melon").
		AddTextInputQuery("Why?", func(resp string) error {
			if len(resp) < 2 {
				return fmt.Errorf("please write a longer response")
			}
			return nil
		}).
		SetFinalizer(func(ctx context.Context, responses []any) {
			botkit.SendMessage(ctx, "responses: %v", responses)
		}).
		Build()

	filedlg := botkit.NewDialogBuilder().
		AddFileInputQuery("Upload a file", nil).
		SetFinalizer(func(ctx context.Context, responses []any) {
			file := responses[0].(io.ReadCloser)
			defer file.Close()
			p := make([]byte, 4)
			file.Read(p)
			botkit.SendReply(ctx, "%x", p)
		}).
		Build()

	bot, err := botkit.NewBot(token,
		//botkit.WithAPIEndpoint("localhost:8080"),
		botkit.WithCommand("hello", cmdHelloWorld),
		botkit.WithCommand("startdlg", cmdStartDialog),
		botkit.WithCommand("filedlg", cmdFileDialog),
		botkit.WithDialog("dlg", dlg),
		botkit.WithDialog("filedlg", filedlg))
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

func cmdFileDialog(ctx context.Context) {
	botkit.StartDialog(ctx, "filedlg")
}
