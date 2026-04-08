package config

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv             string
	LogLevel           string
	HTTPListenAddr     string
	StorageDriver      string
	DatabaseURL        string
	SQLitePath         string
	MigrationsDir      string
	TelegramAdminIDs   map[int64]struct{}
	TelegramBotToken   string
	TelegramAPIBaseURL string

	PublicHost string

	XrayPort               int
	XrayFlow               string
	XrayClientFingerprint  string
	XrayRealityDest        string
	XrayRealityServerNames []string
	XrayServerTemplate     string
	XrayNekorayTemplate    string
	XrayHiddifyTemplate    string
	XrayV2RayNTemplate     string
	XrayActiveConfigPath   string
	XrayArchiveDir         string
	XrayBinaryPath         string
	XrayContainerName      string

	RealityKeysetName      string
	RealitySecretDir       string
	RealityPrivateKeyFile  string
	RealityShortIDPoolSize int
	RealityShortIDBytes    int

	MTProtoTemplate     string
	MTProtoActiveConfig string
	MTProtoArchiveDir   string
	MTProtoPort         int
	MTProtoPublicHost   string
	MTProtoSecretFile   string
	MTProtoSecretName   string
	MTProtoContainer    string

	DockerAPIBaseURL     string
	DockerRestartTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		HTTPListenAddr:        getEnv("HTTP_LISTEN_ADDR", ":8080"),
		StorageDriver:         strings.ToLower(getEnv("STORAGE_DRIVER", "sqlite")),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		SQLitePath:            getEnv("SQLITE_PATH", filepath.Clean("control-plane.db")),
		MigrationsDir:         getEnv("MIGRATIONS_DIR", "./migrations"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramAPIBaseURL:    strings.TrimRight(getEnv("TELEGRAM_API_BASE_URL", "https://api.telegram.org"), "/"),
		PublicHost:            strings.TrimSpace(os.Getenv("PUBLIC_HOST")),
		XrayFlow:              getEnv("XRAY_FLOW", "xtls-rprx-vision"),
		XrayClientFingerprint: getEnv("XRAY_CLIENT_FP", "chrome"),
		XrayRealityDest:       strings.TrimSpace(os.Getenv("XRAY_REALITY_DEST")),
		XrayServerTemplate:    getEnv("XRAY_SERVER_TEMPLATE", "infra/xray/templates/config.json.tmpl"),
		XrayNekorayTemplate:   getEnv("XRAY_NEKORAY_TEMPLATE", "infra/xray/templates/client/nekoray.json.tmpl"),
		XrayHiddifyTemplate:   getEnv("XRAY_HIDDIFY_TEMPLATE", "infra/xray/templates/client/hiddify.json.tmpl"),
		XrayV2RayNTemplate:    getEnv("XRAY_V2RAYN_TEMPLATE", "infra/xray/templates/client/v2rayn.json.tmpl"),
		XrayActiveConfigPath:  getEnv("XRAY_ACTIVE_CONFIG_PATH", "infra/xray/generated/config.json"),
		XrayArchiveDir:        getEnv("XRAY_ARCHIVE_DIR", "infra/xray/generated/history"),
		XrayBinaryPath:        getEnv("XRAY_BINARY_PATH", "xray"),
		XrayContainerName:     getEnv("XRAY_CONTAINER_NAME", "xray-edge"),
		RealityKeysetName:     getEnv("REALITY_KEYSET_NAME", "primary"),
		RealitySecretDir:      getEnv("REALITY_SECRET_DIR", "deploy/secrets/reality"),
		RealityPrivateKeyFile: getEnv("REALITY_PRIVATE_KEY_FILE_NAME", "active.key"),
		MTProtoTemplate:       getEnv("MTPROTO_TEMPLATE", "infra/mtg/mtg.toml.tmpl"),
		MTProtoActiveConfig:   getEnv("MTPROTO_ACTIVE_CONFIG_PATH", "infra/mtg/generated/mtg.toml"),
		MTProtoArchiveDir:     getEnv("MTPROTO_ARCHIVE_DIR", "infra/mtg/generated/history"),
		MTProtoPublicHost:     strings.TrimSpace(os.Getenv("MTPROTO_PUBLIC_HOST")),
		MTProtoSecretFile:     getEnv("MTPROTO_SECRET_FILE", "deploy/secrets/mtproto/secret"),
		MTProtoSecretName:     getEnv("MTPROTO_SECRET_NAME", "primary"),
		MTProtoContainer:      getEnv("MTPROTO_CONTAINER_NAME", "mtg-edge"),
		DockerAPIBaseURL:      strings.TrimRight(getEnv("DOCKER_API_BASE_URL", "http://localhost:2375"), "/"),
	}

	var err error
	cfg.XrayPort, err = getEnvInt("XRAY_PORT", 4443)
	if err != nil {
		return Config{}, err
	}
	cfg.MTProtoPort, err = getEnvInt("MTPROTO_PORT", 4430)
	if err != nil {
		cfg.MTProtoPort, err = getEnvInt("MTG_PORT", 4430)
		if err != nil {
			return Config{}, err
		}
	}
	timeoutSeconds, err := getEnvInt("DOCKER_RESTART_TIMEOUT_SECONDS", 20)
	if err != nil {
		return Config{}, err
	}
	cfg.DockerRestartTimeout = time.Duration(timeoutSeconds) * time.Second

	cfg.RealityShortIDPoolSize, err = getEnvInt("REALITY_SHORT_ID_POOL_SIZE", 64)
	if err != nil {
		return Config{}, err
	}
	cfg.RealityShortIDBytes, err = getEnvInt("REALITY_SHORT_ID_BYTES", 8)
	if err != nil {
		return Config{}, err
	}

	admins, err := parseAdminWhitelist(os.Getenv("TELEGRAM_ADMIN_IDS"))
	if err != nil {
		return Config{}, err
	}
	cfg.TelegramAdminIDs = admins
	cfg.XrayRealityServerNames = splitList(os.Getenv("XRAY_REALITY_SERVER_NAMES"))
	if len(cfg.XrayRealityServerNames) == 0 {
		if legacy := strings.TrimSpace(os.Getenv("XRAY_REALITY_SERVER_NAME")); legacy != "" {
			cfg.XrayRealityServerNames = []string{legacy}
		}
	}

	if cfg.StorageDriver == "postgres" && cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL обязателен при STORAGE_DRIVER=postgres")
	}
	if cfg.StorageDriver == "sqlite" && cfg.SQLitePath == "" {
		return Config{}, errors.New("SQLITE_PATH обязателен при STORAGE_DRIVER=sqlite")
	}
	if cfg.PublicHost == "" {
		return Config{}, errors.New("PUBLIC_HOST обязателен")
	}
	if cfg.XrayRealityDest == "" {
		return Config{}, errors.New("XRAY_REALITY_DEST обязателен")
	}
	if len(cfg.XrayRealityServerNames) == 0 {
		return Config{}, errors.New("XRAY_REALITY_SERVER_NAMES обязателен")
	}
	if cfg.RealityShortIDPoolSize < 1 {
		return Config{}, errors.New("REALITY_SHORT_ID_POOL_SIZE должен быть больше 0")
	}
	if cfg.RealityShortIDBytes < 1 || cfg.RealityShortIDBytes > 8 {
		return Config{}, errors.New("REALITY_SHORT_ID_BYTES должен быть в диапазоне 1..8")
	}
	if cfg.MTProtoSecretFile == "" {
		return Config{}, errors.New("MTPROTO_SECRET_FILE обязателен")
	}

	return cfg, nil
}

func (c Config) PrimaryServerName() string {
	return c.XrayRealityServerNames[0]
}

func (c Config) MTProtoHost() string {
	return cmp.Or(c.MTProtoPublicHost, c.PublicHost)
}

func (c Config) RealityPrivateKeyPath(fileName string) string {
	return filepath.Join(c.RealitySecretDir, fileName)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New(key + " must be an integer")
	}
	return value, nil
}

func parseAdminWhitelist(raw string) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, errors.New("TELEGRAM_ADMIN_IDS должен содержать только целые Telegram user ID")
		}
		result[value] = struct{}{}
	}
	return result, nil
}

func splitList(raw string) []string {
	var result []string
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}
