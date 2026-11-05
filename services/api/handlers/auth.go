package handlers

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dadidudidap/url-shortener/shared/pb"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userPayload struct {
	ID    int32  `json:"id"`
	Email string `json:"email"`
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

// RegisterHandler proxies account creation to the auth service.
func RegisterHandler(client pb.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		user, err := client.Register(r.Context(), &pb.RegisterRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			switch status.Code(err) {
			case codes.AlreadyExists:
				http.Error(w, "Email is already registered", http.StatusConflict)
			case codes.InvalidArgument:
				http.Error(w, status.Convert(err).Message(), http.StatusBadRequest)
			default:
				http.Error(w, "Auth service unavailable", http.StatusBadGateway)
			}
			return
		}

		writeJSON(w, http.StatusCreated, userPayload{ID: user.GetId(), Email: user.GetEmail()})
	}
}

// LoginHandler exchanges credentials for a bearer token.
func LoginHandler(client pb.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req credentials
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		resp, err := client.Login(r.Context(), &pb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			switch status.Code(err) {
			case codes.Unauthenticated:
				http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			case codes.InvalidArgument:
				http.Error(w, status.Convert(err).Message(), http.StatusBadRequest)
			default:
				http.Error(w, "Auth service unavailable", http.StatusBadGateway)
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"token": resp.GetToken(),
			"user":  userPayload{ID: resp.GetUser().GetId(), Email: resp.GetUser().GetEmail()},
		})
	}
}
