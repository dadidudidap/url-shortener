package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/handlers"
	"github.com/dadidudidap/url-shortener/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	database.Init()
	if err := database.Pool.Ping(context.Background()); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	database.InitRedis()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000" // Default port if not specified
	} else if _, err := strconv.Atoi(port); err != nil {
		log.Fatal("Invalid PORT environment variable")
	}
	
	server := &http.Server{
		Addr:    ":" + port,
		Handler: routes.RegisterRoutes(),
	}

	go handlers.StartClickWorker()

	go func() {
		log.Println("Starting server on http://127.0.0.1:" + port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Println("Shutdown error:", err)
	}
}
