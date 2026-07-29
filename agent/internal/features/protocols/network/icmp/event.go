package icmp

import "time"

type Event struct {
	SourceIP      string
	DestinationIP string
	Bytes         int64
	Timestamp     time.Time
}
