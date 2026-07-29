package exporter

import (
	"context"
	"time"
)

func Retry(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	var last error
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			last = err
		} else {
			return nil
		}
		if i == attempts-1 || delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}
