package mtproto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"control-plane/internal/config"
	"control-plane/internal/templatex"
)

type Manager struct {
	cfg       config.Config
	templates *templatex.Engine
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		templates: templatex.New(),
	}
}

func (m *Manager) LoadSecret() (string, string, error) {
	raw, err := os.ReadFile(m.cfg.MTProtoSecretFile)
	if err != nil {
		return "", "", fmt.Errorf("не удалось прочитать MTProto secret file: %w", err)
	}
	secret := strings.TrimSpace(string(raw))
	if secret == "" {
		return "", "", fmt.Errorf("MTProto secret file пуст")
	}
	sum := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(sum[:]), nil
}

func (m *Manager) Link(secret string) string {
	encodedSecret := secret
	if raw, err := hex.DecodeString(secret); err == nil {
		encodedSecret = base64.RawURLEncoding.EncodeToString(raw)
	}
	return "tg://proxy?server=" + url.QueryEscape(m.cfg.MTProtoHost()) +
		"&port=" + url.QueryEscape(strconv.Itoa(m.cfg.MTProtoPort)) +
		"&secret=" + url.QueryEscape(encodedSecret)
}

func (m *Manager) RenderConfig(secret string) (string, error) {
	publicIPv4 := ""
	if ip := net.ParseIP(m.cfg.MTProtoHost()); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			publicIPv4 = ipv4.String()
		}
	}
	return m.templates.RenderFile(m.cfg.MTProtoTemplate, struct {
		Secret     string
		Port       int
		PublicIPv4 string
	}{
		Secret:     secret,
		Port:       m.cfg.MTProtoPort,
		PublicIPv4: publicIPv4,
	})
}

func (m *Manager) WriteAtomic(path string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if errRetry := os.Rename(tmpPath, path); errRetry != nil {
			return errRetry
		}
	}
	return nil
}
