package xray

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"control-plane/internal/config"
	"control-plane/internal/templatex"
)

type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

type ServerRenderInput struct {
	LogLevel          string
	Port              int
	RealityDest       string
	ServerNamesJSON   string
	RealityPrivateKey string
	ShortIDsJSON      string
	ClientsJSON       string
}

type ClientProfileData struct {
	Label       string
	Host        string
	Port        int
	UUID        string
	ServerName  string
	Fingerprint string
	PublicKey   string
	ShortID     string
	Flow        string
	URI         string
}

type Manager struct {
	cfg       config.Config
	templates *templatex.Engine
	logger    *slog.Logger
}

func NewManager(cfg config.Config, logger *slog.Logger) *Manager {
	return &Manager{
		cfg:       cfg,
		templates: templatex.New(),
		logger:    logger,
	}
}

func (m *Manager) GenerateKeyPair(ctx context.Context) (KeyPair, error) {
	cmd := exec.CommandContext(ctx, m.cfg.XrayBinaryPath, "x25519")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return KeyPair{}, fmt.Errorf("не удалось выполнить xray x25519: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var pair KeyPair
	for line := range strings.Lines(string(output)) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		value := extractValue(line)
		switch {
		case strings.Contains(lower, "private"):
			pair.PrivateKey = value
		case strings.Contains(lower, "public"), strings.Contains(lower, "password"):
			pair.PublicKey = value
		}
	}

	if pair.PrivateKey == "" || pair.PublicKey == "" {
		return KeyPair{}, fmt.Errorf("xray x25519 вернул неожиданный вывод: %s", strings.TrimSpace(string(output)))
	}
	return pair, nil
}

func (m *Manager) RenderServerConfig(input ServerRenderInput) (string, error) {
	rendered, err := m.templates.RenderFile(m.cfg.XrayServerTemplate, input)
	if err != nil {
		return "", err
	}
	if !json.Valid([]byte(rendered)) {
		return "", fmt.Errorf("сгенерирован невалидный JSON Xray")
	}
	return rendered, nil
}

func (m *Manager) RenderProfile(format string, data ClientProfileData) (string, error) {
	path, err := m.templatePath(format)
	if err != nil {
		return "", err
	}
	return m.templates.RenderFile(path, data)
}

func (m *Manager) BuildURI(data ClientProfileData) string {
	return fmt.Sprintf(
		"vless://%s@%s:%d?type=tcp&security=reality&pbk=%s&fp=%s&sni=%s&sid=%s&flow=%s&encryption=none#%s",
		data.UUID,
		data.Host,
		data.Port,
		url.QueryEscape(data.PublicKey),
		url.QueryEscape(data.Fingerprint),
		url.QueryEscape(data.ServerName),
		url.QueryEscape(data.ShortID),
		url.QueryEscape(data.Flow),
		url.PathEscape(data.Label),
	)
}

func (m *Manager) ValidateConfig(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, m.cfg.XrayBinaryPath, "run", "-test", "-config", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("валидация Xray завершилась ошибкой: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *Manager) WriteAtomic(path string, content []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(path)
		if errRetry := os.Rename(tmpPath, path); errRetry != nil {
			return errRetry
		}
	}
	m.logger.Debug("файл Xray записан", "path", path)
	return nil
}

func SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func MarshalJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (m *Manager) templatePath(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "nekoray":
		return m.cfg.XrayNekorayTemplate, nil
	case "hiddify":
		return m.cfg.XrayHiddifyTemplate, nil
	case "v2rayn":
		return m.cfg.XrayV2RayNTemplate, nil
	default:
		return "", fmt.Errorf("неподдерживаемый формат профиля %q", format)
	}
}

func extractValue(line string) string {
	for _, sep := range []string{":", "="} {
		if before, after, ok := strings.Cut(line, sep); ok && strings.TrimSpace(before) != "" {
			return strings.TrimSpace(after)
		}
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
