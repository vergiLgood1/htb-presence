//go:build !windows

package discord

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// candidatePaths returns the Unix socket paths Discord is known to listen on,
// in the order they should be probed.
func candidatePaths() []string {
	var dirs []string

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	dirs = append(dirs,
		runtimeDir,
		filepath.Join(runtimeDir, "app", "com.discordapp.Discord"),       // Flatpak
		filepath.Join(runtimeDir, "app", "com.discordapp.DiscordCanary"), // Flatpak canary
		os.Getenv("TMPDIR"),
		"/tmp",
	)

	var paths []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for i := 0; i < 10; i++ {
			paths = append(paths, filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i)))
		}
	}
	return paths
}

// dialPath connects to a Unix socket.
func dialPath(ctx context.Context, path string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", path)
}
