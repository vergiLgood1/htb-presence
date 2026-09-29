package htb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// newTestClient returns a client pointed at a test server running handler.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("aaa.bbb.ccc", WithBaseURL(srv.URL))
}

func TestCurrentActivityActiveMachine(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer aaa.bbb.ccc" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer aaa.bbb.ccc")
		}
		switch r.URL.Path {
		case "/machine/active":
			io.WriteString(w, `{"info":{"id":42,"ip":"10.10.10.42","expires_at":"2026-09-18 20:00:00"}}`)
		case "/machine/profile/42":
			io.WriteString(w, `{"info":{"name":"Keeper","os":"Linux","difficultyText":"Easy","avatar":"https://cdn.example/keeper.png"}}`)
		default:
			http.NotFound(w, r)
		}
	})

	activity, err := client.CurrentActivity(context.Background())
	if err != nil {
		t.Fatalf("CurrentActivity: %v", err)
	}
	if activity.Machine == nil {
		t.Fatal("Machine is nil, want a machine")
	}
	m := activity.Machine
	if m.ID != 42 {
		t.Errorf("ID = %d, want 42", m.ID)
	}
	if m.Name != "Keeper" {
		t.Errorf("Name = %q, want Keeper", m.Name)
	}
	if m.OS != "Linux" {
		t.Errorf("OS = %q, want Linux", m.OS)
	}
	if m.Difficulty != "Easy" {
		t.Errorf("Difficulty = %q, want Easy", m.Difficulty)
	}
	if m.IP != "10.10.10.42" {
		t.Errorf("IP = %q, want 10.10.10.42", m.IP)
	}
	if m.AvatarURL != "https://cdn.example/keeper.png" {
		t.Errorf("AvatarURL = %q, want the profile avatar", m.AvatarURL)
	}
	wantExpiry := time.Date(2026, 9, 18, 20, 0, 0, 0, time.UTC)
	if !m.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %s, want %s", m.ExpiresAt, wantExpiry)
	}
}

func TestCurrentActivityNoActiveMachine(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"info":null}`)
	})

	activity, err := client.CurrentActivity(context.Background())
	if err != nil {
		t.Fatalf("CurrentActivity: %v", err)
	}
	if activity.Machine != nil {
		t.Errorf("Machine = %+v, want nil", activity.Machine)
	}
}

func TestCurrentActivityErrors(t *testing.T) {
	const validActive = `{"info":{"id":42,"ip":"10.10.10.42","expires_at":"2026-09-18 20:00:00"}}`

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr error
	}{
		{
			name: "redirect to login is an auth failure",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "/login")
				w.WriteHeader(http.StatusFound)
			},
			wantErr: ErrAuth,
		},
		{
			name:    "unauthorized is an auth failure",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			wantErr: ErrAuth,
		},
		{
			name: "rate limited",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			wantErr: ErrRateLimited,
		},
		{
			name:    "malformed json",
			handler: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{not json`) },
			wantErr: ErrUnexpectedResponse,
		},
		{
			name:    "active info without id",
			handler: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"info":{}}`) },
			wantErr: ErrUnexpectedResponse,
		},
		{
			name: "profile missing name",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/machine/active" {
					io.WriteString(w, validActive)
					return
				}
				io.WriteString(w, `{"info":{}}`)
			},
			wantErr: ErrUnexpectedResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, tt.handler)
			_, err := client.CurrentActivity(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestRateLimitErrorRetryAfter(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.CurrentActivity(context.Background())
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("err = %v, want *RateLimitError", err)
	}
	if rle.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %s, want 30s", rle.RetryAfter)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "120", 120 * time.Second},
		{"garbage", "soon", 0},
		{"negative", "-5", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.in); got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestUser(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/info":
			io.WriteString(w, `{"info":{"id":3967824,"name":"Yodev1"}}`)
		case "/user/profile/basic/3967824":
			io.WriteString(w, `{"profile":{"rank":"Noob","points":120}}`)
		default:
			http.NotFound(w, r)
		}
	})

	user, err := client.User(context.Background())
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if user.ID != 3967824 || user.Name != "Yodev1" || user.Rank != "Noob" || user.Points != 120 {
		t.Errorf("User = %+v, want 3967824/Yodev1/Noob/120", user)
	}
}

func TestUserErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr error
	}{
		{
			name:    "info missing id",
			handler: func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"info":{"name":"Yodev1"}}`) },
			wantErr: ErrUnexpectedResponse,
		},
		{
			name: "profile missing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/user/info" {
					io.WriteString(w, `{"info":{"id":1,"name":"Yodev1"}}`)
					return
				}
				io.WriteString(w, `{}`)
			},
			wantErr: ErrUnexpectedResponse,
		},
		{
			name:    "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			wantErr: ErrAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, tt.handler)
			if _, err := client.User(context.Background()); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}

func TestResolveAssetURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"absolute https", "https://cdn.example/a.png", "https://cdn.example/a.png"},
		{"absolute http", "http://cdn.example/a.png", "http://cdn.example/a.png"},
		{"relative", "/storage/avatars/a.png", "https://labs.hackthebox.com/storage/avatars/a.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveAssetURL(tt.in); got != tt.want {
				t.Errorf("resolveAssetURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCurrentActivityChallenge(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/machine/active":
			io.WriteString(w, `{"info":null}`)
		case "/season/machine/active":
			http.NotFound(w, r)
		case "/challenge/active":
			io.WriteString(w, `{"info":{"id":7,"expires_at":"2026-09-18 21:00:00"}}`)
		case "/challenge/info/7":
			io.WriteString(w, `{"info":{"id":7,"name":"Phonebook","category_name":"Web","difficulty":"Easy"}}`)
		default:
			http.NotFound(w, r)
		}
	})

	activity, err := client.CurrentActivity(context.Background())
	if err != nil {
		t.Fatalf("CurrentActivity: %v", err)
	}
	if activity.Machine != nil {
		t.Fatalf("Machine = %+v, want nil", activity.Machine)
	}
	ch := activity.Challenge
	if ch == nil || ch.Name != "Phonebook" || ch.Category != "Web" || ch.Difficulty != "Easy" || ch.ID != 7 {
		t.Fatalf("Challenge = %+v", ch)
	}
	wantExpiry := time.Date(2026, 9, 18, 21, 0, 0, 0, time.UTC)
	if !ch.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %s, want %s", ch.ExpiresAt, wantExpiry)
	}
}

func TestCurrentActivitySeasonMachine(t *testing.T) {
	tests := []struct {
		name      string
		season    string
		wantID    int
		wantSpawn bool
	}{
		{
			name:      "spawned with play info",
			season:    `{"data":{"id":9,"ip":"10.10.11.9","play_info":{"expires_at":"2026-09-18 22:00:00"}}}`,
			wantID:    9,
			wantSpawn: true,
		},
		{
			name:      "spawned flag without an ip",
			season:    `{"data":{"id":9,"ip":null,"play_info":{"is_spawned":true,"expires_at":"2026-09-18 22:00:00"}}}`,
			wantID:    9,
			wantSpawn: true,
		},
		{
			// The real shape of the endpoint for an account that never
			// touched the season machine: a data object full of metadata
			// and a play_info that says nothing is spawned.
			name:      "season machine not spawned",
			season:    `{"data":{"id":984,"name":"Layover","ip":null,"play_info":{"is_spawned":false,"is_spawning":false,"is_active":false,"active_player_count":0,"expires_at":null}}}`,
			wantSpawn: false,
		},
		{
			name:      "missing play info and no ip",
			season:    `{"data":{"id":984,"name":"Layover","ip":null}}`,
			wantSpawn: false,
		},
		{
			name:      "spawning alone is not enough",
			season:    `{"data":{"id":984,"ip":null,"play_info":{"is_spawned":false,"is_spawning":true,"is_active":false}}}`,
			wantSpawn: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/machine/active":
					io.WriteString(w, `{"info":null}`)
				case "/season/machine/active":
					io.WriteString(w, tt.season)
				case "/machine/profile/9":
					io.WriteString(w, `{"info":{"name":"Seasonal","os":"Windows","difficultyText":"Hard","authUserInUserOwns":true,"authUserInRootOwns":true}}`)
				default:
					http.NotFound(w, r)
				}
			})

			activity, err := client.CurrentActivity(context.Background())
			if err != nil {
				t.Fatalf("CurrentActivity: %v", err)
			}
			m := activity.Machine
			if !tt.wantSpawn {
				if m != nil {
					t.Fatalf("Machine = %+v, want nil for an unspawned season machine", m)
				}
				return
			}
			if m == nil || m.ID != tt.wantID || m.Name != "Seasonal" || m.OS != "Windows" || !m.UserOwned || !m.RootOwned {
				t.Fatalf("Machine = %+v", m)
			}
		})
	}
}

func TestVPNConnected(t *testing.T) {
	labs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(labs.Close)
	account := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"product":"labs","connection":{"ip4":"10.10.14.5"}}]`)
	}))
	t.Cleanup(account.Close)

	client := NewClient("aaa.bbb.ccc", WithBaseURL(labs.URL), WithAccountURL(account.URL))
	vpn, err := client.VPNConnected(context.Background())
	if err != nil {
		t.Fatalf("VPNConnected: %v", err)
	}
	if !vpn.Connected || vpn.Product != "labs" {
		t.Errorf("VPN = %+v, want connected labs", vpn)
	}
}

// TestParseConnectionStatusObject covers the object shape recorded by the
// community API docs, including the "not connected" marker, which is a string
// rather than an object.
func TestParseConnectionStatusObject(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		want      bool
		wantError bool
	}{
		{
			name: "documented connected",
			body: `{"status":"1","connection":{"name":"Propolis","ip4":"10.10.14.12","ip6":"dead:beef:2::100a","down":"0","up":"0.01"}}`,
			want: true,
		},
		{
			name: "documented not connected",
			body: `{"status":"0","connection":"not connected"}`,
			want: false,
		},
		{
			name: "status zero with a null connection",
			body: `{"status":"0","connection":null}`,
			want: false,
		},
		{
			name: "tunnel object without a status",
			body: `{"connection":{"ip4":"10.10.14.5"}}`,
			want: true,
		},
		{
			name: "not connected marker without a status",
			body: `{"connection":"not connected"}`,
			want: false,
		},
		{
			name:      "null connection without a status",
			body:      `{"connection":null}`,
			wantError: true,
		},
		{
			name:      "empty object",
			body:      `{}`,
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vpn, err := parseConnectionStatus([]byte(tc.body))
			if tc.wantError {
				if err == nil {
					t.Fatalf("parseConnectionStatus(%s) = %+v, want an error", tc.body, vpn)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConnectionStatus(%s): %v", tc.body, err)
			}
			if vpn.Connected != tc.want {
				t.Errorf("Connected = %v, want %v", vpn.Connected, tc.want)
			}
		})
	}
}

// TestCurrentActivityLive hits the real HTB API. It only runs when HTB_TOKEN is
// set, so the default `go test ./...` stays offline:
//
//	HTB_TOKEN=<app-token> go test ./internal/htb -run Live -v
func TestCurrentActivityLive(t *testing.T) {
	token := os.Getenv("HTB_TOKEN")
	if token == "" {
		t.Skip("set HTB_TOKEN to run this live test against the real HTB API")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	activity, err := NewClient(token).CurrentActivity(ctx)
	if err != nil {
		t.Fatalf("CurrentActivity: %v", err)
	}
	if activity.Machine == nil {
		t.Log("no active machine")
		return
	}
	m := activity.Machine
	t.Logf("active machine: id=%d name=%q os=%q difficulty=%q ip=%q expires_at=%s",
		m.ID, m.Name, m.OS, m.Difficulty, m.IP, m.ExpiresAt)
}

// TestUserLive hits the real HTB API. It only runs when HTB_TOKEN is set.
func TestUserLive(t *testing.T) {
	token := os.Getenv("HTB_TOKEN")
	if token == "" {
		t.Skip("set HTB_TOKEN to run this live test against the real HTB API")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	user, err := NewClient(token).User(ctx)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	t.Logf("user: name=%q rank=%q points=%d", user.Name, user.Rank, user.Points)
}

// TestMachineProfileCache shows that a poll reuses a fresh machine profile and
// refetches it once it expires, the machine changes, or caching is switched off.
func TestMachineProfileCache(t *testing.T) {
	var profileRequests []string
	machineID := "42"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/machine/active":
			io.WriteString(w, `{"info":{"id":`+machineID+`,"expires_at":"2026-09-18 20:00:00"}}`)
		case "/machine/profile/42", "/machine/profile/7":
			profileRequests = append(profileRequests, r.URL.Path)
			io.WriteString(w, `{"info":{"name":"`+r.URL.Path[len("/machine/profile/"):]+`","os":"Linux","difficultyText":"Easy"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	now := time.Unix(1000, 0)
	client := NewClient("aaa.bbb.ccc", WithBaseURL(srv.URL), WithProfileTTL(time.Minute))
	client.now = func() time.Time { return now }

	fetch := func(t *testing.T) *Machine {
		t.Helper()
		activity, err := client.CurrentActivity(context.Background())
		if err != nil {
			t.Fatalf("CurrentActivity: %v", err)
		}
		if activity.Machine == nil {
			t.Fatal("CurrentActivity returned no machine")
		}
		return activity.Machine
	}

	if m := fetch(t); m.Name != "42" {
		t.Fatalf("name = %q, want the fetched profile", m.Name)
	}
	if len(profileRequests) != 1 {
		t.Fatalf("profile requests = %v, want one", profileRequests)
	}

	if m := fetch(t); m.Name != "42" || len(profileRequests) != 1 {
		t.Errorf("a poll inside the TTL refetched the profile: %v", profileRequests)
	}

	now = now.Add(time.Minute)
	fetch(t)
	if len(profileRequests) != 2 {
		t.Errorf("profile requests after the TTL = %v, want a refetch", profileRequests)
	}

	machineID = "7"
	if m := fetch(t); m.Name != "7" {
		t.Errorf("name after switching machines = %q, want the new profile", m.Name)
	}
	if len(profileRequests) != 3 || profileRequests[2] != "/machine/profile/7" {
		t.Errorf("profile requests after switching machines = %v", profileRequests)
	}
}

// TestMachineProfileCacheDisabled shows that a zero TTL fetches the profile on
// every poll.
func TestMachineProfileCacheDisabled(t *testing.T) {
	var profileCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/machine/active":
			io.WriteString(w, `{"info":{"id":42,"expires_at":"2026-09-18 20:00:00"}}`)
		case "/machine/profile/42":
			profileCalls++
			io.WriteString(w, `{"info":{"name":"Uncached","os":"Linux","difficultyText":"Easy"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	client := NewClient("aaa.bbb.ccc", WithBaseURL(srv.URL), WithProfileTTL(0))
	for i := 0; i < 2; i++ {
		if _, err := client.CurrentActivity(context.Background()); err != nil {
			t.Fatalf("CurrentActivity: %v", err)
		}
	}
	if profileCalls != 2 {
		t.Errorf("profile calls = %d, want 2 with caching disabled", profileCalls)
	}
}
