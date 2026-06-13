# Paygate

Платёжный шлюз

## Стек

| Компонент | Технология |
|-----------|-----------|
| БД | ScyllaDB |
| Обработка трафика | Traefik |
| Frontend | TypeScript, React, Tamagui |
| Backend | Golang |
| Виртуализация | Docker |

## Архитектура

```
Frontend (React + Tamagui)  →  Traefik  →  Backend (Go)  →  ScyllaDB
                                       ↘  Bank API
```

## API Endpoints

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/payments/init` | Инициализация платежа |
| POST | `/api/v1/payments/confirm` | Подтверждение 3DS |
| GET | `/api/v1/payments/status?bank_session_id=` | Статус платежа |
| POST | `/api/v1/payments/refund` | Возврат средств |
| POST | `/api/v1/webhooks/bank` | Webhook от банка |
| GET | `/health` | Health check |
| GET | `/swagger/` | Swagger UI |

## State Machine

```
INITIATED → PENDING_3DS → CONFIRMED → COMPLETED ─→ REFUNDED
                                         └→ FAILED
```

## Быстрый старт

```bash
# Клонировать репозиторий
git clone git@github.com:Gv1don/paygate.git
cd paygate

# Настроить переменные окружения
cp .env.example .env
# Отредактировать .env при необходимости

# Запустить все сервисы
docker-compose up -d

# Проверить health
curl http://localhost/health
```

После запуска (`docker-compose up -d`):
- **Frontend**: `http://localhost`
- **API**: `http://localhost/api/v1/payments/*`
- **Swagger UI**: `http://localhost/swagger/`
- **ScyllaDB**: `localhost:9042`
- **Traefik Dashboard**: `http://localhost:8080`

## Структура проекта

```
├── backend/
│   ├── cmd/server/main.go           # Точка входа
│   ├── internal/
│   │   ├── config/                  # Конфигурация (env vars)
│   │   ├── db/                      # Подключение к ScyllaDB
│   │   ├── handlers/                # REST обработчики
│   │   ├── middleware/              # CORS, Logging, Recovery
│   │   ├── models/                  # Модели и state machine
│   │   └── service/                 # Бизнес-логика
│   ├── docs/                        # Swagger документация
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/                         # React + Tamagui
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── gateway/
│   └── traefik/                     # Traefik конфигурация
├── docker-compose.yml               # Оркестрация сервисов
└── .env.example                     # Шаблон переменных окружения
```

## Переменные окружения

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `SERVER_PORT` | `8081` | Порт бэкенда |
| `SCYLLA_ADDR` | `scylladb:9042` | Адрес ScyllaDB |
| `SCYLLA_KEYSPACE` | `paygate` | Имя keyspace |
| `BANK_API_URL` | `https://bankapi.example.com` | URL API банка |
| `BANK_SECRET` | — | Секретный ключ для вебхуков |
| `LOG_LEVEL` | `info` | Уровень логирования |

## Тесты

```bash
cd backend
go test ./... -v -count=1
```

## Запуск без Docker

```bash
# Бэкенд
cd backend
SCYLLA_ADDR=localhost:9042 go run ./cmd/server

# Фронтенд
cd frontend
npm install
npm run dev
```
