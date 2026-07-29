package network

import "net/netip"

func IsPrivateIP(ip netip.Addr) bool {
	return ip.IsPrivate()
}

func IsLoopback(ip netip.Addr) bool {
	return ip.IsLoopback()
}
