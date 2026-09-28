package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// TargetStat is the time spent on one machine or challenge.
type TargetStat struct {
	Name     string
	Sessions int
	Total    time.Duration
}

// Summary is a roll-up of a session history file.
type Summary struct {
	Sessions int
	Total    time.Duration
	Unique   int
	Longest  Session
	Targets  []TargetStat
}

// SummarizeFile reads a JSONL history file and rolls it up.
func SummarizeFile(path string) (Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return Summary{}, fmt.Errorf("opening history file: %w", err)
	}
	defer f.Close()
	return Summarize(f)
}

// Summarize rolls up JSONL session records from r. A malformed line is an error.
func Summarize(r io.Reader) (Summary, error) {
	var summary Summary
	byKey := map[string]*TargetStat{}
	order := []string{}

	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var session Session
		if err := json.Unmarshal([]byte(line), &session); err != nil {
			return Summary{}, fmt.Errorf("history line %d: %w", lineNo, err)
		}
		dur := session.EndedAt.Sub(session.StartedAt)
		if dur < 0 {
			dur = 0
		}
		summary.Sessions++
		summary.Total += dur
		if summary.Longest.StartedAt.IsZero() || dur > summary.Longest.EndedAt.Sub(summary.Longest.StartedAt) {
			summary.Longest = session
		}

		key := session.key()
		stat, ok := byKey[key]
		if !ok {
			stat = &TargetStat{Name: session.Name()}
			byKey[key] = stat
			order = append(order, key)
		}
		stat.Sessions++
		stat.Total += dur
	}
	if err := sc.Err(); err != nil {
		return Summary{}, fmt.Errorf("reading history: %w", err)
	}

	summary.Unique = len(byKey)
	summary.Targets = make([]TargetStat, 0, len(order))
	for _, key := range order {
		summary.Targets = append(summary.Targets, *byKey[key])
	}
	sort.Slice(summary.Targets, func(i, j int) bool {
		if summary.Targets[i].Total == summary.Targets[j].Total {
			return summary.Targets[i].Name < summary.Targets[j].Name
		}
		return summary.Targets[i].Total > summary.Targets[j].Total
	})
	return summary, nil
}

// Format renders a summary for the terminal.
func Format(s Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "sessions: %d\n", s.Sessions)
	fmt.Fprintf(&b, "total:    %s\n", s.Total.Round(time.Second))
	fmt.Fprintf(&b, "unique:   %d\n", s.Unique)
	if s.Sessions > 0 {
		fmt.Fprintf(&b, "longest:  %s (%s)\n", s.Longest.Name(), s.Longest.EndedAt.Sub(s.Longest.StartedAt).Round(time.Second))
	}
	if len(s.Targets) > 0 {
		b.WriteString("by time:\n")
	}
	limit := len(s.Targets)
	if limit > 5 {
		limit = 5
	}
	for _, target := range s.Targets[:limit] {
		fmt.Fprintf(&b, "  %s  %s  (%d)\n", target.Name, target.Total.Round(time.Second), target.Sessions)
	}
	return b.String()
}
