package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/handlers"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("No .env file found in either directory, relying on system environment variables")
		}
	}

	database.Init()
	if err := database.Pool.Ping(context.Background()); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	database.InitRedis()

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("Analytics Worker booted up. Waiting for RabbitMQ messages...")

	rabbitURL := database.RabbitURL()
	handlers.StartClickWorker(appCtx, rabbitURL)

	<-appCtx.Done()
	log.Println("Worker shutting down gracefully...")
}
