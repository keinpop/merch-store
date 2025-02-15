package user

import (
	"database/sql"
	"errors"
	"proj/internal/types"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// newTestDBRepository создает мок базы данных и репозиторий для тестов
func newTestDBRepository(t *testing.T) (*UserDBRepository, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Не удалось создать мок базу данных: %v", err)
	}

	logger := zap.NewNop().Sugar()

	return NewUserDBRepository(db, logger), mock
}

func TestUserDBRepository_Authorize(t *testing.T) {
	// Генерируем реальный хэш пароля для теста
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("correct_password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Failed to generate hashed password: %v", err)
	}
	// Тестовые случаи
	tests := []struct {
		name          string
		login         string
		password      string
		mockDBSetup   func(sqlmock.Sqlmock)
		expectedUser  User
		expectedError error
	}{
		{
			name:     "SuccessExistingUser",
			login:    "existing_user",
			password: "correct_password",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем запрос для поиска существующего пользователя
				mock.ExpectQuery("SELECT user_id, login, hash_password, amount_in_wallet FROM users WHERE login = \\$1").
					WithArgs("existing_user").
					WillReturnRows(sqlmock.NewRows([]string{"user_id", "login", "hash_password", "amount_in_wallet"}).
						AddRow("user1", "existing_user", hashedPassword, 100))
			},
			expectedUser: User{
				UserID:         "user1",
				Login:          "existing_user",
				passwordHash:   string(hashedPassword),
				AmountInWallet: 100,
			},
			expectedError: nil,
		},
		{
			name:     "SuccessNewUser",
			login:    "new_user",
			password: "new_password",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id, login, hash_password, amount_in_wallet FROM users WHERE login = \\$1").
					WithArgs("new_user").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectExec("INSERT INTO users \\(user_id, login, hash_password, amount_in_wallet\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
					WithArgs(sqlmock.AnyArg(), "new_user", sqlmock.AnyArg(), startAmountOfMoney).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedUser: User{
				Login:          "new_user",
				AmountInWallet: startAmountOfMoney,
			},
			expectedError: nil,
		},
		{
			name:     "InvalidPassword",
			login:    "existing_user",
			password: "wrong_password",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id, login, hash_password, amount_in_wallet FROM users WHERE login = \\$1").
					WithArgs("existing_user").
					WillReturnRows(sqlmock.NewRows([]string{"user_id", "login", "hash_password", "amount_in_wallet"}).
						AddRow("user1", "existing_user", "$2a$10$hashed_password", 100))
			},
			expectedUser:  User{},
			expectedError: ErrBadPassword,
		},
		{
			name:     "DatabaseError",
			login:    "existing_user",
			password: "correct_password",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id, login, hash_password, amount_in_wallet FROM users WHERE login = \\$1").
					WithArgs("existing_user").
					WillReturnError(errors.New("database error"))
			},
			expectedUser:  User{},
			expectedError: ErrInternalDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newTestDBRepository(t)

			// Настраиваем мок базы данных
			tt.mockDBSetup(mock)

			user, err := repo.Authorize(tt.login, tt.password)

			// Проверяем результаты
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUser.Login, user.Login)
				assert.Equal(t, tt.expectedUser.AmountInWallet, user.AmountInWallet)
				// Для нового пользователя проверяем, что UserID и passwordHash были сгенерированы
				if tt.name == "SuccessNewUser" {
					assert.NotEmpty(t, user.UserID)
					assert.NotEmpty(t, user.passwordHash)
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserDBRepository_Info(t *testing.T) {
	// Тестовые случаи
	tests := []struct {
		name          string
		userID        string
		mockDBSetup   func(sqlmock.Sqlmock) // Настройка мока базы данных
		expectedInfo  types.InfoResponse
		expectedError error
	}{
		{
			name:   "Success",
			userID: "user1",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем запрос для получения количества монет
				mock.ExpectQuery("SELECT amount_in_wallet FROM users WHERE user_id = \\$1").
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// Мокируем запрос для получения инвентаря
				mock.ExpectQuery("SELECT type, quantity FROM items WHERE user_id = \\$1").
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"type", "quantity"}).
						AddRow(0, 2). // TypeItemTShirt
						AddRow(1, 1)) // TypeItemCup

				// Мокируем запрос для получения полученных транзакций
				mock.ExpectQuery("SELECT u_from.login AS from_user, t.amount FROM transactions t JOIN users u_from ON t.sender = u_from.user_id WHERE t.receiver = \\$1").
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"from_user", "amount"}).
						AddRow("user2", 50))

				// Мокируем запрос для отправленных транзакций
				mock.ExpectQuery("SELECT u_to.login AS to_user, t.amount FROM transactions t JOIN users u_to ON t.receiver = u_to.user_id WHERE t.sender = \\$1").
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"to_user", "amount"}).
						AddRow("user3", 30))
			},
			expectedInfo: types.InfoResponse{
				Coins: 100,
				Inventory: []types.Item{
					{Type: "t-shirt", Quantity: 2}, // TypeItemTShirt
					{Type: "cup", Quantity: 1},     // TypeItemCup
				},
				CoinHistory: types.Transaction{
					Received: []types.ReceivedTrans{
						{FromUser: "user2", Amount: 50},
					},
					Sent: []types.SentTrans{
						{ToUser: "user3", Amount: 30},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:   "UserNotFound",
			userID: "user2",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем запрос для получения количества монет, который вернет ошибку
				mock.ExpectQuery("SELECT amount_in_wallet FROM users WHERE user_id = \\$1").
					WithArgs("user2").
					WillReturnError(sql.ErrNoRows)
			},
			expectedInfo:  types.InfoResponse{},
			expectedError: ErrUserNotFound,
		},
		{
			name:   "DatabaseError",
			userID: "user3",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем запрос для получения количества монет, который вернет ошибку
				mock.ExpectQuery("SELECT amount_in_wallet FROM users WHERE user_id = \\$1").
					WithArgs("user3").
					WillReturnError(errors.New("database error"))
			},
			expectedInfo:  types.InfoResponse{},
			expectedError: ErrInternalDB,
		},
	}

	// Запуск тестов
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок базы данных и репозиторий
			repo, mock := newTestDBRepository(t)

			// Настраиваем мок базы данных
			tt.mockDBSetup(mock)

			// Вызываем метод Info
			info, err := repo.Info(tt.userID)

			// Проверяем результаты
			assert.Equal(t, tt.expectedInfo, info)
			assert.Equal(t, tt.expectedError, err)

			// Проверяем, что все ожидания мока базы данных выполнены
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserDBRepository_SendCoin(t *testing.T) {
	// Тестовые случаи
	tests := []struct {
		name          string
		userID        string
		toUserLogin   string
		amount        int
		mockDBSetup   func(sqlmock.Sqlmock) // Настройка мока базы данных
		expectedError error
	}{
		{
			name:        "Success",
			userID:      "user1",
			toUserLogin: "user2",
			amount:      50,
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// Проверка баланса (FOR UPDATE)
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// Списание средств
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet - \$1 WHERE user_id = \$2`).
					WithArgs(50, "user1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Зачисление средств получателю
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet \+ \$1 WHERE login = \$2`).
					WithArgs(50, "user2").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Поиск ID получателя
				mock.ExpectQuery(`SELECT user_id FROM users WHERE login = \$1`).
					WithArgs("user2").
					WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user2"))

				// Добавление транзакции
				mock.ExpectExec(`INSERT INTO transactions \(sender, receiver, amount\) VALUES \(\$1, \$2, \$3\)`).
					WithArgs("user1", "user2", 50).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: nil,
		},
		{
			name:        "InsufficientFunds",
			userID:      "user1",
			toUserLogin: "user2",
			amount:      150,
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем начало транзакции
				mock.ExpectBegin()

				// Мокируем запрос для проверки баланса отправителя
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// Мокируем откат транзакции
				mock.ExpectRollback()
			},
			expectedError: ErrInsufficientFunds,
		},
		{
			name:        "SenderNotFound",
			userID:      "user1",
			toUserLogin: "user2",
			amount:      50,
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем начало транзакции
				mock.ExpectBegin()

				// Мокируем запрос для проверки баланса отправителя
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnError(sql.ErrNoRows)

				// Мокируем откат транзакции
				mock.ExpectRollback()
			},
			expectedError: ErrUserNotFound,
		},
		{
			name:        "ReceiverNotFound",
			userID:      "user1",
			toUserLogin: "user2",
			amount:      50,
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем начало транзакции
				mock.ExpectBegin()

				// Мокируем запрос для проверки баланса отправителя
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// Мокируем запрос для списания средств с отправителя
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet - \$1 WHERE user_id = \$2`).
					WithArgs(50, "user1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Мокируем запрос для зачисления средств получателю
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet \+ \$1 WHERE login = \$2`).
					WithArgs(50, "user2").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// Мокируем запрос для получения ID получателя
				mock.ExpectQuery(`SELECT user_id FROM users WHERE login = \$1`).
					WithArgs("user2").
					WillReturnError(sql.ErrNoRows) // Пользователь не найден

				// Мокируем откат транзакции
				mock.ExpectRollback()
			},
			expectedError: ErrUserNotFound,
		},
		{
			name:        "DatabaseError",
			userID:      "user1",
			toUserLogin: "user2",
			amount:      50,
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				// Мокируем начало транзакции
				mock.ExpectBegin()

				// Мокируем запрос для проверки баланса отправителя
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnError(errors.New("database error"))

				// Мокируем откат транзакции
				mock.ExpectRollback()
			},
			expectedError: ErrInternalDB,
		},
	}

	// Запуск тестов
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем мок базы данных и репозиторий
			repo, mock := newTestDBRepository(t)

			// Настраиваем мок базы данных
			tt.mockDBSetup(mock)

			// Вызываем метод SendCoin
			err := repo.SendCoin(tt.userID, tt.toUserLogin, tt.amount)

			// Проверяем результаты
			assert.Equal(t, tt.expectedError, err)

			// Проверяем, что все ожидания мока базы данных выполнены
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserDBRepository_BuyItem(t *testing.T) {
	// Тестовые случаи
	tests := []struct {
		name          string
		userID        string
		itemTitle     string
		mockDBSetup   func(sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name:      "SuccessNewItem",
			userID:    "user1",
			itemTitle: "t-shirt",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// getItemByTitle
				mock.ExpectQuery(`SELECT price FROM store WHERE type = \$1`).
					WithArgs(types.TypeItemTShirt).
					WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(50))

				// enoughCoinsInWallet
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// chargeOffFromWallet
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet - \$1 WHERE user_id = \$2`).
					WithArgs(50, "user1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// addItemInInventory (item not exists)
				mock.ExpectQuery(`SELECT type FROM items WHERE user_id = \$1 AND type = \$2`).
					WithArgs("user1", types.TypeItemTShirt).
					WillReturnError(sql.ErrNoRows)

				mock.ExpectExec(`INSERT INTO items \(user_id, type, quantity\) VALUES \(\$1, \$2, \$3\)`).
					WithArgs("user1", types.TypeItemTShirt, 1).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: nil,
		},
		{
			name:      "SuccessExistingItem",
			userID:    "user1",
			itemTitle: "cup",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// getItemByTitle
				mock.ExpectQuery(`SELECT price FROM store WHERE type = \$1`).
					WithArgs(types.TypeItemCup).
					WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(30))

				// enoughCoinsInWallet
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(100))

				// chargeOffFromWallet
				mock.ExpectExec(`UPDATE users SET amount_in_wallet = amount_in_wallet - \$1 WHERE user_id = \$2`).
					WithArgs(30, "user1").
					WillReturnResult(sqlmock.NewResult(1, 1))

				// addItemInInventory (item exists)
				mock.ExpectQuery(`SELECT type FROM items WHERE user_id = \$1 AND type = \$2`).
					WithArgs("user1", types.TypeItemCup).
					WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow(types.TypeItemCup))

				mock.ExpectExec(`UPDATE items SET quantity = quantity \+ 1 WHERE user_id = \$1 AND type = \$2`).
					WithArgs("user1", types.TypeItemCup).
					WillReturnResult(sqlmock.NewResult(1, 1))

				mock.ExpectCommit()
			},
			expectedError: nil,
		},
		{
			name:      "InsufficientFunds",
			userID:    "user1",
			itemTitle: "book",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// getItemByTitle
				mock.ExpectQuery(`SELECT price FROM store WHERE type = \$1`).
					WithArgs(types.TypeItemBook).
					WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(100))

				// enoughCoinsInWallet
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnRows(sqlmock.NewRows([]string{"amount_in_wallet"}).AddRow(50))

				mock.ExpectRollback()
			},
			expectedError: ErrInsufficientFunds,
		},
		{
			name:      "ItemNotFound",
			userID:    "user1",
			itemTitle: "invalid-item",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectRollback()
			},
			expectedError: ErrItemNotFound,
		},
		{
			name:      "UserNotFound",
			userID:    "user1",
			itemTitle: "t-shirt",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// getItemByTitle
				mock.ExpectQuery(`SELECT price FROM store WHERE type = \$1`).
					WithArgs(types.TypeItemTShirt).
					WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(50))

				// enoughCoinsInWallet
				mock.ExpectQuery(`SELECT amount_in_wallet FROM users WHERE user_id = \$1 FOR UPDATE`).
					WithArgs("user1").
					WillReturnError(sql.ErrNoRows)

				mock.ExpectRollback()
			},
			expectedError: ErrUserNotFound,
		},
		{
			name:      "DatabaseError",
			userID:    "user1",
			itemTitle: "t-shirt",
			mockDBSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()

				// getItemByTitle
				mock.ExpectQuery(`SELECT price FROM store WHERE type = \$1`).
					WithArgs(types.TypeItemTShirt).
					WillReturnError(errors.New("db error"))

				mock.ExpectRollback()
			},
			expectedError: ErrInternalDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newTestDBRepository(t)
			tt.mockDBSetup(mock)

			err := repo.BuyItem(tt.userID, tt.itemTitle)
			assert.Equal(t, tt.expectedError, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
