//go:build windows

package vpn

import (
	"context"
	"fmt"
	"os/exec"
)

func readRoutes(ctx context.Context) ([]route, error) {
	out, err := exec.CommandContext(ctx, "route", "print", "-4").Output()
	if err != nil {
		return nil, fmt.Errorf("reading routes: %w", err)
	}
	return parseWindowsRoute(string(out)), nil
}
