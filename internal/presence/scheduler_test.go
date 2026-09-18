package presence

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/htb"
)

type fakeFetcher struct {
	activity *htb.Activity
	err      error
}

func (f *fakeFetcher) CurrentActivity(context.Context) (*htb.Activity, error) {
	return f.activity, f.err
}

type fakeClient struct {
	mu      sync.Mutex
	updates []*discord.Activity
	err     error
	closed  bool
}

func (c *fakeClient) SetActivity(_ context.Context, a *discord.Activity) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.updates = append(c.updates, a)
	return nil
}

func (c *fakeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeClient) snapshot() []*discord.Activity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*discord.Activity(nil), c.updates...)
}

func active(id int, name string) *htb.Activity {
	return &htb.Activity{Machine: &htb.Machine{
		ID: id, Name: name, OS: "Linux", Difficulty: "Easy",
	}}
}

// testScheduler returns a scheduler wired to client whose tick is driven
// manually by the test (Interval is intentionally large).
func testScheduler(fetcher htb.Fetcher, client DiscordClient) *Scheduler {
	return &Scheduler{
		Fetcher:  fetcher,
		Connect:  func(context.Context) (DiscordClient, error) { return client, nil },
		Interval: time.Hour,
		Options:  Options{ShowTimer: true},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      func() time.Time { return time.Unix(1000, 0) },
	}
}

func TestTickPublishesThenSkipsUnchanged(t *testing.T) {
	client := &fakeClient{}
	s := testScheduler(&fakeFetcher{activity: active(289, "Vaccine")}, client)

	s.tick(context.Background())
	s.tick(context.Background())

	updates := client.snapshot()
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1 (second tick is unchanged)", len(updates))
	}
	if updates[0].Details != "Vaccine" {
		t.Errorf("Details = %q, want Vaccine", updates[0].Details)
	}
}

func TestTickPublishesOnChange(t *testing.T) {
	fetcher := &fakeFetcher{activity: active(289, "Vaccine")}
	client := &fakeClient{}
	s := testScheduler(fetcher, client)

	s.tick(context.Background())
	fetcher.activity = active(290, "Keeper")
	s.tick(context.Background())

	updates := client.snapshot()
	if len(updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(updates))
	}
	if updates[1].Details != "Keeper" {
		t.Errorf("second Details = %q, want Keeper", updates[1].Details)
	}
}

func TestTickPublishesIdle(t *testing.T) {
	client := &fakeClient{}
	s := testScheduler(&fakeFetcher{}, client)

	s.tick(context.Background())

	updates := client.snapshot()
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(updates))
	}
	if updates[0].Details != "Hack The Box" {
		t.Errorf("Details = %q, want Hack The Box", updates[0].Details)
	}
}

func TestTickKeepsLastPresenceOnFetchError(t *testing.T) {
	fetcher := &fakeFetcher{activity: active(289, "Vaccine")}
	client := &fakeClient{}
	s := testScheduler(fetcher, client)

	s.tick(context.Background())
	fetcher.err = errors.New("network down")
	s.tick(context.Background())

	if updates := client.snapshot(); len(updates) != 1 {
		t.Fatalf("updates = %d, want 1 (no clear on fetch error)", len(updates))
	}
}

func TestTickRetriesWhenDiscordUnavailable(t *testing.T) {
	client := &fakeClient{}
	connects := 0
	s := testScheduler(&fakeFetcher{activity: active(289, "Vaccine")}, client)
	s.Connect = func(context.Context) (DiscordClient, error) {
		connects++
		if connects == 1 {
			return nil, errors.New("discord not running")
		}
		return client, nil
	}

	s.tick(context.Background())
	if updates := client.snapshot(); len(updates) != 0 {
		t.Fatalf("updates = %d, want 0 while disconnected", len(updates))
	}

	s.tick(context.Background())
	if updates := client.snapshot(); len(updates) != 1 {
		t.Fatalf("updates = %d, want 1 after reconnect", len(updates))
	}
}

func TestTickReconnectsAfterPublishFailure(t *testing.T) {
	failing := &fakeClient{err: errors.New("ipc dropped")}
	healthy := &fakeClient{}
	connects := 0

	s := testScheduler(&fakeFetcher{activity: active(289, "Vaccine")}, failing)
	s.Connect = func(context.Context) (DiscordClient, error) {
		connects++
		if connects == 1 {
			return failing, nil
		}
		return healthy, nil
	}

	s.tick(context.Background())
	if !failing.closed {
		t.Error("failing client was not closed after a publish error")
	}

	s.tick(context.Background())

	if connects != 2 {
		t.Errorf("connects = %d, want 2", connects)
	}
	if updates := healthy.snapshot(); len(updates) != 1 {
		t.Errorf("healthy updates = %d, want 1 (republished after reconnect)", len(updates))
	}
}

func TestRunClearsPresenceOnShutdown(t *testing.T) {
	client := &fakeClient{}
	s := testScheduler(&fakeFetcher{activity: active(289, "Vaccine")}, client)
	s.Interval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	waitFor(t, func() bool { return len(client.snapshot()) > 0 })
	cancel()
	<-done

	updates := client.snapshot()
	if last := updates[len(updates)-1]; last != nil {
		t.Errorf("last update = %+v, want nil (presence cleared on shutdown)", last)
	}
	if !client.closed {
		t.Error("client was not closed on shutdown")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition was not met before the deadline")
}
