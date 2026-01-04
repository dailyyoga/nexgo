package cron

import "fmt"

var (
	// ErrNoTasks is returned when attempting to add a chain job with no tasks
	ErrNoTasks = fmt.Errorf("cron: no tasks provided")

	// ErrInvalidSpec is returned when a cron spec string is invalid
	ErrInvalidSpec = fmt.Errorf("cron: invalid cron spec")

	// ErrCronClosed is returned when attempting to operate on a closed cron manager
	ErrCronClosed = fmt.Errorf("cron: cron manager is closed")
)
