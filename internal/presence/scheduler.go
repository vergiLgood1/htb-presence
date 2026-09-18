package presence

import (
	"context"
	"errors"
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

// DefaultMaxBackoff caps the delay between retries after repeated failures.
const DefaultMaxBackoff = 10 * time.Minute

// Scheduler polls HTB and keeps Discord Rich Presence in sync.
//
// It fails soft: a failed HTB fetch keeps the last known presence, and a failed
// Discord call drops the connection so the next poll reconnects. The delay
// between polls backs off while failures continue and returns to Interval once
// they clear.
type Scheduler struct {
	Fetcher  htb.Fetcher
	Connect  ClientFactory
	Interval time.Duration
	Options  Options

	// MaxBackoff caps the delay between retries; defaults to DefaultMaxBackoff.
	MaxBackoff time.Duration

	// Logger receives diagnostic output; defaults to slog.Default.
	Logger *slog.Logger

	// Now supplies the current time, for tests. Defaults to time.Now.
	Now func() time.Time

	client    DiscordClient
	session   session
	last      *discord.Activity
	published bool
	failures  int
}

// Run polls until ctx is cancelled, then clears the presence and disconnects.
func (s *Scheduler) Run(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			s.shutdown()
			return
		case <-timer.C:
			timer.Reset(s.tick(ctx))
		}
	}
}

// tick runs one poll cycle and returns the delay until the next one.
func (s *Scheduler) tick(ctx context.Context) time.Duration {
	activity, err := s.Fetcher.CurrentActivity(ctx)
	if err != nil {
		s.failures++
		delay := s.retryDelay(err)
		s.logger().Warn("fetching HTB activity failed, keeping last presence",
			"error", err, "retry_in", delay)
		return delay
	}

	if s.client == nil {
		client, err := s.Connect(ctx)
		if err != nil {
			s.failures++
			delay := s.retryDelay(err)
			s.logger().Warn("discord unavailable, will retry", "error", err, "retry_in", delay)
			return delay
		}
		s.client = client
		s.published = false
		s.logger().Info("connected to discord")
	}

	start := s.session.Start(machineID(activity), s.now())
	next := Map(activity, Options{ShowTimer: s.Options.ShowTimer, SessionStart: start})

	if s.published && s.last != nil && *s.last == *next {
		s.failures = 0
		return s.interval()
	}

	if err := s.client.SetActivity(ctx, next); err != nil {
		s.failures++
		delay := s.retryDelay(err)
		s.logger().Warn("updating discord presence failed, will reconnect",
			"error", err, "retry_in", delay)
		s.client.Close()
		s.client = nil
		s.published = false
		return delay
	}

	s.last = next
	s.published = true
	s.failures = 0
	s.logger().Info("presence updated", "details", next.Details, "state", next.State)
	return s.interval()
}

// retryDelay returns how long to wait before the next attempt after a failure.
//
// Rate limits honor Retry-After; authentication failures retry at a slower but
// fixed interval (the user may fix the token without restarting); everything
// else backs off exponentially, capped by MaxBackoff.
func (s *Scheduler) retryDelay(err error) time.Duration {
	interval := s.interval()

	var rateLimit *htb.RateLimitError
	if errors.As(err, &rateLimit) && rateLimit.RetryAfter > interval {
		return rateLimit.RetryAfter
	}

	if errors.Is(err, htb.ErrAuth) {
		return min(5*interval, s.maxBackoff())
	}

	exponent := s.failures - 1
	if exponent < 0 {
		exponent = 0
	}
	if exponent > 10 {
		exponent = 10
	}
	return min(interval<<exponent, s.maxBackoff())
}

// interval returns the configured poll interval, defaulting to one second.
func (s *Scheduler) interval() time.Duration {
	if s.Interval > 0 {
		return s.Interval
	}
	return time.Second
}

// maxBackoff returns the configured backoff cap, defaulting to DefaultMaxBackoff.
func (s *Scheduler) maxBackoff() time.Duration {
	if s.MaxBackoff > 0 {
		return s.MaxBackoff
	}
	return DefaultMaxBackoff
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
