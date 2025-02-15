package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"proj/internal/session"
	"proj/internal/user"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

const (
	JWTFieldUserID = "id"

	UsernameMaxLen = 32
	PasswordMaxLen = 72
)

type UserHandlers struct {
	UserRepo user.UserRepo
	Sessions session.SessionManagerRepo
	Logger   *zap.SugaredLogger
}

func (h *UserHandlers) Info(w http.ResponseWriter, r *http.Request) {
	// Для начала получим данные о пользователе из jwt,
	// а именно его айди
	userID := GetUserDataByJWT(
		w, r, JWTFieldUserID,
		h.Sessions.GetSecret(), h.Logger,
	)
	if userID == "" {
		// не отправим ошибку, тк в функции уже это предусмотрено
		return
	}

	info, err := h.UserRepo.Info(userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
			return
		}

		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(info); err != nil {
		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	h.Logger.Infof("successfully received information for userID - %s -", userID)
}

type SendCoinRequest struct {
	ToUser string `json:"toUser"`
	Amount int    `json:"amount"`
}

func (h *UserHandlers) SendCoin(w http.ResponseWriter, r *http.Request) {
	var req SendCoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
		return
	}

	userID := GetUserDataByJWT(
		w, r, JWTFieldUserID,
		h.Sessions.GetSecret(), h.Logger,
	)

	err := h.UserRepo.SendCoin(userID, req.ToUser, req.Amount)
	if err != nil {
		// Если ошибки связаны с отправкой несуществующему пользователю или
		// недостаточно средств -> 400
		if errors.Is(err, user.ErrUserNotFound) || errors.Is(err, user.ErrInsufficientFunds) {
			SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
			return
		}

		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	w.WriteHeader(http.StatusOK)
	h.Logger.Infof(
		"coins sent successfully from userID - %s - to username - %s -",
		userID,
		req.ToUser,
	)
}

func (h *UserHandlers) BuyItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemTitle := vars["item"]

	userID := GetUserDataByJWT(
		w, r, JWTFieldUserID,
		h.Sessions.GetSecret(), h.Logger,
	)

	err := h.UserRepo.BuyItem(userID, itemTitle)
	if err != nil {
		if errors.Is(err, user.ErrItemNotFound) ||
			errors.Is(err, user.ErrInsufficientFunds) ||
			errors.Is(err, user.ErrUserNotFound) {
			SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
			return
		}

		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	w.WriteHeader(http.StatusOK)
	h.Logger.Infof("item - %s - purchased successfully for userID - %s -", itemTitle, userID)
}

type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

/*
Я подумал, при какой ситуации мы можем получать 401
Если пароль неверный, то это же 400. Но, в целом, можем и 401.
А так же сделаем ограничение на длину имени и пароля
*/
func (h *UserHandlers) Auth(w http.ResponseWriter, r *http.Request) {
	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
		return
	}

	if len(req.Username) > UsernameMaxLen || len(req.Password) > PasswordMaxLen {
		err := errors.New("username or password has invalid size")
		SendErrorTo(w, err, http.StatusBadRequest, h.Logger)
		return
	}

	u, err := h.UserRepo.Authorize(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, user.ErrBadPassword) {
			SendErrorTo(w, err, http.StatusUnauthorized, h.Logger)
			return
		}

		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	sess, token, err := h.Sessions.Create(w, u.UserID, u.Login)
	if err != nil {
		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(AuthResponse{Token: token}); err != nil {
		SendErrorTo(w, err, http.StatusInternalServerError, h.Logger)
		return
	}

	h.Logger.Infof("session - %s - successfully created for userID - %s -", sess.ID, sess.UserID)
}
