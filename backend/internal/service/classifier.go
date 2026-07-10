package service

import (
	"fmt"
	"net/netip"
)

const (
	ScopeInternalSameTenant  = "internal_same_tenant"
	ScopeInternalCrossTenant = "internal_cross_tenant"
	ScopeUnknownInternal     = "unknown_internal"
	ScopeExternalPublic      = "external_public"
	ScopeExternalPrivate     = "external_private"
	ScopeUnknown             = "unknown"
)

type Classifier struct {
	prefixes                  []netip.Prefix
	unregisteredInternalScope string
}

func NewClassifier(cidrs []string, unregisteredInternalScope string) (*Classifier, error) {
	if unregisteredInternalScope == "" {
		unregisteredInternalScope = ScopeExternalPrivate
	}
	if unregisteredInternalScope != ScopeExternalPrivate && unregisteredInternalScope != ScopeUnknownInternal {
		return nil, fmt.Errorf("unsupported unregistered internal scope %q", unregisteredInternalScope)
	}
	c := &Classifier{unregisteredInternalScope: unregisteredInternalScope}
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse internal CIDR %q: %w", cidr, err)
		}
		c.prefixes = append(c.prefixes, prefix.Masked())
	}
	return c, nil
}

func (c *Classifier) IsConfiguredInternal(ip string) bool {
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

func (c *Classifier) IsInternal(ip string) bool {
	return c.IsConfiguredInternal(ip)
}

func (c *Classifier) Scope(sourceTenant, destinationTenant string, destinationRegistered bool, dstIP string) string {
	if destinationRegistered {
		if sourceTenant == destinationTenant {
			return ScopeInternalSameTenant
		}
		return ScopeInternalCrossTenant
	}
	addr, err := netip.ParseAddr(dstIP)
	if err != nil || !addr.IsValid() {
		return ScopeUnknown
	}
	if c.IsConfiguredInternal(dstIP) {
		return c.unregisteredInternalScope
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return ScopeExternalPrivate
	}
	return ScopeExternalPublic
}
