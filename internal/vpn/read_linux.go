//go:build linux

package vpn

import (
	"context"
	"fmt"
	"os"
)

func readRoutes(context.Context) ([]route, error) {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return nil, fmt.Errorf("reading routes: %w", err)
	}
	return parseProcNetRoute(string(data)), nil
}
