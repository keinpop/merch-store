# Запуск
Запускать лучше всего через make из корня проекта, он упрощает все действия до 1 кнопки:

```bash
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
```

# Краткое описание папок

### cmd
Точка входа в приложение, здесь инициализируются все компоненты и поднимается сервер
### config
Конфиг со всеми необходимыми данными для запуска 
### cover 
При открытии cover.html можно увидеть процент покрытия каждого файла с бизнеслогикой.   
Процент покрытия удовлетворяет условиям.
### db
Здесь находится файл миграции для базы данных.
### internal
Вся реализация проекта и тестов для ее модулей.
### test 
Файл с e2e-тестами под некоторые сценарии (сами сценарии описаны над тестами).

# Вопросы и проблемы
В ходе решения задачи, с проблемами не столкнулся. Но в целом все рассуждения и логика написана в комментариях над функциями.

# Нагрузочное тестирование
Провел небольшое нагрузочное тестирование при помощи k6s. Код теста в файле load_test.js. А вот результат:
```bash
avito-tech % k6 run load_test.js 

         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: load_test.js
        output: -

     scenarios: (100.00%) 1 scenario, 300 max VUs, 50s max duration (incl. graceful stop):
              * default: Up to 300 looping VUs for 20s over 4 stages (gracefulRampDown: 30s, gracefulStop: 30s)


     ✓ auth status is 200

     checks.........................: 100.00% 971 out of 971
     data_received..................: 401 kB  17 kB/s
     data_sent......................: 183 kB  7.9 kB/s
     http_req_blocked...............: avg=136.78µs min=2µs     med=6µs     max=778µs   p(90)=473µs   p(95)=532.5µs 
     http_req_connecting............: avg=101.59µs min=0s      med=0s      max=622µs   p(90)=360µs   p(95)=401.49µs
   ✓ http_req_duration..............: avg=60.72ms  min=52.75ms med=60.36ms max=86.46ms p(90)=65.69ms p(95)=67.59ms 
       { expected_response:true }...: avg=60.72ms  min=52.75ms med=60.36ms max=86.46ms p(90)=65.69ms p(95)=67.59ms 
   ✓ http_req_failed................: 0.00%   0 out of 971
     http_req_receiving.............: avg=35.31µs  min=17µs    med=33µs    max=161µs   p(90)=53µs    p(95)=60µs    
     http_req_sending...............: avg=34.33µs  min=6µs     med=27µs    max=161µs   p(90)=74µs    p(95)=89.49µs 
     http_req_tls_handshaking.......: avg=0s       min=0s      med=0s      max=0s      p(90)=0s      p(95)=0s      
     http_req_waiting...............: avg=60.65ms  min=52.65ms med=60.31ms max=86.35ms p(90)=65.62ms p(95)=67.49ms 
     http_reqs......................: 971     42.116416/s
     iteration_duration.............: avg=3.06s    min=3.05s   med=3.06s   max=3.08s   p(90)=3.06s   p(95)=3.06s   
     iterations.....................: 971     42.116416/s
     vus............................: 9       min=9          max=299
     vus_max........................: 300     min=300        max=300


running (23.1s), 000/300 VUs, 971 complete and 0 interrupted iterations
default ✓ [======================================] 000/300 VUs  20s
```