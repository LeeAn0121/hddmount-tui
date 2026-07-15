package diskutil

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// LogPath is where every mount/unmount/format action gets recorded, so an
// operator can audit what this tool has done to the machine's disks without
// digging through shell history.
const LogPath = "/var/log/hddmount.log"

// LogEvent appends one line to LogPath. Logging is best-effort: if the log
// file can't be opened (e.g. /var/log is read-only), the action itself still
// proceeds, it just goes unrecorded.
func LogEvent(action, target string, detail string, err error) {
	status := "OK"
	if err != nil {
		status = "FAIL: " + err.Error()
	}
	detail = strings.ReplaceAll(strings.TrimSpace(detail), "\n", " | ")

	f, ferr := os.OpenFile(LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  action=%-12s target=%-20s status=%s  %s\n",
		time.Now().Format(time.RFC3339), action, target, status, detail)
}
