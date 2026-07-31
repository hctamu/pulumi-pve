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

package proxmox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

func TestSDNListDiffers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		differ  proxmox.FieldDiffer
		state   any
		changed bool
	}{
		{
			name:    "peer reordering changes diff",
			differ:  proxmox.PeerList{"node-b", "node-a"},
			state:   proxmox.PeerList{"node-a", "node-b"},
			changed: true,
		},
		{
			name:    "node reordering does not change diff",
			differ:  proxmox.NodeList{"node-b", "node-a"},
			state:   proxmox.NodeList{"node-a", "node-b"},
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.changed, len(tt.differ.DiffFrom("nodes", tt.state)) > 0)
		})
	}
}
