// Package presence maps HTB activity to Discord Rich Presence and drives the
// poll loop that keeps the two in sync.
package presence

import (
	"strings"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/htb"
)

// LargeImageAsset is the Discord asset key for the Hack The Box logo.
const LargeImageAsset = "htb"

// Options controls how activity is rendered.
type Options struct {
	// ShowTimer includes an elapsed-time timer on the presence.
	ShowTimer bool

	// SessionStart is when the current machine session began, as far as this
	// process can tell. It is only used when ShowTimer is set.
	SessionStart time.Time
}

// Map renders the current HTB activity as a Discord activity. A nil activity,
// or one without an active machine, maps to a neutral browsing state rather
// than stale data.
func Map(activity *htb.Activity, opts Options) *discord.Activity {
	if activity == nil || activity.Machine == nil {
		return &discord.Activity{
			Details:    "Hack The Box",
			State:      "Browsing…",
			LargeImage: LargeImageAsset,
			LargeText:  "Hack The Box",
		}
	}

	m := activity.Machine
	name := m.Name
	if name == "" {
		name = "A machine"
	}

	out := &discord.Activity{
		Details:    name,
		State:      joinNonEmpty(" · ", m.OS, m.Difficulty),
		LargeImage: LargeImageAsset,
		LargeText:  "Hack The Box",
	}
	if opts.ShowTimer && !opts.SessionStart.IsZero() {
		out.StartTime = opts.SessionStart
	}
	return out
}

// joinNonEmpty joins the non-blank parts with sep.
func joinNonEmpty(sep string, parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}
