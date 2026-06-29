#!/bin/bash
set -e

cd ~/sirius/programs/go/microservices

# 1. Удалить старые образы (чтобы не было путаницы)
echo "==> Удаление старых образов..."
docker rmi -f api-service:latest img-service:latest push-service:latest 2>/dev/null || true
docker rmi -f api-service:v2 img-service:v2 push-service:v2 2>/dev/null || true

# 2. Пересобрать образы с новым тегом v2 (без кэша, чтобы гарантировать обновление)
echo "==> Сборка образов с тегом v2..."
docker build --no-cache -t api-service:v2 ./api-service
docker build --no-cache -t img-service:v2 ./img-service
docker build --no-cache -t push-service:v2 ./push-service

# 3. Загрузить в minikube
echo "==> Загрузка образов в minikube..."
minikube image load api-service:v2
minikube image load img-service:v2
minikube image load push-service:v2

# 4. Обновить деплойменты на новый тег
echo "==> Обновление деплойментов..."
kubectl set image -n sirius deployment/api-service api-service=api-service:v2
kubectl set image -n sirius deployment/img-service img-service=img-service:v2
kubectl set image -n sirius deployment/push-service push-service=push-service:v2

# 5. Перезапустить деплойменты и дождаться готовности
echo "==> Перезапуск деплойментов..."
kubectl rollout restart -n sirius deployment/api-service
kubectl rollout restart -n sirius deployment/img-service
kubectl rollout restart -n sirius deployment/push-service

kubectl rollout status -n sirius deployment/api-service
kubectl rollout status -n sirius deployment/img-service
kubectl rollout status -n sirius deployment/push-service

# 6. Удалить S3_EXTERNAL_URL (если есть)
echo "==> Удаление S3_EXTERNAL_URL..."
kubectl set env -n sirius deployment/api-service S3_EXTERNAL_URL-
kubectl rollout restart -n sirius deployment/api-service
kubectl rollout status -n sirius deployment/api-service

echo "✅ Готово. Теперь запустите порт-форварды и ./tests.sh"