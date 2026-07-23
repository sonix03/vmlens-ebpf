package deepflow

import (
	"strings"
	"testing"
	"time"
)

func TestDeepFlowL4WhereFiltersInventoryAndControlPorts(t *testing.T) {
	where := deepFlowL4Where(QueryFilter{
		Window:        5 * time.Minute,
		AllowedIPs:    []string{"10.20.20.130", "10.20.20.199"},
		ExcludedIPs:   []string{"10.20.20.125"},
		ExcludedPorts: []int{30033, 18081, 30033},
	})

	for _, want := range []string{
		"time > now() - INTERVAL 300 SECOND",
		"toString(ip4_0) IN ('10.20.20.130','10.20.20.199')",
		"toString(ip4_0) NOT IN ('10.20.20.125')",
		"client_port NOT IN (18081,30033)",
		"server_port NOT IN (18081,30033)",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("where %q does not contain %q", where, want)
		}
	}
}

func TestDeepFlowL7WhereFiltersDeepFlowControlResources(t *testing.T) {
	where := deepFlowL7Where(QueryFilter{
		Window:                     time.Minute,
		AllowedIPs:                 []string{"10.20.20.130"},
		ExcludedL7ResourcePrefixes: []string{"/trident."},
	})

	for _, want := range []string{
		"time > now() - INTERVAL 60 SECOND",
		"toString(ip4_0) IN ('10.20.20.130')",
		"NOT startsWith(request_resource, '/trident.')",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("where %q does not contain %q", where, want)
		}
	}
}
