//go:build windows

package discord

import (
	"context"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// candidatePaths returns the Windows named-pipe paths Discord listens on, in
// the order they should be probed.
func candidatePaths() []string {
	paths := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		paths = append(paths, fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i))
	}
	return paths
}

// dialPath connects to a Windows named pipe. Go's standard library cannot dial
// named pipes, so this uses go-winio (pure Go, no CGo).
func dialPath(ctx context.Context, path string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, path)
}
