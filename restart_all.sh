#!/bin/bash
set -e

echo "==> Удаляю старый кластер..."
kind delete cluster --name go-server || true

echo "==> Создаю новый кластер..."
kind create cluster --name go-server --config kind-config.yaml

echo "==> Собираю образы и загружаю в kind..."
docker build -t go-server:latest .
docker pull postgres:16
kind load docker-image go-server:latest --name go-server
kind load docker-image postgres:16 --name go-server

echo "==> Применяю манифесты..."
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/postgres-deployment.yaml
kubectl apply -f k8s/app-deployment.yaml
kubectl apply -f k8s/app-service.yaml

echo "==> Ожидаю готовности подов..."
kubectl wait --for=condition=Ready pods --all -n go-app --timeout=120s

echo "==> Запускаю проброс порта..."
kubectl port-forward -n go-app service/go-server-service 5000:80 &
PF_PID=$!
sleep 2

echo "==> Тестирую..."
curl -s http://localhost:5000/api/users
curl -X POST http://localhost:5000/api/users \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@example.com"}' && echo ""

echo "==> Сервер готов. Проброс порта PID=$PF_PID. Для остановки нажми Ctrl+C и удали кластер."
wait $PF_PID
