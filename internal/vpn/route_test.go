package vpn

import "testing"

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
