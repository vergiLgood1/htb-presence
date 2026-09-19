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

func TestMapMachineAvatar(t *testing.T) {
	withAvatar := func() *htb.Activity {
		return &htb.Activity{Machine: &htb.Machine{
			ID: 289, Name: "Vaccine", OS: "Linux", Difficulty: "Easy",
			AvatarURL: "https://cdn.example/vaccine.png",
		}}
	}

	t.Run("shown as the large image", func(t *testing.T) {
		got := Map(withAvatar(), Options{ShowMachineName: true})
		if got.LargeImage != "https://cdn.example/vaccine.png" {
			t.Errorf("LargeImage = %q, want the avatar URL", got.LargeImage)
		}
		if got.LargeText != "Vaccine" {
			t.Errorf("LargeText = %q, want Vaccine", got.LargeText)
		}
		if got.SmallImage != LargeImageAsset {
			t.Errorf("SmallImage = %q, want %q", got.SmallImage, LargeImageAsset)
		}
	})

	t.Run("hidden with the machine name", func(t *testing.T) {
		got := Map(withAvatar(), Options{ShowMachineName: false})
		if got.LargeImage != LargeImageAsset {
			t.Errorf("LargeImage = %q, want the generic asset when the name is hidden", got.LargeImage)
		}
		if got.Details != "Hack The Box" {
			t.Errorf("Details = %q, want generic", got.Details)
		}
	})

	t.Run("falls back when there is no avatar", func(t *testing.T) {
		activity := withAvatar()
		activity.Machine.AvatarURL = ""
		got := Map(activity, Options{ShowMachineName: true})
		if got.LargeImage != LargeImageAsset {
			t.Errorf("LargeImage = %q, want %q", got.LargeImage, LargeImageAsset)
		}
	})
}

func TestSession(t *testing.T) {
	first := time.Unix(1000, 0)
	second := time.Unix(2000, 0)

	var s session

	if start, ended := s.observe(nil, first); !start.IsZero() || ended != nil {
		t.Errorf("observe(idle) = (%s, %+v), want (zero, nil)", start, ended)
	}
	if start, ended := s.observe(&htb.Machine{ID: 289, Name: "Vaccine"}, first); !start.Equal(first) || ended != nil {
		t.Errorf("observe(Vaccine) = (%s, %+v), want (%s, nil)", start, ended, first)
	}
	if start, ended := s.observe(&htb.Machine{ID: 289, Name: "Vaccine"}, second); !start.Equal(first) || ended != nil {
		t.Errorf("observe(same machine) = (%s, %+v), want (%s, nil)", start, ended, first)
	}

	start, ended := s.observe(&htb.Machine{ID: 290, Name: "Keeper"}, second)
	if !start.Equal(second) {
		t.Errorf("new machine start = %s, want %s", start, second)
	}
	if ended == nil {
		t.Fatal("expected the Vaccine session to end")
	}
	if ended.MachineID != 289 || ended.MachineName != "Vaccine" ||
		!ended.StartedAt.Equal(first) || !ended.EndedAt.Equal(second) {
		t.Errorf("ended = %+v", ended)
	}

	start, ended = s.observe(nil, second)
	if !start.IsZero() {
		t.Errorf("idle start = %s, want zero", start)
	}
	if ended == nil || ended.MachineID != 290 {
		t.Errorf("ended = %+v, want the Keeper session", ended)
	}
}
