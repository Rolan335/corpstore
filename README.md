# Corpstore

Corpstore is a simple file storage service with HTTP API and Telegram bot interface. Users register, login, upload files, list files, and download files with JWT authentication.

## Quick Start

1. Configure environment variables:

```
cp .env.example .env
```

2. Run with Docker:

```
docker compose up --build
```

HTTP API: `http://localhost:8080`  
Health API: `http://localhost:8081`

## Configuration

Environment variables (see `.env.example`):

- `DATABASE_URL` - Postgres DSN.
- `CORPSTORE_DATA_DIR` - data directory for file bytes.
- `AUTH_JWT_SECRET` - JWT signing secret.
- `TELEGRAM_BOT_TOKEN` - Telegram bot token (optional; if missing, bot is not started).

## HTTP API

Base URL: `http://localhost:8080`

Auth header for protected routes:

```
Authorization: Bearer <jwt>
```

### Register

`POST /reg`

Creates a user and returns the user id.

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

Errors:
- `400` invalid body or missing fields
- `500` failed to create user

### Login

`POST /login`

Validates credentials and returns JWT token.

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

Errors:
- `400` invalid body or missing fields
- `401` invalid credentials

### Upload Files

`POST /files`

Uploads one or more files. Multipart form field name must be `files`.  
Requires auth.

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

Errors:
- `400` invalid multipart body or missing `files`
- `401` unauthorized
- `500` failed to save file

### Delete File

`DELETE /files/:id`

Deletes a file by id.  
Requires auth. Only owner can delete.

Request:

```
DELETE /files/a1d5...
Authorization: Bearer <jwt>
```

Response (204): no content

Errors:
- `400` missing id
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found

### Share File

`POST /files/:id/share`

Grants access to a file by username.  
Requires auth. Only owner can share.

Request:

```
POST /files/a1d5.../share
Content-Type: application/json
Authorization: Bearer <jwt>

{
  "username": "bob"
}
```

Response (204): no content

Errors:
- `400` invalid body
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found
- `500` failed to share file

### List Shared Files

`GET /files/shared`

Lists files shared with the authenticated user.  
Requires auth.

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
    "owner_id": "4e23b7a9-4f3c-4d68-9a8c-4c7f0ce6d5ce"
  }
]
```

Errors:
- `401` unauthorized
- `500` failed to list shared files

### Download Shared File

`GET /files/shared/:id`

Downloads a shared file by id.  
Requires auth. Must be owner or shared user.

Request:

```
GET /files/shared/a1d5...
Authorization: Bearer <jwt>
```

Response (200):
- Binary data with `Content-Disposition: attachment; filename="<original>"`.

Errors:
- `400` missing id
- `401` unauthorized
- `403` forbidden
- `404` file not found

### Download File

`GET /files/:id`

Downloads a file by id.  
Requires auth. Only owner can download.

Request:

```
GET /files/a1d5...
Authorization: Bearer <jwt>
```

Response (200):
- Binary data with `Content-Disposition: attachment; filename="<original>"`.

Errors:
- `400` missing id
- `401` unauthorized
- `403` forbidden (not owner)
- `404` file not found

### List Files

`GET /files`

Lists files owned by the authenticated user.  
Requires auth.

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

Errors:
- `401` unauthorized
- `500` failed to list files

## Health API

Base URL: `http://localhost:8081`

### Liveness

`GET /live`  
Returns `200` if service is alive.

### Readiness

`GET /ready`  
Returns `200` if service is ready.

## Telegram Bot

Bot is started only if `TELEGRAM_BOT_TOKEN` is set.

### Start

Command:

```
/start
```

Shows main menu buttons.

### Register

UI flow:
- Press `Register`
- Enter username
- Enter password

### Login

UI flow:
- Press `Login`
- Enter username
- Enter password

### Upload File

Send a document or photo (no caption required).  
You must be logged in.

Response:
- `file saved` with `name` and `id` on success
- error message on failure

Notes:
- Max file size: 20MB.

### My Files

Press `My files` to get a list of your files as buttons.

Click a file to see details:
- UUID
- filename
- buttons: `Download`, `Delete`, `Share`

### Shared Files

Press `Shared files` to get a list of shared files as buttons.

Click a shared file to see details:
- UUID
- filename
- button: `Download`

### Download File (legacy command)

Command:

```
/get <file-id>
```

Response:
- Sends document if owner matches.
- Error message if not found or forbidden.

### Delete File (UI)

From file details, press `Delete`.

### Share File (UI)

From file details, press `Share` and enter target username.
