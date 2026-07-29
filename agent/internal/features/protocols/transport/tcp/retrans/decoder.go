package retrans

import (
	"encoding/binary"
	"fmt"
)

func Decode(sample []byte) (Event, error) {
	if len(sample) != 8 {
		return Event{}, fmt.Errorf("unexpected retrans sample size %d, want 8", len(sample))
	}
	return Event{Count: binary.LittleEndian.Uint64(sample)}, nil
}
