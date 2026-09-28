// Package vpn detects a local Hack The Box VPN route.
//
// It is a fallback for when HTB's connection-status endpoint cannot be
// reached. A match requires a route to an HTB lab prefix with a prefix
// length of at least 16, so a default route or a broad 10.0.0.0/8 does not
// count. On Linux and macOS the route must also leave through a tunnel
// interface.
package vpn

import (
	"context"
	"net"
	"strings"
	"time"
)

// probeTimeout bounds how long a platform route lookup may take.
const probeTimeout = 2 * time.Second

// LocalConnected reports whether this machine has a route to an HTB lab
// network. A lookup error yields false plus the error; callers can still
// keep the last presence.
func LocalConnected(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	routes, err := readRoutes(ctx)
	if err != nil {
		return false, err
	}
	return coversHTB(routes), nil
}

// route is one IPv4 route from the local table.
type route struct {
	iface      string
	dest       net.IP
	mask       net.IPMask
	requireTun bool
}

var htbProbes = []net.IP{
	net.ParseIP("10.10.10.1").To4(),
	net.ParseIP("10.10.11.1").To4(),
	net.ParseIP("10.129.0.1").To4(),
}

// coversHTB reports whether any route contains an HTB lab address.
func coversHTB(routes []route) bool {
	for _, r := range routes {
		if r.requireTun && !tunLike(r.iface) {
			continue
		}
		ones, bits := r.mask.Size()
		if bits != 32 || ones < 16 {
			continue
		}
		network := &net.IPNet{IP: r.dest.Mask(r.mask), Mask: r.mask}
		for _, ip := range htbProbes {
			if network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// tunLike reports whether name looks like a tunnel interface.
func tunLike(name string) bool {
	n := strings.ToLower(name)
	for _, prefix := range []string{"tun", "tap", "utun", "wintun"} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}
