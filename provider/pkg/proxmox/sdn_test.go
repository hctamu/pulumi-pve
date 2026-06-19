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

package proxmox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSDNName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid short name", input: "vpool1"},
		{name: "valid 8-char name", input: "vxlan123"},
		{name: "valid single letter", input: "v"},
		{name: "empty string", input: "", wantErr: true},
		{name: "9 chars exceeds limit", input: "vnetpool4", wantErr: true},
		{name: "starts with digit", input: "1vpool", wantErr: true},
		{name: "contains hyphen", input: "vnet-1", wantErr: true},
		{name: "contains underscore", input: "vnet_1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSDNName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(
					t, err.Error(),
					"must start with a letter, contain only letters and numbers, and be at most 8 characters",
				)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
