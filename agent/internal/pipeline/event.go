package pipeline

import "time"

type Event interface {
	Type() string
	Time() time.Time
}
