package sqlite

import (
	"context"
	"log/slog"

	"control-plane/internal/storage/sqlstore"

	_ "modernc.org/sqlite"
)

func New(ctx context.Context, path string, logger *slog.Logger) (*sqlstore.Store, error) {
	return sqlstore.NewSQLite(ctx, path, logger)
}
