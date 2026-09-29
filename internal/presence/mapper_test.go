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

	if got.Details != "Vaccine (Linux · Very Easy)" {
		t.Errorf("Details = %q, want Vaccine (Linux · Very Easy)", got.Details)
	}
	if got.State != "" {
		t.Errorf("State = %q, want empty when no flags or rank", got.State)
	}
	if got.LargeImage != LargeImageAsset {
		t.Errorf("LargeImage = %q, want %q", got.LargeImage, LargeImageAsset)
	}
	if !got.StartTime.Equal(start) {
		t.Errorf("StartTime = %s, want %s", got.StartTime, start)
	}
	if !got.EndTime.IsZero() {
		t.Errorf("EndTime = %s, want zero (elapsed timer only)", got.EndTime)
	}
}

func TestMapHidesMachineName(t *testing.T) {
	got := Map(&htb.Activity{Machine: &htb.Machine{
		ID: 289, Name: "Vaccine", OS: "Linux", Difficulty: "Very Easy",
	}}, Options{ShowMachineName: false})

	if got.Details != "Linux · Very Easy" {
		t.Errorf("Details = %q, want Linux · Very Easy", got.Details)
	}
	if got.State != "" {
		t.Errorf("State = %q, want empty", got.State)
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
			opts: Options{ShowMachineName: true, ShowRank: true, ShowPoints: true},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "Noob · 120 pts",
		},
		{
			name: "rank without points",
			opts: Options{ShowMachineName: true, ShowRank: true, ShowPoints: false},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "Noob",
		},
		{
			name: "points without rank",
			opts: Options{ShowMachineName: true, ShowPoints: true},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "120 pts",
		},
		{
			name: "shown without points",
			opts: Options{ShowMachineName: true, ShowRank: true},
			user: &htb.User{Rank: "Noob"},
			want: "Noob",
		},
		{
			name: "zero points still shown",
			opts: Options{ShowMachineName: true, ShowRank: true, ShowPoints: true},
			user: &htb.User{Rank: "Noob"},
			want: "Noob · 0 pts",
		},
		{
			name: "hidden",
			opts: Options{ShowMachineName: true, ShowRank: false},
			user: &htb.User{Rank: "Noob", Points: 120},
			want: "",
		},
		{
			name: "no user loaded",
			opts: Options{ShowMachineName: true, ShowRank: true},
			user: nil,
			want: "",
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
		if got.SmallImage != "linux" {
			t.Errorf("SmallImage = %q, want linux", got.SmallImage)
		}
		if got.SmallText != "Linux" {
			t.Errorf("SmallText = %q, want Linux", got.SmallText)
		}
	})

	t.Run("hidden with the machine name", func(t *testing.T) {
		got := Map(withAvatar(), Options{ShowMachineName: false})
		if got.LargeImage != LargeImageAsset {
			t.Errorf("LargeImage = %q, want the generic asset when the name is hidden", got.LargeImage)
		}
		if got.Details != "Linux · Easy" {
			t.Errorf("Details = %q, want Linux · Easy", got.Details)
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

func TestMapChallengeButtonsFlagsAndIdle(t *testing.T) {
	expiry := time.Unix(4_000, 0)

	t.Run("challenge", func(t *testing.T) {
		got := Map(&htb.Activity{
			Challenge: &htb.Challenge{ID: 7, Name: "Phonebook", Category: "Web", Difficulty: "Easy", ExpiresAt: expiry},
			User:      &htb.User{ID: 5, Rank: "Noob"},
		}, Options{ShowMachineName: true, ShowTimer: true, ShowButtons: true, ShowRank: true, SessionStart: time.Unix(500, 0)})
		if got.Details != "Phonebook (Web · Easy)" || got.State != "Noob" {
			t.Fatalf("details/state = %q / %q, want Phonebook (Web · Easy) / Noob", got.Details, got.State)
		}
		if !got.EndTime.IsZero() {
			t.Errorf("EndTime = %s, want zero (elapsed timer only)", got.EndTime)
		}
		if len(got.Buttons) != 2 || got.Buttons[0].Label != "Open challenge" || got.Buttons[1].URL != "https://app.hackthebox.com/users/5" {
			t.Errorf("buttons = %+v", got.Buttons)
		}
	})

	t.Run("flags", func(t *testing.T) {
		got := Map(&htb.Activity{Machine: &htb.Machine{Name: "Vaccine", OS: "Linux", UserOwned: true}}, Options{ShowMachineName: true, ShowFlags: true})
		if got.Details != "Vaccine (Linux)" {
			t.Errorf("Details = %q, want Vaccine (Linux)", got.Details)
		}
		if got.State != "Flags: user" {
			t.Errorf("State = %q, want Flags: user", got.State)
		}
	})

	t.Run("clear when idle", func(t *testing.T) {
		if got := Map(&htb.Activity{}, Options{ClearWhenIdle: true}); got != nil {
			t.Errorf("Map = %+v, want nil", got)
		}
	})

	t.Run("custom idle", func(t *testing.T) {
		got := Map(nil, Options{IdleText: "In the labs"})
		if got.State != "In the labs" {
			t.Errorf("State = %q", got.State)
		}
	})

	t.Run("vpn", func(t *testing.T) {
		got := Map(&htb.Activity{VPN: htb.VPN{Connected: true, Product: "fortresses"}}, Options{ClearWhenIdle: true})
		if got == nil || got.State != "On the VPN · fortresses" {
			t.Fatalf("vpn presence = %+v", got)
		}
	})
}

// TestMapExpiryTooltip covers the absolute expiry that is attached to the
// large image hover text instead of rendering as an IPC countdown.
func TestMapExpiryTooltip(t *testing.T) {
	future := time.Date(2026, 10, 3, 9, 50, 0, 0, time.UTC)
	opts := Options{ShowMachineName: true, ShowTimer: true}

	t.Run("avatar hover carries expiry", func(t *testing.T) {
		m := &htb.Machine{
			ID:         1,
			Name:       "Layover",
			OS:         "Linux",
			Difficulty: "Medium",
			AvatarURL:  "https://cdn.example/layover.png",
			ExpiresAt:  future,
		}
		got := Map(&htb.Activity{Machine: m}, opts)
		if got.Details != "Layover (Linux · Medium)" {
			t.Errorf("Details = %q, want Layover (Linux · Medium)", got.Details)
		}
		wantHover := "Layover · ends " + future.In(time.Local).Format("2 Jan 15:04")
		if got.LargeText != wantHover {
			t.Errorf("LargeText = %q, want %q", got.LargeText, wantHover)
		}
		if !got.EndTime.IsZero() {
			t.Errorf("EndTime = %s, want zero (countdown replaced by elapsed timer)", got.EndTime)
		}
	})

	t.Run("without expiry hover is target name", func(t *testing.T) {
		m := &htb.Machine{
			ID:         1,
			Name:       "Layover",
			OS:         "Linux",
			Difficulty: "Medium",
			AvatarURL:  "https://cdn.example/layover.png",
		}
		got := Map(&htb.Activity{Machine: m}, opts)
		if got.LargeText != "Layover" {
			t.Errorf("LargeText = %q, want Layover", got.LargeText)
		}
	})
}

func TestSession(t *testing.T) {
	first := time.Unix(1000, 0)
	second := time.Unix(2000, 0)

	var s session

	if start, ended := s.observe(targetRef{}, first); !start.IsZero() || ended != nil {
		t.Errorf("observe(idle) = (%s, %+v), want (zero, nil)", start, ended)
	}
	vaccine := targetRef{kind: "machine", id: 289, name: "Vaccine"}
	if start, ended := s.observe(vaccine, first); !start.Equal(first) || ended != nil {
		t.Errorf("observe(Vaccine) = (%s, %+v), want (%s, nil)", start, ended, first)
	}
	if start, ended := s.observe(vaccine, second); !start.Equal(first) || ended != nil {
		t.Errorf("observe(same machine) = (%s, %+v), want (%s, nil)", start, ended, first)
	}

	start, ended := s.observe(targetRef{kind: "machine", id: 290, name: "Keeper"}, second)
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

	start, ended = s.observe(targetRef{}, second)
	if !start.IsZero() {
		t.Errorf("idle start = %s, want zero", start)
	}
	if ended == nil || ended.MachineID != 290 {
		t.Errorf("ended = %+v, want the Keeper session", ended)
	}
}
