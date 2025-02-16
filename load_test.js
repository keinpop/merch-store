import http from 'k6/http';
import { check, sleep } from 'k6';

// Конфигурация теста
export let options = {
    stages: [
        { duration: "5s", target: 50 },
        { duration: "5s", target: 100 },
        { duration: "5s", target: 200 },
        { duration: "5s", target: 300 },
    ],    
        thresholds: {
        http_req_duration: ['p(95)<70'],  // 95% запросов должны выполняться быстрее 70 мс
        http_req_failed: ['rate<0.0001'],  // Частота ошибок должна быть меньше 0.01% (99.99% успешных запросов)
    },
};

// Тестовый сценарий
export default function () {
    // Шаг 1: Аутентификация пользователя
    let authPayload = JSON.stringify({
        username: `user${__VU}`,  // Уникальный логин для каждого виртуального пользователя
        password: 'testpassword',
    });

    let authRes = http.post('http://localhost:8080/api/auth', authPayload, {
        headers: { 'Content-Type': 'application/json' },
    });

    // Проверка успешности авторизации
    check(authRes, {
        'auth status is 200': (r) => r.status === 200,
    });

    sleep(3);
}