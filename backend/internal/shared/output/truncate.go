// Package output contains presentation helpers for command output.
package output

// Truncate limits text to max bytes and marks truncated values with an
// ellipsis.
func Truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
