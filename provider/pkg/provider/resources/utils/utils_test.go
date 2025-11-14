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
	"context"
	"errors"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
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

	input := map[int]string{
		10: "ten",
		1:  "one",
		5:  "five",
		2:  "two",
	}
	expected := []int{1, 2, 5, 10}

	result := utils.GetSortedMapKeys(input)
	assert.Equal(t, expected, result, "Integer keys should be sorted numerically")
}

// TestGetSortedMapKeys_Consistency tests that multiple calls return the same order.
func TestGetSortedMapKeys_Consistency(t *testing.T) {
	t.Parallel()

	input := map[string]string{
		"z": "last",
		"a": "first",
		"m": "middle",
		"b": "second",
	}

	var previousResult []string
	for i := 0; i < 5; i++ {
		result := utils.GetSortedMapKeys(input)

		if i == 0 {
			previousResult = result
			assert.Equal(t, []string{"a", "b", "m", "z"}, result, "First result should be sorted")
		} else {
			assert.Equal(t, previousResult, result, "Results should be consistent across multiple calls")
		}
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
	assert.True(t, utils.IsNotFound(errors.New("resource 'x' does not exist")))
	assert.False(t, utils.IsNotFound(errors.New("some other error")))
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestDeleteResourceSuccess(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/users/testuser")).Reply(reply.OK()),
	).Enable()

	_, err := utils.DeleteResource(utils.DeletedResource{
		Ctx: context.Background(), ResourceID: "testuser", URL: "/access/users/testuser", ResourceType: "user",
	})
	require.NoError(t, err)
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestDeleteResourceClientError(t *testing.T) {
	original := client.GetProxmoxClientFn
	defer func() { client.GetProxmoxClientFn = original }()
	client.GetProxmoxClientFn = func(ctx context.Context) (*px.Client, error) { return nil, errors.New("client error") }

	_, err := utils.DeleteResource(utils.DeletedResource{
		Ctx: context.Background(), ResourceID: "x", URL: "/access/users/x", ResourceType: "user",
	})
	require.Error(t, err)
	assert.EqualError(t, err, "client error")
}

//nolint:paralleltest // Test sets global environment variable, therefore do not parallelize!
func TestDeleteResourceDeleteError(t *testing.T) {
	mockServer, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	mockServer.AddMocks(
		mocha.Delete(expect.URLPath("/access/users/testuser")).Reply(reply.InternalServerError()),
	).Enable()

	_, err := utils.DeleteResource(utils.DeletedResource{
		Ctx: context.Background(), ResourceID: "testuser", URL: "/access/users/testuser", ResourceType: "user",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete user testuser")
}
