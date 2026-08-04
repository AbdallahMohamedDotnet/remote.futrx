// Package output contains presentation helpers for command output.
package output

// TruncateTail limits text to its last max bytes and marks truncated values
// with a leading ellipsis. Command transcripts put the error at the end, so
// the tail is the part worth keeping — head truncation twice hid a build's
// real failure behind pages of healthy apt output.
func TruncateTail(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return "..." + value[len(value)-max:]
}
