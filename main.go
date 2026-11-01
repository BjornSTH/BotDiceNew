// BotDice — Telegram-бот для бросков кубиков.
//
// Токен не хранится ни в коде, ни в файлах: при запуске он вводится в консоль
// со скрытым вводом (или подаётся через stdin), после чего бот уходит в фон.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"botdice/internal/dice"
)

// daemonEnv помечает фоновый процесс, запущенный самим ботом.
const daemonEnv = "BOTDICE_DAEMON"

// Сколько сообщений обрабатывается одновременно.
const workers = 16

func main() {
	foreground := flag.Bool("fg", false, "работать в текущей консоли, не уходя в фон")
	logPath := flag.String("log", "botdice.log", "файл лога фонового процесса")
	flag.Parse()

	if os.Getenv(daemonEnv) == "1" {
		runDaemon()
		return
	}

	token, err := readToken()
	if err != nil {
		log.Fatal(err)
	}

	if *foreground {
		err := run(token, func(username string) {
			log.Printf("Бот @%s запущен. Ctrl+C — остановить.", username)
		})
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	absLog, err := filepath.Abs(*logPath)
	if err != nil {
		log.Fatal(err)
	}
	pid, username, err := detach(token, absLog)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Бот @%s работает в фоне, PID %d.\nЛог: %s\nОстановить: kill %d\n", username, pid, absLog, pid)
}

// run запускает бота и блокируется до SIGINT/SIGTERM.
// ready вызывается, когда Telegram принял токен.
func run(token string, ready func(username string)) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b, err := bot.New(token,
		bot.WithDefaultHandler(handleUpdate),
		bot.WithWorkers(workers),
		bot.WithAllowedUpdates(bot.AllowedUpdates{models.AllowedUpdateMessage}),
		bot.WithErrorsHandler(func(err error) {
			log.Print(redact(err.Error(), token))
		}),
	)
	if err != nil {
		return errors.New(redact(err.Error(), token))
	}
	me, err := b.GetMe(ctx)
	if err != nil {
		return errors.New(redact(err.Error(), token))
	}

	ready(me.Username)
	b.Start(ctx)
	log.Print("Бот остановлен")
	return nil
}

func handleUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}
	reply := buildReply(msg.Text, playerName(msg.From), dice.Crypto)
	if reply == "" {
		return
	}

	params := &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   reply,
		ReplyParameters: &models.ReplyParameters{
			MessageID:                msg.ID,
			AllowSendingWithoutReply: true,
		},
	}
	if msg.IsTopicMessage {
		params.MessageThreadID = msg.MessageThreadID
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("не удалось ответить в чат %d: %v", msg.Chat.ID, err)
	}
}

func playerName(u *models.User) string {
	switch {
	case u == nil:
		return "Игрок"
	case u.Username != "":
		return u.Username
	case u.FirstName != "":
		return u.FirstName
	default:
		return "Игрок"
	}
}
