package middleware

import (
	"context"
	"net/http"

	"jenkinsAgent/config"
	"jenkinsAgent/internal/store"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UsernameKey contextKey = "username"
	UserRoleKey contextKey = "user_role"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Decode session cookie (simple format: username)
		username := cookie.Value
		user, err := store.NewUserStore().GetByUsername(username)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, user.ID)
		ctx = context.WithValue(ctx, UsernameKey, user.Username)
		ctx = context.WithValue(ctx, UserRoleKey, string(user.Role))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUsername(r *http.Request) string {
	if v, ok := r.Context().Value(UsernameKey).(string); ok {
		return v
	}
	return ""
}

func SetSession(w http.ResponseWriter, username string, secretKey string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    username,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7, // 7 days
	})
	_ = config.Global // reference to ensure import
}

func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}
