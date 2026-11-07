package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db"
)

const (
	defaultSweepInterval = 720 * time.Hour
	defaultSweepBatch    = 1000
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

	interval, err := sweepInterval()
	if err != nil {
		log.Fatal(err)
	}
	batchSize, err := sweepBatchSize()
	if err != nil {
		log.Fatal(err)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Sweeper started: every %s, batches of %d", interval, batchSize)
	Run(appCtx, db.New(database.Pool), interval, batchSize, log.Printf)
	log.Println("Sweeper shutting down gracefully...")
}

func sweepInterval() (time.Duration, error) {
	raw := os.Getenv("SWEEP_INTERVAL")
	if raw == "" {
		return defaultSweepInterval, nil
	}
	return time.ParseDuration(raw)
}

func sweepBatchSize() (int32, error) {
	raw := os.Getenv("SWEEP_BATCH_SIZE")
	if raw == "" {
		return defaultSweepBatch, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, strconv.ErrRange
	}
	return int32(value), nil
}
