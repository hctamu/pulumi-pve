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

// Package ha_test contains unit tests for HA resource operations.
//
// Test Organization:
// - Unit tests: Direct method calls with mocked HAOperations interface
// - Error scenarios: Operation failures, validation errors, edge cases
// - Lifecycle tests: See ha_lifecycle_test.go
package proxmox_test

import (
	"context"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/stretchr/testify/assert"
)

func TestHAStateValidation(t *testing.T) {
	t.Parallel()

	// Test state validation for all valid and invalid state values
	tests := []struct {
		name      string
		state     proxmox.HAState
		wantError bool
	}{
		{"valid ignored", proxmox.HAStateIgnored, false},
		{"valid started", proxmox.HAStateStarted, false},
		{"valid stopped", proxmox.HAStateStopped, false},
		{"invalid state", proxmox.HAState("invalid"), true},
		{"empty state", proxmox.HAState(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.state.ValidateState(context.Background())
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
