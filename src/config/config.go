package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// GitConfig groups Git-related settings.
type GitConfig struct {
	BinPath                       string `json:"binPath"`
	Remote                        string `json:"remote"`
	LocalDirectory                string `json:"localDirectory"`
	PullIntervalSec               int    `json:"pullIntervalSec"`
	PushIntervalSec               int    `json:"pushIntervalSec"`
	Author                        string `json:"author"`
	CommitMessagePrefix           string `json:"commitMessagePrefix"`
	CommitMessageAppendRemoteAddr string `json:"commitMessageAppendRemoteAddr"`
	pushIntervalDefined           bool   `json:"-"`
}

// Config encapsulates runtime and build-time options.
type Config struct {
	Live                   bool           `json:"live"`
	Editable               bool           `json:"editable"`
	Listen                 string         `json:"listen"`
	Git                    GitConfig      `json:"git"`
	WebhookEnabled         bool           `json:"webHook"`
	WebhookListen          string         `json:"webHookListen"`
	WebhookAuthPreShared   string         `json:"webHookAuthPreShared"`
	OutputDir              string         `json:"outputDir"`
	TemplateDir            string         `json:"templateDir"`
	HomeDoc                string         `json:"homeDoc"`
	BaseURL                string         `json:"baseUrl"`
	SiteName               string         `json:"siteName"`
	IgnoreHeader           bool           `json:"ignoreHeader"`
	IgnoreFooter           bool           `json:"ignoreFooter"`
	ServerFooter           string         `json:"serverFooter"`
	EnableTLS              bool           `json:"enableTLS"`
	TLSCert                string         `json:"tlsCert"`
	TLSKey                 string         `json:"tlsKey"`
	LogLevel               string         `json:"logLevel"`
	TrustedProxies         []string       `json:"trustedProxies"`
	TrustedRemoteAddrLevel int            `json:"trustedRemoteAddrLevel"`
	PrivatePagesPrefix     []string       `json:"privatePagesPrefix"`
	PullInterval           time.Duration  `json:"-"`
	PushInterval           time.Duration  `json:"-"`
	trustedProxyPrefixes   []netip.Prefix `json:"-"`
	privatePagePrefixes    []string       `json:"-"`
}

func (g *GitConfig) UnmarshalJSON(data []byte) error {
	type rawGitConfig struct {
		BinPath                       string `json:"binPath"`
		Remote                        string `json:"remote"`
		LocalDirectory                string `json:"localDirectory"`
		PullIntervalSec               int    `json:"pullIntervalSec"`
		PushIntervalSec               *int   `json:"pushIntervalSec"`
		Author                        string `json:"author"`
		CommitMessagePrefix           string `json:"commitMessagePrefix"`
		CommitMessageAppendRemoteAddr string `json:"commitMessageAppendRemoteAddr"`
	}

	var raw rawGitConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	g.BinPath = raw.BinPath
	g.Remote = raw.Remote
	g.LocalDirectory = raw.LocalDirectory
	g.PullIntervalSec = raw.PullIntervalSec
	g.Author = raw.Author
	g.CommitMessagePrefix = raw.CommitMessagePrefix
	g.CommitMessageAppendRemoteAddr = raw.CommitMessageAppendRemoteAddr
	g.pushIntervalDefined = raw.PushIntervalSec != nil
	if raw.PushIntervalSec != nil {
		g.PushIntervalSec = *raw.PushIntervalSec
	} else {
		g.PushIntervalSec = 0
	}
	return nil
}

// Load reads configuration from disk and applies sane defaults.
func Load(path string) (*Config, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(bytes, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyDefaults() error {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.WebhookListen == "" {
		c.WebhookListen = ":8081"
	}
	if c.OutputDir == "" {
		c.OutputDir = "./dist"
	}
	if c.TemplateDir == "" {
		c.TemplateDir = "./template"
	}
	c.WebhookAuthPreShared = strings.TrimSpace(c.WebhookAuthPreShared)
	c.HomeDoc = normalizeHomeDoc(c.HomeDoc)

	c.SiteName = strings.TrimSpace(c.SiteName)
	if c.SiteName == "" {
		c.SiteName = "DN42 Wiki Go"
	}

	c.Git.BinPath = strings.TrimSpace(c.Git.BinPath)
	c.Git.Remote = strings.TrimSpace(c.Git.Remote)
	c.Git.LocalDirectory = strings.TrimSpace(c.Git.LocalDirectory)

	if c.Git.BinPath == "" {
		c.Git.BinPath = "git"
	}
	if c.Git.LocalDirectory == "" {
		c.Git.LocalDirectory = "./repo"
	}
	if c.Git.PullIntervalSec <= 0 {
		c.Git.PullIntervalSec = 300
	}
	if !c.Git.pushIntervalDefined {
		c.Git.PushIntervalSec = 0
	}
	if c.Git.PushIntervalSec < 0 {
		c.Git.PushIntervalSec = 0
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.TrustedRemoteAddrLevel <= 0 {
		c.TrustedRemoteAddrLevel = 1
	}

	c.Git.Author = strings.TrimSpace(c.Git.Author)
	if c.Git.Author == "" {
		c.Git.Author = "Anonymous <anonymous@localhost>"
	}

	if err := c.compileTrustedProxies(); err != nil {
		return err
	}
	if err := c.compilePrivatePages(); err != nil {
		return err
	}

	c.PullInterval = time.Duration(c.Git.PullIntervalSec) * time.Second
	if c.Git.Remote == "" {
		c.PullInterval = 0
	}
	if c.Git.PushIntervalSec > 0 {
		c.PushInterval = time.Duration(c.Git.PushIntervalSec) * time.Second
	} else {
		c.PushInterval = 0
	}
	if c.Git.Remote == "" {
		c.PushInterval = 0
	}
	return nil
}

func (c *Config) validate() error {
	if c.PullInterval < 0 {
		return fmt.Errorf("negative pull interval")
	}
	if c.PushInterval < 0 {
		return fmt.Errorf("negative push interval")
	}
	if c.EnableTLS {
		if c.TLSCert == "" || c.TLSKey == "" {
			return fmt.Errorf("tls enabled but certificates missing")
		}
	}
	return nil
}

func (c *Config) IsPathPrivate(route string) bool {
	if len(c.privatePagePrefixes) == 0 {
		return false
	}
	normalized, err := normalizeRoute(route)
	if err != nil {
		return false
	}
	if normalized == "" {
		normalized = "/"
	}
	for _, prefix := range c.privatePagePrefixes {
		if prefix == "/" {
			return true
		}
		if normalized == prefix || strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
}

func (c *Config) compilePrivatePages() error {
	if c.privatePagePrefixes != nil {
		c.privatePagePrefixes = c.privatePagePrefixes[:0]
	}
	seen := map[string]struct{}{}
	for _, raw := range c.PrivatePagesPrefix {
		norm, err := normalizeRoute(raw)
		if err != nil {
			return fmt.Errorf("invalid private route prefix %q: %w", raw, err)
		}
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		c.privatePagePrefixes = append(c.privatePagePrefixes, norm)
	}
	return nil
}

func (c *Config) compileTrustedProxies() error {
	if c.trustedProxyPrefixes != nil {
		c.trustedProxyPrefixes = c.trustedProxyPrefixes[:0]
	}
	for _, entry := range c.TrustedProxies {
		token := strings.TrimSpace(entry)
		if token == "" {
			continue
		}
		if strings.Contains(token, "/") {
			prefix, err := netip.ParsePrefix(token)
			if err != nil {
				return fmt.Errorf("invalid trusted proxy %q: %w", entry, err)
			}
			c.trustedProxyPrefixes = append(c.trustedProxyPrefixes, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(token)
		if err != nil {
			return fmt.Errorf("invalid trusted proxy %q: %w", entry, err)
		}
		var prefix netip.Prefix
		if addr.Is4() {
			prefix = netip.PrefixFrom(addr, 32)
		} else {
			prefix = netip.PrefixFrom(addr, 128)
		}
		c.trustedProxyPrefixes = append(c.trustedProxyPrefixes, prefix)
	}
	return nil
}

func normalizeRoute(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		cleaned = "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if strings.HasPrefix(cleaned, "/..") || strings.Contains(cleaned, "/../") {
		return "", fmt.Errorf("path escapes root")
	}
	if cleaned != "/" {
		cleaned = strings.TrimSuffix(cleaned, "/")
		if cleaned == "" {
			cleaned = "/"
		}
	}
	return cleaned, nil
}

func normalizeHomeDoc(input string) string {
	trimmed := strings.TrimSpace(input)
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if trimmed == "" {
		trimmed = "Home.md"
	}
	if !strings.HasSuffix(strings.ToLower(trimmed), ".md") {
		trimmed += ".md"
	}
	cleaned := path.Clean(trimmed)
	for strings.HasPrefix(cleaned, "./") {
		cleaned = strings.TrimPrefix(cleaned, "./")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		cleaned = "Home.md"
	}
	return filepath.ToSlash(cleaned)
}

// IsTrustedProxy reports whether the provided address is within the trusted proxy list.
func (c *Config) IsTrustedProxy(addr netip.Addr) bool {
	for _, prefix := range c.trustedProxyPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// RemoteAddrFromRequest attempts to determine the originating client address.
// It inspects X-Forwarded-For headers and falls back to the direct remote
// address when no trusted proxy information is available.
func (c *Config) RemoteAddrFromRequest(r *http.Request) (netip.Addr, []netip.Addr) {
	chain := c.remoteAddrChain(r)
	if len(chain) == 0 {
		return netip.Addr{}, nil
	}

	allowed := max(c.TrustedRemoteAddrLevel, 0)

	idx := len(chain) - 1
	for idx > 0 {
		current := chain[idx]
		if !c.IsTrustedProxy(current) {
			break
		}
		if allowed == 0 {
			break
		}
		idx--
		allowed--
	}

	return chain[idx], chain
}

func (c *Config) remoteAddrChain(r *http.Request) []netip.Addr {
	chain := make([]netip.Addr, 0, 4)

	header := r.Header.Get("X-Forwarded-For")
	if header != "" {
		parts := strings.SplitSeq(header, ",")
		for raw := range parts {
			token := strings.TrimSpace(raw)
			if token == "" {
				continue
			}
			if addr, err := netip.ParseAddr(token); err == nil {
				chain = append(chain, addr)
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if addr, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
		chain = append(chain, addr)
	}
	return chain
}
