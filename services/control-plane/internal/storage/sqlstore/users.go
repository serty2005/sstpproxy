package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"control-plane/internal/domain"
)

type userRepo struct {
	db      queryer
	dialect Dialect
}

func (r *userRepo) Create(ctx context.Context, user domain.User) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO users (
			id, username, display_name, uuid, reality_short_id_id, is_active, comment, traffic_limit_bytes, created_at, updated_at, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`), user.ID, user.Username, user.DisplayName, user.UUID, user.RealityShortIDID, user.IsActive, user.Comment, user.TrafficLimitBytes, user.CreatedAt, user.UpdatedAt, user.RevokedAt)
	return normalizeWriteErr(err)
}

func (r *userRepo) UpdateRealityShortID(ctx context.Context, userID string, shortIDID *string, at time.Time) error {
	result, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE users
		SET reality_short_id_id = ?, updated_at = ?
		WHERE id = ?
	`), shortIDID, at, userID)
	if err != nil {
		return err
	}
	return expectAffected(result)
}

func (r *userRepo) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, rebind(r.dialect, userSelect+` ORDER BY u.created_at DESC`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *userRepo) ListActive(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, rebind(r.dialect, userSelect+` WHERE u.is_active = ? AND u.revoked_at IS NULL ORDER BY u.created_at ASC`), true)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *userRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, rebind(r.dialect, userSelect+` WHERE u.id = ?`), id))
}

func (r *userRepo) GetByUUID(ctx context.Context, value string) (domain.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, rebind(r.dialect, userSelect+` WHERE u.uuid = ?`), value))
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, rebind(r.dialect, userSelect+` WHERE u.username = ?`), username))
}

func (r *userRepo) Revoke(ctx context.Context, id string, at time.Time) error {
	result, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE users
		SET is_active = ?, revoked_at = ?, updated_at = ?
		WHERE id = ?
	`), false, at, at, id)
	if err != nil {
		return err
	}
	return expectAffected(result)
}

func scanUsers(rows *sql.Rows) ([]domain.User, error) {
	var users []domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func scanUser(scanner interface {
	Scan(...any) error
}) (domain.User, error) {
	var user domain.User
	var shortIDID sql.NullString
	var shortID sql.NullString
	var comment sql.NullString
	var traffic sql.NullInt64
	var revokedAt sql.NullTime
	err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.UUID,
		&shortIDID,
		&shortID,
		&user.IsActive,
		&comment,
		&traffic,
		&user.CreatedAt,
		&user.UpdatedAt,
		&revokedAt,
	)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	user.RealityShortIDID = stringPtr(shortIDID)
	user.RealityShortID = stringPtr(shortID)
	user.Comment = stringPtr(comment)
	user.TrafficLimitBytes = int64Ptr(traffic)
	user.RevokedAt = timePtr(revokedAt)
	return user, nil
}

const userSelect = `
SELECT
	u.id,
	u.username,
	u.display_name,
	u.uuid,
	u.reality_short_id_id,
	rs.short_id,
	u.is_active,
	u.comment,
	u.traffic_limit_bytes,
	u.created_at,
	u.updated_at,
	u.revoked_at
FROM users u
LEFT JOIN reality_short_ids rs ON rs.id = u.reality_short_id_id
`
