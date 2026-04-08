package postgres

import (
	"context"
	"log/slog"

	"control-plane/internal/storage/sqlstore"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func New(ctx context.Context, dsn string, logger *slog.Logger) (*sqlstore.Store, error) {
	return sqlstore.NewPostgres(ctx, dsn, logger)
}
