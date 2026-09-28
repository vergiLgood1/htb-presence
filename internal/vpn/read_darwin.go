//go:build darwin

package vpn

import (
	"context"
	"fmt"
	"os/exec"
)

func readRoutes(ctx context.Context) ([]route, error) {
	out, err := exec.CommandContext(ctx, "netstat", "-rn", "-f", "inet").Output()
	if err != nil {
		return nil, fmt.Errorf("reading routes: %w", err)
	}
	return parseNetstatRN(string(out)), nil
}
