package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"control-plane/internal/config"
	"control-plane/internal/dockerctl"
	"control-plane/internal/domain"
	"control-plane/internal/mtproto"
	"control-plane/internal/xray"

	"github.com/google/uuid"
)

const (
	renderedKindXray    = "xray-server"
	renderedKindMTProto = "mtproto"
)

type Store = domain.Store

type AuditService struct {
	store domain.Store
}

type RealityShortIDService struct {
	cfg config.Config
}

type RealityKeyService struct {
	cfg      config.Config
	store    domain.Store
	xray     *xray.Manager
	shortIDs *RealityShortIDService
	audit    *AuditService
}

type UserService struct {
	store    domain.Store
	keys     *RealityKeyService
	shortIDs *RealityShortIDService
	audit    *AuditService
}

type ProfileService struct {
	cfg   config.Config
	store domain.Store
	keys  *RealityKeyService
	xray  *xray.Manager
	audit *AuditService
}

type XrayConfigService struct {
	cfg   config.Config
	store domain.Store
	keys  *RealityKeyService
	xray  *xray.Manager
	audit *AuditService
}

type XrayApplyService struct {
	cfg    config.Config
	config *XrayConfigService
	xray   *xray.Manager
	docker *dockerctl.Client
	audit  *AuditService
}

type MTProtoService struct {
	cfg     config.Config
	store   domain.Store
	manager *mtproto.Manager
	audit   *AuditService
}

type HealthService struct {
	cfg    config.Config
	store  domain.Store
	docker *dockerctl.Client
	keys   *RealityKeyService
}

type Facade struct {
	store    domain.Store
	logger   *slog.Logger
	Audit    *AuditService
	Keys     *RealityKeyService
	Users    *UserService
	Profiles *ProfileService
	XrayCfg  *XrayConfigService
	Xray     *XrayApplyService
	MTProto  *MTProtoService
	Healthz  *HealthService
}

func New(store domain.Store, cfg config.Config, xrayManager *xray.Manager, mtgManager *mtproto.Manager, docker *dockerctl.Client, logger *slog.Logger) *Facade {
	audit := &AuditService{store: store}
	shortIDs := &RealityShortIDService{cfg: cfg}
	keys := &RealityKeyService{
		cfg:      cfg,
		store:    store,
		xray:     xrayManager,
		shortIDs: shortIDs,
		audit:    audit,
	}
	users := &UserService{
		store:    store,
		keys:     keys,
		shortIDs: shortIDs,
		audit:    audit,
	}
	profiles := &ProfileService{
		cfg:   cfg,
		store: store,
		keys:  keys,
		xray:  xrayManager,
		audit: audit,
	}
	xrayConfig := &XrayConfigService{
		cfg:   cfg,
		store: store,
		keys:  keys,
		xray:  xrayManager,
		audit: audit,
	}

	return &Facade{
		store:    store,
		logger:   logger,
		Audit:    audit,
		Keys:     keys,
		Users:    users,
		Profiles: profiles,
		XrayCfg:  xrayConfig,
		Xray: &XrayApplyService{
			cfg:    cfg,
			config: xrayConfig,
			xray:   xrayManager,
			docker: docker,
			audit:  audit,
		},
		MTProto: &MTProtoService{
			cfg:     cfg,
			store:   store,
			manager: mtgManager,
			audit:   audit,
		},
		Healthz: &HealthService{
			cfg:    cfg,
			store:  store,
			docker: docker,
			keys:   keys,
		},
	}
}

func (f *Facade) Bootstrap(ctx context.Context) error {
	actor := systemActor("bootstrap")
	if _, err := f.Keys.EnsureActive(ctx); err != nil {
		return err
	}
	if _, err := f.MTProto.RenderAndActivate(ctx, actor, false); err != nil {
		return err
	}
	return f.Xray.Bootstrap(ctx, actor)
}

func (f *Facade) AddUser(ctx context.Context, actor domain.Actor, params domain.CreateUserParams) (domain.User, error) {
	return f.Users.Add(ctx, actor, params)
}

func (f *Facade) RevokeUser(ctx context.Context, actor domain.Actor, ref string) (domain.User, error) {
	return f.Users.Revoke(ctx, actor, ref)
}

func (f *Facade) ListUsers(ctx context.Context, actor domain.Actor) ([]domain.User, error) {
	return f.Users.List(ctx, actor)
}

func (f *Facade) UserLink(ctx context.Context, actor domain.Actor, ref string) (string, error) {
	return f.Profiles.Link(ctx, actor, ref)
}

func (f *Facade) UserProfile(ctx context.Context, actor domain.Actor, ref, format string) (string, error) {
	return f.Profiles.Profile(ctx, actor, ref, format)
}

func (f *Facade) RenderXrayConfig(ctx context.Context, actor domain.Actor) (domain.XrayRenderResult, error) {
	return f.XrayCfg.Render(ctx, actor)
}

func (f *Facade) ApplyXrayConfig(ctx context.Context, actor domain.Actor) (domain.ApplyResult, error) {
	return f.Xray.Apply(ctx, actor)
}

func (f *Facade) RotateRealityKeyset(ctx context.Context, actor domain.Actor) (domain.ApplyResult, error) {
	if _, err := f.Keys.Rotate(ctx, actor); err != nil {
		return domain.ApplyResult{}, err
	}
	return f.Xray.Apply(ctx, actor)
}

func (f *Facade) MTProtoLink(ctx context.Context, actor domain.Actor) (string, error) {
	return f.MTProto.Invite(ctx, actor)
}

func (f *Facade) Health(ctx context.Context) domain.HealthReport {
	return f.Healthz.Health(ctx)
}

func (f *Facade) Ready(ctx context.Context) domain.ReadinessReport {
	return f.Healthz.Ready(ctx)
}

func (s *AuditService) Write(ctx context.Context, actor domain.Actor, action, subjectType string, subjectID *string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.store.Audits().Create(ctx, domain.AuditEvent{
		ID:          uuid.NewString(),
		ActorType:   actor.Type,
		ActorID:     actor.ID,
		Action:      action,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		PayloadJSON: string(raw),
		CreatedAt:   time.Now().UTC(),
	})
}

func (s *RealityShortIDService) GeneratePool(keysetID string, size int, now time.Time) ([]domain.RealityShortID, error) {
	items := make([]domain.RealityShortID, 0, size)
	seen := make(map[string]struct{}, size)
	for len(items) < size {
		value, err := s.randomShortID()
		if err != nil {
			return nil, err
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, domain.RealityShortID{
			ID:        uuid.NewString(),
			KeysetID:  keysetID,
			ShortID:   value,
			IsActive:  true,
			CreatedAt: now,
		})
	}
	return items, nil
}

func (s *RealityShortIDService) Allocate(ctx context.Context, tx domain.TxStore, keysetID string, userID string, now time.Time) (domain.RealityShortID, error) {
	free, err := tx.RealityShortIDs().GetFreeByKeyset(ctx, keysetID)
	if err == nil {
		if err := tx.RealityShortIDs().AssignToUser(ctx, free.ID, userID); err == nil {
			free.AssignedUserID = &userID
			return free, nil
		}
	}
	return s.CreateUnique(ctx, tx, keysetID, &userID, now)
}

func (s *RealityShortIDService) CreateUnique(ctx context.Context, tx domain.TxStore, keysetID string, assignedUserID *string, now time.Time) (domain.RealityShortID, error) {
	for range 16 {
		value, err := s.randomShortID()
		if err != nil {
			return domain.RealityShortID{}, err
		}
		item := domain.RealityShortID{
			ID:             uuid.NewString(),
			KeysetID:       keysetID,
			ShortID:        value,
			AssignedUserID: assignedUserID,
			IsActive:       true,
			CreatedAt:      now,
		}
		if err := tx.RealityShortIDs().Create(ctx, item); err == nil {
			return item, nil
		} else if err != domain.ErrAlreadyExists {
			return domain.RealityShortID{}, err
		}
	}
	return domain.RealityShortID{}, fmt.Errorf("не удалось подобрать уникальный shortId")
}

func (s *RealityShortIDService) randomShortID() (string, error) {
	buf := make([]byte, s.cfg.RealityShortIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *HealthService) Health(ctx context.Context) domain.HealthReport {
	return domain.HealthReport{
		Status: "ok",
		Time:   time.Now().UTC(),
	}
}

func (s *HealthService) Ready(ctx context.Context) domain.ReadinessReport {
	report := domain.ReadinessReport{
		Status:        "ok",
		Time:          time.Now().UTC(),
		Storage:       "ok",
		Docker:        "ok",
		ActiveKeyset:  "ok",
		XrayConfig:    "ok",
		MTProtoConfig: "ok",
	}

	if err := s.store.Ping(ctx); err != nil {
		report.Status = "degraded"
		report.Storage = err.Error()
	}
	if err := s.docker.Ping(ctx); err != nil {
		report.Status = "degraded"
		report.Docker = err.Error()
	}
	if _, err := s.keys.EnsureActive(ctx); err != nil {
		report.Status = "degraded"
		report.ActiveKeyset = err.Error()
	}
	if _, err := os.Stat(s.cfg.XrayActiveConfigPath); err != nil {
		report.Status = "degraded"
		report.XrayConfig = err.Error()
	}
	if _, err := os.Stat(s.cfg.MTProtoActiveConfig); err != nil {
		report.Status = "degraded"
		report.MTProtoConfig = err.Error()
	}
	return report
}

func systemActor(id string) domain.Actor {
	return domain.Actor{Type: "system", ID: id}
}

func actorLabel(actor domain.Actor) *string {
	if actor.Type == "" {
		return nil
	}
	value := actor.Type
	if actor.ID != "" {
		value += ":" + actor.ID
	}
	return &value
}

func lookupUser(ctx context.Context, store domain.Store, ref string) (domain.User, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return domain.User{}, fmt.Errorf("%w: пользователь не указан", domain.ErrValidation)
	}
	if user, err := store.Users().GetByID(ctx, ref); err == nil {
		return user, nil
	}
	if user, err := store.Users().GetByUUID(ctx, ref); err == nil {
		return user, nil
	}
	return store.Users().GetByUsername(ctx, normalizeUsername(ref))
}

func profileLabel(user domain.User) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return user.DisplayName
	}
	return user.Username
}
