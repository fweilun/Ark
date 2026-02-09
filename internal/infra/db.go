// README: Postgres connection pool initialization using pgxpool.
package infra

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
)

func NewDB(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
    return pgxpool.New(ctx, dsn)
}
