package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dadidudidap/url-shortener/database"
	"github.com/dadidudidap/url-shortener/routes"
	"github.com/dadidudidap/url-shortener/ws"
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
	go ws.Manager.ListenRedis()

	// Analytics is optional at startup: if RabbitMQ is down both workers keep retrying.
	rabbitURL := database.RabbitURL()
	database.Rabbit = database.NewRabbitPublisher(rabbitURL)

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		database.Rabbit.Run(appCtx)
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	} else if _, err := strconv.Atoi(port); err != nil {
		log.Fatal("Invalid PORT environment variable")
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: routes.RegisterRoutes(),
	}

	go func() {
		log.Println("Starting server on http://127.0.0.1:" + port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-appCtx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Println("Shutdown error:", err)
	}
	// Shutdown ignores upgraded connections, so close viewers explicitly.
	ws.Manager.CloseAll()

	workers.Wait()
	database.Rabbit.Close()
}
