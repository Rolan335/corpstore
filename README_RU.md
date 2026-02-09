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

### Delete File

`DELETE /files/:id`

Удаляет файл по id.  
Требует авторизации. Только владелец может удалить.

Request:

```
DELETE /files/a1d5...
Authorization: Bearer <jwt>
```

Response (204): без тела

Ошибки:
- `400` missing id
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found

### Share File

`POST /files/:id/share`

Даёт доступ к файлу пользователю по username.  
Требует авторизации. Только владелец может делиться.

Request:

```
POST /files/a1d5.../share
Content-Type: application/json
Authorization: Bearer <jwt>

{
  "username": "bob"
}
```

Response (204): без тела

Ошибки:
- `400` invalid body
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found
- `404` user not found
- `500` failed to share file

### List Shared Files

`GET /files/shared`

Возвращает список файлов, расшаренных текущему пользователю.  
Требует авторизации.

Request:

```
GET /files/shared
Authorization: Bearer <jwt>
```

Response (200):

```
[
  {
    "id": "a1d5...",
    "filename": "shared.txt",
    "owner_id": "4e23b7a9-4f3c-4d68-9a8c-4c7f0ce6d5ce",
    "owner_tg_first_name": "Alex",
    "owner_tg_last_name": "Ivanov",
    "owner_tg_username": "alex"
  }
]
```

Ошибки:
- `401` unauthorized
- `500` failed to list shared files

### Download Shared File

`GET /files/shared/:id`

Скачивает расшаренный файл по id.  
Требует авторизации. Доступен владельцу и тем, кому файл расшарен.

Request:

```
GET /files/shared/a1d5...
Authorization: Bearer <jwt>
```

Response (200):
- Бинарные данные с `Content-Disposition: attachment; filename="<original>"`.

Ошибки:
- `400` missing id
- `401` unauthorized
- `403` forbidden
- `404` file not found

### List Granted Users

`GET /files/:id/shared-users`

Возвращает список пользователей, у которых есть доступ к файлу.  
Требует авторизации. Только владелец может смотреть.

Request:

```
GET /files/a1d5.../shared-users
Authorization: Bearer <jwt>
```

Response (200):

```
[
  { "user_id": "uuid...", "username": "@bob" }
]
```

Ошибки:
- `401` unauthorized
- `403` forbidden
- `404` file not found
- `500` failed to list granted users

### Revoke Share

`DELETE /files/:id/share/:username`

Отзывает доступ у username.  
Требует авторизации. Только владелец может отзывать.

Request:

```
DELETE /files/a1d5.../share/@bob
Authorization: Bearer <jwt>
```

Response (204): без тела

Ошибки:
- `401` unauthorized
- `403` forbidden
- `404` file not found
- `404` user not found
- `500` failed to revoke share

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

### Start

Команда:

```
/start
```

Показывает главное меню.

### Telegram Flow (Overview)

1. Открой `/start`.
2. Бот автоматически регистрирует тебя по Telegram аккаунту.
3. Кнопки:
- `My files` — список своих файлов.
- `Shared files` — список файлов, которыми поделились с тобой.
4. Отправь любой файл или фото — он загрузится.
5. Нажми на файл, чтобы:
- скачать
- удалить (только свои)
- поделиться (только свои)
6. `Shared with` показывает пользователей, у которых есть доступ, и позволяет отозвать доступ.

### Register

UI flow:
- Нажми `Register`
- Введи username
- Введи password

### Login

UI flow:
- Нажми `Login`
- Введи username
- Введи password

### Upload File

Отправь документ или фото (подпись не нужна).  
Нужно быть залогиненным.

Ответ:
- `file saved` с `name` и `id` если успех
- сообщение об ошибке если провал

Примечания:
- Максимальный размер файла: 20MB.

### My Files

Нажми `My files`, чтобы получить список своих файлов кнопками.

Клик по файлу показывает:
- UUID
- filename
- кнопки: `Download`, `Delete`, `Share`, `Shared with`

### Shared Files

Нажми `Shared files`, чтобы получить список расшаренных файлов кнопками.

Клик по расшаренному файлу показывает:
- UUID
- filename
- кнопку: `Download`

### Shared With (Owners Only)

В карточке файла нажми `Shared with`, чтобы увидеть пользователей и отозвать доступ.

### Download File (legacy command)

Команда:

```
/get <file-id>
```

Ответ:
- Отправляет документ, если владелец совпадает.
- Сообщение об ошибке, если файл не найден или доступ запрещён.

### Delete File (UI)

В карточке файла нажми `Delete`.

### Share File (UI)

В карточке файла нажми `Share` и введи username.
