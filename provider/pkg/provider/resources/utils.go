package resources

import (
	"unicode"
)

// DifferPtr compares two pointers to values and returns true if they are different.
func DifferPtr[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}

// EndsWithLetter checks if the last character of a string is a letter.
func EndsWithLetter(str string) bool {
	if len(str) == 0 {
		return false // Handle empty string
	}

	lastChar := rune(str[len(str)-1])
	return unicode.IsLetter(lastChar)
}
