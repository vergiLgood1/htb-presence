package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderAppendsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	r, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { r.Close() })

	first := Session{
		MachineID:   289,
		MachineName: "Vaccine",
		StartedAt:   time.Unix(1000, 0).UTC(),
		EndedAt:     time.Unix(4600, 0).UTC(),
	}
	second := Session{
		MachineID:   290,
		MachineName: "Keeper",
		StartedAt:   time.Unix(5000, 0).UTC(),
		EndedAt:     time.Unix(9000, 0).UTC(),
	}
	for _, s := range []Session{first, second} {
		if err := r.Record(s); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading history: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("history has %d lines, want 2", len(lines))
	}

	var got Session
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("decoding line 1: %v", err)
	}
	if got != first {
		t.Errorf("line 1 = %+v, want %+v", got, first)
	}
}

func TestNilRecorderIsSafe(t *testing.T) {
	var r *Recorder
	if err := r.Record(Session{MachineID: 1}); err != nil {
		t.Errorf("Record on nil recorder = %v, want nil", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close on nil recorder = %v, want nil", err)
	}
}

func TestSummarize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sessions := []Session{
		{Kind: "machine", MachineID: 1, MachineName: "Vaccine", StartedAt: time.Unix(0, 0).UTC(), EndedAt: time.Unix(3600, 0).UTC()},
		{Kind: "machine", MachineID: 1, MachineName: "Vaccine", StartedAt: time.Unix(4000, 0).UTC(), EndedAt: time.Unix(4000+1800, 0).UTC()},
		{Kind: "challenge", ChallengeID: 7, ChallengeName: "Phonebook", StartedAt: time.Unix(8000, 0).UTC(), EndedAt: time.Unix(8000+600, 0).UTC()},
	}
	for _, s := range sessions {
		if err := r.Record(s); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()

	summary, err := SummarizeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Sessions != 3 || summary.Unique != 2 {
		t.Fatalf("summary = %+v, want 3 sessions and 2 targets", summary)
	}
	if summary.Targets[0].Name != "Vaccine" || summary.Targets[0].Total != 90*time.Minute {
		t.Fatalf("top target = %+v, want Vaccine 90m", summary.Targets[0])
	}
	if !strings.Contains(Format(summary), "Vaccine") {
		t.Fatalf("Format = %q", Format(summary))
	}
}

func TestOpenCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested.jsonl")
	if _, err := Open(path); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("history file was not created: %v", err)
	}
}
