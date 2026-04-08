package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"control-plane/internal/domain"
)

type renderedConfigRepo struct {
	db      queryer
	dialect Dialect
}

func (r *renderedConfigRepo) NextVersion(ctx context.Context, kind string) (int64, error) {
	var version int64
	err := r.db.QueryRowContext(ctx, rebind(r.dialect, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM rendered_configs
		WHERE kind = ?
	`), kind).Scan(&version)
	return version, err
}

func (r *renderedConfigRepo) Create(ctx context.Context, item domain.RenderedConfig) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO rendered_configs (
			id, kind, version, path, sha256, created_at, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`), item.ID, item.Kind, item.Version, item.Path, item.SHA256, item.CreatedAt, item.CreatedBy)
	return normalizeWriteErr(err)
}

func (r *renderedConfigRepo) LatestByKind(ctx context.Context, kind string) (domain.RenderedConfig, error) {
	return scanRenderedConfig(r.db.QueryRowContext(ctx, rebind(r.dialect, `
		SELECT id, kind, version, path, sha256, created_at, created_by
		FROM rendered_configs
		WHERE kind = ?
		ORDER BY version DESC
		LIMIT 1
	`), kind))
}

type auditRepo struct {
	db      queryer
	dialect Dialect
}

func (r *auditRepo) Create(ctx context.Context, item domain.AuditEvent) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO audit_events (
			id, actor_type, actor_id, action, subject_type, subject_id, payload_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), item.ID, item.ActorType, item.ActorID, item.Action, item.SubjectType, item.SubjectID, item.PayloadJSON, item.CreatedAt)
	return err
}

type mtprotoSecretRepo struct {
	db      queryer
	dialect Dialect
}

func (r *mtprotoSecretRepo) Create(ctx context.Context, item domain.MTProtoSecret) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		INSERT INTO mtproto_secrets (
			id, name, secret_file_path, secret_sha256, is_active, created_at, rotated_at, comment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), item.ID, item.Name, item.SecretFilePath, item.SecretSHA256, item.IsActive, item.CreatedAt, item.RotatedAt, item.Comment)
	return normalizeWriteErr(err)
}

func (r *mtprotoSecretRepo) GetActive(ctx context.Context) (domain.MTProtoSecret, error) {
	return scanMTProtoSecret(r.db.QueryRowContext(ctx, rebind(r.dialect, `
		SELECT id, name, secret_file_path, secret_sha256, is_active, created_at, rotated_at, comment
		FROM mtproto_secrets
		WHERE is_active = ?
		ORDER BY created_at DESC
		LIMIT 1
	`), true))
}

func (r *mtprotoSecretRepo) DeactivateAll(ctx context.Context, at time.Time) error {
	_, err := r.db.ExecContext(ctx, rebind(r.dialect, `
		UPDATE mtproto_secrets
		SET is_active = ?, rotated_at = ?
		WHERE is_active = ?
	`), false, at, true)
	return err
}

func scanRenderedConfig(scanner interface {
	Scan(...any) error
}) (domain.RenderedConfig, error) {
	var item domain.RenderedConfig
	var createdBy sql.NullString
	err := scanner.Scan(&item.ID, &item.Kind, &item.Version, &item.Path, &item.SHA256, &item.CreatedAt, &createdBy)
	if err != nil {
		return domain.RenderedConfig{}, notFound(err)
	}
	item.CreatedBy = stringPtr(createdBy)
	return item, nil
}

func scanMTProtoSecret(scanner interface {
	Scan(...any) error
}) (domain.MTProtoSecret, error) {
	var item domain.MTProtoSecret
	var rotatedAt sql.NullTime
	var comment sql.NullString
	err := scanner.Scan(&item.ID, &item.Name, &item.SecretFilePath, &item.SecretSHA256, &item.IsActive, &item.CreatedAt, &rotatedAt, &comment)
	if err != nil {
		return domain.MTProtoSecret{}, notFound(err)
	}
	item.RotatedAt = timePtr(rotatedAt)
	item.Comment = stringPtr(comment)
	return item, nil
}
