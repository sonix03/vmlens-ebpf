package packet

import (
	"encoding/binary"
	"testing"
)

// The C side is __attribute__((packed)); encoding/binary is packed too, so the
// two layouts must agree byte for byte or every event decodes as garbage.
func TestRawFlowEventMatchesCLayout(t *testing.T) {
	const cStructSize = 8 + 8 + 16 + 16 + 4 + 2 + 2 + 2 + 1 + 1 + 4 + 4 + 4 + 4 + 4 + 2
	if got := binary.Size(rawFlowEvent{}); got != cStructSize {
		t.Fatalf("Go struct is %d bytes, C struct is %d", got, cStructSize)
	}
}
