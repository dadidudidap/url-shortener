package main

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dadidudidap/url-shortener/shared/database/db"
	"github.com/dadidudidap/url-shortener/shared/pb"
	"github.com/dadidudidap/url-shortener/shared/token"
)

const minPasswordLength = 8

type server struct {
	pb.UnimplementedAuthServiceServer
	queries *db.Queries
	secret  string
	ttl     time.Duration
}

func newServer(queries *db.Queries, secret string, ttl time.Duration) *server {
	return &server{queries: queries, secret: secret, ttl: ttl}
}

func normalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validateCredentials(email, password string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("valid email required")
	}
	if len(password) < minPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.UserResponse, error) {
	email := normalizeEmail(req.GetEmail())
	if err := validateCredentials(email, req.GetPassword()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not process password")
	}

	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
	})
	if isDuplicateKey(err) {
		return nil, status.Error(codes.AlreadyExists, "email already registered")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create user")
	}

	return &pb.UserResponse{Id: user.ID, Email: user.Email}, nil
}

func (s *server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := normalizeEmail(req.GetEmail())
	invalid := status.Error(codes.Unauthenticated, "invalid email or password")

	if err := validateCredentials(email, req.GetPassword()); err != nil {
		return nil, invalid
	}

	user, err := s.queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, invalid
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not look up user")
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) != nil {
		return nil, invalid
	}

	signed, err := token.Sign(s.secret, token.Claims{UserID: user.ID, Email: user.Email}, s.ttl)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not issue token")
	}

	return &pb.LoginResponse{
		Token: signed,
		User:  &pb.UserResponse{Id: user.ID, Email: user.Email},
	}, nil
}
