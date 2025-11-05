/* Copyright 2025, Pulumi Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package resources provides utility functions for resource management.
package resources

import (
	"cmp"
	"slices"
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
	if str == "" {
		return false // Handle empty string
	}

	lastChar := rune(str[len(str)-1])
	return unicode.IsLetter(lastChar)
}

// GetSortedMapKeys returns the keys of a map as a slice in no particular order.
func GetSortedMapKeys[K cmp.Ordered, V any](inMap map[K]V) []K {
	keys := make([]K, 0, len(inMap))
	for key := range inMap {
		keys = append(keys, key)
	}

	slices.Sort(keys)
	return keys
}
