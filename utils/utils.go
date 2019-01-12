package utils

import (
	"fmt"
	"time"
)

func Trace(name string) func() {
	now := time.Now()
	return func() {
		end := time.Since(now)
		if end > time.Millisecond {
			fmt.Printf("end of trace for %s: %v\n", name, end)
		}
	}
}
