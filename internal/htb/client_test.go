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
			io.WriteString(w, `{"info":{"name":"Keeper","os":"Linux","difficultyText":"Easy"}}`)
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
	if user.Name != "Yodev1" || user.Rank != "Noob" || user.Points != 120 {
		t.Errorf("User = %+v, want Yodev1/Noob/120", user)
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
