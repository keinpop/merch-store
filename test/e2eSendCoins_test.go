package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"proj/internal/handlers"
	"proj/internal/types"
	"testing"
)

/*
В данном тесте, мы сделаем следующее:
  - Авторизуем троих друзей
  - Один друг потратит деньги на вещи, что у него останется
    0 монет
  - Но друзья захотят купить крутые парные розовые худи и чтобы
    их товарищ был с ними, они отправят ему по 250 монет
  - В конце проверим, что все транзакции верны
*/
func TestSendCoin(t *testing.T) {
	// Имена пользователей
	users := []string{"friend1", "friend2", "friend3"}
	tokens := make(map[string]string)

	// Авторизуем всех трех друзей
	for _, username := range users {
		authReq := handlers.AuthRequest{
			Username: username,
			Password: "testpass",
		}

		authBody, _ := json.Marshal(authReq)
		authRes, _ := http.Post("http://localhost:8080/api/auth",
			"application/json", bytes.NewBuffer(authBody))

		var authResp handlers.AuthResponse
		json.NewDecoder(authRes.Body).Decode(&authResp)
		tokens[username] = authResp.Token
		authRes.Body.Close()
	}

	// Friend3 покупает товары до остатка 0 монет
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/buy/pink-hoody", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["friend3"])
	req.Header.Set("Content-Type", "application/json")
	client.Do(req)

	req, _ = http.NewRequest("GET", "http://localhost:8080/api/buy/hoody", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["friend3"])
	req.Header.Set("Content-Type", "application/json")
	client.Do(req)

	req, _ = http.NewRequest("GET", "http://localhost:8080/api/buy/umbrella", nil)
	req.Header.Set("Authorization", "Bearer "+tokens["friend3"])
	req.Header.Set("Content-Type", "application/json")
	client.Do(req)

	// Шаг 3: Friend1 и Friend2 отправляют по 250 монет Friend3
	sendMoney := func(from, to string, amount int) {
		sendReq := types.SentTrans{
			ToUser: to,
			Amount: amount,
		}

		sendBody, _ := json.Marshal(sendReq)
		req, _ := http.NewRequest("POST", "http://localhost:8080/api/sendCoin",
			bytes.NewBuffer(sendBody))
		req.Header.Set("Authorization", "Bearer "+tokens[from])
		req.Header.Set("Content-Type", "application/json")
		client.Do(req)
	}

	sendMoney("friend1", "friend3", 250)
	sendMoney("friend2", "friend3", 250)

	// Проверяем балансы и историю транзакций
	getUserInfo := func(username string) types.InfoResponse {
		req, _ := http.NewRequest("GET", "http://localhost:8080/api/info", nil)
		req.Header.Set("Authorization", "Bearer "+tokens[username])
		res, _ := client.Do(req)

		var info types.InfoResponse
		json.NewDecoder(res.Body).Decode(&info)
		res.Body.Close()
		return info
	}

	// Проверяем баланс friend3
	friend3Info := getUserInfo("friend3")
	if friend3Info.Coins != 500 {
		t.Errorf("Invalid balance for friend3: %d", friend3Info.Coins)
	}

	// Проверяем историю транзакций
	receivedCount := 0
	for _, tx := range friend3Info.CoinHistory.Received {
		if tx.Amount == 250 && (tx.FromUser == "friend1" || tx.FromUser == "friend2") {
			receivedCount++
		}
	}
	if receivedCount != 2 {
		t.Errorf("Expected 2 received transactions, got %d", receivedCount)
	}

	// Проверяем отправленные транзакции у friend1 и friend2
	for _, user := range []string{"friend1", "friend2"} {
		info := getUserInfo(user)
		sentCorrect := false
		for _, tx := range info.CoinHistory.Sent {
			if tx.ToUser == "friend3" && tx.Amount == 250 {
				sentCorrect = true
				break
			}
		}
		if !sentCorrect {
			t.Errorf("%s has no correct sent transaction", user)
		}
	}

	fmt.Println("All transactions verified successfully!")
}
