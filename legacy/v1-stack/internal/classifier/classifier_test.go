package classifier

import "testing"

func TestScope(t *testing.T) {
	c, err := New([]string{"10.0.0.0/8", "192.168.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	for ip, want := range map[string]string{
		"10.0.1.30":   "internal",
		"192.168.1.1": "internal",
		"8.8.8.8":     "external",
		"not-an-ip":   "unknown",
	} {
		if got := c.Scope(ip); got != want {
			t.Errorf("Scope(%q)=%q, want %q", ip, got, want)
		}
	}
}
