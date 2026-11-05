package token

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int32
	Email  string
}

func Sign(secret string, claims Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	raw := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   strconv.Itoa(int(claims.UserID)),
		"email": claims.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
	})
	return raw.SignedString([]byte(secret))
}

func Verify(secret, raw string) (Claims, error) {
	parsed, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return Claims{}, err
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}

	sub, err := claims.GetSubject()
	if err != nil {
		return Claims{}, err
	}
	id, err := strconv.Atoi(sub)
	if err != nil || id <= 0 {
		return Claims{}, errors.New("invalid token subject")
	}

	email, _ := claims["email"].(string)
	return Claims{UserID: int32(id), Email: email}, nil
}
