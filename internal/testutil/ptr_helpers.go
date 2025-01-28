package testutil

import "time"

// String returns a pointer to the given string
func String(s string) *string {
	return &s
}

// Bool returns a pointer to the given bool
func Bool(b bool) *bool {
	return &b
}

// Int returns a pointer to the given int
func Int(i int) *int {
	return &i
}

// Time returns a pointer to the given time.Time
func Time(t time.Time) *time.Time {
	return &t
}
