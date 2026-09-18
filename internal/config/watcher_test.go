package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherDetectsChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	changes := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Watcher{Path: path, Poll: 5 * time.Millisecond}.Watch(ctx, func() error {
		changes <- struct{}{}
		return nil
	})

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}

	select {
	case <-changes:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not detect the change")
	}
}

func TestWatcherRetriesAfterError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := make(chan struct{}, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var failFirst atomic.Bool
	failFirst.Store(true)
	go Watcher{Path: path, Poll: 5 * time.Millisecond}.Watch(ctx, func() error {
		calls <- struct{}{}
		if failFirst.CompareAndSwap(true, false) {
			return errors.New("invalid config")
		}
		return nil
	})

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 2; i++ {
		select {
		case <-calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("callback call %d did not happen (change should be retried)", i)
		}
	}
}

func TestWatcherStopsOnCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Watcher{Path: path, Poll: time.Millisecond}.Watch(ctx, func() error { return nil })
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop on context cancellation")
	}
}
