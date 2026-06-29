# Микросервисное приложение для загрузки и хранения изображений

## Содержание
- [Архитектура](#архитектура)
- [Требования](#требования)
- [Быстрый старт через Docker Compose](#быстрый-старт-через-docker-compose)
- [Запуск в Kubernetes (minikube)](#запуск-в-kubernetes-minikube)
- [Тестирование](#тестирование)
- [Устранение неполадок](#устранение-неполадок)
- [Структура проекта](#структура-проекта)

---

## Архитектура

```
Клиент
  │
  ▼
[api-service :8080]  ──── JWT проверка ──── id.klsh.ru (Keycloak)
  │        │
  │        └──── Pre-Signed URL ──── [Minio :9000]
  │                                       │
  │                                  WebHook PUT
  │                                       │
  │                                       ▼
  │                               [img-service :8081]
  │                                       │
  │                          ┌────────────┼────────────┐
  │                          ▼            ▼            ▼
  │                     [large]      [medium]      [small]
  │                    1024x1024     512x512       128x128
  │                          └────────────┼────────────┘
  │                                       │ WebHook
  │                                       ▼
  │                               [push-service :8082]
  │                                       │
  └──── GET /api/img ──── [Postgres :5432] ◄───────────┘
```

**Сервисы**:
- **api-service** – HTTP‑gateway, аутентификация JWT, выдача Pre‑Signed URL, получение метаданных.
- **img-service** – обработка изображений (создание preview в формате JPEG).
- **push-service** – имитация уведомлений (логирование).
- **Postgres** – хранение метаданных.
- **Minio** – S3‑совместимое объектное хранилище.

---

## Требования

- **Docker** и **Docker Compose** (для локального запуска)
- **minikube** и **kubectl** (для запуска в Kubernetes)
- **Go 1.22** (только для разработки, не требуется для запуска)
- Утилиты: `curl`, `jq`, `python3` (для тестов)
- Прокси (если используется) должен быть отключён или настроен для доступа к `id.klsh.ru`

---

## Быстрый старт через Docker Compose

Этот способ подходит для локальной разработки и быстрой проверки.

### 1. Клонирование и переход в директорию

```bash
cd ~/sirius/programs/go/microservices   # или путь к вашему проекту
```

### 2. Очистка предыдущих контейнеров (опционально)

```bash
docker-compose down -v
```

### 3. Сборка и запуск

```bash
docker-compose up -d --build
```

Проверьте, что все контейнеры запущены:

```bash
docker-compose ps
```

Убедитесь, что `createbuckets` завершился успешно (статус `Exited (0)`):

```bash
docker-compose logs createbuckets
```

### 4. Проверка работоспособности

```bash
curl http://localhost:8080/healthz   # должно вернуть "ok"
```

### 5. Получение JWT‑токена (если сервер id.klsh.ru доступен)

```bash
TOKEN=$(curl -s -X POST https://id.klsh.ru/realms/sirius/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=exam-client&grant_type=password&username=student&password=P@ssw0rd&scope=openid" \
  | jq -r '.access_token')
echo $TOKEN
```

### 6. Полный тестовый цикл

Для проверки всех этапов выполните скрипт `test_compose.sh` (если он есть) или последовательно команды:

```bash
# 1. Запрос Pre‑Signed URL
RESP=$(curl -s -X POST http://localhost:8080/api/img \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test.png","type":"image/png","size":232517}')
OBJECT_ID=$(echo "$RESP" | jq -r '.data.object_id')
UPLOAD_URL=$(echo "$RESP" | jq -r '.data.endpoint')

# 2. Загрузка файла
curl -X PUT "$UPLOAD_URL" -H "Content-Type: image/png" --upload-file test.png

# 3. Ожидание обработки (20 сек)
sleep 20

# 4. Проверка статуса
curl -s "http://localhost:8080/api/img?id=$OBJECT_ID" -H "Authorization: Bearer $TOKEN" | jq .
```

### 7. Остановка

```bash
docker-compose down -v
```

---

## Запуск в Kubernetes (minikube)

Для запуска в кластере используется minikube. Все манифесты находятся в папке `k8s/`.

### 1. Убедитесь, что minikube запущен и работает

```bash
minikube status
```

Если minikube не запущен:

```bash
minikube start --driver=docker
```

### 2. Освободите место на диске (рекомендуется)

```bash
docker system prune -a -f
```

### 3. Соберите образы с новым тегом (например, `v2`) и загрузите в minikube

Проще всего использовать скрипт `build-and-deploy.sh`, который выполняет все шаги автоматически:

```bash
chmod +x build-and-deploy.sh
./build-and-deploy.sh
```

Скрипт:
- удаляет старые образы,
- собирает образы с тегом `v2` (без кэша),
- загружает их в minikube,
- обновляет деплойменты на новый тег,
- перезапускает поды,
- удаляет переменную `S3_EXTERNAL_URL` (чтобы Pre‑Signed URL возвращал оригинальный хост `minio:9000`).

Если вы хотите выполнить шаги вручную:

```bash
# Сборка
docker build --no-cache -t api-service:v2 ./api-service
docker build --no-cache -t img-service:v2 ./img-service
docker build --no-cache -t push-service:v2 ./push-service

# Загрузка в minikube
minikube image load api-service:v2
minikube image load img-service:v2
minikube image load push-service:v2

# Обновление деплойментов
kubectl set image -n sirius deployment/api-service api-service=api-service:v2
kubectl set image -n sirius deployment/img-service img-service=img-service:v2
kubectl set image -n sirius deployment/push-service push-service=push-service:v2

kubectl rollout restart -n sirius deployment/api-service
kubectl rollout restart -n sirius deployment/img-service
kubectl rollout restart -n sirius deployment/push-service

# Удалить S3_EXTERNAL_URL, если она задана
kubectl set env -n sirius deployment/api-service S3_EXTERNAL_URL-
kubectl rollout restart -n sirius deployment/api-service
```

### 4. Примените все манифесты (если они ещё не применены)

Если вы не выполняли `build-and-deploy.sh`, примените манифесты вручную (с отключённой валидацией для обхода ошибок OpenAPI):

```bash
kubectl apply -f k8s/namespace.yaml --validate=false
kubectl apply -f k8s/secrets.yaml --validate=false
kubectl apply -f k8s/postgres/ --validate=false
kubectl apply -f k8s/minio/ --validate=false
kubectl apply -f k8s/push-service/ --validate=false
kubectl apply -f k8s/img-service/ --validate=false
kubectl apply -f k8s/api-service/ --validate=false
```

### 5. Дождитесь готовности всех подов

```bash
kubectl get pods -n sirius -w
```

Все поды должны перейти в состояние `Running` (кроме `minio-init` – он будет `Completed`).

### 6. Добавьте запись в `/etc/hosts` (для разрешения `minio` с хоста)

Pre‑Signed URL возвращает хост `minio:9000`. Чтобы `curl` мог обращаться к нему через порт‑форвард, добавьте:

```bash
echo "127.0.0.1 minio" | sudo tee -a /etc/hosts
```

### 7. Пробросьте порты API и Minio (в двух отдельных терминалах)

**Терминал 1:**
```bash
kubectl port-forward -n sirius svc/api-service 8080:8080
```

**Терминал 2:**
```bash
kubectl port-forward -n sirius svc/minio 9000:9000
```

### 8. Запустите тесты

```bash
./tests.sh
```

Скрипт `tests.sh` автоматически проверит все этапы: здоровье, метрики, аутентификацию, загрузку, обработку, уведомления, доступность файлов. Убедитесь, что перед запуском отключены прокси-переменные (скрипт `scrypt.sh` делает это).

---

## Тестирование

### Автоматический тест (Kubernetes)

```bash
./tests.sh
```

### Ручной тест (Docker Compose или Kubernetes)

Выполните команды, описанные в разделе «Быстрый старт». Ожидаемый результат: загрузка файла (HTTP 200), статус `ready` и доступные preview.

### Проверка метрик

```bash
curl http://localhost:8080/metrics | grep api_requests_total
```

### Просмотр логов

Для Docker Compose:

```bash
docker-compose logs api-service
docker-compose logs img-service
docker-compose logs push-service
```

Для Kubernetes:

```bash
kubectl logs -n sirius deploy/api-service
kubectl logs -n sirius deploy/img-service
kubectl logs -n sirius deploy/push-service
```

---

## Устранение неполадок

### 1. Pre‑Signed URL возвращает 403 Forbidden

- Убедитесь, что `S3_EXTERNAL_URL` **не задана** в поде API (команда `kubectl set env -n sirius deployment/api-service S3_EXTERNAL_URL-`).
- Проверьте, что в `/etc/hosts` добавлена запись `127.0.0.1 minio`.
- Убедитесь, что порт‑форвард Minio работает.
- Проверьте, что Minio принимает `Host: minio` – в манифесте должно быть `MINIO_DOMAIN: localhost,minio`.

### 2. Токен не получен (ошибка `Connection refused`)

Сервер `id.klsh.ru` может быть недоступен. Временно отключите проверку JWT в `api-service/internal/middleware/auth.go` (закомментируйте проверку и подставьте фиктивный `sub`) для локального тестирования. **Не используйте в продакшене.**

### 3. Ошибка `Empty reply from server` при загрузке

Проверьте, что порт‑форвард Minio активен и что вы не используете прокси. Выполните `unset http_proxy https_proxy` перед `curl`.

### 4. Нехватка места в Docker

Выполните `docker system prune -a -f` и, если нужно, удалите старые образы вручную. Для minikube также выполните `minikube ssh -- docker system prune -f`.

### 5. Поды не стартуют (ImagePullBackOff)

Убедитесь, что образы загружены в minikube: `minikube image list | grep api-service`. Если нет – загрузите заново.

---

## Структура проекта

```
microservices/
├── init.sql                         # DDL таблицы image
├── docker-compose.yml               # Локальный запуск
├── build-and-deploy.sh              # Автоматический деплой в K8s
├── tests.sh                         # End‑to‑end тест для K8s
├── scrypt.sh                        # Отключение прокси
├── prepare.sh                       # Порт‑форварды
│
├── api-service/                     # API‑сервис
│   ├── Dockerfile
│   ├── go.mod
│   └── internal/
│       ├── config/                  # Конфигурация
│       ├── handler/                 # Обработчики HTTP
│       ├── middleware/              # JWT‑валидация
│       ├── model/                   # Структуры данных
│       ├── storage/                 # Postgres и Minio клиенты
│       └── metrics/                 # Prometheus метрики
│
├── img-service/                     # Сервис обработки изображений
│   ├── Dockerfile
│   ├── go.mod
│   └── internal/
│       ├── config/
│       ├── handler/                 # WebHook от Minio
│       ├── queue/                   # Очередь и логика ресайза
│       └── storage/                 # Postgres и Minio
│
├── push-service/                    # Сервис уведомлений
│   ├── Dockerfile
│   ├── go.mod
│   └── internal/
│       ├── config/
│       ├── handler/                 # POST /notify
│       └── storage/                 # Чтение из БД
│
└── k8s/                             # Манифесты Kubernetes
    ├── namespace.yaml
    ├── secrets.yaml
    ├── postgres/
    ├── minio/
    ├── api-service/
    ├── img-service/
    └── push-service/
```

---

## Заключение

Проект демонстрирует полный цикл работы микросервисного приложения – от аутентификации до обработки изображений и уведомлений. Он может быть запущен как локально (Docker Compose), так и в кластере Kubernetes (minikube). Для успешного запуска важно соблюдать настройки сетевого доступа (прокси, `/etc/hosts`) и правильно управлять переменными окружения.

При возникновении вопросов обращайтесь к секции [Устранение неполадок](#устранение-неполадок) или к исходному коду.