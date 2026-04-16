package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"control-plane/internal/config"
	"control-plane/internal/domain"
)

type Service interface {
	AddUser(context.Context, domain.Actor, domain.CreateUserParams) (domain.User, error)
	RevokeUser(context.Context, domain.Actor, string) (domain.User, error)
	ListUsers(context.Context, domain.Actor) ([]domain.User, error)
	UserLink(context.Context, domain.Actor, string) (string, error)
	UserProfile(context.Context, domain.Actor, string, string) (string, error)
	RenderXrayConfig(context.Context, domain.Actor) (domain.XrayRenderResult, error)
	ApplyXrayConfig(context.Context, domain.Actor) (domain.ApplyResult, error)
	MTProtoLink(context.Context, domain.Actor) (string, error)
	Health(context.Context) domain.HealthReport
	Ready(context.Context) domain.ReadinessReport
}

type Bot struct {
	cfg        config.Config
	service    Service
	logger     *slog.Logger
	httpClient *http.Client
}

type updatesResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

type update struct {
	UpdateID int     `json:"update_id"`
	Message  message `json:"message"`
}

type message struct {
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From struct {
		ID int64 `json:"id"`
	} `json:"from"`
	Text string `json:"text"`
}

func NewBot(cfg config.Config, service Service, logger *slog.Logger) *Bot {
	httpClient := &http.Client{
		Timeout: 40 * time.Second,
	}
	if cfg.TelegramProxyURL != "" {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		proxyURL, err := url.Parse(cfg.TelegramProxyURL)
		if err != nil {
			logger.Error("некорректный TELEGRAM_PROXY_URL, использую прямое подключение", "error", err, "url", cfg.TelegramProxyURL)
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
			httpClient.Transport = transport
		}
	}

	return &Bot{
		cfg:        cfg,
		service:    service,
		logger:     logger,
		httpClient: httpClient,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			b.logger.Error("ошибка Telegram getUpdates", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1
			if strings.TrimSpace(upd.Message.Text) == "" {
				continue
			}
			b.handleMessage(ctx, upd.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg message) {
	actor := domain.Actor{Type: "telegram", ID: fmt.Sprintf("%d", msg.From.ID)}
	if !b.isAdmin(msg.From.ID) {
		_ = b.sendMessage(ctx, msg.Chat.ID, "Доступ запрещён.")
		return
	}

	command, args := splitCommand(msg.Text)
	switch command {
	case "/start", "/help":
		_ = b.sendMessage(ctx, msg.Chat.ID, helpText())
	case "/users":
		users, err := b.service.ListUsers(ctx, actor)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось получить список пользователей.")
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, formatUsers(users))
	case "/user_add":
		displayName := strings.TrimSpace(args)
		if displayName == "" {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Использование: /user_add <display_name>")
			return
		}
		user, err := b.service.AddUser(ctx, actor, domain.CreateUserParams{DisplayName: displayName})
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось создать пользователя: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, fmt.Sprintf("Пользователь создан.\nID: %s\nUUID: %s\nshortId: %s", user.ID, user.UUID, valueOrDash(user.RealityShortID)))
	case "/user_revoke":
		user, err := b.service.RevokeUser(ctx, actor, args)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось деактивировать пользователя: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, fmt.Sprintf("Пользователь деактивирован: %s", user.DisplayName))
	case "/user_link":
		link, err := b.service.UserLink(ctx, actor, args)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось собрать ссылку: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, trimMessage(link))
	case "/user_profile":
		parts := strings.Fields(args)
		if len(parts) != 2 {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Использование: /user_profile <user_id_or_uuid> <nekoray|hiddify|v2rayn>")
			return
		}
		profile, err := b.service.UserProfile(ctx, actor, parts[0], parts[1])
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось собрать профиль: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, trimMessage(profile))
	case "/xray_render":
		rendered, err := b.service.RenderXrayConfig(ctx, actor)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось отрендерить Xray config: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, trimMessage(rendered.Config))
	case "/xray_apply":
		result, err := b.service.ApplyXrayConfig(ctx, actor)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось применить Xray config: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, fmt.Sprintf("Xray config применён.\nВерсия: %d\nSHA256: %s", result.Record.Version, result.Record.SHA256))
	case "/mtproto":
		link, err := b.service.MTProtoLink(ctx, actor)
		if err != nil {
			_ = b.sendMessage(ctx, msg.Chat.ID, "Не удалось собрать MTProto ссылку: "+err.Error())
			return
		}
		_ = b.sendMessage(ctx, msg.Chat.ID, link)
	case "/health":
		report := b.service.Ready(ctx)
		_ = b.sendMessage(ctx, msg.Chat.ID, fmt.Sprintf(
			"status=%s\nstorage=%s\ndocker=%s\nxray=%s\nmtproto=%s",
			report.Status,
			report.Storage,
			report.Docker,
			report.XrayConfig,
			report.MTProtoConfig,
		))
	default:
		_ = b.sendMessage(ctx, msg.Chat.ID, "Неизвестная команда. Используйте /help.")
	}
}

func (b *Bot) getUpdates(ctx context.Context, offset int) ([]update, error) {
	payload := map[string]any{
		"offset":          offset,
		"timeout":         30,
		"allowed_updates": []string{"message"},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiURL("/getUpdates"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var decoded updatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded.Result, nil
}

func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.apiURL("/sendMessage"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (b *Bot) apiURL(method string) string {
	return fmt.Sprintf("%s/bot%s%s", b.cfg.TelegramAPIBaseURL, b.cfg.TelegramBotToken, method)
}

func (b *Bot) isAdmin(id int64) bool {
	_, ok := b.cfg.TelegramAdminIDs[id]
	return ok
}

func splitCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	parts := strings.SplitN(text, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.TrimSpace(parts[1])
}

func helpText() string {
	return strings.Join([]string{
		"/start",
		"/help",
		"/users",
		"/user_add <display_name>",
		"/user_revoke <user_id_or_uuid>",
		"/user_link <user_id_or_uuid>",
		"/user_profile <user_id_or_uuid> <nekoray|hiddify|v2rayn>",
		"/xray_render",
		"/xray_apply",
		"/mtproto",
		"/health",
	}, "\n")
}

func formatUsers(users []domain.User) string {
	if len(users) == 0 {
		return "Пользователей пока нет."
	}
	lines := make([]string, 0, len(users))
	for _, user := range users {
		state := "active"
		if !user.IsActive {
			state = "revoked"
		}
		lines = append(lines, fmt.Sprintf(
			"%s | %s | %s | sid=%s | %s",
			user.ID,
			user.DisplayName,
			user.UUID,
			valueOrDash(user.RealityShortID),
			state,
		))
	}
	return strings.Join(lines, "\n")
}

func trimMessage(text string) string {
	const max = 3500
	if len(text) <= max {
		return text
	}
	return text[:max] + "\n...обрезано..."
}

func valueOrDash(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}
	return *value
}
