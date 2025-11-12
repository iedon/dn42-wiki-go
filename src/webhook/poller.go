package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/iedon/dn42-wiki-go/config"
	"github.com/iedon/dn42-wiki-go/site"
)

// Poller keeps a registration fresh with the remote notification service
// and triggers local updates when the upstream indicates changes.
type Poller struct {
	cfg       *config.Config
	svc       *site.Service
	logger    *slog.Logger
	client    *http.Client
	pollURL   string
	userAgent string
}

// NewPoller constructs a polling manager when webhook polling is enabled.
func NewPoller(cfg *config.Config, svc *site.Service, logger *slog.Logger, userAgent string) (*Poller, error) {
	if cfg == nil || svc == nil {
		return nil, fmt.Errorf("missing configuration or service instance")
	}
	if !cfg.Webhook.Enabled || !cfg.Webhook.Polling.Enabled {
		return nil, fmt.Errorf("webhook polling is disabled")
	}

	interval := cfg.Webhook.Polling.Interval()
	if interval <= 0 {
		return nil, fmt.Errorf("invalid polling interval")
	}

	client := &http.Client{Timeout: 20 * time.Second}

	return &Poller{
		cfg:       cfg,
		svc:       svc,
		logger:    logger,
		client:    client,
		pollURL:   cfg.Webhook.Polling.Endpoint,
		userAgent: userAgent,
	}, nil
}

// Run starts the background refresh loop until the context is cancelled.
func (p *Poller) Run(ctx context.Context) {
	interval := p.cfg.Webhook.Polling.Interval()
	p.execute(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.execute(ctx)
		}
	}
}

func (p *Poller) execute(ctx context.Context) {
	if err := p.refreshRegistration(ctx); err != nil {
		p.logger.Warn("webhook poll", "error", err)
		return
	}
	if err := p.svc.Pull(ctx); err != nil {
		p.logger.Warn("webhook poll pull", "error", err)
	}
}

func (p *Poller) refreshRegistration(ctx context.Context) error {
	repo := p.cfg.Git.RepositoryPath()
	if repo == "" {
		return fmt.Errorf("repository path unavailable")
	}
	body := pollRequest{
		Webhook: p.cfg.Webhook.Polling.CallbackURL,
		Repos:   []string{repo},
		Ping:    true,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal poll body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.pollURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("construct poll request: %w", err)
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", p.cfg.Webhook.Secret)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("poll request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("poll request failed: %s (%s)", resp.Status, strings.TrimSpace(string(data)))
	}

	// Drain the body to allow connection reuse. The payload is informational only.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

type pollRequest struct {
	Webhook string   `json:"webhook"`
	Repos   []string `json:"repos"`
	Ping    bool     `json:"ping,omitempty"`
}
