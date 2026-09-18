//go:build !windows

package discord

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDiscord starts a Unix-socket server that runs handler for the first
// connection, mimicking a local Discord client. It returns the socket path.
func fakeDiscord(t *testing.T, handler func(conn net.Conn)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "discord-ipc-0")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listening on fake socket: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()
	return path
}

// readHandshake reads and validates the opening handshake, replying READY.
func readHandshake(t *testing.T, conn net.Conn, clientID string) {
	t.Helper()
	op, data, err := readFrame(conn)
	if err != nil {
		t.Errorf("reading handshake: %v", err)
		return
	}
	if op != opHandshake {
		t.Errorf("first frame op = %d, want %d (handshake)", op, opHandshake)
		return
	}

	var hs struct {
		V        int    `json:"v"`
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(data, &hs); err != nil {
		t.Errorf("decoding handshake: %v", err)
		return
	}
	if hs.V != 1 || hs.ClientID != clientID {
		t.Errorf("handshake = %+v, want v=1 client_id=%q", hs, clientID)
	}

	if err := writeFrame(conn, opFrame, map[string]any{"cmd": "DISPATCH", "evt": "READY"}); err != nil {
		t.Errorf("replying READY: %v", err)
	}
}

func TestSetActivity(t *testing.T) {
	got := make(chan map[string]any, 1)
	path := fakeDiscord(t, func(conn net.Conn) {
		readHandshake(t, conn, "123")

		op, data, err := readFrame(conn)
		if err != nil {
			t.Errorf("reading SET_ACTIVITY: %v", err)
			return
		}
		if op != opFrame {
			t.Errorf("op = %d, want frame", op)
		}
		var req map[string]any
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("decoding SET_ACTIVITY: %v", err)
			return
		}
		got <- req

		writeFrame(conn, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "evt": nil})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, "123", WithSocketPath(path))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	start := time.Unix(1000, 0)
	err = client.SetActivity(ctx, &Activity{
		Details:    "Vaccine",
		State:      "Linux · Very Easy",
		LargeImage: "htb",
		StartTime:  start,
	})
	if err != nil {
		t.Fatalf("SetActivity: %v", err)
	}

	req := <-got
	if req["cmd"] != "SET_ACTIVITY" {
		t.Errorf("cmd = %v, want SET_ACTIVITY", req["cmd"])
	}
	if nonce, _ := req["nonce"].(string); nonce == "" {
		t.Error("nonce is missing")
	}

	args, ok := req["args"].(map[string]any)
	if !ok {
		t.Fatalf("args = %v, want an object", req["args"])
	}
	if pid, ok := args["pid"].(float64); !ok || pid <= 0 {
		t.Errorf("args.pid = %v, want a positive pid", args["pid"])
	}

	activity, ok := args["activity"].(map[string]any)
	if !ok {
		t.Fatalf("args.activity = %v, want an object", args["activity"])
	}
	if activity["details"] != "Vaccine" {
		t.Errorf("details = %v, want Vaccine", activity["details"])
	}
	if activity["state"] != "Linux · Very Easy" {
		t.Errorf("state = %v", activity["state"])
	}
	timestamps, ok := activity["timestamps"].(map[string]any)
	if !ok {
		t.Fatalf("timestamps = %v, want an object", activity["timestamps"])
	}
	if int64(timestamps["start"].(float64)) != start.UnixMilli() {
		t.Errorf("timestamps.start = %v, want %d", timestamps["start"], start.UnixMilli())
	}
}

func TestSetActivityClear(t *testing.T) {
	got := make(chan map[string]any, 1)
	path := fakeDiscord(t, func(conn net.Conn) {
		readHandshake(t, conn, "123")
		_, data, err := readFrame(conn)
		if err != nil {
			return
		}
		var req map[string]any
		json.Unmarshal(data, &req)
		got <- req
		writeFrame(conn, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "evt": nil})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, "123", WithSocketPath(path))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	if err := client.SetActivity(ctx, nil); err != nil {
		t.Fatalf("SetActivity(nil): %v", err)
	}

	args := (<-got)["args"].(map[string]any)
	if args["activity"] != nil {
		t.Errorf("args.activity = %v, want nil", args["activity"])
	}
}

func TestSetActivityAnswersPing(t *testing.T) {
	done := make(chan error, 1)
	path := fakeDiscord(t, func(conn net.Conn) {
		readHandshake(t, conn, "123")
		if _, _, err := readFrame(conn); err != nil {
			done <- err
			return
		}

		if err := writeRawFrame(conn, opPing, []byte(`{"seq":1}`)); err != nil {
			done <- err
			return
		}
		op, data, err := readFrame(conn)
		if err != nil {
			done <- err
			return
		}
		if op != opPong {
			done <- errors.New("expected a pong reply")
			return
		}
		if string(data) != `{"seq":1}` {
			done <- errors.New("pong payload did not match the ping")
			return
		}
		writeFrame(conn, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "evt": nil})
		done <- nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Dial(ctx, "123", WithSocketPath(path))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	if err := client.SetActivity(ctx, &Activity{Details: "Vaccine"}); err != nil {
		t.Fatalf("SetActivity: %v", err)
	}
	if err := <-done; err != nil {
		t.Errorf("server: %v", err)
	}
}

func TestSetActivityRejected(t *testing.T) {
	path := fakeDiscord(t, func(conn net.Conn) {
		readHandshake(t, conn, "123")
		if _, _, err := readFrame(conn); err != nil {
			return
		}
		writeFrame(conn, opFrame, map[string]any{
			"cmd": "SET_ACTIVITY",
			"evt": "ERROR",
			"data": map[string]any{
				"code":    4000,
				"message": "invalid activity",
			},
		})
	})

	client, err := Dial(context.Background(), "123", WithSocketPath(path))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close()

	err = client.SetActivity(context.Background(), &Activity{Details: "Vaccine"})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v, want a rejection error", err)
	}
}

func TestDialNotRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := Dial(context.Background(), "123", WithSocketPath(path)); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
}

func TestDialHandshakeRejected(t *testing.T) {
	path := fakeDiscord(t, func(conn net.Conn) {
		if _, _, err := readFrame(conn); err != nil {
			return
		}
		writeFrame(conn, opFrame, map[string]any{"cmd": "DISPATCH", "evt": "ERROR"})
	})

	_, err := Dial(context.Background(), "123", WithSocketPath(path))
	if err == nil {
		t.Fatal("expected handshake error, got nil")
	}
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("err = %v, want ErrProtocol", err)
	}
}
