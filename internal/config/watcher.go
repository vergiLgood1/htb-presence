package config

import (
	"context"
	"crypto/sha256"
	"os"
	"time"
)

// DefaultWatchInterval is how often a Watcher checks the config file.
const DefaultWatchInterval = 5 * time.Second

// Watcher polls a config file for content changes.
//
// It polls instead of using an OS notification API so the project stays
// dependency-free; config edits are rare, so a few seconds of latency is fine.
type Watcher struct {
	// Path is the config file to watch.
	Path string

	// Poll is the interval between checks; defaults to DefaultWatchInterval.
	Poll time.Duration
}

// Watch calls onChange whenever the file's contents change, until ctx is done.
// The initial contents do not trigger a call. If onChange returns an error the
// change is retried on the next poll.
func (w Watcher) Watch(ctx context.Context, onChange func() error) {
	poll := w.Poll
	if poll <= 0 {
		poll = DefaultWatchInterval
	}

	last := w.fingerprint()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := w.fingerprint()
			if current == last {
				continue
			}
			if err := onChange(); err == nil {
				last = current
			}
		}
	}
}

// fingerprint returns a hash of the file's contents, or the zero hash when the
// file cannot be read.
func (w Watcher) fingerprint() [sha256.Size]byte {
	data, err := os.ReadFile(w.Path)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(data)
}
