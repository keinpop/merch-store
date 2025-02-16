package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"proj/internal/handlers"
	"testing"
)

/*
В данном тесте мы проверим, что авторизация корректно работает:
  - Сначала сделаем огромный пароль (> 72)
  - Потом слишком большой ник (> 32)
  - Потом у тестируемого получится войти и купит желанную ручку
*/
func TestAuth(t *testing.T) {
	// Большой пароль
	authReq := handlers.AuthRequest{
		Username: "mr.bombastic",
		Password: "JVGMdnmDFY4Gspv8FHqg8Bwmpu17j6YKjuP6Rvmz80VN3Vuz6ahSuZKl44woQPpIcjCjqKeN5OSS12VDQIkdQBuPdP4UrCP0F",
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

	if authRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("Auth failed with status: %d", authRes.StatusCode)
	}

	// Большой ник
	authReq = handlers.AuthRequest{
		Username: "mr.bombastic52JVGMdnmDFY4Gspv8FHqg8Bwmpu17j6YKjuP6Rvmz80VN3Vuz6ahSuZKl44woQPpIcjCjqKeN5OSS12VDQIkdQBuPd",
		Password: "1a2b3c4dAGGGG",
	}

	authReqBody, err = json.Marshal(authReq)
	if err != nil {
		t.Fatalf("Failed to marshal auth request: %v", err)
	}

	authRes, err = http.Post("http://localhost:8080/api/auth", "application/json", bytes.NewBuffer(authReqBody))
	if err != nil {
		t.Fatalf("Failed to send auth request: %v", err)
	}
	defer authRes.Body.Close()

	if authRes.StatusCode != http.StatusBadRequest {
		t.Fatalf("Auth failed with status: %d", authRes.StatusCode)
	}

	// Успешный вход
	// Авторизация пользователя
	authReq = handlers.AuthRequest{
		Username: "mr.pen",
		Password: "1a2b3c4dAGGGG",
	}

	authReqBody, err = json.Marshal(authReq)
	if err != nil {
		t.Fatalf("Failed to marshal auth request: %v", err)
	}

	authRes, err = http.Post("http://localhost:8080/api/auth", "application/json", bytes.NewBuffer(authReqBody))
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

	// покупка ручки
	req, err := http.NewRequest("GET", "http://localhost:8080/api/buy/pen", nil)
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
		t.Fatalf("Buy failed with status: %d", buyRes.StatusCode)
	}

	fmt.Println("Item pen purchased successfully")
}
