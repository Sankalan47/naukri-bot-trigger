package naukri

import "strings"

// toggleTrailingFullStop appends a trailing full stop, or removes one if the
// text already ends with two, so repeated calls oscillate between one and
// two trailing full stops instead of growing without bound. This produces a
// trivial, harmless edit used to bump Naukri's "profile last updated" signal.
func toggleTrailingFullStop(text string) string {
	trimmed := strings.TrimRight(text, " \t\r\n")
	if strings.HasSuffix(trimmed, "..") {
		return strings.TrimSuffix(trimmed, ".")
	}
	return trimmed + "."
}
