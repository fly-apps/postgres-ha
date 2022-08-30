package check

import "time"

var divs = []time.Duration{
	time.Duration(1), time.Duration(10), time.Duration(100), time.Duration(1000)}

func RoundDuration(d time.Duration, digits int) time.Duration {
	switch {
	case d > time.Second:
		d = d.Round(time.Second / divs[digits])
	case d > time.Millisecond:
		d = d.Round(time.Millisecond / divs[digits])
	case d > time.Microsecond:
		d = d.Round(time.Microsecond / divs[digits])
	}
	return d
}
