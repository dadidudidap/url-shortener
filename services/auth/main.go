package main

import (
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	"github.com/dadidudidap/url-shortener/shared/database"
	"github.com/dadidudidap/url-shortener/shared/database/db"
	"github.com/dadidudidap/url-shortener/shared/pb"
)

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	ttl := 24 * time.Hour
	if raw := os.Getenv("JWT_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatalf("Invalid JWT_TTL: %v", err)
		}
		ttl = parsed
	}

	database.Init()

	host := os.Getenv("AUTH_GRPC_HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("AUTH_GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	addr := net.JoinHostPort(host, port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, newServer(db.New(database.Pool), secret, ttl))

	log.Printf("Auth gRPC Microservice booting up on %s...", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
