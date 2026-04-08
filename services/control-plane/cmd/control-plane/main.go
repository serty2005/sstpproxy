package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"control-plane/internal/app"
	"control-plane/internal/config"
	"control-plane/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtime, err := app.New(ctx, cfg, log)
	if err != nil {
		log.Error("инициализация приложения завершилась ошибкой", "error", err)
		os.Exit(1)
	}
	defer runtime.Close()

	server := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           runtime.HTTP,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Go(func() {
		log.Info("HTTP сервер запущен", "addr", cfg.HTTPListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	})

	if cfg.TelegramBotToken != "" {
		wg.Go(func() {
			if err := runtime.Bot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- err
			}
		})
	} else {
		log.Info("Telegram-бот отключён", "reason", "пустой TELEGRAM_BOT_TOKEN")
	}

	select {
	case <-ctx.Done():
		log.Info("получен сигнал остановки")
	case err := <-errCh:
		log.Error("runtime ошибка", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("ошибка graceful shutdown HTTP сервера", "error", err)
	}
	wg.Wait()
}
