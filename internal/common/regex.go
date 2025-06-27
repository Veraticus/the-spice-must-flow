package common

import "regexp"

// MatchRegex compiles and matches a regex pattern against a string.
// Returns true if the pattern matches, false otherwise.
// Returns an error if the pattern is invalid.
func MatchRegex(pattern, text string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}
