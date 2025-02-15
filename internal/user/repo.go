package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"proj/internal/types"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	startAmountOfMoney = 1000

	// Правило хорошего тона заранее аллоцировать слайсы,
	// 10 - кажется более менее оптимальное число
	AllocSize = 10

	DefaultQuantityOnFirstPurchase = 1
)

var (
	ErrInternalDB        = errors.New("database internal error")
	ErrInternalGo        = errors.New("go internal error")
	ErrBadPassword       = errors.New("invalid password")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrUserNotFound      = errors.New("user not found")
	ErrItemNotFound      = errors.New("item not found")
)

type UserDBRepository struct {
	DB     *sql.DB
	Logger *zap.SugaredLogger
}

func NewUserDBRepository(db *sql.DB, l *zap.SugaredLogger) *UserDBRepository {
	return &UserDBRepository{
		DB:     db,
		Logger: l,
	}
}

/*
Вообще, я бы разделил Auth и Register на два метода
чтобы было проще работать, если пользователь только регистрируется.
Но удовлетворяя ТЗ сделаем в одной, но немного посложнее:

	Есть ли такой юзер по логину?
		* Да => Сверим пароли:
			* Совпало - ОК
			* Не совпало - Неверный пароль
		* Нет:
			* Создадим его
*/
func (ur *UserDBRepository) Authorize(login, password string) (User, error) {
	// проверим, что юзер уже существует
	var u User

	// Проверим, есть ли пользователь с таким именем
	query := `
	SELECT user_id, login, hash_password, amount_in_wallet
	FROM users
	WHERE login = $1
	`
	err := ur.DB.QueryRow(query, login).Scan(
		&u.UserID, &u.Login, &u.passwordHash, &u.AmountInWallet,
	)
	if err != nil {
		//  Если такого нет (с таким же логином), то создадим его
		if errors.Is(err, sql.ErrNoRows) {
			return createNewUser(login, password, ur)
		}

		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return User{}, ErrInternalDB
	}

	// Сверим пароли
	if err := bcrypt.CompareHashAndPassword([]byte(u.passwordHash), []byte(password)); err != nil {
		// Пароли не совпали
		ur.Logger.Errorf("%v. More details: user - %s - enter invalid password",
			ErrBadPassword, login,
		)
		return User{}, ErrBadPassword
	}

	// Пароли совпали - возвращаем
	ur.Logger.Infof("user - %s - logged in again", login)
	return u, nil
}

// Вспомогательная функция, чтобы сделать функцию Authorize более читаемой
func createNewUser(l, p string, ur *UserDBRepository) (User, error) {
	// кодируем пароль
	hp, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalGo, err)
		return User{}, err
	}

	// создаем нового пользователя
	q := `
	INSERT INTO users (user_id, login, hash_password, amount_in_wallet)
	VALUES ($1, $2, $3, $4)
	`
	newID := uuid.New().String()
	_, err = ur.DB.Exec(q, newID, l, hp, startAmountOfMoney)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return User{}, err
	}

	u := User{
		UserID:         newID,
		Login:          l,
		passwordHash:   string(hp),
		AmountInWallet: startAmountOfMoney,
	}
	ur.Logger.Infof("new user - %s - created", l)
	return u, nil
}

// Функция для получении пользователю информации
func (ur *UserDBRepository) Info(userID string) (types.InfoResponse, error) {
	var info types.InfoResponse

	// запрос для coins
	query := `
	SELECT amount_in_wallet
	FROM users
	WHERE user_id = $1
	`
	err := ur.DB.QueryRow(query, userID).Scan(&info.Coins)
	if err != nil {
		// Если такого пользователя нет, ошибка в запросе
		if errors.Is(err, sql.ErrNoRows) {
			ur.Logger.Errorf("%v. More details: %v", ErrUserNotFound, err)
			return types.InfoResponse{}, ErrUserNotFound
		}

		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return types.InfoResponse{}, ErrInternalDB
	}

	// Запрос для инвентаря
	items, err := getInventory(userID, ur)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return types.InfoResponse{}, ErrInternalDB
	}
	info.Inventory = items

	// Запрос для истории транзакций (moneyHistory)
	coinHistory, err := getCoinHistory(userID, ur)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return types.InfoResponse{}, ErrInternalDB
	}
	info.CoinHistory = coinHistory

	return info, nil
}

// Функция для получения инвентаря
func getInventory(userID string, ur *UserDBRepository) ([]types.Item, error) {
	q := `
	SELECT type, quantity
	FROM items
	WHERE user_id = $1
	`
	rows, err := ur.DB.Query(q, userID)
	if err != nil {
		// Если такого пользователя нет, ошибка в запросе
		if errors.Is(err, sql.ErrNoRows) {
			ur.Logger.Errorf("%v. More details: %v", ErrUserNotFound, err)
			return nil, ErrUserNotFound
		}

		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		}
	}()

	items := make([]types.Item, 0, AllocSize)

	for rows.Next() {
		var i types.Item
		var t int // переменная для числового кода типа
		err = rows.Scan(&t, &i.Quantity)
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
			return nil, err
		}

		i.Type = types.CodeToStringItem(t)

		items = append(items, i)
	}

	if err = rows.Err(); err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}

	return items, nil
}

// Функция для получения истории транзакций
func getCoinHistory(
	userID string,
	ur *UserDBRepository,
) (types.Transaction, error) {
	// собранный результат
	var trs types.Transaction

	// Получим сначала все операции, где пользователю
	// отправляли монеты (received)
	rts, err := getReceivedTransactions(userID, ur)
	if err != nil {
		return types.Transaction{}, err
	}
	trs.Received = rts

	// Получим теперь все операции, где пользователь
	// отправлял монеты (sent)
	sts, err := getSentTransactions(userID, ur)
	if err != nil {
		return types.Transaction{}, err
	}
	trs.Sent = sts

	return trs, nil
}

// Вспомогательная функция для декомпозиции getCoinHistory,
// чтобы в случае дополнительного функционала было
// проще добавлять и читать код
func getReceivedTransactions(userID string, ur *UserDBRepository) ([]types.ReceivedTrans, error) {
	q := `
	SELECT 
        u_from.login AS from_user,
        t.amount
    FROM transactions t
    JOIN users u_from ON t.sender = u_from.user_id
    WHERE t.receiver = $1  
	`
	rows, err := ur.DB.Query(q, userID)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		}
	}()

	res := make([]types.ReceivedTrans, 0, AllocSize)

	for rows.Next() {
		var rt types.ReceivedTrans

		err = rows.Scan(&rt.FromUser, &rt.Amount)
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
			return nil, err
		}

		res = append(res, rt)
	}

	if err = rows.Err(); err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}

	return res, nil
}

// Вспомогательная функция для декомпозиции getCoinHistory,
// чтобы в случае дополнительного функционала было
// проще добавлять и читать код
func getSentTransactions(userID string, ur *UserDBRepository) ([]types.SentTrans, error) {
	q := `
	SELECT
        u_to.login AS to_user,
        t.amount
    FROM transactions t
    JOIN users u_to ON t.receiver = u_to.user_id
    WHERE t.sender = $1
	`
	rows, err := ur.DB.Query(q, userID)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		}
	}()
	res := make([]types.SentTrans, 0, AllocSize)

	for rows.Next() {
		var st types.SentTrans

		err = rows.Scan(&st.ToUser, &st.Amount)
		if err != nil {
			ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
			return nil, err
		}

		res = append(res, st)
	}

	if err = rows.Err(); err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, err
	}

	return res, nil
}

/*
Функция для отправки денег юзеру, разобьем на 4 подфункции:
  - достаточно ли средств -> enoughCoinsInWallet
  - списание 			  -> chargeOffFromWallet
  - отправка 			  -> sendCoinsToWallet
  - добавление транзакции -> addNewTransactions
*/
func (ur *UserDBRepository) SendCoin(userID, toUserLogin string, amount int) error {
	// Начинаем транзакцию в бд, чтобы обеспечить атомарность нашего запроса
	tx, err := ur.DB.BeginTx(context.Background(), nil)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}
	defer tx.Rollback()

	// можем ли списать
	if err = enoughCoinsInWallet(userID, amount, tx, ur.Logger); err != nil {
		return err
	}

	// списание со счета отправителя
	if err = chargeOffFromWallet(userID, amount, tx, ur.Logger); err != nil {
		return err
	}

	// отправка на счет получателя
	if err = sendCoinsToWallet(toUserLogin, amount, tx, ur.Logger); err != nil {
		return err
	}

	// добавление транзакции о проведенной операции
	if err = addNewTransactions(userID, toUserLogin, amount, tx, ur.Logger); err != nil {
		return err
	}

	// Если все произошло успешно, завершаем транзакцию бд
	if err := tx.Commit(); err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return ErrInternalDB
	}

	return nil
}

// Отправка денег на счет получателя
func sendCoinsToWallet(toUserLogin string, amount int, tx *sql.Tx, l *zap.SugaredLogger) error {
	q := `
	UPDATE users
	SET amount_in_wallet = amount_in_wallet + $1
	WHERE login = $2
 	`

	_, err := tx.Exec(q, amount, toUserLogin)
	if err != nil {
		fmt.Println(err)
		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return ErrInternalDB
	}

	return nil
}

func addNewTransactions(
	senderID, receiverLogin string,
	amount int,
	tx *sql.Tx,
	l *zap.SugaredLogger,
) error {
	// получим id для получателя, тк есть у нас только его ник
	q := `
	SELECT user_id
	FROM users
	WHERE login = $1
	`
	var receiverID string
	err := tx.QueryRow(q, receiverLogin).Scan(&receiverID)
	if err != nil {
		// Если такого пользователя нет, ошибка в запросе
		if errors.Is(err, sql.ErrNoRows) {
			l.Errorf("%v. More details: %v", ErrUserNotFound, err)
			return ErrUserNotFound
		}

		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}

	// Добавляем транзакцию для отправителя и получателя
	q = `
	INSERT INTO transactions (sender, receiver, amount)
	VALUES ($1, $2, $3)
	`
	_, err = tx.Exec(q, senderID, receiverID, amount)
	if err != nil {
		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}

	return nil
}

/*
Функция для покупки предметов пользователем. Декомпозируем на:
  - Получим предмет с его ценой 				   -> getItemByTitle
  - Проверим, можно ли списать такую сумму с счета -> enoughCoinsInWallet
  - Списываем со счета 							   -> chargeOffFromWallet
  - Добавляем предмет в инвентарь				   -> addItemInInventory

(если такой уже был - увеличиваем количество)
*/
func (ur *UserDBRepository) BuyItem(userID, itemTitle string) error {
	tx, err := ur.DB.BeginTx(context.Background(), nil)
	if err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}
	defer tx.Rollback()

	// получили данные о предмете из бд
	item, err := getItemByTitle(itemTitle, tx, ur.Logger)
	if err != nil {
		return err
	}

	// можем ли списать данную сумму со счета
	err = enoughCoinsInWallet(userID, item.Price, tx, ur.Logger)
	if err != nil {
		return err
	}

	// списываем деньги со счета
	err = chargeOffFromWallet(userID, item.Price, tx, ur.Logger)
	if err != nil {
		return err
	}

	// добавляем предмет в инвентарь
	err = addItemInInventory(userID, itemTitle, tx, ur.Logger)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		ur.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return ErrInternalDB
	}

	return nil
}

// Функция получения данных о предмете
func getItemByTitle(itemTitle string, tx *sql.Tx, l *zap.SugaredLogger) (types.ItemInStore, error) {
	itemCode := types.StringToCodeItem(itemTitle)
	if itemCode == types.TypeItemError {
		l.Errorf("%v", ErrItemNotFound)
		return types.ItemInStore{}, ErrItemNotFound
	}
	var i types.ItemInStore
	i.Type = itemCode

	q := `
	SELECT price
	FROM store
	WHERE type = $1
	`
	err := tx.QueryRow(q, itemCode).Scan(&i.Price)
	if err != nil {
		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return types.ItemInStore{}, ErrInternalDB
	}

	return i, nil
}

/*
Функция добавления предмета в инвентарь
  - Если предмета нет - добавим его с количеством 1
  - если есть просто инкрементим количество
*/
func addItemInInventory(userID, titleItem string, tx *sql.Tx, l *zap.SugaredLogger) error {
	// проверим, есть ли такой предмет
	q := `
	SELECT type 
	FROM items
	WHERE user_id = $1 AND type = $2
	`
	var exists int
	typeItemCode := types.StringToCodeItem(titleItem)
	err := tx.QueryRow(q, userID, typeItemCode).Scan(&exists)
	if err != nil {
		// Если такого нет, создадим предмет
		if errors.Is(err, sql.ErrNoRows) {
			err = createNewItemInInventory(userID, typeItemCode, tx)
			if err != nil {
				l.Errorf("%v. More details: %v", ErrInternalDB, err)
				return err
			}

			l.Info("added new item - %s - for user_id - %s -", titleItem, userID)
			return nil
		}

		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}

	// Если такой предмет есть
	q = `
	UPDATE items 
	SET quantity = quantity + 1
	WHERE user_id = $1 AND type = $2
	`
	_, err = tx.Exec(q, userID, typeItemCode)
	if err != nil {
		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return err
	}

	return nil
}

func createNewItemInInventory(userID string, codeItem int, tx *sql.Tx) error {
	q := `
	INSERT INTO items (user_id, type, quantity)
	VALUES ($1, $2, $3)
	`
	_, err := tx.Exec(q, userID, codeItem, DefaultQuantityOnFirstPurchase)
	if err != nil {
		return err
	}
	return nil
}

// Проверка на наличие нужного количества средств
func enoughCoinsInWallet(userID string, amount int, tx *sql.Tx, l *zap.SugaredLogger) error {
	// FOR UPDATE позволяет блокировать баланс на время транзакции
	q := `
	SELECT amount_in_wallet
	FROM users
	WHERE user_id = $1
	FOR UPDATE
	`
	var AmountInWallet int
	err := tx.QueryRow(q, userID).Scan(&AmountInWallet)
	if err != nil {
		// Если мы не нашли такого пользователя
		if errors.Is(err, sql.ErrNoRows) {
			l.Errorf("%v. More details: %v", ErrUserNotFound, err)
			return ErrUserNotFound
		}

		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return ErrInternalDB
	}

	// Если недостаточно средств
	if AmountInWallet-amount < 0 {
		return ErrInsufficientFunds
	}

	return nil
}

// Списание со счета средств
func chargeOffFromWallet(userID string, amount int, tx *sql.Tx, l *zap.SugaredLogger) error {
	q := `
	UPDATE users 
	SET amount_in_wallet = amount_in_wallet - $1
	WHERE user_id = $2
	`

	_, err := tx.Exec(q, amount, userID)
	if err != nil {
		l.Errorf("%v. More details: %v", ErrInternalDB, err)
		return ErrInternalDB
	}

	return nil
}
