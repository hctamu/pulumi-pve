/*
	Copyright 2025, Pulumi Corporation.

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

// Package proxmox provides interfaces and domain types for interacting with Proxmox VE.
package proxmox

import (
	p "github.com/pulumi/pulumi-go-provider"
)

// FieldDiffer is implemented by input types that require custom diff logic.
// DiffFrom compares the receiver (input/new value) against state (prior value)
// and returns Pulumi property diffs keyed by property path. Nil pointer receivers
// are valid; implementations must handle them.
type FieldDiffer interface {
	DiffFrom(name string, state any) map[string]p.PropertyDiff
}
