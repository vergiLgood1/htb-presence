package presence

import (
	"context"
	"log/slog"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/htb"
)

// DiscordClient is the part of a Discord IPC client the scheduler uses.
type DiscordClient interface {
	// SetActivity publishes activity; a nil activity clears the presence.
	SetActivity(ctx context.Context, activity *discord.Activity) error
	// Close releases the IPC connection.
	Close() error
}

// ClientFactory connects to the local Discord client.
type ClientFactory func(ctx context.Context) (DiscordClient, error)

// Scheduler polls HTB and keeps Discord Rich Presence in sync.
//
// It fails soft: a failed HTB fetch keeps the last known presence, and a failed
// Discord call drops the connection so the next tick reconnects.
type Scheduler struct {
	Fetcher  htb.Fetcher
	Connect  ClientFactory
	Interval time.Duration
	Options  Options

	// Logger receives diagnostic output; defaults to slog.Default.
	Logger *slog.Logger

	// Now supplies the current time, for tests. Defaults to time.Now.
	Now func() time.Time

	client    DiscordClient
	session   session
	last      *discord.Activity
	published bool
}

// Run polls until ctx is cancelled, then clears the presence and disconnects.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			s.shutdown()
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs one poll cycle.
func (s *Scheduler) tick(ctx context.Context) {
	if s.client == nil {
		client, err := s.Connect(ctx)
		if err != nil {
			s.logger().Warn("discord unavailable, will retry next poll", "error", err)
			return
		}
		s.client = client
		s.published = false
		s.logger().Info("connected to discord")
	}

	activity, err := s.Fetcher.CurrentActivity(ctx)
	if err != nil {
		s.logger().Warn("fetching HTB activity failed, keeping last presence", "error", err)
		return
	}

	start := s.session.Start(machineID(activity), s.now())
	next := Map(activity, Options{ShowTimer: s.Options.ShowTimer, SessionStart: start})

	if s.published && s.last != nil && *s.last == *next {
		return
	}

	if err := s.client.SetActivity(ctx, next); err != nil {
		s.logger().Warn("updating discord presence failed, will reconnect", "error", err)
		s.client.Close()
		s.client = nil
		s.published = false
		return
	}

	s.last = next
	s.published = true
	s.logger().Info("presence updated", "details", next.Details, "state", next.State)
}

// shutdown clears the presence and closes the Discord connection.
func (s *Scheduler) shutdown() {
	if s.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.client.SetActivity(ctx, nil); err != nil {
		s.logger().Warn("clearing presence failed", "error", err)
	}
	if err := s.client.Close(); err != nil {
		s.logger().Warn("closing discord connection failed", "error", err)
	}
	s.client = nil
}

func (s *Scheduler) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// machineID returns the active machine's id, or 0 when there is none.
func machineID(activity *htb.Activity) int {
	if activity == nil || activity.Machine == nil {
		return 0
	}
	return activity.Machine.ID
}

// session remembers when the observed machine session started, so the presence
// timer stays stable across polls.
type session struct {
	machineID int
	start     time.Time
}

// Start returns the session start time for the given machine, resetting it when
// the active machine changes. It returns the zero time when idle.
func (s *session) Start(machineID int, now time.Time) time.Time {
	if machineID == 0 {
		s.machineID = 0
		s.start = time.Time{}
		return time.Time{}
	}
	if machineID != s.machineID {
		s.machineID = machineID
		s.start = now
	}
	return s.start
}
