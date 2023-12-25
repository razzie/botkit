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

	bot, err := botkit.NewBot(token,
		botkit.WithCommand("hello", cmdHelloWorld),
		botkit.WithCommand("startdlg", cmdStartDialog),
		botkit.WithDialog("dlg1", dlg1Handler))
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
	botkit.StartDialog(ctx, "dlg1")
}

func dlg1Handler(ctx context.Context, dlg *botkit.Dialog) *botkit.Query {
	q := dlg.LastQuery()
	if q != nil {
		choices := dlg.UserChoices()
		botkit.SendMessage(ctx, "choices: %v", choices)
	} else {
		return botkit.NewMultiChoiseQuery("q1", "Pick your choices", "Apple", "Orange", "Banana")
	}
	return nil
}
