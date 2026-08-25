package main

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type authClaims struct {
	AdminID     uint   `json:"admin_id"`
	AuthVersion uint   `json:"auth_version"`
	Username    string `json:"username"`
	jwt.RegisteredClaims
}

func signToken(secret string, admin Admin) (string, error) {
	now := time.Now()
	claims := authClaims{
		AdminID:     admin.ID,
		AuthVersion: admin.AuthVersion,
		Username:    admin.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func parseAuthToken(secret, tokenString string) (*authClaims, error) {
	if tokenString == "" {
		return nil, errors.New("missing token")
	}
	claims := &authClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid || claims.AdminID == 0 || claims.AuthVersion == 0 {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
