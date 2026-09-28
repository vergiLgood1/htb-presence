// Package history appends a local JSONL log of completed HTB sessions.
//
// History is optional and disabled unless a file is configured; entries stay on
// the user's machine and are never uploaded anywhere.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Session is one completed machine or challenge session.
type Session struct {
	// Kind is "machine" or "challenge". Empty is treated as a machine so
	// records written before challenges were tracked still summarize.
	Kind          string    `json:"kind,omitempty"`
	MachineID     int       `json:"machine_id,omitempty"`
	MachineName   string    `json:"machine_name,omitempty"`
	ChallengeID   int       `json:"challenge_id,omitempty"`
	ChallengeName string    `json:"challenge_name,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
}

// Name is the display name of the target.
func (s Session) Name() string {
	if s.ChallengeName != "" {
		return s.ChallengeName
	}
	if s.MachineName != "" {
		return s.MachineName
	}
	return "unknown"
}

// key groups sessions that are the same target.
func (s Session) key() string {
	if s.ChallengeID != 0 {
		return fmt.Sprintf("c:%d", s.ChallengeID)
	}
	if s.MachineID != 0 {
		return fmt.Sprintf("m:%d", s.MachineID)
	}
	return "n:" + s.Name()
}

// Recorder appends sessions to a JSONL file. The zero value and a nil
// *Recorder are both safe to use and discard the data.
type Recorder struct {
	mu sync.Mutex
	f  *os.File
}

// Open opens (creating if needed) the history file at path for appending.
func Open(path string) (*Recorder, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening history file: %w", err)
	}
	return &Recorder{f: f}, nil
}

// Record appends one session as a JSON line.
func (r *Recorder) Record(s Session) error {
	if r == nil || r.f == nil {
		return nil
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encoding session: %w", err)
	}
	data = append(data, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.f.Write(data); err != nil {
		return fmt.Errorf("writing session: %w", err)
	}
	return nil
}

// Close closes the underlying file.
func (r *Recorder) Close() error {
	if r == nil || r.f == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}
