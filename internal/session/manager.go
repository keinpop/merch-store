package session

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"go.uber.org/zap"
)

var (
	ErrUnexpectedMethod = errors.New("unexpected signing method")
	ErrInternalDB       = errors.New("database error")
	ErrInternalGo       = errors.New("golang lib errors")
	ErrSingingToken     = errors.New("error signing token")

	sessKey SessionKey = "sessionKey"
)

type SessionKey string

const (
	FieldSessionID = "session_id"
)

type SessionManager struct {
	DB          *sql.DB
	Logger      *zap.SugaredLogger
	tokenSecret string
}

func NewSessionManager(db *sql.DB, l *zap.SugaredLogger, secret string) *SessionManager {
	return &SessionManager{
		DB:          db,
		Logger:      l,
		tokenSecret: secret,
	}
}

type SessionManagerRepo interface {
	Check(r *http.Request) (*Session, error)
	Create(w http.ResponseWriter, userID string, login string) (*Session, string, error)

	GetSecret() string
}

func (sm *SessionManager) Check(r *http.Request) (*Session, error) {
	// Получаем значение заголовка Authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		sm.Logger.Errorf("%v", ErrNoAuth)
		return nil, ErrNoAuth
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return nil, ErrNoAuth
	}
	tokenString := strings.TrimPrefix(authHeader, bearerPrefix)
	// Распарсиваем токен, используя секретный ключ
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			sm.Logger.Errorf("%v", ErrUnexpectedMethod)
			return nil, ErrUnexpectedMethod
		}
		return []byte(sm.tokenSecret), nil
	})
	if err != nil || !token.Valid {
		sm.Logger.Errorf("%v. More details: %v", ErrNoAuth, err)
		return nil, ErrNoAuth
	}

	// Извлекаем claims из токена
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims[FieldSessionID] == nil {
		sm.Logger.Errorf("%v. More details: %v", ErrNoAuth, err)
		return nil, ErrNoAuth
	}
	sessionID, ok := claims[FieldSessionID].(string)
	if !ok {
		sm.Logger.Errorf("%v. More details: %v", ErrNoAuth, err)
		return nil, ErrNoAuth
	}

	// Проверяем наличие сессии в базе данных
	var sess Session
	query := `
	SELECT session_id, user_id, start_time, end_time 
	FROM sessions 
	WHERE session_id = $1
	`
	err = sm.DB.QueryRow(query, sessionID).Scan(&sess.ID, &sess.UserID, &sess.StartTime, &sess.EndTime)
	if errors.Is(err, sql.ErrNoRows) {
		sm.Logger.Errorf("%v. More details: %v", ErrNoAuth, err)
		return nil, ErrNoAuth
	} else if err != nil {
		sm.Logger.Errorf("%v.More details: %v", ErrInternalDB, err)
		return nil, ErrInternalDB
	}

	// Проверяем, не истекло ли время действия сессии
	if time.Now().After(sess.EndTime) {
		sm.Logger.Errorf("%v. More details: %v", ErrNoAuth, err)
		return nil, ErrNoAuth
	}

	return &sess, nil
}

type AuthResponse struct {
	Token string `json:"token"`
}

// Немного сложная логика с удалением сессии сделал для того,
// чтобы сессии не скапливались и не засоряли память, а эффективно
// "существовали" - звучит жутко...
func (sm *SessionManager) Create(
	w http.ResponseWriter,
	userID string,
	login string,
) (*Session, string, error) {
	sess := &Session{}

	// Проверяем, существует ли уже сессия и она не просрочена
	query := `
    SELECT session_id, user_id, start_time, end_time
    FROM sessions
    WHERE user_id = $1
    `
	err := sm.DB.QueryRow(query, userID).Scan(&sess.ID, &sess.UserID, &sess.StartTime, &sess.EndTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Если сессия не найдена, создаем новую
			sess = NewSession(userID)
		} else {
			sm.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
			return nil, "", ErrInternalDB
		}
	} else {
		// Если сессия найдена, проверяем, не просрочена ли она
		if sess.EndTime.Before(time.Now()) {
			// Если сессия просрочена, удаляем её
			query = `DELETE FROM sessions WHERE session_id = $1`
			_, err := sm.DB.Exec(query, sess.ID)
			if err != nil {
				sm.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
				return nil, "", ErrInternalDB
			}

			// Создаем новую сессию
			sess = NewSession(userID)
		} else { // Если все ок, просто вернем ее
			return sess, generateJWT(sm, sess, login), nil
		}
	}

	// Вставляем новую сессию в базу данных
	query = `
	INSERT INTO sessions (session_id, user_id, start_time, end_time)
    VALUES ($1, $2, $3, $4)
	`
	_, err = sm.DB.Exec(query, sess.ID, sess.UserID, sess.StartTime, sess.EndTime)
	if err != nil {
		sm.Logger.Errorf("%v. More details: %v", ErrInternalDB, err)
		return nil, "", ErrInternalDB
	}

	return sess, generateJWT(sm, sess, login), nil
}

func generateJWT(sm *SessionManager, sess *Session, login string) string {
	// Генерация JWT токена
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user": map[string]interface{}{
			"login": login,
			"id":    sess.UserID,
		},
		"iat":          sess.StartTime.Unix(),
		"exp":          sess.EndTime.Unix(),
		FieldSessionID: sess.ID,
	})

	token, err := t.SignedString([]byte(sm.tokenSecret))
	if err != nil {
		sm.Logger.Errorf("%v. More details: %v", ErrSingingToken, err)
		return ""
	}

	return token
}

func (sm *SessionManager) GetSecret() string {
	return sm.tokenSecret
}

func ContextWithSession(ctx context.Context, s *Session) context.Context {
	// создаем новый контекст с нашим ключом и сессией
	return context.WithValue(ctx, sessKey, s)
}
