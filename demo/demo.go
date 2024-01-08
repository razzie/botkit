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
			n, _ := file.Read(p)
			p = p[:n]
			botkit.SendReply(ctx, "First %d bytes in hex: %x", n, p)
		}).
		Build()

	bot, err := botkit.NewBot(token,
		//botkit.WithAPIEndpoint("localhost:8080"),
		botkit.WithCommand("hello", cmdHelloWorld),
		botkit.WithCommand("album", cmdAlbum),
		botkit.WithCommand("startdlg", cmdStartDialog),
		botkit.WithCommand("filedlg", cmdFileDialog),
		botkit.WithCommand("sticker", cmdSticker),
		botkit.WithCommand("sum", cmdSum),
		botkit.WithCommand("sumMany", cmdSumMany),
		botkit.WithDialog("dlg", dlg),
		botkit.WithDialog("filedlg", filedlg),
	)
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

func cmdAlbum(ctx context.Context) {
	botkit.SendMedia(ctx,
		botkit.NewPhoto(botkit.URLSource("https://gorzsony.com/img/razchess.png")),
		botkit.NewPhoto(botkit.URLSource("https://gorzsony.com/img/razbox.png")))
}

func cmdStartDialog(ctx context.Context) {
	botkit.StartDialog(ctx, "dlg")
}

func cmdFileDialog(ctx context.Context) {
	botkit.StartDialog(ctx, "filedlg")
}

func cmdSticker(ctx context.Context) {
	botkit.SendSticker(ctx, "pizzabot", -1)
}

func cmdSum(ctx context.Context, a, b int) {
	botkit.SendReply(ctx, "%d", a+b)
}

func cmdSumMany(ctx context.Context, numbers ...int) {
	sum := 0
	for _, n := range numbers {
		sum += n
	}
	botkit.SendReply(ctx, "%d", sum)
}
