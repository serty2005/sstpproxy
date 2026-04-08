package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("объект не найден")
	ErrAlreadyExists = errors.New("объект уже существует")
	ErrUnauthorized  = errors.New("доступ запрещён")
	ErrValidation    = errors.New("ошибка валидации")
	ErrConflict      = errors.New("конфликт состояния")
)

type Actor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type User struct {
	ID                string     `json:"id"`
	Username          string     `json:"username"`
	DisplayName       string     `json:"display_name"`
	UUID              string     `json:"uuid"`
	RealityShortIDID  *string    `json:"reality_short_id_id,omitzero"`
	RealityShortID    *string    `json:"reality_short_id,omitzero"`
	IsActive          bool       `json:"is_active"`
	Comment           *string    `json:"comment,omitzero"`
	TrafficLimitBytes *int64     `json:"traffic_limit_bytes,omitzero"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitzero"`
}

type RealityKeyset struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	PublicKey            string     `json:"public_key"`
	PrivateKeySecretPath string     `json:"private_key_secret_path"`
	IsActive             bool       `json:"is_active"`
	CreatedAt            time.Time  `json:"created_at"`
	RotatedAt            *time.Time `json:"rotated_at,omitzero"`
	Comment              *string    `json:"comment,omitzero"`
}

type RealityShortID struct {
	ID             string     `json:"id"`
	KeysetID       string     `json:"keyset_id"`
	ShortID        string     `json:"short_id"`
	AssignedUserID *string    `json:"assigned_user_id,omitzero"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitzero"`
}

type RenderedConfig struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Version   int64     `json:"version"`
	Path      string    `json:"path"`
	SHA256    string    `json:"sha256"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy *string   `json:"created_by,omitzero"`
}

type AuditEvent struct {
	ID          string    `json:"id"`
	ActorType   string    `json:"actor_type"`
	ActorID     string    `json:"actor_id"`
	Action      string    `json:"action"`
	SubjectType string    `json:"subject_type"`
	SubjectID   *string   `json:"subject_id,omitzero"`
	PayloadJSON string    `json:"payload_json"`
	CreatedAt   time.Time `json:"created_at"`
}

type MTProtoSecret struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	SecretFilePath string     `json:"secret_file_path"`
	SecretSHA256   string     `json:"secret_sha256"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	RotatedAt      *time.Time `json:"rotated_at,omitzero"`
	Comment        *string    `json:"comment,omitzero"`
}

type CreateUserParams struct {
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	Comment           string `json:"comment"`
	TrafficLimitBytes *int64 `json:"traffic_limit_bytes,omitzero"`
}

type XrayRenderResult struct {
	Config string         `json:"config"`
	Record RenderedConfig `json:"record"`
}

type ApplyResult struct {
	Record RenderedConfig `json:"record"`
	Status string         `json:"status"`
}

type HealthReport struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

type ReadinessReport struct {
	Status        string    `json:"status"`
	Time          time.Time `json:"time"`
	Storage       string    `json:"storage"`
	Docker        string    `json:"docker"`
	ActiveKeyset  string    `json:"active_keyset"`
	XrayConfig    string    `json:"xray_config"`
	MTProtoConfig string    `json:"mtproto_config"`
}

type UserRepository interface {
	Create(context.Context, User) error
	UpdateRealityShortID(context.Context, string, *string, time.Time) error
	List(context.Context) ([]User, error)
	ListActive(context.Context) ([]User, error)
	GetByID(context.Context, string) (User, error)
	GetByUUID(context.Context, string) (User, error)
	GetByUsername(context.Context, string) (User, error)
	Revoke(context.Context, string, time.Time) error
}

type RealityKeysetRepository interface {
	Create(context.Context, RealityKeyset) error
	GetActive(context.Context) (RealityKeyset, error)
	List(context.Context) ([]RealityKeyset, error)
	DeactivateAll(context.Context, time.Time) error
}

type RealityShortIDRepository interface {
	Create(context.Context, RealityShortID) error
	CreateBatch(context.Context, []RealityShortID) error
	GetFreeByKeyset(context.Context, string) (RealityShortID, error)
	AssignToUser(context.Context, string, string) error
	ReleaseByUser(context.Context, string) error
	ListActiveByKeyset(context.Context, string) ([]RealityShortID, error)
	DeactivateByKeyset(context.Context, string, time.Time) error
}

type RenderedConfigRepository interface {
	NextVersion(context.Context, string) (int64, error)
	Create(context.Context, RenderedConfig) error
	LatestByKind(context.Context, string) (RenderedConfig, error)
}

type AuditRepository interface {
	Create(context.Context, AuditEvent) error
}

type MTProtoSecretRepository interface {
	Create(context.Context, MTProtoSecret) error
	GetActive(context.Context) (MTProtoSecret, error)
	DeactivateAll(context.Context, time.Time) error
}

type TxStore interface {
	Users() UserRepository
	RealityKeysets() RealityKeysetRepository
	RealityShortIDs() RealityShortIDRepository
	RenderedConfigs() RenderedConfigRepository
	Audits() AuditRepository
	MTProtoSecrets() MTProtoSecretRepository
}

type Store interface {
	TxStore
	InTx(context.Context, func(TxStore) error) error
	Migrate(context.Context, string) error
	Ping(context.Context) error
	Close() error
}
