package vpn

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"
)

// parseProcNetRoute parses Linux /proc/net/route. Destination and mask are
// little-endian hexadecimal.
func parseProcNetRoute(data string) []route {
	var routes []route
	for i, line := range strings.Split(data, "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		dest := parseHexIPv4(fields[1])
		mask := parseHexIPv4(fields[7])
		if dest == nil || mask == nil {
			continue
		}
		routes = append(routes, route{
			iface:      fields[0],
			dest:       dest,
			mask:       net.IPMask(mask),
			requireTun: true,
		})
	}
	return routes
}

// parseHexIPv4 decodes an 8-digit little-endian hex address from /proc/net/route.
func parseHexIPv4(hex string) net.IP {
	if len(hex) != 8 {
		return nil
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return nil
	}
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return net.IP(b)
}

// parseNetstatRN parses `netstat -rn -f inet` output from macOS.
func parseNetstatRN(data string) []route {
	var routes []route
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		dest, mask, ok := parseDestCIDR(fields[0])
		if !ok {
			continue
		}
		iface := fields[len(fields)-1]
		if strings.Contains(iface, ":") || net.ParseIP(iface) != nil {
			iface = fields[3]
		}
		routes = append(routes, route{
			iface:      iface,
			dest:       dest,
			mask:       mask,
			requireTun: true,
		})
	}
	return routes
}

// parseWindowsRoute parses `route print -4` output. Windows does not name the
// tunnel in that table, so a specific HTB prefix is enough.
func parseWindowsRoute(data string) []route {
	var routes []route
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		dest := net.ParseIP(fields[0])
		if dest = dest.To4(); dest == nil {
			continue
		}
		maskIP := net.ParseIP(fields[1])
		if maskIP = maskIP.To4(); maskIP == nil {
			continue
		}
		routes = append(routes, route{
			dest:       dest,
			mask:       net.IPMask(maskIP),
			requireTun: false,
		})
	}
	return routes
}

// parseDestCIDR parses destinations such as "10.10.10/24" or "10.129/16".
func parseDestCIDR(field string) (net.IP, net.IPMask, bool) {
	parts := strings.Split(field, "/")
	if len(parts) != 2 {
		return nil, nil, false
	}
	ipPart := parts[0]
	for strings.Count(ipPart, ".") < 3 {
		ipPart += ".0"
	}
	_, network, err := net.ParseCIDR(ipPart + "/" + parts[1])
	if err != nil || network.IP.To4() == nil {
		return nil, nil, false
	}
	return network.IP.To4(), network.Mask, true
}
