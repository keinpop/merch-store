package session

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestSessionManager(t *testing.T) (*SessionManager, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}

	logger := zap.NewNop().Sugar()

	return NewSessionManager(db, logger, "test-secret"), mock
}

func TestSessionManager_Check(t *testing.T) {
	tests := []struct {
		name          string
		token         string
		mockDBSetup   func(sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name:  "Success",
			token: "valid-token",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE session_id = \$1`).
					WithArgs("session1").
					WillReturnRows(sqlmock.NewRows([]string{"session_id", "user_id", "start_time", "end_time"}).
						AddRow("session1", "user1", time.Now(), time.Now().Add(endTimeDur)))
			},
			expectedError: nil,
		},
		{
			name:  "InvalidToken",
			token: "invalid-token",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Нет запросов к базе данных, так как токен невалидный
			},
			expectedError: ErrNoAuth,
		},
		{
			name:  "SessionNotFound",
			token: "valid-token",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE session_id = \$1`).
					WithArgs("session1").
					WillReturnError(sql.ErrNoRows)
			},
			expectedError: ErrNoAuth,
		},
		{
			name:  "SessionExpired",
			token: "valid-token",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE session_id = \$1`).
					WithArgs("session1").
					WillReturnRows(sqlmock.NewRows([]string{"session_id", "user_id", "start_time", "end_time"}).
						AddRow("session1", "user1", time.Now().Add(-2*endTimeDur), time.Now().Add(-endTimeDur)))
			},
			expectedError: ErrNoAuth,
		},
		{
			name:  "DatabaseError",
			token: "valid-token",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE session_id = \$1`).
					WithArgs("session1").
					WillReturnError(errors.New("database error"))
			},
			expectedError: ErrInternalDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, mock := newTestSessionManager(t)
			tt.mockDBSetup(mock)

			// Создаем http с токеном
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			// Генерация валидного токена для теста
			if tt.token == "valid-token" {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					FieldSessionID: "session1",
				})
				tokenString, _ := token.SignedString([]byte(sm.GetSecret()))
				req.Header.Set("Authorization", "Bearer "+tokenString)
			}

			sess, err := sm.Check(req)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, sess)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSessionManager_Create(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		login         string
		mockDBSetup   func(sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name:   "SuccessNewSession",
			userID: "user1",
			login:  "login1",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				//  запрос для проверки существующей сессии
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE user_id = \$1`).
					WithArgs("user1").
					WillReturnError(sql.ErrNoRows)

				//  запрос для вставки новой сессии
				mock.ExpectExec(`INSERT INTO sessions \(session_id, user_id, start_time, end_time\) VALUES \(\$1, \$2, \$3, \$4\)`).
					WithArgs(sqlmock.AnyArg(), "user1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
		{
			name:   "SuccessExistingSession",
			userID: "user1",
			login:  "login1",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE user_id = \$1`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"session_id", "user_id", "start_time", "end_time"}).
						AddRow("session1", "user1", time.Now(), time.Now().Add(endTimeDur)))
			},
			expectedError: nil,
		},
		{
			name:   "DatabaseError",
			userID: "user1",
			login:  "login1",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				//  запрос для проверки существующей сессии
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE user_id = \$1`).
					WithArgs("user1").
					WillReturnError(errors.New("database error"))
			},
			expectedError: ErrInternalDB,
		},
		{
			name:   "SuccessExpiredSession",
			userID: "user1",
			login:  "login1",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT session_id, user_id, start_time, end_time FROM sessions WHERE user_id = \$1`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"session_id", "user_id", "start_time", "end_time"}).
						AddRow("session1", "user1", time.Now().Add(-2*endTimeDur), time.Now().Add(-endTimeDur))) // Просроченная сессия

				mock.ExpectExec(`DELETE FROM sessions WHERE session_id = \$1`).
					WithArgs("session1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectExec(`INSERT INTO sessions \(session_id, user_id, start_time, end_time\) VALUES \(\$1, \$2, \$3, \$4\)`).
					WithArgs(sqlmock.AnyArg(), "user1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, mock := newTestSessionManager(t)
			tt.mockDBSetup(mock)

			// Вызываем метод Create
			sess, token, err := sm.Create(httptest.NewRecorder(), tt.userID, tt.login)

			// Проверяем результаты
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, sess)
				assert.NotEmpty(t, token)
			}

			// Проверяем, что все ожидания мока выполнены
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
