package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"control-plane/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type RouterService interface {
	AddUser(context.Context, domain.Actor, domain.CreateUserParams) (domain.User, error)
	RevokeUser(context.Context, domain.Actor, string) (domain.User, error)
	ListUsers(context.Context, domain.Actor) ([]domain.User, error)
	UserLink(context.Context, domain.Actor, string) (string, error)
	UserProfile(context.Context, domain.Actor, string, string) (string, error)
	RenderXrayConfig(context.Context, domain.Actor) (domain.XrayRenderResult, error)
	ApplyXrayConfig(context.Context, domain.Actor) (domain.ApplyResult, error)
	RotateRealityKeyset(context.Context, domain.Actor) (domain.ApplyResult, error)
	MTProtoLink(context.Context, domain.Actor) (string, error)
	Health(context.Context) domain.HealthReport
	Ready(context.Context) domain.ReadinessReport
}

func NewRouter(app RouterService, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, app.Health(r.Context()))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		report := app.Ready(r.Context())
		status := http.StatusOK
		if report.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, report)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			users, err := app.ListUsers(r.Context(), actorFromRequest(r))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, users)
		})

		r.Post("/users", func(w http.ResponseWriter, r *http.Request) {
			var req domain.CreateUserParams
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать JSON"})
				return
			}
			user, err := app.AddUser(r.Context(), actorFromRequest(r), req)
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, user)
		})

		r.Post("/users/{ref}/revoke", func(w http.ResponseWriter, r *http.Request) {
			user, err := app.RevokeUser(r.Context(), actorFromRequest(r), chi.URLParam(r, "ref"))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, user)
		})

		r.Get("/users/{ref}/link", func(w http.ResponseWriter, r *http.Request) {
			link, err := app.UserLink(r.Context(), actorFromRequest(r), chi.URLParam(r, "ref"))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"link": link})
		})

		r.Get("/users/{ref}/profiles/{format}", func(w http.ResponseWriter, r *http.Request) {
			profile, err := app.UserProfile(r.Context(), actorFromRequest(r), chi.URLParam(r, "ref"), chi.URLParam(r, "format"))
			if err != nil {
				writeError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(profile))
		})

		r.Post("/xray/render", func(w http.ResponseWriter, r *http.Request) {
			rendered, err := app.RenderXrayConfig(r.Context(), actorFromRequest(r))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, rendered)
		})

		r.Post("/xray/apply", func(w http.ResponseWriter, r *http.Request) {
			result, err := app.ApplyXrayConfig(r.Context(), actorFromRequest(r))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})

		r.Post("/xray/keyset/rotate", func(w http.ResponseWriter, r *http.Request) {
			result, err := app.RotateRealityKeyset(r.Context(), actorFromRequest(r))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})

		r.Get("/mtproto", func(w http.ResponseWriter, r *http.Request) {
			link, err := app.MTProtoLink(r.Context(), actorFromRequest(r))
			if err != nil {
				writeError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"link": link})
		})
	})

	return r
}

func actorFromRequest(r *http.Request) domain.Actor {
	if actor := r.Header.Get("X-Admin-Actor"); actor != "" {
		return domain.Actor{Type: "http", ID: actor}
	}
	return domain.Actor{Type: "http", ID: r.RemoteAddr}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("HTTP запрос", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrAlreadyExists), errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
