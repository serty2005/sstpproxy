package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"control-plane/internal/domain"
	"control-plane/internal/xray"

	"github.com/google/uuid"
)

func (s *UserService) Add(ctx context.Context, actor domain.Actor, params domain.CreateUserParams) (domain.User, error) {
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return domain.User{}, fmt.Errorf("%w: display_name обязателен", domain.ErrValidation)
	}

	keyset, err := s.keys.EnsureActive(ctx)
	if err != nil {
		return domain.User{}, err
	}

	baseUsername := normalizeUsername(params.Username)
	if baseUsername == "" {
		baseUsername = normalizeUsername(displayName)
	}

	var created domain.User
	for range 5 {
		username := baseUsername
		if username == "" {
			username = generateUsername("user")
		}
		if params.Username == "" || baseUsername == "" {
			username = generateUsername(username)
		}

		now := time.Now().UTC()
		user := domain.User{
			ID:          uuid.NewString(),
			Username:    username,
			DisplayName: displayName,
			UUID:        uuid.NewString(),
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if comment := strings.TrimSpace(params.Comment); comment != "" {
			user.Comment = &comment
		}
		user.TrafficLimitBytes = params.TrafficLimitBytes

		err = s.store.InTx(ctx, func(tx domain.TxStore) error {
			if err := tx.Users().Create(ctx, user); err != nil {
				return err
			}
			shortID, err := s.shortIDs.Allocate(ctx, tx, keyset.ID, user.ID, now)
			if err != nil {
				return err
			}
			if err := tx.Users().UpdateRealityShortID(ctx, user.ID, &shortID.ID, now); err != nil {
				return err
			}
			user.RealityShortIDID = &shortID.ID
			user.RealityShortID = &shortID.ShortID
			return s.audit.Write(ctx, actor, "user_add", "user", &user.ID, map[string]any{
				"username":  user.Username,
				"display":   user.DisplayName,
				"keyset_id": keyset.ID,
				"short_id":  shortID.ShortID,
				"user_uuid": user.UUID,
				"is_active": true,
			})
		})
		if err == nil {
			created = user
			break
		}
		if err != domain.ErrAlreadyExists {
			return domain.User{}, err
		}
	}

	if created.ID == "" {
		return domain.User{}, fmt.Errorf("не удалось создать пользователя после нескольких попыток")
	}
	return created, nil
}

func (s *UserService) Revoke(ctx context.Context, actor domain.Actor, ref string) (domain.User, error) {
	user, err := lookupUser(ctx, s.store, ref)
	if err != nil {
		return domain.User{}, err
	}
	if !user.IsActive {
		return user, nil
	}

	now := time.Now().UTC()
	if err := s.store.InTx(ctx, func(tx domain.TxStore) error {
		if err := tx.RealityShortIDs().ReleaseByUser(ctx, user.ID); err != nil {
			return err
		}
		if err := tx.Users().UpdateRealityShortID(ctx, user.ID, nil, now); err != nil {
			return err
		}
		if err := tx.Users().Revoke(ctx, user.ID, now); err != nil {
			return err
		}
		return s.audit.Write(ctx, actor, "user_revoke", "user", &user.ID, map[string]any{
			"username": user.Username,
			"uuid":     user.UUID,
		})
	}); err != nil {
		return domain.User{}, err
	}

	user.IsActive = false
	user.RealityShortIDID = nil
	user.RealityShortID = nil
	user.RevokedAt = &now
	user.UpdatedAt = now
	return user, nil
}

func (s *UserService) List(ctx context.Context, actor domain.Actor) ([]domain.User, error) {
	users, err := s.store.Users().List(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Write(ctx, actor, "user_list", "user", nil, map[string]any{"count": len(users)})
	return users, nil
}

func (s *ProfileService) Link(ctx context.Context, actor domain.Actor, ref string) (string, error) {
	data, subjectID, err := s.profileData(ctx, ref)
	if err != nil {
		return "", err
	}
	link := s.xray.BuildURI(data)
	if err := s.audit.Write(ctx, actor, "user_link", "user", subjectID, map[string]any{"format": "uri"}); err != nil {
		return "", err
	}
	return link, nil
}

func (s *ProfileService) Profile(ctx context.Context, actor domain.Actor, ref, format string) (string, error) {
	data, subjectID, err := s.profileData(ctx, ref)
	if err != nil {
		return "", err
	}
	profile, err := s.xray.RenderProfile(format, data)
	if err != nil {
		return "", err
	}
	if err := s.audit.Write(ctx, actor, "user_profile", "user", subjectID, map[string]any{"format": format}); err != nil {
		return "", err
	}
	return profile, nil
}

func (s *ProfileService) profileData(ctx context.Context, ref string) (xray.ClientProfileData, *string, error) {
	user, err := lookupUser(ctx, s.store, ref)
	if err != nil {
		return xray.ClientProfileData{}, nil, err
	}
	if !user.IsActive {
		return xray.ClientProfileData{}, nil, fmt.Errorf("%w: пользователь деактивирован", domain.ErrValidation)
	}
	if user.RealityShortID == nil {
		return xray.ClientProfileData{}, nil, fmt.Errorf("%w: пользователю не назначен shortId", domain.ErrValidation)
	}

	keyset, err := s.keys.EnsureActive(ctx)
	if err != nil {
		return xray.ClientProfileData{}, nil, err
	}

	data := xray.ClientProfileData{
		Label:       profileLabel(user),
		Host:        s.cfg.PublicHost,
		Port:        s.cfg.XrayPort,
		UUID:        user.UUID,
		ServerName:  s.cfg.PrimaryServerName(),
		Fingerprint: s.cfg.XrayClientFingerprint,
		PublicKey:   keyset.PublicKey,
		ShortID:     *user.RealityShortID,
		Flow:        s.cfg.XrayFlow,
	}
	data.URI = s.xray.BuildURI(data)
	return data, &user.ID, nil
}

func normalizeUsername(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "@"))
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastDash = false
		case ch == '-' || ch == '_' || ch == '.':
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return ""
	}
	return result
}

func generateUsername(base string) string {
	base = normalizeUsername(base)
	if base == "" {
		base = "user"
	}
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("%s-%s", base, hex.EncodeToString(suffix))
}
