// Package presence maps HTB activity to Discord Rich Presence and drives the
// poll loop that keeps the two in sync.
package presence

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/htb"
)

// LargeImageAsset is the Discord asset key for the Hack The Box logo.
const LargeImageAsset = "htb"

// DefaultIdleText is the state shown when nothing is spawned and presence is
// not cleared.
const DefaultIdleText = "Browsing…"

// Options controls how activity is rendered.
type Options struct {
	// ShowMachineName includes the active target name as the presence details.
	// When false, the details stay generic and the target's avatar and link
	// are omitted. This also covers challenge names.
	ShowMachineName bool

	// ShowRank includes the user's rank in the presence state.
	ShowRank bool

	// ShowPoints includes the user's points in the presence state.
	ShowPoints bool

	// ShowTimer includes an elapsed-time timer. The instance expiry is attached
	// to the avatar hover text instead of rendering as a countdown.
	ShowTimer bool

	// ShowFlags includes user/root own markers. Off by default.
	ShowFlags bool

	// ShowButtons adds link buttons for the active target and the HTB profile.
	ShowButtons bool

	// ClearWhenIdle publishes no presence when nothing is spawned and the VPN
	// fallback is not showing.
	ClearWhenIdle bool

	// IdleText is the state used for the neutral fallback. Blank means
	// DefaultIdleText.
	IdleText string

	// VPNFallback asks the scheduler to query VPN status when nothing is
	// spawned. Map itself only looks at Activity.VPN.
	VPNFallback bool

	// SessionStart is when the current target session began, as far as this
	// process can tell. It is only used when ShowTimer is set.
	SessionStart time.Time

	// Now supplies the current time for the expiry countdown. Nil means
	// time.Now.
	Now func() time.Time
}

// wantsUser reports whether rendering needs the HTB user profile.
func (o Options) wantsUser() bool {
	return o.ShowRank || o.ShowPoints || o.ShowButtons
}

// Map renders the current HTB activity as a Discord activity.
//
// A nil result means the presence should be cleared. That happens when
// ClearWhenIdle is set and there is no spawned target and no VPN fallback
// to show.
func Map(activity *htb.Activity, opts Options) *discord.Activity {
	var machine *htb.Machine
	var challenge *htb.Challenge
	vpn := false
	if activity != nil {
		machine = activity.Machine
		challenge = activity.Challenge
		vpn = activity.VPN.Connected && machine == nil && challenge == nil
	}
	idle := machine == nil && challenge == nil && !vpn
	if idle && opts.ClearWhenIdle {
		return nil
	}

	out := &discord.Activity{
		Details:    "Hack The Box",
		LargeImage: LargeImageAsset,
		LargeText:  "Hack The Box",
	}

	var stateParts []string
	spawned := machine != nil || challenge != nil

	switch {
	case machine != nil:
		stateParts = mapMachine(out, machine, opts)
	case challenge != nil:
		stateParts = mapChallenge(out, challenge, opts)
	case vpn:
		label := "On the VPN"
		if product := strings.TrimSpace(activity.VPN.Product); product != "" {
			label += " · " + product
		}
		stateParts = append(stateParts, label)
	default:
		text := strings.TrimSpace(opts.IdleText)
		if text == "" {
			text = DefaultIdleText
		}
		stateParts = append(stateParts, text)
	}

	if opts.ShowRank || opts.ShowPoints {
		if activity != nil && activity.User != nil {
			if r := rankLabel(activity.User, opts.ShowRank, opts.ShowPoints); r != "" {
				stateParts = append(stateParts, r)
			}
		}
	}

	if opts.ShowTimer && spawned && !opts.SessionStart.IsZero() {
		out.StartTime = opts.SessionStart
	}

	out.State = joinNonEmpty(" · ", stateParts...)

	if opts.ShowButtons {
		out.Buttons = append(out.Buttons, buttons(activity, opts)...)
	}
	return out
}

// mapMachine fills the machine-specific presence fields and returns state
// parts.
func mapMachine(out *discord.Activity, m *htb.Machine, opts Options) []string {
	meta := joinNonEmpty(" · ", m.OS, m.Difficulty)
	if opts.ShowMachineName {
		if meta != "" {
			out.Details = fmt.Sprintf("%s (%s)", orDefault(m.Name, "A machine"), meta)
		} else {
			out.Details = orDefault(m.Name, "A machine")
		}
		if m.AvatarURL != "" {
			out.LargeImage = m.AvatarURL
			hover := orDefault(m.Name, "A machine")
			if !m.ExpiresAt.IsZero() {
				hover += " · ends " + m.ExpiresAt.In(time.Local).Format("2 Jan 15:04")
			}
			out.LargeText = hover
		}
	} else if meta != "" {
		out.Details = meta
	}

	var parts []string
	if opts.ShowFlags {
		if f := flagLabel(m); f != "" {
			parts = append(parts, "Flags: "+f)
		}
	}
	applySmallImage(out, osAsset(m.OS), m.OS)
	return parts
}

// mapChallenge fills the challenge-specific presence fields.
func mapChallenge(out *discord.Activity, ch *htb.Challenge, opts Options) []string {
	meta := joinNonEmpty(" · ", ch.Category, ch.Difficulty)
	if opts.ShowMachineName {
		name := orDefault(ch.Name, "A challenge")
		if meta != "" {
			out.Details = fmt.Sprintf("%s (%s)", name, meta)
		} else {
			out.Details = name
		}
		if ch.AvatarURL != "" {
			out.LargeImage = ch.AvatarURL
			hover := name
			if !ch.ExpiresAt.IsZero() {
				hover += " · ends " + ch.ExpiresAt.In(time.Local).Format("2 Jan 15:04")
			}
			out.LargeText = hover
		}
	} else {
		out.Details = "Challenge"
		if meta != "" {
			out.Details = "Challenge (" + meta + ")"
		}
	}

	applySmallImage(out, "", "")
	return nil
}

// applySmallImage sets the small asset. An OS badge wins; otherwise the HTB
// logo is used as the badge when the large image is already a target avatar.
func applySmallImage(out *discord.Activity, asset, text string) {
	if asset != "" {
		out.SmallImage = asset
		out.SmallText = text
		return
	}
	if out.LargeImage != LargeImageAsset {
		out.SmallImage = LargeImageAsset
		out.SmallText = "Hack The Box"
	}
}

// buttons returns the link buttons the user opted into, at most two.
func buttons(activity *htb.Activity, opts Options) []discord.Button {
	var out []discord.Button
	if opts.ShowMachineName && activity != nil {
		if m := activity.Machine; m != nil && strings.TrimSpace(m.Name) != "" {
			out = append(out, discord.Button{
				Label: "Open machine",
				URL:   "https://app.hackthebox.com/machines/" + url.PathEscape(m.Name),
			})
		} else if ch := activity.Challenge; ch != nil && ch.ID != 0 {
			out = append(out, discord.Button{
				Label: "Open challenge",
				URL:   fmt.Sprintf("https://app.hackthebox.com/challenges/%d", ch.ID),
			})
		}
	}
	if activity != nil && activity.User != nil && activity.User.ID != 0 {
		out = append(out, discord.Button{
			Label: "HTB profile",
			URL:   fmt.Sprintf("https://app.hackthebox.com/users/%d", activity.User.ID),
		})
	}
	if len(out) > 2 {
		out = out[:2]
	}
	return out
}

// osAsset maps an HTB OS name to a Discord art-asset key. Unknown systems
// return an empty key so the caller can fall back to the HTB logo.
func osAsset(osName string) string {
	switch strings.ToLower(strings.TrimSpace(osName)) {
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	case "freebsd":
		return "freebsd"
	case "openbsd":
		return "openbsd"
	case "solaris":
		return "solaris"
	default:
		return ""
	}
}

// flagLabel renders which flags the user has submitted.
func flagLabel(m *htb.Machine) string {
	switch {
	case m.UserOwned && m.RootOwned:
		return "user · root"
	case m.UserOwned:
		return "user"
	case m.RootOwned:
		return "root"
	default:
		return ""
	}
}

// rankLabel renders rank and points according to the toggles.
func rankLabel(u *htb.User, showRank, showPoints bool) string {
	var parts []string
	if showRank && strings.TrimSpace(u.Rank) != "" {
		parts = append(parts, u.Rank)
	}
	if showPoints && u.Points > 0 {
		parts = append(parts, fmt.Sprintf("%d pts", u.Points))
	}
	return strings.Join(parts, " · ")
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
