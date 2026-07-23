package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"mailnest-be/internal/auth"
	"mailnest-be/internal/response"
	"mailnest-be/internal/storage"
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

		user, err := a.store.FindUserByID(userID)
		if errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusUnauthorized, "账号已停用或登录已过期")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "鉴权失败")
			return
		}
		if !user.Enabled {
			response.Error(w, http.StatusUnauthorized, "账号已停用或登录已过期")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) adminMiddleware(next http.Handler) http.Handler {
	return a.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "未登录或登录已过期")
			return
		}
		user, err := a.store.FindUserByID(userID)
		if errors.Is(err, storage.ErrNotFound) {
			response.Error(w, http.StatusUnauthorized, "账号已停用或登录已过期")
			return
		}
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "鉴权失败")
			return
		}
		if !user.Enabled {
			response.Error(w, http.StatusUnauthorized, "账号已停用或登录已过期")
			return
		}
		if !user.IsAdmin {
			response.Error(w, http.StatusForbidden, "需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	}))
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
