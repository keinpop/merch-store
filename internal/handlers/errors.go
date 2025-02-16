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

type ServerError struct {
	Errors string `json:"errors"`
}

func (e *ServerError) Error() string {
	return e.Errors
}

func NewErrorServer(err error) ServerError {
	return ServerError{
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
