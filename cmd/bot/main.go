package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/passwordhash/cascade-hub-bot/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatal("bot failed to start or run")
	}
}
