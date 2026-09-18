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

// Session is one completed machine session.
type Session struct {
	MachineID   int       `json:"machine_id"`
	MachineName string    `json:"machine_name"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
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
