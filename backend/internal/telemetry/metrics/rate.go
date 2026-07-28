package metrics

import "time"

func RatePerSecond(count int64, firstSeen, lastSeen time.Time) float64 {
	if count <= 0 {
		return 0
	}
	seconds := lastSeen.Sub(firstSeen).Seconds()
	if seconds < 1 {
		seconds = 1
	}
	return float64(count) / seconds
}
