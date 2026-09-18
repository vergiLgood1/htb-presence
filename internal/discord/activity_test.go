package discord

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestActivityPayload(t *testing.T) {
	start := time.Unix(1700000000, 0)
	got := (&Activity{
		Details:    "Vaccine",
		State:      "Linux · Very Easy",
		LargeImage: "htb",
		LargeText:  "Hack The Box",
		StartTime:  start,
	}).payload()

	if got["details"] != "Vaccine" {
		t.Errorf("details = %v, want Vaccine", got["details"])
	}
	if got["state"] != "Linux · Very Easy" {
		t.Errorf("state = %v, want Linux · Very Easy", got["state"])
	}

	assets, ok := got["assets"].(map[string]any)
	if !ok {
		t.Fatalf("assets = %v, want an object", got["assets"])
	}
	if assets["large_image"] != "htb" || assets["large_text"] != "Hack The Box" {
		t.Errorf("assets = %v", assets)
	}

	timestamps, ok := got["timestamps"].(map[string]any)
	if !ok {
		t.Fatalf("timestamps = %v, want an object", got["timestamps"])
	}
	if timestamps["start"] != start.UnixMilli() {
		t.Errorf("timestamps.start = %v, want %d", timestamps["start"], start.UnixMilli())
	}
}

func TestActivityPayloadEmpty(t *testing.T) {
	if got := (&Activity{}).payload(); len(got) != 0 {
		t.Errorf("payload = %v, want empty", got)
	}
}

func TestActivityPayloadTruncatesByRune(t *testing.T) {
	got := (&Activity{Details: strings.Repeat("é", 200)}).payload()
	details := got["details"].(string)
	if n := utf8.RuneCountInString(details); n != fieldMaxLen {
		t.Errorf("details has %d runes, want %d", n, fieldMaxLen)
	}
}
