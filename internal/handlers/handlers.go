package handlers

import (
	"fmt"
	"net/http"
	"proj/internal/middleware"
	"proj/internal/session"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func GetUserDataByJWT(
	w http.ResponseWriter,
	r *http.Request,
	field string,
	secret string,
	logger *zap.SugaredLogger,
) string {
	t := r.Header.Get("Authorization")
	if t == "" {
		logger.Errorf("%v. More: %v", ErrHeaderNotSet)
		SendErrorTo(w, ErrHeaderNotSet, http.StatusBadRequest, logger)
		return ""
	}

	t = strings.TrimPrefix(t, "Bearer ")

	token, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		// Проверка метода подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logger.Error(session.ErrUnexpectedMethod)
			return nil, session.ErrUnexpectedMethod
		}
		// Возвращаем секрет для проверки подписи
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		fmt.Println(err)
		logger.Error(ErrInvalidToken, err)
		SendErrorTo(w, ErrInvalidToken, http.StatusUnauthorized, logger)
		return ""
	}

	// Извлекаем claims и получаем username
	claims, okToken := token.Claims.(jwt.MapClaims)
	m, okClaims := claims["user"].(map[string]interface{})
	if !okToken || !okClaims || claims["user"].(map[string]interface{})[field] == nil {
		logger.Error(ErrInvalidToken, err)
		SendErrorTo(w, ErrInvalidToken, http.StatusUnauthorized, logger)
		return ""
	}

	return m[field].(string)
}

func NewRouters(uh *UserHandlers, sm *session.SessionManager, logger *zap.SugaredLogger) http.Handler {
	r := mux.NewRouter()

	initHandlers(r, sm, uh)

	return r
}

func initHandlers(
	r *mux.Router,
	sm *session.SessionManager,
	userHandler *UserHandlers,
) {
	authRouter := r.PathPrefix("/api").Subrouter()
	authRouter.Use(middleware.Auth(sm))
	authRouter.HandleFunc("/info", userHandler.Info).Methods("GET")
	authRouter.HandleFunc("/sendCoin", userHandler.SendCoin).Methods("POST")
	authRouter.HandleFunc("/buy/{item}", userHandler.BuyItem).Methods("GET")

	noAuthRouter := r.PathPrefix("/api").Subrouter()
	noAuthRouter.HandleFunc("/auth", userHandler.Auth).Methods("POST")
}
