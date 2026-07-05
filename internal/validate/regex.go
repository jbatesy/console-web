package validate

import "regexp"

// fullMatchRegex wraps pattern as ^(?:pattern)$ so evaluation requires the entire
// value to match, not merely a substring.
func fullMatchRegex(pattern string) string {
	return "^(?:" + pattern + ")$"
}

func matchFull(pattern, value string) (bool, error) {
	return regexp.MatchString(fullMatchRegex(pattern), value)
}
