// Package cron converts minute-based intervals to clock-aligned cron
// expressions (5-field, no seconds). Used by the notify scheduler so that
// hourly checks fire at :00 regardless of when the process was started.
package cron

import "fmt"

// FromMinutes converts an interval in minutes to a clock-aligned 5-field
// cron expression. Returns an error for values that can't be expressed
// cleanly without drift at the hour or day boundary.
//
//	5    -> "*/5 * * * *"     (every 5 minutes, aligned to :00)
//	60   -> "0 * * * *"       (every hour, at minute 0)
//	120  -> "0 */2 * * *"     (every 2 hours, at minute 0)
//
// Sub-hour values must divide 60. Hour-or-greater values must be a multiple
// of 60 whose hour count divides 24.
func FromMinutes(minutes int) (string, error) {
	if minutes <= 0 {
		return "", fmt.Errorf("interval must be positive, got %d", minutes)
	}
	if minutes < 60 {
		if 60%minutes != 0 {
			return "", fmt.Errorf("interval %d minutes must divide 60", minutes)
		}
		return fmt.Sprintf("*/%d * * * *", minutes), nil
	}
	if minutes == 60 {
		return "0 * * * *", nil
	}
	if minutes%60 != 0 {
		return "", fmt.Errorf("interval over 60 minutes must be a multiple of 60, got %d", minutes)
	}
	hours := minutes / 60
	if 24%hours != 0 {
		return "", fmt.Errorf("interval %d minutes (%d hours) must divide 24", minutes, hours)
	}
	return fmt.Sprintf("0 */%d * * *", hours), nil
}
