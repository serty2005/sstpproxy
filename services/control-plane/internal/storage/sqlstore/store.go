package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"control-plane/internal/domain"
)

type Dialect string

const (
	Postgres Dialect = "postgres"
	SQLite   Dialect = "sqlite"
)

type Store struct {
	db      *sql.DB
	dialect Dialect
	logger  *slog.Logger
}

type txStore struct {
	tx      *sql.Tx
	dialect Dialect
}

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewPostgres(ctx context.Context, dsn string, logger *slog.Logger) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(12)
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &Store{db: db, dialect: Postgres, logger: logger}, nil
}

func NewSQLite(ctx context.Context, path string, logger *slog.Logger) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &Store{db: db, dialect: SQLite, logger: logger}, nil
}

func (s *Store) Users() domain.UserRepository {
	return &userRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) RealityKeysets() domain.RealityKeysetRepository {
	return &realityKeysetRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) RealityShortIDs() domain.RealityShortIDRepository {
	return &realityShortIDRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) RenderedConfigs() domain.RenderedConfigRepository {
	return &renderedConfigRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) Audits() domain.AuditRepository {
	return &auditRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) MTProtoSecrets() domain.MTProtoSecretRepository {
	return &mtprotoSecretRepo{db: s.db, dialect: s.dialect}
}

func (s *Store) InTx(ctx context.Context, fn func(domain.TxStore) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	holder := &txStore{tx: tx, dialect: s.dialect}
	if err := fn(holder); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) Migrate(ctx context.Context, dir string) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		var exists int
		if err := s.db.QueryRowContext(ctx, rebind(s.dialect, `SELECT COUNT(1) FROM schema_migrations WHERE version = ?`), file.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(raw)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("не удалось применить миграцию %s: %w", file.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, rebind(s.dialect, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`), file.Name(), time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.logger.Info("миграция применена", "драйвер", s.dialect, "версия", file.Name())
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (t *txStore) Users() domain.UserRepository {
	return &userRepo{db: t.tx, dialect: t.dialect}
}

func (t *txStore) RealityKeysets() domain.RealityKeysetRepository {
	return &realityKeysetRepo{db: t.tx, dialect: t.dialect}
}

func (t *txStore) RealityShortIDs() domain.RealityShortIDRepository {
	return &realityShortIDRepo{db: t.tx, dialect: t.dialect}
}

func (t *txStore) RenderedConfigs() domain.RenderedConfigRepository {
	return &renderedConfigRepo{db: t.tx, dialect: t.dialect}
}

func (t *txStore) Audits() domain.AuditRepository {
	return &auditRepo{db: t.tx, dialect: t.dialect}
}

func (t *txStore) MTProtoSecrets() domain.MTProtoSecretRepository {
	return &mtprotoSecretRepo{db: t.tx, dialect: t.dialect}
}

func rebind(dialect Dialect, query string) string {
	if dialect != Postgres {
		return query
	}
	var builder strings.Builder
	index := 1
	for _, ch := range query {
		if ch == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}
