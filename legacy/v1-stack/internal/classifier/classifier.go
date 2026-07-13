package classifier

import (
	"fmt"
	"net/netip"
)

type Classifier struct {
	internal []netip.Prefix
}

func New(cidrs []string) (*Classifier, error) {
	c := &Classifier{internal: make([]netip.Prefix, 0, len(cidrs))}
	for _, raw := range cidrs {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid internal CIDR %q: %w", raw, err)
		}
		c.internal = append(c.internal, prefix.Masked())
	}
	return c, nil
}

func (c *Classifier) Scope(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "unknown"
	}
	addr = addr.Unmap()
	for _, prefix := range c.internal {
		if prefix.Contains(addr) {
			return "internal"
		}
	}
	return "external"
}

func PortClass(port uint16) string {
	switch port {
	case 22:
		return "ssh"
	case 53:
		return "dns"
	case 80, 8080:
		return "http"
	case 443, 8443:
		return "https"
	case 3306, 5432, 6379, 27017:
		return "database"
	case 25, 465, 587:
		return "mail"
	default:
		return "other"
	}
}
