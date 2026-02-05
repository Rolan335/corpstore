# Corpstore

Corpstore — простой сервис хранения файлов с HTTP API и Telegram ботом. Пользователи регистрируются, логинятся, загружают, смотрят список и скачивают файлы по JWT.

## Быстрый старт

1. Настрой переменные окружения:

```
cp .env.example .env
```

2. Запусти через Docker:

```
docker compose up --build
```

HTTP API: `http://localhost:8080`  
Health API: `http://localhost:8081`

## Конфигурация

Переменные окружения (см. `.env.example`):

- `DATABASE_URL` — строка подключения к Postgres.
- `CORPSTORE_DATA_DIR` — директория для хранения байтов файлов.
- `AUTH_JWT_SECRET` — секрет для подписи JWT.
- `TELEGRAM_BOT_TOKEN` — токен Telegram бота (опционально; если отсутствует, бот не запускается).

## HTTP API

Base URL: `http://localhost:8080`

Заголовок для защищённых ручек:

```
Authorization: Bearer <jwt>
```

### Register

`POST /reg`

Создаёт пользователя и возвращает его id.

Request:

```
POST /reg
Content-Type: application/json

{
  "username": "alice",
  "password": "secret"
}
```

Response (201):

```
{
  "id": "4e23b7a9-4f3c-4d68-9a8c-4c7f0ce6d5ce",
  "username": "alice"
}
```

Ошибки:
- `400` invalid body or missing fields
- `500` failed to create user

### Login

`POST /login`

Проверяет логин/пароль и возвращает JWT.

Request:

```
POST /login
Content-Type: application/json

{
  "username": "alice",
  "password": "secret"
}
```

Response (200):

```
{
  "token": "<jwt>"
}
```

Ошибки:
- `400` invalid body or missing fields
- `401` invalid credentials

### Upload Files

`POST /files`

Загрузка одного или нескольких файлов. Поле формы должно быть `files`.  
Требует авторизации.

Request (curl example):

```
curl -X POST http://localhost:8080/files \
  -H "Authorization: Bearer <jwt>" \
  -F "files=@/path/to/file1.txt" \
  -F "files=@/path/to/file2.jpg"
```

Response (200):

```
[
  { "filename": "file1.txt", "id": "a1d5..." },
  { "filename": "file2.jpg", "id": "b2e6..." }
]
```

Ошибки:
- `400` invalid multipart body or missing `files`
- `401` unauthorized
- `500` failed to save file

### Download File

`GET /files/:id`

Скачивает файл по id.  
Требует авторизации. Только владелец может скачать.

Request:

```
GET /files/a1d5...
Authorization: Bearer <jwt>
```

Response (200):
- Бинарные данные с `Content-Disposition: attachment; filename="<original>"`.

Ошибки:
- `400` missing id
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found

### List Files

`GET /files`

Возвращает список файлов текущего пользователя.  
Требует авторизации.

Request:

```
GET /files
Authorization: Bearer <jwt>
```

Response (200):

```
[
  {
    "id": "a1d5...",
    "filename": "file1.txt",
    "owner_id": "4e23b7a9-4f3c-4d68-9a8c-4c7f0ce6d5ce",
    "created_at": "2026-02-05T10:36:36Z"
  }
]
```

Ошибки:
- `401` unauthorized
- `500` failed to list files

## Health API

Base URL: `http://localhost:8081`

### Liveness

`GET /live`  
Возвращает `200`, если сервис жив.

### Readiness

`GET /ready`  
Возвращает `200`, если сервис готов.

## Telegram Bot

Бот запускается только если задан `TELEGRAM_BOT_TOKEN`.

### Register

Команда:

```
/reg <username> <password>
```

Ответ:
- `registered user id: <id>` если успех
- сообщение об ошибке если провал

### Login

Команда:

```
/login <username> <password>
```

Ответ:
- `token: <jwt>` если успех
- сообщение об ошибке если провал

### Upload File

Отправь документ или фото с подписью:

```
/upload <token>
```

Ответ:
- `file saved with id: <id>` если успех
- сообщение об ошибке если провал

Примечания:
- Максимальный размер файла: 20MB.

### Download File

Команда:

```
/get <file-id> <token>
```

Ответ:
- Отправляет документ, если владелец совпадает.
- Сообщение об ошибке, если файл не найден или доступ запрещён.
