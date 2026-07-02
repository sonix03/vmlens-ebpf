package service

import (
	"fmt"
	"net/netip"
)

type Classifier struct {
	prefixes []netip.Prefix
}

func NewClassifier(cidrs []string) (*Classifier, error) {
	c := &Classifier{}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse internal CIDR %q: %w", cidr, err)
		}
		c.prefixes = append(c.prefixes, prefix.Masked())
	}
	return c, nil
}

func (c *Classifier) IsInternal(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, prefix := range c.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (c *Classifier) Scope(sourceTenant, destinationTenant string, destinationRegistered bool, dstIP string) string {
	if destinationRegistered {
		if sourceTenant == destinationTenant {
			return "internal_same_tenant"
		}
		return "internal_cross_tenant"
	}
	if c.IsInternal(dstIP) {
		return "unknown_internal"
	}
	if addr, err := netip.ParseAddr(dstIP); err == nil && addr.IsValid() {
		return "external_public"
	}
	return "unknown"
}
