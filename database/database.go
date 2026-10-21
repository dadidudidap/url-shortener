package database

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func Init() {
	dbURL := os.Getenv("DATABASE_URL")
	// pgxpool manages a pool of multiplexed connections automatically
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatal("Unable to parse database URL:", err)
	}

	Pool, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("Unable to connect to database:", err)
	}
	
	log.Println("Connected to Postgres via pgxpool!")
}