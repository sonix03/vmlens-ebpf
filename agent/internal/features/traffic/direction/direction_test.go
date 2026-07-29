package direction

import "testing"

func TestFromKernel(t *testing.T) {
	if got := FromKernel(1); got != Ingress {
		t.Fatalf("FromKernel(1)=%q, want %q", got, Ingress)
	}
	if got := FromKernel(2); got != Egress {
		t.Fatalf("FromKernel(2)=%q, want %q", got, Egress)
	}
}
