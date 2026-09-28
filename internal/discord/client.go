// Package discord implements the client side of Discord's Rich Presence IPC
// protocol: it connects to the locally running Discord client, performs the
// handshake, and publishes activity payloads.
//
// Discord exposes Rich Presence over a local socket (a Unix socket on
// Linux/macOS, a named pipe on Windows), so this package only works when the
// Discord desktop client is running on the same machine.
package discord

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Sentinel errors callers can match with errors.Is.
var (
	// ErrNotRunning indicates no local Discord client could be reached.
	ErrNotRunning = errors.New("discord: client is not running")

	// ErrClosed indicates the IPC connection was closed by Discord (or by us).
	ErrClosed = errors.New("discord: connection closed")

	// ErrProtocol indicates Discord sent a response this client did not
	// understand.
	ErrProtocol = errors.New("discord: protocol error")
)

// defaultTimeout bounds a request/response exchange when the caller passes a
// context without a deadline.
const defaultTimeout = 10 * time.Second

// Publisher publishes Rich Presence updates.
type Publisher interface {
	// SetActivity publishes activity. A nil activity clears the presence.
	SetActivity(ctx context.Context, activity *Activity) error
	// Close releases the underlying IPC connection.
	Close() error
}

// Client is a connected Discord Rich Presence IPC client.
type Client struct {
	clientID string
	conn     net.Conn

	// writeMu guards writes to conn; ioMu serializes whole request/response
	// exchanges (including the times writes happen inside them).
	writeMu sync.Mutex
	ioMu    sync.Mutex
}

// Option configures Dial.
type Option func(*dialConfig)

type dialConfig struct {
	socketPath string
}

// WithSocketPath makes Dial use exactly this socket/pipe path instead of
// probing the platform's usual locations. It is mainly useful in tests.
func WithSocketPath(path string) Option {
	return func(c *dialConfig) { c.socketPath = path }
}

// Dial connects to a running Discord client and completes the handshake for the
// given application client ID.
//
// Candidate sockets are probed in order. A client that answers but rejects the
// handshake is reported as a handshake failure: that means Discord is running and
// the client ID (or the login state) is the problem, which a dead socket path
// would only obscure.
func Dial(ctx context.Context, clientID string, opts ...Option) (*Client, error) {
	var cfg dialConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	paths := candidatePaths()
	if cfg.socketPath != "" {
		paths = []string{cfg.socketPath}
	}

	var dialErr error
	var handshakeErr error
	for _, path := range paths {
		conn, err := dialPath(ctx, path)
		if err != nil {
			dialErr = err
			continue
		}

		client := &Client{clientID: clientID, conn: conn}
		if err := client.handshake(ctx); err != nil {
			conn.Close()
			// Keep the first handshake failure: candidates are ordered with
			// the most likely socket first, so a later stale one would only
			// make the message less useful.
			if handshakeErr == nil {
				handshakeErr = fmt.Errorf("over %s: %w", path, err)
			}
			continue
		}
		return client, nil
	}

	switch {
	case handshakeErr != nil:
		return nil, fmt.Errorf("discord: handshake failed, check discord.client_id: %w", handshakeErr)
	case dialErr != nil:
		return nil, fmt.Errorf("%w: %w", ErrNotRunning, dialErr)
	default:
		return nil, fmt.Errorf("%w: no Discord IPC socket found", ErrNotRunning)
	}
}

// handshake performs the initial protocol handshake.
func (c *Client) handshake(ctx context.Context) error {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()

	c.setDeadline(ctx)
	defer c.clearDeadline()

	c.writeMu.Lock()
	err := writeFrame(c.conn, opHandshake, map[string]any{"v": 1, "client_id": c.clientID})
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("sending handshake: %w", err)
	}

	reply, err := c.readReply()
	if err != nil {
		return fmt.Errorf("waiting for handshake reply: %w", err)
	}

	var ready struct {
		Cmd string `json:"cmd"`
		Evt string `json:"evt"`
	}
	if err := json.Unmarshal(reply, &ready); err != nil {
		return fmt.Errorf("%w: decoding handshake reply: %v", ErrProtocol, err)
	}
	if ready.Evt != "READY" {
		return fmt.Errorf("%w: handshake not acknowledged (evt=%q)", ErrProtocol, ready.Evt)
	}
	return nil
}

// SetActivity implements Publisher.
func (c *Client) SetActivity(ctx context.Context, activity *Activity) error {
	var payload any
	if activity != nil {
		payload = activity.payload()
	}

	frame := map[string]any{
		"cmd":   "SET_ACTIVITY",
		"args":  map[string]any{"pid": os.Getpid(), "activity": payload},
		"nonce": newNonce(),
	}

	reply, err := c.exchange(ctx, frame)
	if err != nil {
		return err
	}

	var resp struct {
		Evt  string `json:"evt"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(reply, &resp); err == nil && resp.Evt == "ERROR" {
		return fmt.Errorf("discord: SET_ACTIVITY rejected: %s", resp.Data.Message)
	}
	return nil
}

// Close implements Publisher.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// exchange sends a frame and waits for the matching reply.
func (c *Client) exchange(ctx context.Context, payload any) ([]byte, error) {
	c.ioMu.Lock()
	defer c.ioMu.Unlock()

	c.setDeadline(ctx)
	defer c.clearDeadline()

	c.writeMu.Lock()
	err := writeFrame(c.conn, opFrame, payload)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("sending frame: %w", err)
	}
	return c.readReply()
}

// readReply reads frames until a data frame arrives, answering pings and pongs
// along the way. The caller must hold ioMu.
//
// A peer that hangs up mid-exchange, with or without a close frame, is reported
// as ErrClosed so callers see one "the connection went away" signal instead of a
// bare EOF.
func (c *Client) readReply() ([]byte, error) {
	for {
		op, data, err := readFrame(c.conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("%w: %v", ErrClosed, err)
			}
			return nil, err
		}

		switch op {
		case opPing:
			c.writeMu.Lock()
			err := writeRawFrame(c.conn, opPong, data)
			c.writeMu.Unlock()
			if err != nil {
				return nil, fmt.Errorf("replying to ping: %w", err)
			}
		case opPong:
			continue
		case opClose:
			return nil, ErrClosed
		case opFrame:
			return data, nil
		default:
			continue
		}
	}
}

// setDeadline applies the context deadline (or a default) to the connection.
func (c *Client) setDeadline(ctx context.Context) {
	if deadline, ok := ctx.Deadline(); ok {
		c.conn.SetDeadline(deadline)
		return
	}
	c.conn.SetDeadline(time.Now().Add(defaultTimeout))
}

// clearDeadline removes any previously applied deadline.
func (c *Client) clearDeadline() {
	c.conn.SetDeadline(time.Time{})
}

// newNonce returns a random nonce used to correlate requests and replies.
func newNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}
