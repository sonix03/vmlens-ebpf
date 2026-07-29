package rtt

import (
	"encoding/binary"
	"fmt"
	"math"
)

func Decode(sample []byte) (Event, error) {
	if len(sample) != 8 {
		return Event{}, fmt.Errorf("unexpected RTT sample size %d, want 8", len(sample))
	}
	return Event{ValueMs: math.Float64frombits(binary.LittleEndian.Uint64(sample))}, nil
}
