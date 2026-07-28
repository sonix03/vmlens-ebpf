package metrics

import "time"

func LastErrorAtArg(errorCount int64, observedAt time.Time) any {
	if errorCount <= 0 {
		return nil
	}
	return observedAt
}

func LastErrorAtPtr(errorCount int64, observedAt time.Time) *time.Time {
	if errorCount <= 0 {
		return nil
	}
	value := observedAt
	return &value
}
