package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"proj/internal/session"
	"proj/internal/types"
	"proj/internal/user"
	"reflect"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	MockJWTToken = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NDk2OTk5OTksImlhdCI6MTczOTY1MjQ4OCwic2Vzc2lvbl9pZCI6ImM4ZTRlYzc5LTdlZDctNDAxNy1hNzQ5LTExYWQ1MzU3MmVkMCIsInVzZXIiOnsiaWQiOiJjMWU0ZWM3OS03ZWQ3LTQwMTctYTc0OS0xMWFkNTM1NzJlZDAiLCJsb2dpbiI6InVzZXJuYW1lIn19.qkbQJYL4RjlILmrXHFHp7mBE0ShuzTF8i6sm_a64Ot8"
	MockUserID   = "c1e4ec79-7ed7-4017-a749-11ad53572ed0"
	MockSecret   = "mysuperpupermegaultraSecret"
)

func TestGetUserDataByJWT(t *testing.T) {
	secret := "123"
	tests := []struct {
		name                string
		authorizationHeader string
		expectedStatusCode  int
		setupToken          func() string
		expectedUserID      string
	}{
		{
			name:                "missing authorization header",
			authorizationHeader: "",
			expectedStatusCode:  http.StatusBadRequest,
			setupToken:          nil,
			expectedUserID:      "",
		},
		{
			name:                "invalid token",
			authorizationHeader: "Bearer invalid.token",
			expectedStatusCode:  http.StatusUnauthorized,
			setupToken:          nil,
			expectedUserID:      "",
		},
		{
			name:                "missing field in claims",
			authorizationHeader: "",
			expectedStatusCode:  http.StatusUnauthorized,
			setupToken: func() string {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user": map[string]interface{}{},
				})
				signedToken, err := token.SignedString(secret)
				if err != nil {
					require.Error(t, err)
				}
				return signedToken
			},
			expectedUserID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/some-url", nil)
			if tt.authorizationHeader != "" {
				req.Header.Set("Authorization", tt.authorizationHeader)
			}

			var signedToken string
			if tt.setupToken != nil {
				signedToken = tt.setupToken()
				req.Header.Set("Authorization", "Bearer "+signedToken)
			}

			w := httptest.NewRecorder()

			userID := GetUserDataByJWT(w, req, "id", secret, zap.NewNop().Sugar())

			resp := w.Result()
			defer resp.Body.Close()

			require.Equal(t, resp.StatusCode, tt.expectedStatusCode)
			require.Equal(t, tt.expectedUserID, userID)
		})
	}
}

func NewCtrlAndUserRepos(t *testing.T) (*user.MockUserRepo, *session.MockSessionManagerRepo, *UserHandlers) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := user.NewMockUserRepo(ctrl)
	mockSessionManager := session.NewMockSessionManagerRepo(ctrl)
	logger := zap.NewNop().Sugar()

	handler := &UserHandlers{
		UserRepo: mockUserRepo,
		Sessions: mockSessionManager,
		Logger:   logger,
	}

	return mockUserRepo, mockSessionManager, handler
}
func TestUserHandlers_Info(t *testing.T) {
	tests := map[string]func(t *testing.T){
		"successful info retrieval": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().Info(MockUserID).Return(types.InfoResponse{
				Coins: 100,
				Inventory: []types.Item{
					{Type: "t-shirt", Quantity: 2},
				},
				CoinHistory: types.Transaction{
					Received: []types.ReceivedTrans{
						{FromUser: "user1", Amount: 50},
					},
					Sent: []types.SentTrans{
						{ToUser: "user2", Amount: 30},
					},
				},
			}, nil).Times(1)

			req := httptest.NewRequest("GET", "/info", nil)
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.Info(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
			}

			var response types.InfoResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			expectedResponse := types.InfoResponse{
				Coins: 100,
				Inventory: []types.Item{
					{Type: "t-shirt", Quantity: 2},
				},
				CoinHistory: types.Transaction{
					Received: []types.ReceivedTrans{
						{FromUser: "user1", Amount: 50},
					},
					Sent: []types.SentTrans{
						{ToUser: "user2", Amount: 30},
					},
				},
			}

			if !reflect.DeepEqual(response, expectedResponse) {
				t.Errorf("expected response %v, got %v", expectedResponse, response)
			}
		},

		"user not found": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().Info(MockUserID).Return(types.InfoResponse{}, user.ErrUserNotFound).Times(1)

			req := httptest.NewRequest("GET", "/info", nil)
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.Info(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrUserNotFound.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"internal server error": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().Info(MockUserID).Return(types.InfoResponse{}, errors.New("internal error")).Times(1)

			req := httptest.NewRequest("GET", "/info", nil)
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.Info(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "internal error"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},
	}

	for name, test := range tests {
		t.Run(name, test)
	}
}

func TestUserHandlers_SendCoin(t *testing.T) {
	tests := map[string]func(t *testing.T){
		"successful coin send": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().SendCoin(MockUserID, "recipientUser", 50).Return(nil).Times(1)

			reqBody := SendCoinRequest{
				ToUser: "recipientUser",
				Amount: 50,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/send-coin", bytes.NewBuffer(body))
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.SendCoin(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
			}
		},

		"user not found": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().SendCoin(MockUserID, "nonexistentUser", 50).Return(user.ErrUserNotFound).Times(1)

			reqBody := SendCoinRequest{
				ToUser: "nonexistentUser",
				Amount: 50,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/send-coin", bytes.NewBuffer(body))
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.SendCoin(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrUserNotFound.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"insufficient funds": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().SendCoin(MockUserID, "recipientUser", 1000).Return(user.ErrInsufficientFunds).Times(1)

			reqBody := SendCoinRequest{
				ToUser: "recipientUser",
				Amount: 1000,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/send-coin", bytes.NewBuffer(body))
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.SendCoin(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrInsufficientFunds.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"internal server error": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().SendCoin(MockUserID, "recipientUser", 50).Return(errors.New("internal error")).Times(1)

			reqBody := SendCoinRequest{
				ToUser: "recipientUser",
				Amount: 50,
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/send-coin", bytes.NewBuffer(body))
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.SendCoin(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "internal error"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},
	}

	for name, test := range tests {
		t.Run(name, test)
	}
}

func TestUserHandlers_BuyItem(t *testing.T) {
	tests := map[string]func(t *testing.T){
		"successful item purchase": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().BuyItem(MockUserID, "t-shirt").Return(nil).Times(1)

			req := httptest.NewRequest("POST", "/buy/t-shirt", nil)
			req = mux.SetURLVars(req, map[string]string{"item": "t-shirt"})
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.BuyItem(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
			}
		},

		"item not found": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().BuyItem(MockUserID, "nonexistent-item").Return(user.ErrItemNotFound).Times(1)

			req := httptest.NewRequest("POST", "/buy/nonexistent-item", nil)
			req = mux.SetURLVars(req, map[string]string{"item": "nonexistent-item"})
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.BuyItem(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrItemNotFound.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"insufficient funds": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().BuyItem(MockUserID, "expensive-item").Return(user.ErrInsufficientFunds).Times(1)

			req := httptest.NewRequest("POST", "/buy/expensive-item", nil)
			req = mux.SetURLVars(req, map[string]string{"item": "expensive-item"})
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.BuyItem(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrInsufficientFunds.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"user not found": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().BuyItem(MockUserID, "t-shirt").Return(user.ErrUserNotFound).Times(1)

			req := httptest.NewRequest("POST", "/buy/t-shirt", nil)
			req = mux.SetURLVars(req, map[string]string{"item": "t-shirt"})
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.BuyItem(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrUserNotFound.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"internal server error": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockSessionManager.EXPECT().GetSecret().Return(MockSecret).Times(1)
			mockUserRepo.EXPECT().BuyItem(MockUserID, "t-shirt").Return(errors.New("internal error")).Times(1)

			req := httptest.NewRequest("POST", "/buy/t-shirt", nil)
			req = mux.SetURLVars(req, map[string]string{"item": "t-shirt"})
			req.Header.Set("Authorization", MockJWTToken)
			w := httptest.NewRecorder()

			handler.BuyItem(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "internal error"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},
	}

	for name, test := range tests {
		t.Run(name, test)
	}
}

func TestUserHandlers_Auth(t *testing.T) {
	tests := map[string]func(t *testing.T){
		"successful authentication": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockUser := user.User{
				UserID: MockUserID,
				Login:  "username",
			}
			mockUserRepo.EXPECT().Authorize("username", "password").Return(mockUser, nil).Times(1)

			mockSession := &session.Session{
				ID:     "session-id",
				UserID: MockUserID,
			}
			mockSessionManager.EXPECT().Create(gomock.Any(), MockUserID, "username").Return(mockSession, "token", nil).Times(1)

			reqBody := AuthRequest{
				Username: "username",
				Password: "password",
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/auth", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.Auth(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
			}

			var response AuthResponse
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			expectedToken := "token"
			if response.Token != expectedToken {
				t.Errorf("expected token %q, got %q", expectedToken, response.Token)
			}
		},

		"invalid username or password size": func(t *testing.T) {
			_, _, handler := NewCtrlAndUserRepos(t)

			reqBody := AuthRequest{
				Username: strings.Repeat("a", UsernameMaxLen+1),
				Password: "password",
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/auth", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.Auth(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "username or password has invalid size"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"bad password": func(t *testing.T) {
			mockUserRepo, _, handler := NewCtrlAndUserRepos(t)

			mockUserRepo.EXPECT().Authorize("username", "wrongpassword").Return(user.User{}, user.ErrBadPassword).Times(1)

			reqBody := AuthRequest{
				Username: "username",
				Password: "wrongpassword",
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/auth", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.Auth(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status code %d, got %d", http.StatusUnauthorized, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := user.ErrBadPassword.Error()
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"internal server error during authorization": func(t *testing.T) {
			mockUserRepo, _, handler := NewCtrlAndUserRepos(t)

			mockUserRepo.EXPECT().Authorize("username", "password").Return(user.User{}, errors.New("internal error")).Times(1)

			reqBody := AuthRequest{
				Username: "username",
				Password: "password",
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/auth", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.Auth(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "internal error"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},

		"internal server error during session creation": func(t *testing.T) {
			mockUserRepo, mockSessionManager, handler := NewCtrlAndUserRepos(t)

			mockUser := user.User{
				UserID: MockUserID,
				Login:  "username",
			}
			mockUserRepo.EXPECT().Authorize("username", "password").Return(mockUser, nil).Times(1)

			mockSessionManager.EXPECT().Create(gomock.Any(), MockUserID, "username").Return(nil, "", errors.New("internal error")).Times(1)

			reqBody := AuthRequest{
				Username: "username",
				Password: "password",
			}
			body, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("POST", "/auth", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.Auth(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResponse ErrorServer
			if err := json.NewDecoder(resp.Body).Decode(&errResponse); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			expectedError := "internal error"
			if errResponse.Errors != expectedError {
				t.Errorf("expected error message %q, got %q", expectedError, errResponse.Errors)
			}
		},
	}

	// Запускаем тесты
	for name, test := range tests {
		t.Run(name, test)
	}
}
