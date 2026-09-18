//go:build windows

package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// windowsPipeSupport is false until named-pipe transport is implemented.
//
// Discord exposes Rich Presence over named pipes on Windows
// (\\.\pipe\discord-ipc-N). Go's standard library cannot dial named pipes, so
// this needs either a small syscall implementation or a dependency such as
// github.com/Microsoft/go-winio. Tracked as a Phase 4 (distribution) task.
const windowsPipeSupport = false

// candidatePaths returns the Windows named-pipe paths Discord listens on.
func candidatePaths() []string {
	paths := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		paths = append(paths, fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i))
	}
	return paths
}

// dialPath is a placeholder until named-pipe support lands.
func dialPath(context.Context, string) (net.Conn, error) {
	if !windowsPipeSupport {
		return nil, errors.New("discord: Windows named-pipe transport is not implemented yet")
	}
	return nil, errors.New("discord: unreachable")
}
