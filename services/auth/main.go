package main

import (
	"context"
	"log"
	"net"
	"os"

	// Import the generated protobuf code from your root module
	"github.com/dadidudidap/url-shortener/shared/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedAuthServiceServer
}

// ValidateToken implements the function defined in your .proto file
func (s *server) ValidateToken(ctx context.Context, req *pb.TokenRequest) (*pb.TokenResponse, error) {
	log.Printf("Received token validation request for API Key: %s", req.GetApiKey())

	// Fail closed: an unset AUTH_API_KEY matches no key.
	isValid := req.GetApiKey() != "" && req.GetApiKey() == os.Getenv("AUTH_API_KEY")

	return &pb.TokenResponse{
		IsValid: isValid,
		UserId:  "user_999",
	}, nil
}

func main() {
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
	pb.RegisterAuthServiceServer(grpcServer, &server{})

	log.Printf("Auth gRPC Microservice booting up on %s...", addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
