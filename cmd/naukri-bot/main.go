// Command naukri-bot runs the production Telegram bot, replacing bot.py's
// Application lifecycle (post_init/post_shutdown + run_polling).
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"github.com/sankalan47/naukri-bot/internal/config"
	"github.com/sankalan47/naukri-bot/internal/naukri"
	"github.com/sankalan47/naukri-bot/internal/scheduler"
	"github.com/sankalan47/naukri-bot/internal/telegram"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		log.Fatalf("creating telegram bot: %v", err)
	}

	sessions := make(map[int64]*naukri.Client, len(cfg.Users))
	jobs := make([]scheduler.Job, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		client := naukri.NewClient(u.NaukriEmail, u.NaukriPassword, u.SessionDir)
		sessions[u.TelegramUserID] = client
		jobs = append(jobs, scheduler.Job{ChatID: u.TelegramUserID, Touch: client.TouchBio})
	}
	log.Printf("NaukriBot started with %d user(s)", len(sessions))

	bot := telegram.New(api, sessions)
	go bot.Run()

	go scheduler.Run(jobs, bot.Notify)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	bot.Stop()
	naukri.WaitIdle() // let any in-flight action finish persisting + close its browser
	log.Println("NaukriBot shut down")
}
