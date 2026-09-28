// Package config loads, defaults, and validates the htb-presence configuration
// file.
//
// Nothing outside this package reads the raw config file: every other component
// receives a resolved, typed Config.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Configuration defaults and validation bounds.
const (
	// DefaultPollInterval is the poll cadence used when the config file does
	// not set one. It is deliberately conservative because HTB publishes no
	// rate-limit contract for the internal API (requirements.md, NFR-5).
	DefaultPollInterval = 30 * time.Second

	// MinPollInterval is the lowest poll cadence validation accepts.
	MinPollInterval = 15 * time.Second
)

// Config is the fully-resolved application configuration.
type Config struct {
	HTB     HTB     `yaml:"htb"`
	Discord Discord `yaml:"discord"`
	History History `yaml:"history"`
}

// History holds the optional local session-history settings.
type History struct {
	// File is the path of a JSONL session log. Empty disables history.
	File string `yaml:"file"`
}

// HTB holds the Hack The Box API settings.
type HTB struct {
	APIToken     string   `yaml:"api_token"`
	PollInterval Duration `yaml:"poll_interval"`

	// VPNFallback shows an "On the VPN" presence when nothing is spawned but
	// the HTB VPN is up. It defaults to true.
	VPNFallback bool `yaml:"vpn_fallback"`
}

// Discord holds the Discord Rich Presence settings.
type Discord struct {
	ClientID string `yaml:"client_id"`

	// Privacy toggles. ShowMachineName, ShowRank, ShowPoints, ShowTimer and
	// ShowButtons default to true. ShowFlags and ClearWhenIdle default to false.
	ShowMachineName bool   `yaml:"show_machine_name"`
	ShowRank        bool   `yaml:"show_rank"`
	ShowPoints      bool   `yaml:"show_points"`
	ShowTimer       bool   `yaml:"show_timer"`
	ShowFlags       bool   `yaml:"show_flags"`
	ShowButtons     bool   `yaml:"show_buttons"`
	ClearWhenIdle   bool   `yaml:"clear_when_idle"`
	IdleText        string `yaml:"idle_text"`
}

// Duration is a time.Duration that unmarshals from a Go duration string such as
// "30s" rather than a raw nanosecond count.
type Duration time.Duration

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return fmt.Errorf("expected a duration string such as \"30s\"")
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// Default returns a configuration with defaults applied, before any file values
// are layered on top.
func Default() Config {
	return Config{
		HTB: HTB{
			PollInterval: Duration(DefaultPollInterval),
			VPNFallback:  true,
		},
		Discord: Discord{
			ShowMachineName: true,
			ShowRank:        true,
			ShowPoints:      true,
			ShowTimer:       true,
			ShowButtons:     true,
			IdleText:        "Browsing…",
		},
	}
}

// Load reads, defaults, and validates the configuration file at path.
//
// An invalid config yields an error rather than a partially-applied one, so
// startup fails fast with a clear message.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	cfg.applyEnv()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// DefaultPath returns the OS-specific path of the default config file.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating user config dir: %w", err)
	}
	return filepath.Join(dir, "htb-presence", "config.yaml"), nil
}

// Validate reports whether the configuration is usable.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.HTB.APIToken) == "" {
		return errors.New("htb.api_token is required")
	}
	if err := ValidateToken(c.HTB.APIToken); err != nil {
		return fmt.Errorf("htb.api_token: %w", err)
	}
	if interval := time.Duration(c.HTB.PollInterval); interval < MinPollInterval {
		return fmt.Errorf("htb.poll_interval must be at least %s, got %s", MinPollInterval, interval)
	}
	if strings.TrimSpace(c.Discord.ClientID) == "" {
		return errors.New("discord.client_id is required")
	}
	return nil
}

// applyEnv lets HTB_API_TOKEN and DISCORD_CLIENT_ID override the file.
func (c *Config) applyEnv() {
	if v := strings.TrimSpace(os.Getenv("HTB_API_TOKEN")); v != "" {
		c.HTB.APIToken = v
	}
	if v := strings.TrimSpace(os.Getenv("DISCORD_CLIENT_ID")); v != "" {
		c.Discord.ClientID = v
	}
}

// PermWarning reports when path is readable by group or other users.
// An empty string means the mode is fine or could not be read.
func PermWarning(path string) string {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return ""
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Sprintf("config file %s is mode %o; chmod 600 is recommended", path, fi.Mode().Perm())
	}
	return ""
}

// ValidateToken checks that token looks like an HTB App Token. HTB App Tokens
// are JWTs with three dot-separated segments; this guards against pasting the
// wrong credential (for example an account password).
func ValidateToken(token string) error {
	if strings.Count(strings.TrimSpace(token), ".") != 2 {
		return errors.New("does not look like an HTB App Token (expected three dot-separated segments)")
	}
	return nil
}

// MaskToken returns a redacted form of token suitable for logs. It never
// returns the full token.
func MaskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) < 8 {
		return "****"
	}
	return token[:4] + "…"
}
