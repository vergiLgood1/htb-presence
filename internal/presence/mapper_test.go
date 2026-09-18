package presence

import (
	"testing"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/htb"
)

func TestMapActiveMachine(t *testing.T) {
	start := time.Unix(1000, 0)
	got := Map(&htb.Activity{Machine: &htb.Machine{
		ID:         289,
		Name:       "Vaccine",
		OS:         "Linux",
		Difficulty: "Very Easy",
	}}, Options{ShowMachineName: true, ShowTimer: true, SessionStart: start})

	if got.Details != "Vaccine" {
		t.Errorf("Details = %q, want Vaccine", got.Details)
	}
	if got.State != "Linux · Very Easy" {
		t.Errorf("State = %q, want Linux · Very Easy", got.State)
	}
	if got.LargeImage != LargeImageAsset {
		t.Errorf("LargeImage = %q, want %q", got.LargeImage, LargeImageAsset)
	}
	if !got.StartTime.Equal(start) {
		t.Errorf("StartTime = %s, want %s", got.StartTime, start)
	}
}

func TestMapHidesMachineName(t *testing.T) {
	got := Map(&htb.Activity{Machine: &htb.Machine{
		ID: 289, Name: "Vaccine", OS: "Linux", Difficulty: "Very Easy",
	}}, Options{ShowMachineName: false})

	if got.Details != "Hack The Box" {
		t.Errorf("Details = %q, want generic Hack The Box", got.Details)
	}
	if got.State != "Linux · Very Easy" {
		t.Errorf("State = %q, want OS/difficulty kept", got.State)
	}
}

func TestMapTimerOnlyWhenEnabled(t *testing.T) {
	activity := &htb.Activity{Machine: &htb.Machine{ID: 1, Name: "Vaccine"}}
	start := time.Unix(1000, 0)

	got := Map(activity, Options{ShowMachineName: true, ShowTimer: false, SessionStart: start})
	if !got.StartTime.IsZero() {
		t.Errorf("StartTime = %s, want zero when timer is disabled", got.StartTime)
	}
}

func TestMapIdle(t *testing.T) {
	tests := []struct {
		name     string
		activity *htb.Activity
	}{
		{"nil activity", nil},
		{"no machine", &htb.Activity{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.activity, Options{ShowMachineName: true, ShowRank: true, ShowTimer: true, SessionStart: time.Unix(1000, 0)})
			if got.Details != "Hack The Box" {
				t.Errorf("Details = %q, want Hack The Box", got.Details)
			}
			if got.State != "Browsing…" {
				t.Errorf("State = %q, want Browsing…", got.State)
			}
			if !got.StartTime.IsZero() {
				t.Errorf("StartTime = %s, want zero when idle", got.StartTime)
			}
		})
	}
}

func TestMapWithoutName(t *testing.T) {
	got := Map(&htb.Activity{Machine: &htb.Machine{ID: 1}}, Options{ShowMachineName: true})
	if got.Details != "A machine" {
		t.Errorf("Details = %q, want A machine", got.Details)
	}
}

func TestMapRank(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		user *htb.User
		want string
	}{
		{
			name: "shown with points",
			opts: Options{ShowMachineName: true, ShowRank: true},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "Linux · Easy · Noob · 120 pts",
		},
		{
			name: "shown without points",
			opts: Options{ShowMachineName: true, ShowRank: true},
			user: &htb.User{Rank: "Noob"},
			want: "Linux · Easy · Noob",
		},
		{
			name: "hidden",
			opts: Options{ShowMachineName: true, ShowRank: false},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "Linux · Easy",
		},
		{
			name: "no user loaded",
			opts: Options{ShowMachineName: true, ShowRank: true},
			user: nil,
			want: "Linux · Easy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := active(289, "Vaccine")
			activity.User = tt.user
			if got := Map(activity, tt.opts); got.State != tt.want {
				t.Errorf("State = %q, want %q", got.State, tt.want)
			}
		})
	}
}

func TestSession(t *testing.T) {
	first := time.Unix(1000, 0)
	second := time.Unix(2000, 0)

	var s session

	if got := s.Start(0, first); !got.IsZero() {
		t.Errorf("idle Start = %s, want zero", got)
	}
	if got := s.Start(289, first); !got.Equal(first) {
		t.Errorf("Start = %s, want %s", got, first)
	}
	if got := s.Start(289, second); !got.Equal(first) {
		t.Errorf("Start after same machine = %s, want %s (unchanged)", got, first)
	}
	if got := s.Start(290, second); !got.Equal(second) {
		t.Errorf("Start after machine change = %s, want %s", got, second)
	}
	if got := s.Start(0, second); !got.IsZero() {
		t.Errorf("Start when idle = %s, want zero", got)
	}
}
