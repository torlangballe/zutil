//go:build !zui

package ztime

import "time"

func GetTimeWithServerLocation(t time.Time) time.Time {
	return t
}
