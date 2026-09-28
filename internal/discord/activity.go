package discord

import (
	"strings"
	"time"
	"unicode/utf8"
)

// fieldMaxLen is Discord's documented character limit for text fields such as
// details, state, and asset text.
const fieldMaxLen = 128

// buttonMaxLen is Discord's character limit for a button label.
const buttonMaxLen = 32

// maxButtons is how many buttons a Rich Presence activity may carry.
const maxButtons = 2

// Button is a clickable link on a Rich Presence activity.
type Button struct {
	Label string
	URL   string
}

// Activity is a Discord Rich Presence activity payload.
type Activity struct {
	Details    string
	State      string
	LargeImage string
	LargeText  string
	SmallImage string
	SmallText  string
	// StartTime, when non-zero, renders as an elapsed-time timer.
	StartTime time.Time
	// EndTime, when non-zero, renders as a countdown.
	EndTime time.Time
	Buttons []Button
}

// Same reports whether a and b would render as the same presence.
func Same(a, b *Activity) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Details != b.Details || a.State != b.State ||
		a.LargeImage != b.LargeImage || a.LargeText != b.LargeText ||
		a.SmallImage != b.SmallImage || a.SmallText != b.SmallText ||
		!a.StartTime.Equal(b.StartTime) || !a.EndTime.Equal(b.EndTime) ||
		len(a.Buttons) != len(b.Buttons) {
		return false
	}
	for i := range a.Buttons {
		if a.Buttons[i] != b.Buttons[i] {
			return false
		}
	}
	return true
}

// payload renders the activity as the JSON object Discord expects.
func (a *Activity) payload() map[string]any {
	p := make(map[string]any, 4)

	if s := truncate(a.Details, fieldMaxLen); s != "" {
		p["details"] = s
	}
	if s := truncate(a.State, fieldMaxLen); s != "" {
		p["state"] = s
	}
	timestamps := make(map[string]any, 2)
	if !a.StartTime.IsZero() {
		timestamps["start"] = a.StartTime.UnixMilli()
	}
	if !a.EndTime.IsZero() {
		timestamps["end"] = a.EndTime.UnixMilli()
	}
	if len(timestamps) > 0 {
		p["timestamps"] = timestamps
	}

	if buttons := a.buttonPayload(); len(buttons) > 0 {
		p["buttons"] = buttons
	}

	assets := make(map[string]any, 4)
	if a.LargeImage != "" {
		assets["large_image"] = a.LargeImage
	}
	if s := truncate(a.LargeText, fieldMaxLen); s != "" {
		assets["large_text"] = s
	}
	if a.SmallImage != "" {
		assets["small_image"] = a.SmallImage
	}
	if s := truncate(a.SmallText, fieldMaxLen); s != "" {
		assets["small_text"] = s
	}
	if len(assets) > 0 {
		p["assets"] = assets
	}

	return p
}

// buttonPayload renders up to two buttons with non-empty https URLs.
func (a *Activity) buttonPayload() []map[string]any {
	if len(a.Buttons) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, maxButtons)
	for _, b := range a.Buttons {
		if len(out) == maxButtons {
			break
		}
		label := truncate(strings.TrimSpace(b.Label), buttonMaxLen)
		url := strings.TrimSpace(b.URL)
		if label == "" || !strings.HasPrefix(url, "https://") {
			continue
		}
		out = append(out, map[string]any{"label": label, "url": url})
	}
	return out
}

// truncate limits s to max runes.
func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
