package network

import "net/netip"

type Endpoint struct {
	IP   netip.Addr
	Port uint16
}
