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

package utils_test

import (
	"errors"
	"testing"

	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hctamu/pulumi-pve/provider/pkg/utils"
)

// TestGetSortedMapKeys tests that the utility function correctly sorts map keys.
func TestGetSortedMapKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]string
		expected []string
	}{
		{
			name: "disk interfaces in random order",
			input: map[string]string{
				"scsi3":   "disk3",
				"ide1":    "disk1",
				"scsi0":   "disk0",
				"virtio2": "disk2",
				"sata1":   "disk4",
			},
			expected: []string{"ide1", "sata1", "scsi0", "scsi3", "virtio2"},
		},
		{
			name: "numerical vs alphabetical sorting",
			input: map[string]string{
				"scsi10": "disk10",
				"scsi2":  "disk2",
				"scsi1":  "disk1",
			},
			// Should be alphabetical, not numerical
			expected: []string{"scsi1", "scsi10", "scsi2"},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: []string{},
		},
		{
			name: "single item",
			input: map[string]string{
				"scsi0": "disk0",
			},
			expected: []string{"scsi0"},
		},
		{
			name: "identical prefixes with different numbers",
			input: map[string]string{
				"virtio15": "disk15",
				"virtio2":  "disk2",
				"virtio1":  "disk1",
				"virtio10": "disk10",
			},
			expected: []string{"virtio1", "virtio10", "virtio15", "virtio2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := utils.GetSortedMapKeys(tt.input)
			assert.Equal(t, tt.expected, result, "Keys should be sorted correctly")
		})
	}
}

// TestGetSortedMapKeys_IntKeys tests sorting with integer keys.
func TestGetSortedMapKeys_IntKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[int]string
		expected []int
	}{
		{
			name:     "unsorted int keys",
			input:    map[int]string{10: "ten", 1: "one", 5: "five", 2: "two"},
			expected: []int{1, 2, 5, 10},
		},
		{
			name:     "empty map",
			input:    map[int]string{},
			expected: []int{},
		},
		{
			name:     "single entry",
			input:    map[int]string{42: "forty-two"},
			expected: []int{42},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := utils.GetSortedMapKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetSortedMapKeys_Consistency tests that multiple calls return the same order.
func TestGetSortedMapKeys_Consistency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]string
		expected []string
	}{
		{
			name:     "four entries",
			input:    map[string]string{"z": "last", "a": "first", "m": "middle", "b": "second"},
			expected: []string{"a", "b", "m", "z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			first := utils.GetSortedMapKeys(tt.input)
			require.Equal(t, tt.expected, first)
			for i := 1; i < 5; i++ {
				assert.Equal(t, first, utils.GetSortedMapKeys(tt.input))
			}
		})
	}
}

func TestSliceToString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"a"}, "a"},
		{"sorted", []string{"a", "b", "c"}, "a,b,c"},
		{"unsorted", []string{"c", "a", "b"}, "a,b,c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := utils.SliceToString(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStringToSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"single", "a", []string{"a"}},
		{"trimSpaces", "a, b , c", []string{"a", "b", "c"}},
		{"unsorted", "c,b,a", []string{"a", "b", "c"}},
		{"duplicates", "b,a,b", []string{"a", "b", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := utils.StringToSlice(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSliceStringRoundTrip(t *testing.T) {
	t.Parallel()
	in := []string{"delta", "alpha", "charlie", "bravo"}
	s := utils.SliceToString(in)
	back := utils.StringToSlice(s)
	assert.ElementsMatch(t, []string{"alpha", "bravo", "charlie", "delta"}, back)
	// Ensure sorted order
	assert.Equal(t, []string{"alpha", "bravo", "charlie", "delta"}, back)
}

func TestMapToStringSlice(t *testing.T) {
	t.Parallel()
	m := map[string]api.IntOrBool{
		"beta":  api.IntOrBool(true),
		"alpha": api.IntOrBool(false),
		"gamma": api.IntOrBool(true),
	}
	got := utils.MapToStringSlice(m)
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, got)
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"contains does not exist", errors.New("resource 'x' does not exist"), true},
		{"other error", errors.New("some other error"), false},
		{"empty message", errors.New(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.IsNotFound(tt.err))
		})
	}
}

func TestDifferPtr(t *testing.T) {
	t.Parallel()
	intVal := func(i int) *int { v := i; return &v }
	tests := []struct {
		name     string
		a        *int
		b        *int
		expected bool
	}{
		{"both nil", nil, nil, false},
		{"a nil b non-nil", nil, intVal(1), true},
		{"a non-nil b nil", intVal(1), nil, true},
		{"equal values", intVal(5), intVal(5), false},
		{"different values", intVal(5), intVal(6), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.DifferPtr(tt.a, tt.b))
		})
	}
}

func TestEndsWithLetter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"ends with letter", "abc", true},
		{"ends with digit", "abc1", false},
		{"ends with space", "abc ", false},
		{"single letter", "a", true},
		{"single digit", "9", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.EndsWithLetter(tt.input))
		})
	}
}

func TestPtrEqual(t *testing.T) {
	t.Parallel()
	strVal := func(s string) *string { v := s; return &v }
	tests := []struct {
		name     string
		a        *string
		b        *string
		expected bool
	}{
		{"both nil", nil, nil, true},
		{"a nil b non-nil", nil, strVal("x"), false},
		{"a non-nil b nil", strVal("x"), nil, false},
		{"equal values", strVal("hello"), strVal("hello"), true},
		{"different values", strVal("hello"), strVal("world"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.PtrEqual(tt.a, tt.b))
		})
	}
}

func TestStringSliceChanged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{"both empty", []string{}, []string{}, false},
		{"equal same order", []string{"a", "b"}, []string{"a", "b"}, false},
		{"equal different order", []string{"b", "a"}, []string{"a", "b"}, false},
		{"different length", []string{"a"}, []string{"a", "b"}, true},
		{"different values", []string{"a", "c"}, []string{"a", "b"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.StringSliceChanged(tt.a, tt.b))
		})
	}
}

func TestIntSliceChanged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected bool
	}{
		{"both empty", []int{}, []int{}, false},
		{"equal same order", []int{1, 2, 3}, []int{1, 2, 3}, false},
		{"equal different order", []int{3, 1, 2}, []int{1, 2, 3}, false},
		{"different length", []int{1}, []int{1, 2}, true},
		{"different values", []int{1, 3}, []int{1, 2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.IntSliceChanged(tt.a, tt.b))
		})
	}
}

func TestJoinInts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []int
		expected string
	}{
		{"empty slice", []int{}, ""},
		{"single element", []int{42}, "42"},
		{"multiple elements", []int{1, 2, 3}, "1,2,3"},
		{"negative values", []int{-1, 0, 1}, "-1,0,1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.JoinInts(tt.input))
		})
	}
}

func TestIntSliceDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []int
		b        []int
		expected []int
	}{
		{"both empty", []int{}, []int{}, nil},
		{"a empty", []int{}, []int{1, 2}, nil},
		{"b empty", []int{1, 2}, []int{}, []int{1, 2}},
		{"no overlap", []int{1, 2}, []int{3, 4}, []int{1, 2}},
		{"full overlap", []int{1, 2}, []int{1, 2}, nil},
		{"partial overlap", []int{1, 2, 3}, []int{2}, []int{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.IntSliceDiff(tt.a, tt.b))
		})
	}
}

func TestStringSliceDiff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{"both empty", []string{}, []string{}, nil},
		{"a empty", []string{}, []string{"x"}, nil},
		{"b empty", []string{"x", "y"}, []string{}, []string{"x", "y"}},
		{"no overlap", []string{"a", "b"}, []string{"c", "d"}, []string{"a", "b"}},
		{"full overlap", []string{"a", "b"}, []string{"a", "b"}, nil},
		{"partial overlap", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, utils.StringSliceDiff(tt.a, tt.b))
		})
	}
}
