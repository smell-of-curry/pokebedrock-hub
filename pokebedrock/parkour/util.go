package parkour

import (
	"fmt"
	"time"
)

// formatDuration ...
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%0.2fs", d.Seconds())
}
