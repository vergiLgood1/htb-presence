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
	t.Setenv("HTB_API_TOKEN", "")
	t.Setenv("DISCORD_CLIENT_ID", "")
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
  show_machine_name: false
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
				if cfg.Discord.ShowMachineName || cfg.Discord.ShowRank || cfg.Discord.ShowTimer {
					t.Errorf("ShowMachineName/ShowRank/ShowTimer = %v/%v/%v, want false/false/false",
						cfg.Discord.ShowMachineName, cfg.Discord.ShowRank, cfg.Discord.ShowTimer)
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
				if !cfg.Discord.ShowMachineName || !cfg.Discord.ShowRank || !cfg.Discord.ShowTimer || !cfg.Discord.ShowPoints || !cfg.Discord.ShowButtons {
					t.Errorf("expected name/rank/timer/points/buttons to default on")
				}
				if cfg.Discord.ShowFlags || cfg.Discord.ClearWhenIdle {
					t.Errorf("ShowFlags/ClearWhenIdle = %v/%v, want false/false", cfg.Discord.ShowFlags, cfg.Discord.ClearWhenIdle)
				}
				if !cfg.HTB.VPNFallback {
					t.Error("VPNFallback = false, want true")
				}
				if cfg.Discord.IdleText != "Browsing…" {
					t.Errorf("IdleText = %q, want Browsing…", cfg.Discord.IdleText)
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

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("HTB_API_TOKEN", "env.token.value")
	t.Setenv("DISCORD_CLIENT_ID", "999")
	cfg, err := Load(writeConfig(t, "htb:\n  api_token: \"aaa.bbb.ccc\"\ndiscord:\n  client_id: \"1\"\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTB.APIToken != "env.token.value" || cfg.Discord.ClientID != "999" {
		t.Errorf("token/client = %q/%q, want env overrides", cfg.HTB.APIToken, cfg.Discord.ClientID)
	}
}

func TestWriteTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "htb-presence", "config.yaml")
	if err := WriteTemplate(path, false); err != nil {
		t.Fatalf("WriteTemplate: %v", err)
	}
	if err := WriteTemplate(path, false); err == nil {
		t.Fatal("second WriteTemplate without -force succeeded")
	}
	if err := WriteTemplate(path, true); err != nil {
		t.Fatalf("force WriteTemplate: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "show_flags: false") || !strings.Contains(string(data), "vpn_fallback: true") {
		t.Fatalf("template missing new keys:\n%s", data)
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
