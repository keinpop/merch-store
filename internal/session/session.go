package session

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	endTimeDur = 14 * 24 * time.Hour
)

var (
	ErrNoAuth = errors.New("session not found")
)

type Session struct {
	ID        string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func NewSession(userID string) *Session {
	startTime := time.Now()
	endTime := startTime.Add(endTimeDur)

	return &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		StartTime: startTime,
		EndTime:   endTime,
	}
}
