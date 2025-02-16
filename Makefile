.PHONY: run stop stop-hard run-e2e run-lint run-cover

# Запуск контейнеров через docker-compose
run:
	docker-compose up -d

# Остановка контейнеров 
stop:
	docker-compose down

# Остановка контейнеров с базой с удалением данных
stop-hard:
	docker-compose down -v

# Запуск e2e тестов (важно, чтобы база была чистая, либо без юзеров из тестов)
run-e2e:
	docker-compose down -v
	docker-compose up -d
	go test ./test -v
	docker-compose down -v

# Запуск линтера (проверьте его установку)
run-lint:
	golangci-lint run

# Запуск тестов с покрытием
run-cover:
	go test -coverprofile=cover/handlers.cover ./internal/handlers && go test -coverprofile=cover/user.cover ./internal/user && go test -coverprofile=cover/session.cover ./internal/session && gocovmerge cover/*.cover > cover/merged.cover && go tool cover -html=cover/merged.cover -o cover/cover.html