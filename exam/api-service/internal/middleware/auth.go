package middleware

import (
	"context"
	"net/http"
	"strings"
	
	"api-service/internal/metrics"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const ClaimsKey contextKey = "claims"

type Auth struct {
	jwks keyfunc.Keyfunc
}

func NewAuth(jwks keyfunc.Keyfunc) *Auth {
	return &Auth{jwks: jwks}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			metrics.ReqTotal.WithLabelValues("GET", "/api/img", "401").Inc()
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(hdr, "Bearer ")

		token, err := jwt.Parse(tokenStr, a.jwks.Keyfunc)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, token.Claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SubFromClaims(r *http.Request) (string, bool) {
	claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
	if !ok {
		return "", false
	}
	sub, ok := claims["sub"].(string)
	return sub, ok
}
