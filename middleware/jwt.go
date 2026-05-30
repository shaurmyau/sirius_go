package middleware

import (
    "context"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type contextKey string

const UserIDKey contextKey = "userID"

func JWTAuth(secret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            auth := r.Header.Get("Authorization")
            if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
                http.Error(w, "missing or invalid token", http.StatusUnauthorized)
                return
            }
            tokenStr := auth[7:]
            token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
                if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, jwt.ErrSignatureInvalid
                }
                return []byte(secret), nil
            })
            if err != nil || !token.Valid {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            claims, ok := token.Claims.(jwt.MapClaims)
            if !ok {
                http.Error(w, "invalid claims", http.StatusUnauthorized)
                return
            }
            sub, ok := claims["sub"].(string)
            if !ok {
                http.Error(w, "missing sub claim", http.StatusUnauthorized)
                return
            }
            userID, err := uuid.Parse(sub)
            if err != nil {
                http.Error(w, "invalid sub uuid", http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), UserIDKey, userID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}