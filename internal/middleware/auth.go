package middleware

import (
	"net/http"
	"proj/internal/session"
)

func Auth(sm *session.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Проверка сессии пользователя
			sess, err := sm.Check(r)
			if err != nil {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}

			// Добавляем сессию в контекст и передаем дальше
			ctx := session.ContextWithSession(r.Context(), sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
