package packet

import "testing"

func TestDecodeRawEventRejectsInvalidSize(t *testing.T) {
	if _, err := DecodeRawEvent([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected invalid eBPF sample size error")
	}
}
