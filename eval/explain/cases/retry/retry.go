package evalcase

import (
	"errors"
	"time"
)

var ErrGiveUp = errors.New("retry: attempts exhausted")

func sleep(d time.Duration) { time.Sleep(d) }

// Retry calls op until it succeeds or attempts run out.
func Retry(attempts int, base time.Duration, op func() error) error {
	if attempts <= 0 {
		return ErrGiveUp
	}
	delay := base
	var last error
	for i := 0; i < attempts; i++ {
		if last = op(); last == nil {
			return nil
		}
		if i < attempts-1 {
			sleep(delay)
			delay *= 2
		}
	}
	return errors.Join(ErrGiveUp, last)
}
