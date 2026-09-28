//go:build !linux && !darwin && !windows

package vpn

import "context"

func readRoutes(context.Context) ([]route, error) {
	return nil, nil
}
