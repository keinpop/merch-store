package user

import "proj/internal/types"

type User struct {
	UserID         string `json:"user_id"`
	Login          string `json:"login"`
	passwordHash   string
	AmountInWallet int `json:"amount_in_wallet"`
}

type UserRepo interface {
	Authorize(login, password string) (User, error)

	Info(userID string) (types.InfoResponse, error)
	SendCoin(userID, toUserLogin string, amount int) error
	BuyItem(userID, itemTitle string) error
}
