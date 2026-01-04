package routine

import "fmt"

// ErrPanicRecovered is returned when a panic is recovered in a goroutine
var ErrPanicRecovered = fmt.Errorf("routine: panic recovered")

// ErrPanic returns an error wrapping the recovered panic value
func ErrPanic(recovered any) error {
	return fmt.Errorf("routine: panic recovered: %v", recovered)
}
