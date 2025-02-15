package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

var (
	ErrHeaderNotSet    = errors.New("header not set")
	ErrInvalidToken    = errors.New("invalid token in header")
	ErrInvalidUsername = errors.New("invalid username")
)

type ErrorServer struct {
	Errors string `json:"errors"`
}

func (e *ErrorServer) Error() string {
	return e.Errors
}

func NewErrorServer(err error) ErrorServer {
	return ErrorServer{
		Errors: err.Error(),
	}
}

func SendErrorTo(w http.ResponseWriter, err error, statusCode int, logger *zap.SugaredLogger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if errEncode := json.NewEncoder(w).Encode(NewErrorServer(err)); errEncode != nil {
		logger.Error(errEncode)
	}
}
