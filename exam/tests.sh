#!/bin/bash
# ================================================================
#  tests.sh — end-to-end тест микросервисного приложения
#  Тестовое изображение: ~/sirius/programs/go/microservices/test.png
# ================================================================

# ── Предустановки ────────────────────────────────────────────────────────────
# 1. Убедиться, что порт-форвард API работает
curl -s http://localhost:8080/healthz > /dev/null || echo "⚠️  API недоступен, запустите port-forward"

# 2. Сгенерировать несколько запросов с невалидным токеном (для увеличения метрики 401)
for i in {1..5}; do
  curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/api/img -H "Authorization: Bearer invalid.token.here"
done

# 3. Сделать запрос без токена
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8080/api/img

echo "✅ Метрики 401 созданы. Теперь запустите ./tests.sh"

set -uo pipefail

# ── Цвета ────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

PASS=0; FAIL=0; WARN=0

# ── Конфиг ───────────────────────────────────────────────────────
API_URL="http://localhost:8080"
MINIO_URL="http://localhost:9000"
NS="sirius"
TEST_IMAGE="$HOME/sirius/programs/go/microservices/test.png"
TOKEN_URL="https://id.klsh.ru/realms/sirius/protocol/openid-connect/token"
TOKEN_PAYLOAD="client_id=exam-client&grant_type=password&username=student&password=P@ssw0rd&scope=openid"

PF_API_PID=""
PF_MINIO_PID=""

# ── Хелперы ──────────────────────────────────────────────────────
section() {
  echo ""
  echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
  echo -e "${BOLD}${CYAN}  $1${RESET}"
  echo -e "${BOLD}${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
}

ok()   { echo -e "  ${GREEN}✓${RESET}  $1"; ((PASS++)); }
fail() { echo -e "  ${RED}✗${RESET}  $1"; [[ -n "${2:-}" ]] && echo -e "     ${RED}→ $2${RESET}"; ((FAIL++)); }
warn() { echo -e "  ${YELLOW}!${RESET}  $1"; ((WARN++)); }
info() { echo -e "     ${YELLOW}$1${RESET}"; }

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    ok "$desc (HTTP $actual)"
  else
    fail "$desc" "ожидался HTTP $expected, получен HTTP $actual"
  fi
}

assert_json() {
  local desc="$1" expected="$2" json="$3" path="$4"
  local val
  val=$(echo "$json" | jq -r "$path" 2>/dev/null || echo "__err__")
  if [[ "$val" == "$expected" ]]; then
    ok "$desc → \"$val\""
  else
    fail "$desc" "ожидалось \"$expected\", получено \"$val\""
  fi
}

assert_nonempty() {
  local desc="$1" json="$2" path="$3"
  local val
  val=$(echo "$json" | jq -r "$path" 2>/dev/null || echo "")
  if [[ -n "$val" && "$val" != "null" && "$val" != "__err__" ]]; then
    ok "$desc → \"${val:0:60}...\""
  else
    fail "$desc" "поле пустое или null"
  fi
}

assert_contains() {
  local desc="$1" haystack="$2" needle="$3"
  if echo "$haystack" | grep -q "$needle"; then
    ok "$desc (содержит \"$needle\")"
  else
    fail "$desc" "\"$needle\" не найдено в ответе"
  fi
}

req()      { curl -sf --max-time 10 "$@" 2>/dev/null || true; }
req_code() { curl -s  --max-time 10 -o /dev/null -w "%{http_code}" "$@" 2>/dev/null || echo "000"; }

cleanup() {
  [[ -n "$PF_API_PID"   ]] && kill "$PF_API_PID"   2>/dev/null || true
  [[ -n "$PF_MINIO_PID" ]] && kill "$PF_MINIO_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ════════════════════════════════════════════════════════════════
section "0. ЗАВИСИМОСТИ И ОКРУЖЕНИЕ"
# ════════════════════════════════════════════════════════════════

for cmd in kubectl jq curl; do
  if command -v "$cmd" &>/dev/null; then
    ok "$cmd найден ($(command -v $cmd))"
  else
    fail "$cmd не найден — установите и повторите"
    exit 1
  fi
done

if [[ -f "$TEST_IMAGE" ]]; then
  SIZE=$(stat -c%s "$TEST_IMAGE")
  ok "Тестовое изображение найдено: $TEST_IMAGE ($SIZE байт)"
else
    fail "Файл не найден: $TEST_IMAGE"
    info "Положите любой PNG/JPEG по этому пути и запустите снова"
    exit 1
fi

# ════════════════════════════════════════════════════════════════
section "1. СОСТОЯНИЕ ПОДОВ KUBERNETES"
# ════════════════════════════════════════════════════════════════

EXPECTED_PODS=("postgres" "minio" "api-service" "img-service" "push-service")
ALL_PODS_OK=true

for svc in "${EXPECTED_PODS[@]}"; do
  STATUS=$(kubectl get pods -n "$NS" -l "app=$svc" \
    -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
  RESTARTS=$(kubectl get pods -n "$NS" -l "app=$svc" \
    -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "?")

  if [[ "$STATUS" == "True" ]]; then
    if [[ "$RESTARTS" -gt 3 ]] 2>/dev/null; then
      warn "Pod $svc Ready, но было $RESTARTS рестартов"
    else
      ok "Pod $svc Ready (рестарты: $RESTARTS)"
    fi
  else
    fail "Pod $svc не Ready" "статус: ${STATUS:-не найден}"
    ALL_PODS_OK=false
  fi
done

if [[ "$ALL_PODS_OK" == "false" ]]; then
  echo ""
  warn "Не все поды готовы. Логи проблемных подов:"
  for svc in "${EXPECTED_PODS[@]}"; do
    STATUS=$(kubectl get pods -n "$NS" -l "app=$svc" \
      -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    if [[ "$STATUS" != "True" ]]; then
      echo -e "\n  ${YELLOW}--- $svc ---${RESET}"
      kubectl logs -n "$NS" -l "app=$svc" --tail=10 2>/dev/null | sed 's/^/  /' || true
      kubectl describe pod -n "$NS" -l "app=$svc" 2>/dev/null \
        | grep -A3 "Last State\|Reason\|Exit Code" | head -15 | sed 's/^/  /' || true
    fi
  done
  echo ""
  warn "Запустите: kubectl get pods -n sirius для диагностики"
fi

# ════════════════════════════════════════════════════════════════
section "2. PORT-FORWARD"
# ════════════════════════════════════════════════════════════════

# Убить существующие port-forward если есть
pkill -f "port-forward.*8080" 2>/dev/null || true
pkill -f "port-forward.*9000" 2>/dev/null || true
sleep 1

kubectl port-forward -n "$NS" svc/api-service 8080:8080 &>/dev/null &
PF_API_PID=$!

kubectl port-forward -n "$NS" svc/minio 9000:9000 &>/dev/null &
PF_MINIO_PID=$!

info "Ожидание port-forward (6 сек)..."
sleep 6

if kill -0 "$PF_API_PID" 2>/dev/null; then
  ok "port-forward api-service → localhost:8080"
else
  fail "port-forward api-service не запустился"
  info "Попробуйте вручную: kubectl port-forward -n sirius svc/api-service 8080:8080"
  exit 1
fi

if kill -0 "$PF_MINIO_PID" 2>/dev/null; then
  ok "port-forward minio → localhost:9000"
else
  fail "port-forward minio не запустился"
fi

# ════════════════════════════════════════════════════════════════
section "3. HEALTH CHECK"
# ════════════════════════════════════════════════════════════════

CODE=$(req_code "$API_URL/healthz")
assert_eq "GET /healthz → 200" "200" "$CODE"

BODY=$(req "$API_URL/healthz")
assert_contains "тело ответа содержит ok" "$BODY" "ok"

# img-service и push-service через kubectl exec
for svc_port in "img-service:8081" "push-service:8082"; do
  svc="${svc_port%%:*}"
  port="${svc_port##*:}"
  RESULT=$(kubectl exec -n "$NS" deploy/"$svc" -- \
    curl -s "http://localhost:$port/healthz" 2>/dev/null || echo "")
  if [[ -n "$RESULT" ]]; then
    ok "$svc /healthz отвечает"
  else
    warn "$svc /healthz не ответил"
  fi
done

# ════════════════════════════════════════════════════════════════
section "4. МЕТРИКИ PROMETHEUS"
# ════════════════════════════════════════════════════════════════

METRICS=$(req "$API_URL/metrics")
if [[ -n "$METRICS" ]]; then
  ok "GET /metrics отвечает"
  assert_contains "содержит api_requests_total"        "$METRICS" "api_requests_total"
  assert_contains "содержит go_goroutines"             "$METRICS" "go_goroutines"
  assert_contains "содержит process_cpu_seconds_total" "$METRICS" "process_cpu_seconds_total"
else
  fail "GET /metrics не отвечает или пустой ответ"
fi

# ════════════════════════════════════════════════════════════════
section "5. АУТЕНТИФИКАЦИЯ (Keycloak JWT)"
# ════════════════════════════════════════════════════════════════

TOKEN_RESP=$(curl -s --max-time 15 -X POST "$TOKEN_URL" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "$TOKEN_PAYLOAD" 2>/dev/null || echo "{}")

TOKEN=$(echo "$TOKEN_RESP" | jq -r '.access_token' 2>/dev/null || echo "")

if [[ -n "$TOKEN" && "$TOKEN" != "null" && ${#TOKEN} -gt 50 ]]; then
  ok "JWT токен получен (${#TOKEN} символов)"
  assert_json "token_type == Bearer" "Bearer" "$TOKEN_RESP" ".token_type"
  assert_nonempty "refresh_token присутствует" "$TOKEN_RESP" ".refresh_token"
  assert_nonempty "id_token присутствует"      "$TOKEN_RESP" ".id_token"

  EXPIRES=$(echo "$TOKEN_RESP" | jq -r '.expires_in')
  ok "expires_in = $EXPIRES сек"
else
  fail "Токен не получен — проверьте доступ к id.klsh.ru"
  info "Ответ сервера: ${TOKEN_RESP:0:200}"
  info "Пропускаем тесты требующие авторизацию..."
  section "ИТОГ"
  echo -e "  ${GREEN}Пройдено:${RESET}  $PASS"
  echo -e "  ${RED}Провалено:${RESET} $FAIL"
  echo -e "  ${YELLOW}Предупреждений:${RESET} $WARN"
  exit 1
fi

AUTH="-H \"Authorization: Bearer $TOKEN\""

# ════════════════════════════════════════════════════════════════
section "6. ЗАЩИТА МАРШРУТОВ (401 без токена)"
# ════════════════════════════════════════════════════════════════

assert_eq "GET  /api/img без токена → 401" "401" \
  "$(req_code "$API_URL/api/img")"

assert_eq "POST /api/img без токена → 401" "401" \
  "$(req_code -X POST "$API_URL/api/img" \
     -H "Content-Type: application/json" \
     -d '{"name":"x","type":"image/jpeg","size":100}')"

assert_eq "невалидный токен → 401" "401" \
  "$(req_code "$API_URL/api/img" -H "Authorization: Bearer garbage.token.here")"

# ════════════════════════════════════════════════════════════════
section "7. ВАЛИДАЦИЯ POST /api/img"
# ════════════════════════════════════════════════════════════════

H_AUTH="Authorization: Bearer $TOKEN"
H_CT="Content-Type: application/json"

assert_eq "mime не image/* (PDF) → 400" "400" \
  "$(req_code -X POST "$API_URL/api/img" -H "$H_AUTH" -H "$H_CT" \
     -d '{"name":"doc.pdf","type":"application/pdf","size":1024}')"

assert_eq "size > 10MB → 400" "400" \
  "$(req_code -X POST "$API_URL/api/img" -H "$H_AUTH" -H "$H_CT" \
     -d '{"name":"big.jpg","type":"image/jpeg","size":10485761}')"

LONG=$(python3 -c "print('a'*256)")
assert_eq "name > 255 байт → 400" "400" \
  "$(req_code -X POST "$API_URL/api/img" -H "$H_AUTH" -H "$H_CT" \
     -d "{\"name\":\"$LONG\",\"type\":\"image/jpeg\",\"size\":1024}")"

assert_eq "пустое тело → 400" "400" \
  "$(req_code -X POST "$API_URL/api/img" -H "$H_AUTH" -H "$H_CT" -d '{}')"

assert_eq "ровно 10MB → 200" "200" \
  "$(req_code -X POST "$API_URL/api/img" -H "$H_AUTH" -H "$H_CT" \
     -d '{"name":"edge.jpg","type":"image/jpeg","size":10485760}')"

# ════════════════════════════════════════════════════════════════
section "8. POST /api/img — ПОЛУЧЕНИЕ PRE-SIGNED URL"
# ════════════════════════════════════════════════════════════════

FILE_SIZE=$(stat -c%s "$TEST_IMAGE")
FILE_NAME="test.png"

POST_RESP=$(curl -s --max-time 10 -X POST "$API_URL/api/img" \
  -H "$H_AUTH" -H "$H_CT" \
  -d "{\"name\":\"$FILE_NAME\",\"type\":\"image/png\",\"size\":$FILE_SIZE}")

POST_CODE=$(curl -s --max-time 10 -o /dev/null -w "%{http_code}" -X POST "$API_URL/api/img" \
  -H "$H_AUTH" -H "$H_CT" \
  -d "{\"name\":\"${FILE_NAME}_check\",\"type\":\"image/png\",\"size\":$FILE_SIZE}")

assert_eq "POST /api/img → 200" "200" "$POST_CODE"
assert_json "status == ok" "ok" "$POST_RESP" ".status"
assert_nonempty "data.object_id присутствует" "$POST_RESP" ".data.object_id"
assert_nonempty "data.endpoint присутствует"  "$POST_RESP" ".data.endpoint"

OBJECT_ID=$(echo "$POST_RESP" | jq -r '.data.object_id')
UPLOAD_URL=$(echo "$POST_RESP" | jq -r '.data.endpoint')

if echo "$OBJECT_ID" | grep -qE \
  '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'; then
  ok "object_id имеет формат UUID: $OBJECT_ID"
else
  fail "object_id не UUID" "$OBJECT_ID"
fi

assert_contains "endpoint содержит /upload/"        "$UPLOAD_URL" "/upload/"
assert_contains "endpoint содержит X-Amz-Signature" "$UPLOAD_URL" "X-Amz-Signature"
assert_contains "endpoint содержит X-Amz-Expires"   "$UPLOAD_URL" "X-Amz-Expires"

info "object_id: $OBJECT_ID"
info "endpoint:  ${UPLOAD_URL:0:80}..."

# ════════════════════════════════════════════════════════════════
section "9. GET /api/img — СТАТУС 'upload' (до загрузки файла)"
# ════════════════════════════════════════════════════════════════

assert_eq "статус upload → 406" "406" \
  "$(req_code "$API_URL/api/img?id=$OBJECT_ID" -H "$H_AUTH")"

assert_eq "несуществующий UUID → 404" "404" \
  "$(req_code "$API_URL/api/img?id=00000000-0000-0000-0000-000000000000" -H "$H_AUTH")"

assert_eq "невалидный id → 400" "400" \
  "$(req_code "$API_URL/api/img?id=not-a-uuid" -H "$H_AUTH")"

# ════════════════════════════════════════════════════════════════
section "10. ЗАГРУЗКА ФАЙЛА В S3 (PUT по Pre-Signed URL)"
# ════════════════════════════════════════════════════════════════

# Подменяем внутренний хост minio на localhost
UPLOAD_LOCAL="$UPLOAD_URL"

info "Загружаем: $TEST_IMAGE ($FILE_SIZE байт)"
info "URL: ${UPLOAD_LOCAL:0:80}..."

UPLOAD_CODE=$(curl -s --max-time 30 -o /dev/null -w "%{http_code}" \
  -X PUT "$UPLOAD_LOCAL" \
  -H "Content-Type: image/png" \
  --upload-file "$TEST_IMAGE")

assert_eq "PUT файла в S3 → 200" "200" "$UPLOAD_CODE"

# ════════════════════════════════════════════════════════════════
section "11. ОБРАБОТКА ИЗОБРАЖЕНИЯ (IMG Service)"
# ════════════════════════════════════════════════════════════════

info "Ожидание обработки изображения (20 сек)..."
sleep 20

IMG_LOGS=$(kubectl logs -n "$NS" -l app=img-service --tail=80 2>/dev/null || echo "")

assert_contains "лог: processing started"  "$IMG_LOGS" "processing started"
assert_contains "лог: preview large"       "$IMG_LOGS" "large"
assert_contains "лог: preview medium"      "$IMG_LOGS" "medium"
assert_contains "лог: preview small"       "$IMG_LOGS" "small"
assert_contains "лог: processing done"     "$IMG_LOGS" "processing done"

if echo "$IMG_LOGS" | grep -q "processing failed\|level.*ERROR"; then
  fail "В логах img-service есть ошибки"
  info "Последние 15 строк логов img-service:"
  echo "$IMG_LOGS" | tail -15 | sed 's/^/     /'
else
  ok "Логи img-service без ошибок"
fi

# ════════════════════════════════════════════════════════════════
section "12. УВЕДОМЛЕНИЯ (PUSH Service)"
# ════════════════════════════════════════════════════════════════

PUSH_LOGS=$(kubectl logs -n "$NS" -l app=push-service --tail=30 2>/dev/null || echo "")

assert_contains "лог push: notify"        "$PUSH_LOGS" "notify"
assert_contains "лог push: status=ready"  "$PUSH_LOGS" "ready"
assert_contains "лог push: object=image"  "$PUSH_LOGS" "image"
assert_contains "лог push: filename"      "$PUSH_LOGS" "$FILE_NAME"

# ════════════════════════════════════════════════════════════════
section "13. БАЗА ДАННЫХ — СОСТОЯНИЕ ЗАПИСИ"
# ════════════════════════════════════════════════════════════════

DB_ROW=$(kubectl exec -n "$NS" deploy/postgres -- \
  psql -U dbuser -d exam -t -A \
  -c "SELECT id||'|'||name||'|'||status||'|'||size_large||'|'||size_medium||'|'||size_small \
      FROM public.image WHERE id='$OBJECT_ID';" 2>/dev/null | head -1 || echo "")

if [[ -n "$DB_ROW" ]]; then
  ok "Запись найдена в БД"
  IFS='|' read -r db_id db_name db_status db_large db_medium db_small <<< "$DB_ROW"
  info "id=$db_id name=$db_name status=$db_status large=$db_large medium=$db_medium small=$db_small"

  [[ "$db_status" == "ready" ]]       && ok  "status == ready" \
                                        || fail "status" "ожидался ready, получен $db_status"
  [[ "$db_name" == "$FILE_NAME" ]]    && ok  "name == $FILE_NAME" \
                                        || fail "name в БД" "$db_name"
  [[ "${db_large:-0}"  -gt 0 ]] 2>/dev/null && ok  "size_large  = $db_large байт" \
                                        || fail "size_large == 0"
  [[ "${db_medium:-0}" -gt 0 ]] 2>/dev/null && ok  "size_medium = $db_medium байт" \
                                        || fail "size_medium == 0"
  [[ "${db_small:-0}"  -gt 0 ]] 2>/dev/null && ok  "size_small  = $db_small байт" \
                                        || fail "size_small == 0"
else
  fail "Запись с id=$OBJECT_ID не найдена в БД"
  info "Все записи в таблице:"
  kubectl exec -n "$NS" deploy/postgres -- \
    psql -U dbuser -d exam -c "SELECT id, name, status FROM public.image LIMIT 5;" \
    2>/dev/null | sed 's/^/  /' || true
fi

# ════════════════════════════════════════════════════════════════
section "14. GET /api/img — СТАТУС 'ready'"
# ════════════════════════════════════════════════════════════════

GET_RESP=$(req "$API_URL/api/img?id=$OBJECT_ID" -H "$H_AUTH")
GET_CODE=$(req_code "$API_URL/api/img?id=$OBJECT_ID" -H "$H_AUTH")

assert_eq "GET /api/img → 200" "200" "$GET_CODE"
assert_json "status == ok"             "ok"   "$GET_RESP" ".status"
assert_nonempty "data.original"  "$GET_RESP" ".data.original"
assert_nonempty "data.large"     "$GET_RESP" ".data.large"
assert_nonempty "data.medium"    "$GET_RESP" ".data.medium"
assert_nonempty "data.small"     "$GET_RESP" ".data.small"

assert_contains "original → /original/" "$GET_RESP" "/original/"
assert_contains "large    → /large/"    "$GET_RESP" "/large/"
assert_contains "medium   → /medium/"   "$GET_RESP" "/medium/"
assert_contains "small    → /small/"    "$GET_RESP" "/small/"
assert_contains "large содержит .jpeg"  "$GET_RESP" ".jpeg"
assert_contains "medium содержит .jpeg" "$GET_RESP" ".jpeg"
assert_contains "small содержит .jpeg"  "$GET_RESP" ".jpeg"

# Напечатать итоговые URL
if [[ "$GET_CODE" == "200" ]]; then
  echo ""
  info "Итоговые ссылки на файлы:"
  for field in original large medium small; do
    URL=$(echo "$GET_RESP" | jq -r ".data.$field" 2>/dev/null || echo "")
    echo -e "     ${CYAN}$field:${RESET} $URL"
  done
fi

# ════════════════════════════════════════════════════════════════
section "15. ДОСТУПНОСТЬ ФАЙЛОВ В S3"
# ════════════════════════════════════════════════════════════════

assert_eq "S3: original/$FILE_NAME доступен" "200" \
  "$(req_code "$MINIO_URL/original/$FILE_NAME")"

for bucket in large medium small; do
  assert_eq "S3: $bucket/$OBJECT_ID.jpeg доступен" "200" \
    "$(req_code "$MINIO_URL/$bucket/$OBJECT_ID.jpeg")"
done

# ════════════════════════════════════════════════════════════════
section "16. МЕТРИКИ ПОСЛЕ ТЕСТОВ"
# ════════════════════════════════════════════════════════════════

METRICS=$(req "$API_URL/metrics")
assert_contains "метрика POST 200 > 0" "$METRICS" \
  'api_requests_total{method="POST",path="/api/img",status="200"}'
assert_contains "метрика GET 200 > 0"  "$METRICS" \
  'api_requests_total{method="GET",path="/api/img",status="200"}'
assert_contains "метрика 400 > 0"      "$METRICS" 'status="400"'
assert_contains "метрика 401 > 0"      "$METRICS" 'status="401"'

# ════════════════════════════════════════════════════════════════
section "ИТОГ"
# ════════════════════════════════════════════════════════════════

TOTAL=$((PASS + FAIL + WARN))
echo ""
echo -e "  Всего проверок: ${BOLD}$TOTAL${RESET}"
echo -e "  ${GREEN}${BOLD}Пройдено:${RESET}       $PASS"
echo -e "  ${RED}${BOLD}Провалено:${RESET}      $FAIL"
echo -e "  ${YELLOW}${BOLD}Предупреждений:${RESET} $WARN"
echo ""

if [[ $FAIL -eq 0 ]]; then
  echo -e "  ${GREEN}${BOLD}✓  ВСЕ ТЕСТЫ ПРОЙДЕНЫ${RESET}"
  exit 0
else
  echo -e "  ${RED}${BOLD}✗  ЕСТЬ УПАВШИЕ ТЕСТЫ${RESET}"
  exit 1
fi