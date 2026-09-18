package presence

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/vergiLgood1/htb-presence/internal/discord"
	"github.com/vergiLgood1/htb-presence/internal/history"
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

// SessionRecorder records completed sessions. It is optional.
type SessionRecorder interface {
	Record(history.Session) error
}

// DefaultMaxBackoff caps the delay between retries after repeated failures.
const DefaultMaxBackoff = 10 * time.Minute

// DefaultRankRefresh is how long a fetched rank is reused before refreshing.
// Rank and points change slowly, so this avoids an extra API call every poll.
const DefaultRankRefresh = 10 * time.Minute

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

	// History optionally records completed sessions; nil disables it.
	History SessionRecorder

	// MaxBackoff caps the delay between retries; defaults to DefaultMaxBackoff.
	MaxBackoff time.Duration

	// Logger receives diagnostic output; defaults to slog.Default.
	Logger *slog.Logger

	// RankRefresh is how long a fetched rank is reused before refreshing;
	// defaults to DefaultRankRefresh.
	RankRefresh time.Duration

	// Now supplies the current time, for tests. Defaults to time.Now.
	Now func() time.Time

	client        DiscordClient
	session       session
	last          *discord.Activity
	published     bool
	failures      int
	user          *htb.User
	userFetchedAt time.Time
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

	if s.Options.ShowRank {
		user, err := s.cachedUser(ctx)
		if err != nil {
			s.logger().Warn("fetching HTB rank failed, using the cached value if any", "error", err)
		}
		if user != nil {
			activity.User = user
		}
	}

	start, ended := s.session.observe(machine(activity), s.now())
	if ended != nil && s.History != nil {
		if err := s.History.Record(*ended); err != nil {
			s.logger().Warn("recording session history failed", "error", err)
		}
	}

	next := Map(activity, Options{
		ShowMachineName: s.Options.ShowMachineName,
		ShowRank:        s.Options.ShowRank,
		ShowTimer:       s.Options.ShowTimer,
		SessionStart:    start,
	})

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

// cachedUser returns the user's rank, refreshing it at most every RankRefresh.
// On a refresh error it returns the last known value alongside the error.
func (s *Scheduler) cachedUser(ctx context.Context) (*htb.User, error) {
	if s.user != nil && s.now().Sub(s.userFetchedAt) < s.rankRefresh() {
		return s.user, nil
	}

	user, err := s.Fetcher.User(ctx)
	if err != nil {
		return s.user, err
	}
	s.user = user
	s.userFetchedAt = s.now()
	return user, nil
}

// rankRefresh returns the configured rank refresh interval.
func (s *Scheduler) rankRefresh() time.Duration {
	if s.RankRefresh > 0 {
		return s.RankRefresh
	}
	return DefaultRankRefresh
}

// shutdown clears the presence, records any in-flight session, and closes the
// Discord connection.
func (s *Scheduler) shutdown() {
	if s.session.machineID != 0 && s.History != nil {
		ended := history.Session{
			MachineID:   s.session.machineID,
			MachineName: s.session.machineName,
			StartedAt:   s.session.start,
			EndedAt:     s.now(),
		}
		if err := s.History.Record(ended); err != nil {
			s.logger().Warn("recording session history failed", "error", err)
		}
		s.session = session{}
	}

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

// machine returns the active machine, or nil when there is none.
func machine(activity *htb.Activity) *htb.Machine {
	if activity == nil {
		return nil
	}
	return activity.Machine
}

// session tracks the currently observed machine session.
type session struct {
	machineID   int
	machineName string
	start       time.Time
}

// observe updates the session for the given machine (nil when idle) and returns
// the session start time, along with the previous session if it just ended.
func (s *session) observe(m *htb.Machine, now time.Time) (time.Time, *history.Session) {
	id, name := 0, ""
	if m != nil {
		id, name = m.ID, m.Name
	}

	if id == s.machineID {
		return s.start, nil
	}

	var ended *history.Session
	if s.machineID != 0 {
		ended = &history.Session{
			MachineID:   s.machineID,
			MachineName: s.machineName,
			StartedAt:   s.start,
			EndedAt:     now,
		}
	}

	s.machineID, s.machineName = id, name
	if id == 0 {
		s.start = time.Time{}
	} else {
		s.start = now
	}
	return s.start, ended
}
