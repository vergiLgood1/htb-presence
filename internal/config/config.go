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
}

// HTB holds the Hack The Box API settings.
type HTB struct {
	APIToken     string   `yaml:"api_token"`
	PollInterval Duration `yaml:"poll_interval"`
}

// Discord holds the Discord Rich Presence settings.
type Discord struct {
	ClientID string `yaml:"client_id"`

	// Privacy toggles. All default to true (see Default).
	ShowMachineName bool `yaml:"show_machine_name"`
	ShowRank        bool `yaml:"show_rank"`
	ShowTimer       bool `yaml:"show_timer"`
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
		HTB: HTB{PollInterval: Duration(DefaultPollInterval)},
		Discord: Discord{
			ShowMachineName: true,
			ShowRank:        true,
			ShowTimer:       true,
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
