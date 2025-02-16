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
Так как фронтенд части для e2e нет, то обойдемся дерганьем хэндлеров
Сделаем тест следующего характера:
  - Авторизуем пользователя, как нового
  - Он купит себе 3 вещи
  - И проверит, что все его вещи находятся в инвентаре
    и его не обсчитали:))) проверим, что денег на счету верно
*/
func TestMerchBuy(t *testing.T) {
	// Авторизация пользователя
	authReq := handlers.AuthRequest{
		Username: "mr.bombastic52",
		Password: "1a2b3c4dAGGGG",
	}

	authReqBody, err := json.Marshal(authReq)
	if err != nil {
		t.Fatalf("Failed to marshal auth request: %v", err)
	}

	authRes, err := http.Post("http://localhost:8080/api/auth", "application/json", bytes.NewBuffer(authReqBody))
	if err != nil {
		t.Fatalf("Failed to send auth request: %v", err)
	}
	defer authRes.Body.Close()

	if authRes.StatusCode != http.StatusOK {
		t.Fatalf("Auth failed with status: %d", authRes.StatusCode)
	}

	var authResp handlers.AuthResponse
	if err := json.NewDecoder(authRes.Body).Decode(&authResp); err != nil {
		t.Fatalf("Failed to decode auth response: %v", err)
	}

	token := authResp.Token
	fmt.Printf("User authenticated, token: %s\n", token)

	// Покупка вещей
	itemsToBuy := []string{"t-shirt", "cup", "pen", "pink-hoody"} // Список товаров для покупки

	for _, item := range itemsToBuy {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/buy/"+item, nil)
		if err != nil {
			t.Fatalf("Failed to create buy request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		buyRes, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send buy request: %v", err)
		}
		defer buyRes.Body.Close()

		if buyRes.StatusCode != http.StatusOK {
			t.Fatalf("Buy failed with status: %d, item: %s", buyRes.StatusCode, item)
		}

		fmt.Printf("Item %s purchased successfully\n", item)
	}

	// Получение информации о пользователе
	req, err := http.NewRequest("GET", "http://localhost:8080/api/info", nil)
	if err != nil {
		t.Fatalf("Failed to create info request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	infoRes, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send info request: %v", err)
	}
	defer infoRes.Body.Close()

	if infoRes.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get user info, status: %d", infoRes.StatusCode)
	}

	var userInfo types.InfoResponse
	if err := json.NewDecoder(infoRes.Body).Decode(&userInfo); err != nil {
		t.Fatalf("Failed to decode user info: %v", err)
	}

	// Проверка инвентаря и баланса
	// Проверяем, что все купленные товары есть в инвентаре
	for _, item := range itemsToBuy {
		found := false
		for _, invItem := range userInfo.Inventory {
			if invItem.Type == item && invItem.Quantity == 1 {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Item %s not found in inventory", item)
		}
	}

	// Проверяем, что баланс счёта корректный
	//  Цены товаров: t-shirt (80), cup (20), pen (10), pink-hoody (500)
	expectedBalance := 1000 - 80 - 20 - 10 - 500
	if userInfo.Coins != expectedBalance {
		t.Errorf("Expected balance: %d, got: %d", expectedBalance, userInfo.Coins)
	}

	fmt.Println("Test passed successfully!")
}
