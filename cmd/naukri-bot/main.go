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

	naukriClient := naukri.NewClient(cfg.NaukriEmail, cfg.NaukriPassword)
	if err := naukriClient.Start(); err != nil {
		log.Fatalf("starting naukri client: %v", err)
	}
	log.Println("NaukriBot started")

	bot := telegram.New(api, naukriClient, cfg.AllowedUserID)
	go bot.Run()

	go scheduler.Run(naukriClient.TouchBio, func(msg string) {
		bot.Notify(cfg.AllowedUserID, msg)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	bot.Stop()
	if err := naukriClient.Stop(); err != nil {
		log.Printf("error stopping naukri client: %v", err)
	}
	log.Println("NaukriBot shut down")
}
