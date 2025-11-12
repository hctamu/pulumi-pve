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
	"unicode"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
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

// DeleteResource is used to delete a resource
func DeleteResource(r DeletedResource) (response infer.DeleteResponse, err error) {
	// this function can be used only if resource has DELETE method implemented in proxmox client
	// check: https://pve.proxmox.com/pve-docs/api-viewer/
	l := p.GetLogger(r.Ctx)
	l.Debugf("Deleting %s %s", r.ResourceType, r.ResourceID)

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(r.Ctx); err != nil {
		return response, err
	}

	// perform delete
	if err = pxc.Req(r.Ctx, http.MethodDelete, r.URL, nil, nil); err != nil {
		return response, fmt.Errorf("failed to delete %s %s: %w", r.ResourceType, r.ResourceID, err)
	}

	l.Debugf("Successfully deleted %s %s", r.ResourceType, r.ResourceID)
	return response, nil
}
