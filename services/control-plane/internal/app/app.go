package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"control-plane/internal/config"
	"control-plane/internal/dockerctl"
	"control-plane/internal/httpapi"
	"control-plane/internal/mtproto"
	"control-plane/internal/service"
	postgresstore "control-plane/internal/storage/postgres"
	sqlitestore "control-plane/internal/storage/sqlite"
	"control-plane/internal/telegram"
	"control-plane/internal/xray"
)

type Runtime struct {
	Config   config.Config
	Logger   *slog.Logger
	Services *service.Facade
	HTTP     http.Handler
	Bot      *telegram.Bot
	store    service.Store
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Runtime, error) {
	store, err := openStore(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(ctx, cfg.MigrationsDir); err != nil {
		_ = store.Close()
		return nil, err
	}

	dockerClient := dockerctl.NewClient(cfg.DockerAPIBaseURL, cfg.DockerRestartTimeout, logger)
	xrayManager := xray.NewManager(cfg, logger)
	mtprotoManager := mtproto.NewManager(cfg)
	services := service.New(store, cfg, xrayManager, mtprotoManager, dockerClient, logger)
	if err := services.Bootstrap(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}

	return &Runtime{
		Config:   cfg,
		Logger:   logger,
		Services: services,
		HTTP:     httpapi.NewRouter(services, logger),
		Bot:      telegram.NewBot(cfg, services, logger),
		store:    store,
	}, nil
}

func (r *Runtime) Close() error {
	if r.store == nil {
		return nil
	}
	return r.store.Close()
}

func openStore(ctx context.Context, cfg config.Config, logger *slog.Logger) (service.Store, error) {
	switch cfg.StorageDriver {
	case "sqlite":
		return sqlitestore.New(ctx, cfg.SQLitePath, logger)
	case "postgres":
		return postgresstore.New(ctx, cfg.DatabaseURL, logger)
	default:
		return nil, fmt.Errorf("неподдерживаемый STORAGE_DRIVER=%s", cfg.StorageDriver)
	}
}
