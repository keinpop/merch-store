package main

import (
	"database/sql"
	"fmt"
	"net/http"

	"proj/internal/app"
	"proj/internal/handlers"
	"proj/internal/session"
	"proj/internal/user"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const (
	cfgPath = "config/config.yaml"
)

func main() {
	// init logger
	zapLogger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	logger := zapLogger.Sugar()
	//	тк функция откладывается буду использовать
	// обертку в анонимную функцию
	defer func() {
		err = zapLogger.Sync()
		if err != nil {
			logger.Warnf("error to sync logger: %v", err)
		}
	}()

	// парсим конфиг
	c, err := app.NewConfig(cfgPath)
	if err != nil {
		logger.Fatalf("error to parsing config: %v", err)
	}

	// init db
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=disable",
		c.CfgDB.Host, c.CfgDB.Port, c.CfgDB.Login, c.CfgDB.Password, c.CfgDB.Database,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatalf("error to database start: %v", err)
	}

	db.SetMaxOpenConns(c.MaxOpenConns)

	err = db.Ping()
	if err != nil {
		logger.Infof("Failed to get response to ping: %v", err)
	}

	sm := session.NewSessionManager(db, logger, c.Secret)
	ur := user.NewUserDBRepository(db, logger)

	userHandler := &handlers.UserHandlers{
		Logger:   logger,
		UserRepo: ur,
		Sessions: sm,
	}

	r := handlers.NewRouters(userHandler, sm, logger)
	logger.Infow("starting server",
		"type", "START",
		"addr", c.ServerPort,
	)

	err = http.ListenAndServe(c.ServerPort, r)
	if err != nil {
		panic("cant start server")
	}
}
