package pg

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Handler func(tx pgx.Tx) error

type PgpoolProvider interface {
	Tx(ctx context.Context, hdl Handler) error
	Close()
	Ping(ctx context.Context) error
}

// Ошибки
var (
	errPgxpoolBegin  = errors.New("Ошибка подключения начала транзакции")
	errPgxpoolExec   = errors.New("Ошибка выполнения транзакции")
	errPgxpoolCommit = errors.New("Ошибка коммита")
)
