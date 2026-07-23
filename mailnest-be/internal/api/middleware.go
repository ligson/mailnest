package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"mailnest-be/internal/auth"
	"mailnest-be/internal/response"
)

type contextKey string

const userIDKey contextKey = "userID"

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		tokenValue, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || strings.TrimSpace(tokenValue) == "" {
			response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}

		userID, err := auth.ParseToken(tokenValue, a.cfg.App.JWTSecret)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) issueToken(userID int64) (string, error) {
	expireHours := a.cfg.App.JWTExpireHours
	if expireHours <= 0 {
		expireHours = 168
	}
	return auth.GenerateToken(userID, a.cfg.App.JWTSecret, time.Duration(expireHours)*time.Hour)
}

func currentUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(userIDKey).(int64)
	return userID, ok
}
