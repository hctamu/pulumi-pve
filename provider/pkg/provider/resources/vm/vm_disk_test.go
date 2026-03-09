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

package vm_test

import (
	"context"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

func TestVMDiffDisksChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputDisks    []*proxmox.Disk
		stateDisks    []*proxmox.Disk
		expectChange  bool
		expectDiffKey string
	}{
		{
			name: "disk size changed",
			inputDisks: []*proxmox.Disk{
				{
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk interface changed",
			inputDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi1",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk storage changed",
			inputDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false, // Same size and interface
			expectDiffKey: "",
		},
		{
			name: "disk added",
			inputDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "disk removed",
			inputDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			expectChange:  true,
			expectDiffKey: "disks",
		},
		{
			name: "no disk changes",
			inputDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*proxmox.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			expectChange:  false,
			expectDiffKey: "",
		},
		{
			name:          "both empty",
			inputDisks:    []*proxmox.Disk{},
			stateDisks:    []*proxmox.Disk{},
			expectChange:  false,
			expectDiffKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmInstance := &vmResource.VM{}
			req := infer.DiffRequest[proxmox.VMInputs, proxmox.VMOutputs]{
				ID: "100",
				Inputs: proxmox.VMInputs{
					Name:  testutils.Ptr("test-vm"),
					Disks: tt.inputDisks,
				},
				State: proxmox.VMOutputs{
					VMInputs: proxmox.VMInputs{
						Name:  testutils.Ptr("test-vm"),
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vmInstance.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				assert.Equal(t, p.Update, resp.DetailedDiff[tt.expectDiffKey].Kind)
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}
