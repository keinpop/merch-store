package types

// Типы для предметов, тк в бд храним кодом (числом).
const (
	TypeItemTShirt = iota
	TypeItemCup
	TypeItemBook
	TypeItemPen
	TypeItemPowerbank
	TypeItemHoody
	TypeItemUmbrella
	TypeItemSocks
	TypeItemWallet
	TypeItemPinkHoody

	TypeItemError = -1
)

var (
	itemsPoolForCode = []string{
		"t-shirt", "cup", "book",
		"pen", "powerbank", "hoody",
		"umbrella", "socks", "wallet",
		"pink-hoody",
	}

	itemsPoolForTitle = map[string]int{
		"t-shirt": TypeItemTShirt, "cup": TypeItemCup,
		"book": TypeItemBook, "pen": TypeItemPen,
		"powerbank": TypeItemPowerbank, "hoody": TypeItemHoody,
		"umbrella": TypeItemUmbrella, "socks": TypeItemSocks,
		"wallet": TypeItemWallet, "pink-hoody": TypeItemPinkHoody,
	}
)

// Тип ответа информации о пользователе.
type InfoResponse struct {
	Coins       int         `json:"coins"`
	Inventory   []Item      `json:"inventory"`
	CoinHistory Transaction `json:"coinHistory"`
}

// Структура для предметов у юзера.
type Item struct {
	Type     string `json:"type"`     // тип возвращаем как строку
	Quantity int    `json:"quantity"` // количество таких предметов у нас в инвентаре
}

func CodeToStringItem(code int) string {
	return itemsPoolForCode[code]
}

func StringToCodeItem(title string) int {
	code, found := itemsPoolForTitle[title]
	if !found {
		return TypeItemError
	}

	return code
}

// Структура для хранения данных о предмете из бд.
type ItemInStore struct {
	Type  int
	Price int
}

type Transaction struct {
	Received []ReceivedTrans `json:"received"`
	Sent     []SentTrans     `json:"sent"`
}

type ReceivedTrans struct {
	FromUser string `json:"fromUser,omitempty"`
	Amount   int    `json:"amount"`
}

type SentTrans struct {
	ToUser string `json:"toUser,omitempty"`
	Amount int    `json:"amount"`
}
