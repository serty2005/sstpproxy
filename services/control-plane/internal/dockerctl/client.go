package dockerctl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL        string
	restartTimeout time.Duration
	httpClient     *http.Client
	logger         *slog.Logger
}

type ContainerInfo struct {
	Name  string `json:"Name"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
		Health  struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
}

func NewClient(baseURL string, restartTimeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		restartTimeout: restartTimeout,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docker ping завершился статусом %s", resp.Status)
	}
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, name string) (ContainerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/containers/%s/json", c.baseURL, url.PathEscape(name)), nil)
	if err != nil {
		return ContainerInfo{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ContainerInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return ContainerInfo{}, fmt.Errorf("inspect контейнера завершился ошибкой %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var info ContainerInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return ContainerInfo{}, err
	}
	return info, nil
}

func (c *Client) RestartContainer(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/containers/%s/restart?t=%d", c.baseURL, url.PathEscape(name), int(c.restartTimeout.Seconds())),
		nil,
	)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restart контейнера завершился ошибкой %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	deadline := time.Now().Add(c.restartTimeout)
	for time.Now().Before(deadline) {
		info, err := c.InspectContainer(ctx, name)
		if err == nil && isReady(info) {
			c.logger.Info("контейнер перезапущен", "container", name, "state", info.State.Status)
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("контейнер %s не стал готов за %s", name, c.restartTimeout)
}

func isReady(info ContainerInfo) bool {
	if !info.State.Running {
		return false
	}
	return info.State.Health.Status == "" || info.State.Health.Status == "healthy"
}
