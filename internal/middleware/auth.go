package middleware

import (
	"context"
	"net/http"

	"github.com/farigab/bragdoc/internal/security"
)

type contextKey string

// UserLoginKey is the context key for the authenticated user login.
const UserLoginKey contextKey = "userLogin"

// Auth is a chi middleware that validates the JWT cookie and stores the
// authenticated userLogin in the request context. Routes protected by this
// middleware can retrieve the login via UserLoginFromContext.
func Auth(jwtSvc security.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			userLogin, err := jwtSvc.ExtractUserLogin(cookie.Value)
			if err != nil || userLogin == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserLoginKey, userLogin)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserLoginFromContext extracts the authenticated user login from the request context.
// Returns the login and true if present, or empty string and false otherwise.
func UserLoginFromContext(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(UserLoginKey).(string)
	return login, ok && login != ""
}
