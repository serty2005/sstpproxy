package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"control-plane/internal/domain"
	"control-plane/internal/xray"

	"github.com/google/uuid"
)

func (s *RealityKeyService) EnsureActive(ctx context.Context) (domain.RealityKeyset, error) {
	keyset, err := s.store.RealityKeysets().GetActive(ctx)
	if err == nil {
		return keyset, nil
	}
	if err != domain.ErrNotFound {
		return domain.RealityKeyset{}, err
	}

	now := time.Now().UTC()
	keyPair, err := s.xray.GenerateKeyPair(ctx)
	if err != nil {
		return domain.RealityKeyset{}, err
	}

	secretPath := s.cfg.RealityPrivateKeyPath(s.cfg.RealityPrivateKeyFile)
	if err := s.xray.WriteAtomic(secretPath, []byte(keyPair.PrivateKey+"\n"), 0o600); err != nil {
		return domain.RealityKeyset{}, err
	}

	keyset = domain.RealityKeyset{
		ID:                   uuid.NewString(),
		Name:                 s.cfg.RealityKeysetName,
		PublicKey:            keyPair.PublicKey,
		PrivateKeySecretPath: secretPath,
		IsActive:             true,
		CreatedAt:            now,
	}

	pool, err := s.shortIDs.GeneratePool(keyset.ID, s.cfg.RealityShortIDPoolSize, now)
	if err != nil {
		return domain.RealityKeyset{}, err
	}

	if err := s.store.InTx(ctx, func(tx domain.TxStore) error {
		if err := tx.RealityKeysets().Create(ctx, keyset); err != nil {
			return err
		}
		if err := tx.RealityShortIDs().CreateBatch(ctx, pool); err != nil {
			return err
		}
		return s.audit.Write(ctx, systemActor("bootstrap"), "reality_keyset_bootstrap", "reality_keyset", &keyset.ID, map[string]any{
			"name":               keyset.Name,
			"short_id_pool_size": len(pool),
		})
	}); err != nil {
		return domain.RealityKeyset{}, err
	}

	return keyset, nil
}

func (s *RealityKeyService) Rotate(ctx context.Context, actor domain.Actor) (domain.RealityKeyset, error) {
	now := time.Now().UTC()
	keyPair, err := s.xray.GenerateKeyPair(ctx)
	if err != nil {
		return domain.RealityKeyset{}, err
	}

	fileName := fmt.Sprintf("reality-%s.key", now.Format("20060102-150405"))
	secretPath := s.cfg.RealityPrivateKeyPath(fileName)
	if err := s.xray.WriteAtomic(secretPath, []byte(keyPair.PrivateKey+"\n"), 0o600); err != nil {
		return domain.RealityKeyset{}, err
	}

	keyset := domain.RealityKeyset{
		ID:                   uuid.NewString(),
		Name:                 fmt.Sprintf("%s-%s", s.cfg.RealityKeysetName, now.Format("20060102-150405")),
		PublicKey:            keyPair.PublicKey,
		PrivateKeySecretPath: secretPath,
		IsActive:             true,
		CreatedAt:            now,
	}

	err = s.store.InTx(ctx, func(tx domain.TxStore) error {
		oldKeyset, err := tx.RealityKeysets().GetActive(ctx)
		if err != nil && err != domain.ErrNotFound {
			return err
		}
		activeUsers, err := tx.Users().ListActive(ctx)
		if err != nil {
			return err
		}

		poolSize := s.cfg.RealityShortIDPoolSize
		if poolSize < len(activeUsers)+4 {
			poolSize = len(activeUsers) + 4
		}
		pool, err := s.shortIDs.GeneratePool(keyset.ID, poolSize, now)
		if err != nil {
			return err
		}

		if oldKeyset.ID != "" {
			if err := tx.RealityKeysets().DeactivateAll(ctx, now); err != nil {
				return err
			}
			if err := tx.RealityShortIDs().DeactivateByKeyset(ctx, oldKeyset.ID, now); err != nil {
				return err
			}
		}
		if err := tx.RealityKeysets().Create(ctx, keyset); err != nil {
			return err
		}
		if err := tx.RealityShortIDs().CreateBatch(ctx, pool); err != nil {
			return err
		}

		for i, user := range activeUsers {
			shortID := pool[i]
			if err := tx.RealityShortIDs().AssignToUser(ctx, shortID.ID, user.ID); err != nil {
				return err
			}
			if err := tx.Users().UpdateRealityShortID(ctx, user.ID, &shortID.ID, now); err != nil {
				return err
			}
		}

		return s.audit.Write(ctx, actor, "reality_keyset_rotate", "reality_keyset", &keyset.ID, map[string]any{
			"short_id_pool_size": poolSize,
			"rebound_users":      len(activeUsers),
		})
	})
	if err != nil {
		return domain.RealityKeyset{}, err
	}

	return keyset, nil
}

func (s *XrayConfigService) Render(ctx context.Context, actor domain.Actor) (domain.XrayRenderResult, error) {
	return s.render(ctx, actor, true)
}

func (s *XrayConfigService) render(ctx context.Context, actor domain.Actor, audit bool) (domain.XrayRenderResult, error) {
	keyset, err := s.keys.EnsureActive(ctx)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}

	privateKeyRaw, err := os.ReadFile(keyset.PrivateKeySecretPath)
	if err != nil {
		return domain.XrayRenderResult{}, fmt.Errorf("не удалось прочитать private key: %w", err)
	}

	shortIDs, err := s.store.RealityShortIDs().ListActiveByKeyset(ctx, keyset.ID)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}
	users, err := s.store.Users().ListActive(ctx)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}

	type client struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Flow  string `json:"flow"`
		Level int    `json:"level"`
	}

	clients := make([]client, 0, len(users))
	for _, user := range users {
		clients = append(clients, client{
			ID:    user.UUID,
			Email: user.Username,
			Flow:  s.cfg.XrayFlow,
			Level: 0,
		})
	}

	serverNamesJSON, err := xray.MarshalJSON(s.cfg.XrayRealityServerNames)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}
	shortValues := make([]string, 0, len(shortIDs))
	for _, item := range shortIDs {
		shortValues = append(shortValues, item.ShortID)
	}
	shortIDsJSON, err := xray.MarshalJSON(shortValues)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}
	clientsJSON, err := xray.MarshalJSON(clients)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}

	rendered, err := s.xray.RenderServerConfig(xray.ServerRenderInput{
		LogLevel:          s.cfg.LogLevel,
		Port:              s.cfg.XrayPort,
		RealityDest:       s.cfg.XrayRealityDest,
		ServerNamesJSON:   serverNamesJSON,
		RealityPrivateKey: strings.TrimSpace(string(privateKeyRaw)),
		ShortIDsJSON:      shortIDsJSON,
		ClientsJSON:       clientsJSON,
	})
	if err != nil {
		return domain.XrayRenderResult{}, err
	}

	version, err := s.store.RenderedConfigs().NextVersion(ctx, renderedKindXray)
	if err != nil {
		return domain.XrayRenderResult{}, err
	}
	path := filepath.Join(s.cfg.XrayArchiveDir, fmt.Sprintf("xray-config-v%06d.json", version))
	if err := s.xray.WriteAtomic(path, []byte(rendered), 0o640); err != nil {
		return domain.XrayRenderResult{}, err
	}

	record := domain.RenderedConfig{
		ID:        uuid.NewString(),
		Kind:      renderedKindXray,
		Version:   version,
		Path:      path,
		SHA256:    xray.SHA256Hex([]byte(rendered)),
		CreatedAt: time.Now().UTC(),
		CreatedBy: actorLabel(actor),
	}
	if err := s.store.RenderedConfigs().Create(ctx, record); err != nil {
		return domain.XrayRenderResult{}, err
	}
	if audit {
		if err := s.audit.Write(ctx, actor, "xray_render", "rendered_config", &record.ID, map[string]any{
			"version":     record.Version,
			"path":        record.Path,
			"users":       len(users),
			"short_ids":   len(shortValues),
			"reality_key": keyset.ID,
		}); err != nil {
			return domain.XrayRenderResult{}, err
		}
	}

	return domain.XrayRenderResult{Config: rendered, Record: record}, nil
}

func (s *XrayApplyService) Bootstrap(ctx context.Context, actor domain.Actor) error {
	rendered, err := s.config.render(ctx, actor, false)
	if err != nil {
		return err
	}
	if err := s.xray.ValidateConfig(ctx, rendered.Record.Path); err != nil {
		return err
	}
	return s.xray.WriteAtomic(s.cfg.XrayActiveConfigPath, []byte(rendered.Config), 0o640)
}

func (s *XrayApplyService) Apply(ctx context.Context, actor domain.Actor) (domain.ApplyResult, error) {
	rendered, err := s.config.render(ctx, actor, false)
	if err != nil {
		return domain.ApplyResult{}, err
	}
	if err := s.xray.ValidateConfig(ctx, rendered.Record.Path); err != nil {
		return domain.ApplyResult{}, err
	}
	if err := s.xray.WriteAtomic(s.cfg.XrayActiveConfigPath, []byte(rendered.Config), 0o640); err != nil {
		return domain.ApplyResult{}, err
	}
	if err := s.docker.RestartContainer(ctx, s.cfg.XrayContainerName); err != nil {
		return domain.ApplyResult{}, err
	}
	if err := s.audit.Write(ctx, actor, "xray_apply", "rendered_config", &rendered.Record.ID, map[string]any{
		"version": rendered.Record.Version,
		"path":    rendered.Record.Path,
		"sha256":  rendered.Record.SHA256,
	}); err != nil {
		return domain.ApplyResult{}, err
	}
	return domain.ApplyResult{Record: rendered.Record, Status: "applied"}, nil
}

func (s *MTProtoService) Invite(ctx context.Context, actor domain.Actor) (string, error) {
	secret, _, err := s.manager.LoadSecret()
	if err != nil {
		return "", err
	}
	if _, err := s.syncMetadata(ctx, actor, false); err != nil {
		return "", err
	}
	link := s.manager.Link(secret)
	if err := s.audit.Write(ctx, actor, "mtproto_invite", "mtproto_secret", nil, map[string]any{
		"host": s.cfg.MTProtoHost(),
		"port": s.cfg.MTProtoPort,
	}); err != nil {
		return "", err
	}
	return link, nil
}

func (s *MTProtoService) RenderAndActivate(ctx context.Context, actor domain.Actor, audit bool) (domain.RenderedConfig, error) {
	secret, _, err := s.manager.LoadSecret()
	if err != nil {
		return domain.RenderedConfig{}, err
	}
	meta, err := s.syncMetadata(ctx, actor, audit)
	if err != nil {
		return domain.RenderedConfig{}, err
	}
	rendered, err := s.manager.RenderConfig(secret)
	if err != nil {
		return domain.RenderedConfig{}, err
	}
	version, err := s.store.RenderedConfigs().NextVersion(ctx, renderedKindMTProto)
	if err != nil {
		return domain.RenderedConfig{}, err
	}
	path := filepath.Join(s.cfg.MTProtoArchiveDir, fmt.Sprintf("mtproto-v%06d.toml", version))
	if err := s.manager.WriteAtomic(path, []byte(rendered), 0o640); err != nil {
		return domain.RenderedConfig{}, err
	}
	if err := s.manager.WriteAtomic(s.cfg.MTProtoActiveConfig, []byte(rendered), 0o640); err != nil {
		return domain.RenderedConfig{}, err
	}
	record := domain.RenderedConfig{
		ID:        uuid.NewString(),
		Kind:      renderedKindMTProto,
		Version:   version,
		Path:      path,
		SHA256:    xray.SHA256Hex([]byte(rendered)),
		CreatedAt: time.Now().UTC(),
		CreatedBy: actorLabel(actor),
	}
	if err := s.store.RenderedConfigs().Create(ctx, record); err != nil {
		return domain.RenderedConfig{}, err
	}
	if audit {
		if err := s.audit.Write(ctx, actor, "mtproto_render", "rendered_config", &record.ID, map[string]any{
			"version":   version,
			"secret_id": meta.ID,
		}); err != nil {
			return domain.RenderedConfig{}, err
		}
	}
	return record, nil
}

func (s *MTProtoService) syncMetadata(ctx context.Context, actor domain.Actor, audit bool) (domain.MTProtoSecret, error) {
	_, sha256, err := s.manager.LoadSecret()
	if err != nil {
		return domain.MTProtoSecret{}, err
	}
	active, err := s.store.MTProtoSecrets().GetActive(ctx)
	if err == nil && active.SecretFilePath == s.cfg.MTProtoSecretFile && active.SecretSHA256 == sha256 {
		return active, nil
	}
	if err != nil && err != domain.ErrNotFound {
		return domain.MTProtoSecret{}, err
	}

	now := time.Now().UTC()
	record := domain.MTProtoSecret{
		ID:             uuid.NewString(),
		Name:           s.cfg.MTProtoSecretName,
		SecretFilePath: s.cfg.MTProtoSecretFile,
		SecretSHA256:   sha256,
		IsActive:       true,
		CreatedAt:      now,
	}
	if err := s.store.InTx(ctx, func(tx domain.TxStore) error {
		if err := tx.MTProtoSecrets().DeactivateAll(ctx, now); err != nil {
			return err
		}
		if err := tx.MTProtoSecrets().Create(ctx, record); err != nil {
			return err
		}
		if audit {
			return s.audit.Write(ctx, actor, "mtproto_secret_sync", "mtproto_secret", &record.ID, map[string]any{
				"name": record.Name,
				"path": record.SecretFilePath,
			})
		}
		return nil
	}); err != nil {
		return domain.MTProtoSecret{}, err
	}
	return record, nil
}
