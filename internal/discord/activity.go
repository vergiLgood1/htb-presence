package discord

import (
	"time"
	"unicode/utf8"
)

// fieldMaxLen is Discord's documented character limit for text fields such as
// details, state, and asset text.
const fieldMaxLen = 128

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
	if !a.StartTime.IsZero() {
		p["timestamps"] = map[string]any{"start": a.StartTime.UnixMilli()}
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

// truncate limits s to max runes.
func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
