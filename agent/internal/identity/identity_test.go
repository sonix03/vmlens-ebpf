package identity

import (
	"strings"
	"testing"
)

func TestParseDefaultRouteInterfaces(t *testing.T) {
	routes := `Iface Destination Gateway Flags RefCnt Use Metric Mask
eth0 00000000 0101A8C0 0003 0 0 100 00000000
eth0 0001A8C0 00000000 0001 0 0 100 00FFFFFF
docker0 000011AC 00000000 0001 0 0 0 0000FFFF
eth1 00000000 0100000A 0003 0 0 200 00000000`
	got := parseDefaultRouteInterfaces(strings.NewReader(routes))
	if _, ok := got["eth0"]; !ok {
		t.Fatal("eth0 default route not detected")
	}
	if _, ok := got["eth1"]; !ok {
		t.Fatal("eth1 default route not detected")
	}
	if _, ok := got["docker0"]; ok {
		t.Fatal("connected Docker route treated as default")
	}
}
