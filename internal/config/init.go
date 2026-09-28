package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Template is the starter config written by WriteTemplate.
const Template = `# htb-presence configuration.
# HTB_API_TOKEN and DISCORD_CLIENT_ID override api_token and client_id when set.

htb:
  api_token: "your-htb-app-token"   # Profile → Settings → App Tokens
  poll_interval: 30s                # minimum 15s
  vpn_fallback: true                # "On the VPN" when nothing is spawned

discord:
  client_id: "your-discord-application-id"
  show_machine_name: true           # also hides challenge names, avatars, and target links
  show_rank: true
  show_points: true
  show_timer: true                  # elapsed playing time; expiry moves to avatar hover
  show_flags: false                 # user/root owns; off because it is spoilery
  show_buttons: true                # "Open machine" / "Open challenge" and "HTB profile"
  clear_when_idle: false            # clear presence instead of showing idle text
  idle_text: "Browsing…"

history:
  file: ""                          # optional JSONL session log; empty disables it
`

// InitMessage is printed after a config file is written.
func InitMessage(path string) string {
	return fmt.Sprintf(`Wrote %s (mode 600).

Next:
1. Put your HTB App Token and Discord application ID in that file, or export HTB_API_TOKEN and DISCORD_CLIENT_ID.
2. In the Discord developer portal open your application → Rich Presence → Art Assets and upload:
   - htb (the logo, used as the large image)
   - linux, windows, freebsd, openbsd, solaris (optional OS badges, used as the small image)
3. Run htb-presence. Stop it with Ctrl-C; it clears presence on exit.
4. Optional background service: docs/examples/htb-presence.service (Linux) or docs/examples/htb-presence.plist (macOS).
`, path)
}

// WriteTemplate creates path from Template. An existing file is left in place
// unless force is set.
func WriteTemplate(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("config already exists at %s (use -force to overwrite)", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(Template), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
