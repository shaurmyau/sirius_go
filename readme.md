# Go REST API с PostgreSQL, JWT-авторизацией и метриками

Реализация академического задания: сервер на Go с полным CRUD для сущностей User и Profile, развёрнутый сначала в Docker Compose (с Nginx), затем в Kubernetes, с метриками Prometheus (подходит для VictoriaMetrics) и JWT-аутентификацией.

## Этапы выполнения

1. **Docker Compose** – сущности User/Profile, CRUD, PostgreSQL, Nginx-прокси.
2. **Kubernetes** – миграция в полноценный кластер (используется `kind`), Nginx исключён.
3. **Метрики** – счётчики запросов по операциям, гистограммы длительности, счётчики клиентских (4xx) и серверных (5xx) ошибок, а также вычисляемые процентили (Summary).
4. **JWT middleware** – защита профилей: доступ только с валидным токеном, извлечение `sub` и проверка UUID.

## Требования

Для локального запуска и тестирования необходимо:

- **Docker** и **Docker Compose** (для этапа 1)
- **Go 1.21+** (только для сборки/модификации кода, необязательно)
- **kind** (Kubernetes IN Docker) – создание локального кластера
- **kubectl** – управление кластером
- **Python 3** и библиотека `PyJWT` для генерации токенов в тестах: `pip install pyjwt`
- **curl** – для вызовов API

## Структура проекта

```
go-server-project/
├── main.go                # точка входа
├── go.mod / go.sum        # зависимости
├── Dockerfile             # сборка контейнера
├── docker-compose.yml     # этап 1 (Docker Compose + Nginx + PostgreSQL)
├── nginx.conf             # конфигурация Nginx
├── handler/
│   ├── user.go            # CRUD пользователей
│   └── profile.go         # CRUD профилей (требуется JWT)
├── repository/
│   ├── migrate.go         # создание таблиц
│   ├── user_repo.go       # работа с таблицей users
│   └── profile_repo.go    # работа с таблицей profiles
├── middleware/
│   ├── jwt.go             # middleware проверки JWT
│   └── metrics.go         # метрики (счётчики, гистограмма, summary)
├── k8s/
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── postgres-deployment.yaml
│   ├── app-deployment.yaml
│   └── app-service.yaml
├── kind-config.yaml       # конфигурация кластера kind
├── test.sh                # скрипт полного тестирования API
└── restart_all.sh         # скрипт полного перезапуска в Kubernetes
```

## 1. Локальный запуск через Docker Compose (Этап 1)

Убедитесь, что порты `80` и `5432` не заняты.

```bash
# Сборка и запуск всех сервисов (app, db, nginx)
docker-compose up --build
```

Приложение будет доступно на `http://localhost` (Nginx проксирует на `:8080`).  
PostgreSQL доступен локально на `localhost:5432` (при необходимости отладки).

**Остановка:** `Ctrl+C`, затем `docker-compose down`.

## 2. Запуск в Kubernetes с помощью kind (Этап 2)

### Установка kind и kubectl

- **kubectl:** https://kubernetes.io/docs/tasks/tools/
- **kind:** https://kind.sigs.k8s.io/docs/user/quick-start/#installation

### Создание кластера

```bash
# Создаём кластер из трёх узлов (1 control-plane, 2 worker)
kind create cluster --name go-server --config kind-config.yaml
```

### Сборка и загрузка образов

```bash
# Собрать образ сервера
docker build -t go-server:latest .

# Загрузить образы в кластер (чтобы не тянуть из сети)
docker pull postgres:16
kind load docker-image go-server:latest --name go-server
kind load docker-image postgres:16 --name go-server
```

### Применение манифестов

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/postgres-deployment.yaml
kubectl apply -f k8s/app-deployment.yaml
kubectl apply -f k8s/app-service.yaml
```

### Ожидание готовности подов

```bash
# Дождаться статуса Running для всех подов в namespace go-app
kubectl wait --for=condition=Ready pods --all -n go-app --timeout=120s
```

### Доступ к приложению

Сервис опубликован как NodePort (порт 30080), но для простоты используем проброс порта:

```bash
kubectl port-forward -n go-app service/go-server-service 5000:80
```

Приложение будет доступно на `http://localhost:5000`.

Вместо проброса можно обратиться к IP любого узла кластера (показан командой `kubectl get nodes -o wide`):

```bash
curl http://172.18.0.2:30080/api/users
```

## 3. Тестирование API

### 3.1 CRUD пользователей (без токена)

```bash
# Создать
curl -X POST http://localhost:5000/api/users \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com"}'

# Получить список
curl http://localhost:5000/api/users

# Получить одного (подставьте UUID)
curl http://localhost:5000/api/users/<uuid>

# Обновить
curl -X PUT http://localhost:5000/api/users/<uuid> \
  -H "Content-Type: application/json" \
  -d '{"username":"alice2","email":"alice2@example.com"}'

# Удалить
curl -X DELETE http://localhost:5000/api/users/<uuid>
```

### 3.2 JWT и профили (требуется токен)

Для доступа к `/api/profile` необходимо передавать заголовок `Authorization: Bearer <JWT>`.
Токен должен содержать в поле `sub` UUID существующего пользователя.  
Секрет, используемый сервером: `supersecret` (можно изменить через `JWT_SECRET`).

**Генерация токена (Python):**

```bash
# Установите зависимость один раз: pip install pyjwt
# Подставьте реальный UUID
python3 -c "import jwt; print(jwt.encode({'sub':'<uuid>'}, 'supersecret', algorithm='HS256'))"
```

**Запросы с токеном:**

```bash
TOKEN="полученный_токен"

# Создать профиль
curl -X POST http://localhost:5000/api/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","lname":"Smith"}'

# Получить профиль
curl http://localhost:5000/api/profile -H "Authorization: Bearer $TOKEN"

# Обновить
curl -X PUT http://localhost:5000/api/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","lname":"Johnson"}'

# Удалить
curl -X DELETE http://localhost:5000/api/profile -H "Authorization: Bearer $TOKEN"
```

Ошибки авторизации возвращают `401 Unauthorized`, дубликат профиля – `409 Conflict`.

### 3.3 Автоматизированное тестирование

Запустите скрипт `test.sh`, который выполнит все вышеперечисленные проверки и выведет результаты:

```bash
chmod +x test.sh
./test.sh
```

Скрипт проверит:
- создание, получение, обновление пользователя
- обработку дубликатов
- отсутствие доступа к профилям без токена
- создание, получение, обновление, удаление профиля
- наличие метрик и квантилей времени обработки

### 3.4 Метрики

Метрики доступны по адресу `http://localhost:5000/metrics` в формате Prometheus.

Используемые метрики:
- `app_request_total` – счётчик запросов с метками `entity`, `operation`, `status_class`
- `app_request_duration_seconds` – гистограмма времени выполнения
- `app_request_duration_summary_seconds` – Summary с вычисленными квантилями (0.5, 0.9, 0.99)
- `app_client_errors_total` – счётчик клиентских ошибок (4xx)
- `app_server_errors_total` – счётчик серверных ошибок (5xx)

Пример просмотра квантилей:

```bash
curl -s http://localhost:5000/metrics | grep quantile
```

## 4. Kubernetes Dashboard (опционально)

Для визуального контроля за состоянием кластера:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml
kubectl wait --for=condition=Ready pod -l k8s-app=kubernetes-dashboard -n kubernetes-dashboard --timeout=120s
kubectl -n kubernetes-dashboard port-forward service/kubernetes-dashboard 8443:443
```

Откройте `https://localhost:8443`, примите сертификат.  
Для входа получите токен:

```bash
kubectl -n kubernetes-dashboard create token admin-user
```

Создайте админ-пользователя, если требуется (см. файл `dashboard-admin.yaml`).

## 5. Полный перезапуск в Kubernetes

Для быстрого переразвёртывания всего окружения используйте скрипт `restart_all.sh`:

```bash
chmod +x restart_all.sh
./restart_all.sh
```

Он последовательно удалит старый кластер (если есть), создаст новый, загрузит образы, применит манифесты и запустит проброс порта.

## Примечания

- Все пароли и ключи по умолчанию (`postgres/postgres`, `supersecret`) заданы для демонстрации. При необходимости измените их в `secret.yaml` и `docker-compose.yml`.
- В Kubernetes образы подгружаются в кластер через `kind load docker-image`, чтобы избежать загрузок из интернета и ускорить старт.
- База данных инициализируется автоматически при запуске приложения (создаются таблицы `users` и `profiles`).
- Для продакшена рекомендуется добавить health-чеки, настроить ресурсы и использовать внешний балансировщик.

## Заключение

Проект полностью покрывает все четыре этапа задания, предоставляет гибкое развёртывание как локально (docker-compose), так и в Kubernetes, а также включает мониторинг и JWT-авторизацию.
