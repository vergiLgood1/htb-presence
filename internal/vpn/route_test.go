package vpn

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLocalConnectedLive reads this machine's real route table and reports what
// the fallback would decide. It only runs when HTB_VPN_LIVE is set, so the
// default `go test ./...` stays deterministic:
//
//	HTB_VPN_LIVE=1 go test ./internal/vpn -run Live -v
//
// A machine without the HTB VPN legitimately has no lab route, so the test
// asserts only that the lookup itself succeeds and logs every route it saw.
// The tunnel_like column is what to check first when a live VPN is not
// detected: a tunnel interface that is not named tun*/tap*/utun*/wintun*
// (a WireGuard wg0, for example) is parsed but never counts.
func TestLocalConnectedLive(t *testing.T) {
	if os.Getenv("HTB_VPN_LIVE") == "" {
		t.Skip("set HTB_VPN_LIVE=1 to read the real route table")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	routes, err := readRoutes(ctx)
	if err != nil {
		t.Fatalf("readRoutes: %v", err)
	}
	t.Logf("%d IPv4 routes", len(routes))
	for _, r := range routes {
		ones, _ := r.mask.Size()
		t.Logf("  iface=%s dest=%s/%d tunnel_like=%v counts_for_htb=%v",
			r.iface, r.dest, ones, tunLike(r.iface), coversHTB([]route{r}))
	}

	connected, err := LocalConnected(ctx)
	if err != nil {
		t.Fatalf("LocalConnected: %v", err)
	}
	t.Logf("local HTB route detected: %v", connected)
}

func TestCoversHTB(t *testing.T) {
	proc := "" +
		"Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"tun0 000A0A0A 00000000 0001 0 0 0 00FFFFFF 0 0 0\n" +
		"eth0 000A0A0A 00000000 0001 0 0 0 00FFFFFF 0 0 0\n"
	routes := parseProcNetRoute(proc)
	if !coversHTB(routes) {
		t.Fatal("tun0 route to 10.10.10.0/24 was not detected")
	}

	onlyEth := routes[1:]
	if coversHTB(onlyEth) {
		t.Fatal("a non-tunnel interface should not count")
	}
}

func TestParseNetstatAndWindows(t *testing.T) {
	mac := "10.10.10/24     10.10.14.1     UGSc     utun3\n" +
		"default            192.168.1.1    UGScg    en0\n"
	if !coversHTB(parseNetstatRN(mac)) {
		t.Fatal("utun3 route was not detected")
	}

	win := "10.129.0.0    255.255.0.0    10.10.14.1    10.10.14.5    25\n" +
		"0.0.0.0      0.0.0.0        192.168.1.1   192.168.1.20  25\n"
	if !coversHTB(parseWindowsRoute(win)) {
		t.Fatal("windows 10.129.0.0/16 route was not detected")
	}
}

func TestBroadRoutesDoNotMatch(t *testing.T) {
	win := "10.0.0.0    255.0.0.0    10.0.0.1    10.0.0.2    1\n"
	if coversHTB(parseWindowsRoute(win)) {
		t.Fatal("10.0.0.0/8 should not count as an HTB route")
	}
}
