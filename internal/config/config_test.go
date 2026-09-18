package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid explicit values",
			yaml: `
htb:
  api_token: "aaa.bbb.ccc"
  poll_interval: 45s
discord:
  client_id: "1234567890"
  show_rank: false
  show_timer: false
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.HTB.APIToken != "aaa.bbb.ccc" {
					t.Errorf("APIToken = %q", cfg.HTB.APIToken)
				}
				if got := time.Duration(cfg.HTB.PollInterval); got != 45*time.Second {
					t.Errorf("PollInterval = %s, want 45s", got)
				}
				if cfg.Discord.ClientID != "1234567890" {
					t.Errorf("ClientID = %q", cfg.Discord.ClientID)
				}
				if cfg.Discord.ShowRank || cfg.Discord.ShowTimer {
					t.Errorf("ShowRank/ShowTimer = %v/%v, want false/false",
						cfg.Discord.ShowRank, cfg.Discord.ShowTimer)
				}
			},
		},
		{
			name: "defaults applied when omitted",
			yaml: `
htb:
  api_token: "aaa.bbb.ccc"
discord:
  client_id: "1234567890"
`,
			check: func(t *testing.T, cfg *Config) {
				if got := time.Duration(cfg.HTB.PollInterval); got != DefaultPollInterval {
					t.Errorf("PollInterval = %s, want default %s", got, DefaultPollInterval)
				}
				if !cfg.Discord.ShowRank || !cfg.Discord.ShowTimer {
					t.Errorf("ShowRank/ShowTimer = %v/%v, want true/true",
						cfg.Discord.ShowRank, cfg.Discord.ShowTimer)
				}
			},
		},
		{
			name:    "missing token",
			yaml:    "discord:\n  client_id: \"1\"\n",
			wantErr: "api_token is required",
		},
		{
			name:    "token is not an app token",
			yaml:    "htb:\n  api_token: \"notatoken\"\ndiscord:\n  client_id: \"1\"\n",
			wantErr: "does not look like an HTB App Token",
		},
		{
			name:    "missing client id",
			yaml:    "htb:\n  api_token: \"aaa.bbb.ccc\"\n",
			wantErr: "client_id is required",
		},
		{
			name:    "poll interval below minimum",
			yaml:    "htb:\n  api_token: \"aaa.bbb.ccc\"\n  poll_interval: 5s\ndiscord:\n  client_id: \"1\"\n",
			wantErr: "poll_interval must be at least",
		},
		{
			name:    "poll interval not a duration",
			yaml:    "htb:\n  api_token: \"aaa.bbb.ccc\"\n  poll_interval: soon\ndiscord:\n  client_id: \"1\"\n",
			wantErr: "invalid duration",
		},
		{
			name:    "malformed yaml",
			yaml:    "htb: [unclosed\n",
			wantErr: "parsing config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(writeConfig(t, tt.yaml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("htb-presence", "config.yaml")) {
		t.Errorf("DefaultPath = %q, want suffix htb-presence/config.yaml", path)
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"short", "abc", "****"},
		{"app token", "eyJhbGciOiJIUzI1NiJ9.aaa.bbb", "eyJh…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskToken(tt.in); got != tt.want {
				t.Errorf("MaskToken(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(MaskToken(tt.in), tt.in) && tt.in != "" {
				t.Errorf("MaskToken(%q) leaked the input", tt.in)
			}
		})
	}
}
