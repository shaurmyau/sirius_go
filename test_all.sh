set -euo pipefail

BASE_URL="${1:-http://localhost:5000}"
JWT_SECRET="supersecret"
PASS=0
FAIL=0

# Цвета
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Функция для тестирования с выводом результата
test_step() {
    local desc="$1"
    local expected_code="$2"
    local cmd="$3"
    local grep_pattern="${4:-}"

    echo -n "Testing: $desc ... "
    response=$(eval "$cmd" 2>&1)
    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | sed '$d')

    if [[ "$http_code" == "$expected_code" ]]; then
        if [[ -n "$grep_pattern" ]]; then
            if echo "$body" | grep -q "$grep_pattern"; then
                echo -e "${GREEN}OK${NC} (code $http_code, pattern found)"
                PASS=$((PASS+1))
            else
                echo -e "${RED}FAIL${NC} (code $http_code, pattern NOT found)"
                echo "Body: $body"
                FAIL=$((FAIL+1))
            fi
        else
            echo -e "${GREEN}OK${NC} (code $http_code)"
            PASS=$((PASS+1))
        fi
    else
        echo -e "${RED}FAIL${NC} (expected $expected_code, got $http_code)"
        echo "Response: $body"
        FAIL=$((FAIL+1))
    fi
}

echo "======================================="
echo " Starting full API test on $BASE_URL"
echo "======================================="
echo ""

# 1. Проверка health (просто получить /api/users, ожидаем 200 и JSON)
test_step "GET /api/users (empty list)" "200" "curl -s -o /dev/null -w '%{http_code}' $BASE_URL/api/users"

# 2. Создание пользователя
CREATE_RESP=$(curl -s -w '\n%{http_code}' -X POST "$BASE_URL/api/users" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com"}')
USER_ID=$(echo "$CREATE_RESP" | sed '$d' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
HTTP_CODE=$(echo "$CREATE_RESP" | tail -n 1)

if [[ "$HTTP_CODE" == "201" && -n "$USER_ID" ]]; then
    echo -e "Testing: POST /api/users (create) ... ${GREEN}OK${NC} (created $USER_ID)"
    PASS=$((PASS+1))
else
    echo -e "Testing: POST /api/users (create) ... ${RED}FAIL${NC}"
    echo "Response: $CREATE_RESP"
    FAIL=$((FAIL+1))
fi

# 3. Получение списка пользователей (должен содержать testuser)
test_step "GET /api/users (list contains testuser)" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/api/users" \
  "testuser"

# 4. Получение пользователя по ID
test_step "GET /api/users/$USER_ID" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/api/users/$USER_ID" \
  "$USER_ID"

# 5. Обновление пользователя
test_step "PUT /api/users/$USER_ID (update)" "200" \
  "curl -s -w '\n%{http_code}' -X PUT $BASE_URL/api/users/$USER_ID \
   -H 'Content-Type: application/json' \
   -d '{\"username\":\"updateduser\",\"email\":\"updated@example.com\"}'" \
  "updateduser"

# 6. Генерация JWT токена для профиля (используем Python)
TOKEN=$(python3 -c "import jwt; print(jwt.encode({'sub':'$USER_ID'}, '$JWT_SECRET', algorithm='HS256'))" 2>/dev/null)
if [[ -z "$TOKEN" ]]; then
    echo -e "Failed to generate JWT token. Ensure PyJWT is installed (pip install pyjwt) or python3 available."
    exit 1
fi

# 7. Создание профиля с токеном
test_step "POST /api/profile (create profile with JWT)" "201" \
  "curl -s -w '\n%{http_code}' -X POST $BASE_URL/api/profile \
   -H 'Authorization: Bearer $TOKEN' \
   -H 'Content-Type: application/json' \
   -d '{\"name\":\"Test\",\"lname\":\"User\"}'" \
  "Test"

# 8. Получение профиля с токеном
test_step "GET /api/profile (get profile with JWT)" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/api/profile \
   -H 'Authorization: Bearer $TOKEN'" \
  "Test"

# 9. Обновление профиля
test_step "PUT /api/profile (update profile)" "200" \
  "curl -s -w '\n%{http_code}' -X PUT $BASE_URL/api/profile \
   -H 'Authorization: Bearer $TOKEN' \
   -H 'Content-Type: application/json' \
   -d '{\"name\":\"Updated\",\"lname\":\"User\"}'" \
  "Updated"

# 10. Доступ к профилю БЕЗ токена (ожидаем 401)
test_step "GET /api/profile (unauthorized, no token)" "401" \
  "curl -s -w '\n%{http_code}' $BASE_URL/api/profile"

# 11. Удаление профиля с токеном (ожидаем 204)
test_step "DELETE /api/profile (delete profile)" "204" \
  "curl -s -w '\n%{http_code}' -X DELETE $BASE_URL/api/profile \
   -H 'Authorization: Bearer $TOKEN'" ""

# 12. Удаление пользователя (ожидаем 204)
test_step "DELETE /api/users/$USER_ID (delete user)" "204" \
  "curl -s -w '\n%{http_code}' -X DELETE $BASE_URL/api/users/$USER_ID" ""

# 13. Проверка метрик (должны содержать summary quantile)
test_step "GET /metrics (contains quantile)" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/metrics" \
  "app_request_duration_summary_seconds.*quantile=\"0.5\""

# 14. Проверка client/server error counters
test_step "GET /metrics (has app_client_errors_total)" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/metrics" \
  "app_client_errors_total"

test_step "GET /metrics (has app_server_errors_total)" "200" \
  "curl -s -w '\n%{http_code}' $BASE_URL/metrics" \
  "app_server_errors_total"

echo ""
echo "======================================="
echo " Test Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}"
echo "======================================="

if [[ $FAIL -gt 0 ]]; then
    exit 1
else
    exit 0
fi