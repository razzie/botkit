package main

import (
	"bufio"
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
		SetFinalizer(func(ctx *botkit.Context, responses []any) {
			ctx.SendMessage("responses: %v", responses)
		}).
		Build()

	filedlg := botkit.NewDialogBuilder().
		AddFileInputQuery("Upload a file", nil).
		SetFinalizer(func(ctx *botkit.Context, responses []any) {
			file := responses[0].(io.Reader)
			p := make([]byte, 4)
			n, _ := file.Read(p)
			p = p[:n]
			ctx.SendReply("First %d bytes in hex: %x", n, p)
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

func cmdHelloWorld() botkit.CommandResponse {
	return botkit.SendMessage("Hello World!")
}

func cmdAlbum(ctx *botkit.Context) {
	ctx.SendMedia(
		botkit.NewPhoto(botkit.URLSource("https://gorzsony.com/img/razchess.png")),
		botkit.NewPhoto(botkit.URLSource("https://gorzsony.com/img/razbox.png")))
}

func cmdStartDialog(ctx *botkit.Context) {
	ctx.StartDialog("dlg")
}

func cmdFileDialog(ctx *botkit.Context) {
	ctx.StartDialog("filedlg")
}

func cmdSticker(ctx *botkit.Context) {
	ctx.SendSticker("pizzabot", -1)
}

func cmdSum(ctx *botkit.Context, a, b int) {
	ctx.SendReply("%d", a+b)
}

func cmdSumMany(ctx *botkit.Context, numbers ...int) {
	sum := 0
	for _, n := range numbers {
		sum += n
	}
	ctx.SendReply("%d", sum)
}
