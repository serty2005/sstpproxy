package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"control-plane/internal/domain"
)

type realityKeysetRepo struct {
	db      queryer
	dialect Dialect
}

func (r *realityKeysetRepo) Create(ctx context.Context, item domain.RealityKeyset) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO reality_keysets (
			id, name, public_key, private_key_secret_path, is_active, created_at, rotated_at, comment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), item.ID, item.Name, item.PublicKey, item.PrivateKeySecretPath, item.IsActive, item.CreatedAt, item.RotatedAt, item.Comment)
	return normalizeWriteErr(err)
}

func (r *realityKeysetRepo) GetActive(ctx context.Context) (domain.RealityKeyset, error) {
	return scanRealityKeyset(r.db.QueryRowContext(ctx, rebind(r.dialect, `
		SELECT id, name, public_key, private_key_secret_path, is_active, created_at, rotated_at, comment
		FROM reality_keysets
		WHERE is_active = ?
		ORDER BY created_at DESC
		LIMIT 1
	`), true))
}

func (r *realityKeysetRepo) List(ctx context.Context) ([]domain.RealityKeyset, error) {
	rows, err := r.db.QueryContext(ctx, rebind(r.dialect, `
		SELECT id, name, public_key, private_key_secret_path, is_active, created_at, rotated_at, comment
		FROM reality_keysets
		ORDER BY created_at DESC
	`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.RealityKeyset
	for rows.Next() {
		item, err := scanRealityKeyset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *realityKeysetRepo) DeactivateAll(ctx context.Context, at time.Time) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE reality_keysets
		SET is_active = ?, rotated_at = ?
		WHERE is_active = ?
	`), false, at, true)
	return err
}

type realityShortIDRepo struct {
	db      queryer
	dialect Dialect
}

func (r *realityShortIDRepo) Create(ctx context.Context, item domain.RealityShortID) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO reality_short_ids (
			id, keyset_id, short_id, assigned_user_id, is_active, created_at, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`), item.ID, item.KeysetID, item.ShortID, item.AssignedUserID, item.IsActive, item.CreatedAt, item.RevokedAt)
	return normalizeWriteErr(err)
}

func (r *realityShortIDRepo) CreateBatch(ctx context.Context, items []domain.RealityShortID) error {
	for _, item := range items {
		if err := r.Create(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (r *realityShortIDRepo) GetFreeByKeyset(ctx context.Context, keysetID string) (domain.RealityShortID, error) {
	return scanRealityShortID(r.db.QueryRowContext(ctx, rebind(r.dialect, `
		SELECT id, keyset_id, short_id, assigned_user_id, is_active, created_at, revoked_at
		FROM reality_short_ids
		WHERE keyset_id = ? AND assigned_user_id IS NULL AND is_active = ? AND revoked_at IS NULL
		ORDER BY created_at ASC
		LIMIT 1
	`), keysetID, true))
}

func (r *realityShortIDRepo) AssignToUser(ctx context.Context, shortIDID string, userID string) error {
	result, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE reality_short_ids
		SET assigned_user_id = ?
		WHERE id = ? AND assigned_user_id IS NULL
	`), userID, shortIDID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *realityShortIDRepo) ReleaseByUser(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE reality_short_ids
		SET assigned_user_id = NULL
		WHERE assigned_user_id = ?
	`), userID)
	return err
}

func (r *realityShortIDRepo) ListActiveByKeyset(ctx context.Context, keysetID string) ([]domain.RealityShortID, error) {
	rows, err := r.db.QueryContext(ctx, rebind(r.dialect, `
		SELECT id, keyset_id, short_id, assigned_user_id, is_active, created_at, revoked_at
		FROM reality_short_ids
		WHERE keyset_id = ? AND is_active = ? AND revoked_at IS NULL
		ORDER BY created_at ASC
	`), keysetID, true)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.RealityShortID
	for rows.Next() {
		item, err := scanRealityShortID(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *realityShortIDRepo) DeactivateByKeyset(ctx context.Context, keysetID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE reality_short_ids
		SET is_active = ?, revoked_at = ?
		WHERE keyset_id = ? AND is_active = ?
	`), false, at, keysetID, true)
	return err
}

func scanRealityKeyset(scanner interface {
	Scan(...any) error
}) (domain.RealityKeyset, error) {
	var item domain.RealityKeyset
	var rotatedAt sql.NullTime
	var comment sql.NullString
	err := scanner.Scan(&item.ID, &item.Name, &item.PublicKey, &item.PrivateKeySecretPath, &item.IsActive, &item.CreatedAt, &rotatedAt, &comment)
	if err != nil {
		return domain.RealityKeyset{}, notFound(err)
	}
	item.RotatedAt = timePtr(rotatedAt)
	item.Comment = stringPtr(comment)
	return item, nil
}

func scanRealityShortID(scanner interface {
	Scan(...any) error
}) (domain.RealityShortID, error) {
	var item domain.RealityShortID
	var assigned sql.NullString
	var revokedAt sql.NullTime
	err := scanner.Scan(&item.ID, &item.KeysetID, &item.ShortID, &assigned, &item.IsActive, &item.CreatedAt, &revokedAt)
	if err != nil {
		return domain.RealityShortID{}, notFound(err)
	}
	item.AssignedUserID = stringPtr(assigned)
	item.RevokedAt = timePtr(revokedAt)
	return item, nil
}
