package audio

import "time"

func timeNewTimer(seconds float64) <-chan time.Time {
	return time.After(time.Duration(seconds * float64(time.Second)))
}
