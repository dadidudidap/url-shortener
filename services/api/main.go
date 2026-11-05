package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/dadidudidap/url-shortener/services/api/routes"
	"github.com/dadidudidap/url-shortener/services/api/ws"
	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// Dial the gRPC Auth Service
	authHost := os.Getenv("AUTH_GRPC_HOST")
	if authHost == "" {
		authHost = "localhost"
	}
	authPort := os.Getenv("AUTH_GRPC_PORT")
	if authPort == "" {
		authPort = "50051"
	}
	authAddr := net.JoinHostPort(authHost, authPort)

	grpcConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth gRPC service: %v", err)
	}
	defer grpcConn.Close()

	authClient := pb.NewAuthServiceClient(grpcConn)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	} else if _, err := strconv.Atoi(port); err != nil {
		log.Fatal("Invalid PORT environment variable")
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: routes.RegisterRoutes(authClient, jwtSecret),
	}

	go func() {
		log.Println("Starting server on port http://127.0.0.1:" + port)
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
