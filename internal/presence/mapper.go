// Package presence maps HTB activity to Discord Rich Presence and drives the
// poll loop that keeps the two in sync.
package presence

import (
	"fmt"
	"strings"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/htb"
)

// LargeImageAsset is the Discord asset key for the Hack The Box logo.
const LargeImageAsset = "htb"

// Options controls how activity is rendered.
type Options struct {
	// ShowMachineName includes the machine name as the presence details; when
	// false the details stay generic.
	ShowMachineName bool

	// ShowRank includes the user's rank and points in the presence state.
	ShowRank bool

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
	idle := activity == nil || activity.Machine == nil

	out := &discord.Activity{
		Details:    "Hack The Box",
		LargeImage: LargeImageAsset,
		LargeText:  "Hack The Box",
	}

	var stateParts []string
	if idle {
		stateParts = append(stateParts, "Browsing…")
	} else {
		m := activity.Machine
		if opts.ShowMachineName {
			out.Details = orDefault(m.Name, "A machine")
		}
		stateParts = append(stateParts, m.OS, m.Difficulty)

		// The machine avatar is only shown when the name is, since the image
		// would otherwise reveal a machine the user chose to hide.
		if opts.ShowMachineName && m.AvatarURL != "" {
			out.LargeImage = m.AvatarURL
			out.LargeText = orDefault(m.Name, "A machine")
			out.SmallImage = LargeImageAsset
			out.SmallText = "Hack The Box"
		}
	}

	if opts.ShowRank && activity != nil && activity.User != nil {
		stateParts = append(stateParts, rankLabel(activity.User))
	}
	out.State = joinNonEmpty(" · ", stateParts...)

	if opts.ShowTimer && !idle && !opts.SessionStart.IsZero() {
		out.StartTime = opts.SessionStart
	}
	return out
}

// rankLabel renders a user's rank and points, e.g. "Pro Hacker · 1234 pts".
func rankLabel(u *htb.User) string {
	if u.Rank == "" {
		return ""
	}
	if u.Points > 0 {
		return fmt.Sprintf("%s · %d pts", u.Rank, u.Points)
	}
	return u.Rank
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

// orDefault returns s, or fallback when s is blank.
func orDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
