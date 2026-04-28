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

// Package utils provides utility functions for resource management.
//
//nolint:revive // Generic package name is acceptable for cross-resource utilities
package utils

import (
	"cmp"
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	api "github.com/luthermonson/go-proxmox"
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

// SliceToString is used to convert a slice of strings to a comma-separated string
func SliceToString(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	// Sort for consistent output and easier comparison
	sortedSlice := make([]string, len(slice))
	copy(sortedSlice, slice)
	sort.Strings(sortedSlice)
	return strings.Join(sortedSlice, ",")
}

// StringToSlice is used to convert a comma-separated string to a slice of strings
func StringToSlice(str string) []string {
	if str == "" {
		return []string{}
	}
	parts := strings.Split(str, ",")
	slice := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			slice = append(slice, trimmed)
		}
	}
	// Sort for consistent output and easier comparison
	sort.Strings(slice)
	return slice
}

// MapToStringSlice converts a map with string keys to a sorted slice of strings
func MapToStringSlice(m map[string]api.IntOrBool) []string {
	slice := make([]string, 0, len(m))
	for key := range m {
		slice = append(slice, key)
	}
	sort.Strings(slice)
	return slice
}

// IsNotFound checks if the error indicates that the resource was not found.
func IsNotFound(err error) bool {
	return strings.Contains(err.Error(), "does not exist")
}

// DeletedResource is used to delete a resource with DeleteResource function
type DeletedResource struct {
	Ctx          context.Context
	ResourceID   string
	URL          string
	ResourceType string
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

// PtrEqual compares two pointers of any comparable type.
func PtrEqual[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// StringSliceChanged returns true when the two tag slices contain different sets of values,
// ignoring order.
func StringSliceChanged(a, b []string) bool {
	if len(a) != len(b) {
		return true
	}
	sortedA := make([]string, len(a))
	copy(sortedA, a)
	sort.Strings(sortedA)
	sortedB := make([]string, len(b))
	copy(sortedB, b)
	sort.Strings(sortedB)
	return strings.Join(sortedA, ",") != strings.Join(sortedB, ",")
}

// JoinInts converts a slice of ints to a comma-separated string.
func JoinInts(ints []int) string {
	parts := make([]string, len(ints))
	for i, v := range ints {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

// IntSliceChanged returns true when the two int slices contain different sets of values,
// ignoring order.
func IntSliceChanged(a, b []int) bool {
	if len(a) != len(b) {
		return true
	}
	sortedA := make([]int, len(a))
	copy(sortedA, a)
	sort.Ints(sortedA)
	sortedB := make([]int, len(b))
	copy(sortedB, b)
	sort.Ints(sortedB)
	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return true
		}
	}
	return false
}

// IntSliceDiff returns elements present in a but not in b.
func IntSliceDiff(a, b []int) []int {
	bSet := make(map[int]struct{}, len(b))
	for _, v := range b {
		bSet[v] = struct{}{}
	}
	var diff []int
	for _, v := range a {
		if _, ok := bSet[v]; !ok {
			diff = append(diff, v)
		}
	}
	return diff
}

// StringSliceDiff returns elements present in a but not in b.
func StringSliceDiff(a, b []string) []string {
	bSet := make(map[string]struct{}, len(b))
	for _, v := range b {
		bSet[v] = struct{}{}
	}
	var diff []string
	for _, v := range a {
		if _, ok := bSet[v]; !ok {
			diff = append(diff, v)
		}
	}
	return diff
}
