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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// TestVMReadPreservesNumaFalse verifies that when a user explicitly sets numa: false,
// it is preserved in the state after a read operation. This test addresses the issue where
// numa: false was not preserved, causing spurious diffs on every update.
func TestVMReadPreservesNumaFalse(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	mockOps := &mockVMOperations{
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			// Simulate what Proxmox returns: numa=0 means the field is nil in our domain model
			return proxmox.VMInputs{
				VMID: testutils.Ptr(vmID),
				Name: "numa-false-test",
				CPU: &proxmox.CPU{
					Cores:   testutils.Ptr(2),
					Sockets: testutils.Ptr(1),
					Numa:    nil, // API returns nil when numa is 0 (disabled)
				},
			}, nil
		},
	}

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps:  mockOps,
	}

	numaFalse := false
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
			CPU: &proxmox.CPU{
				Cores:   testutils.Ptr(2),
				Sockets: testutils.Ptr(1),
				Numa:    &numaFalse, // User specified numa: false
			},
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)

	// preserveInputs must keep the user's explicit numa: false in Inputs
	require.NotNil(t, resp.Inputs.CPU)
	require.NotNil(t, resp.Inputs.CPU.Numa, "numa: false must not be dropped to nil by preserveInputs")
	assert.False(t, *resp.Inputs.CPU.Numa)

	// State must also preserve user's numa: false so the state file records user intent
	require.NotNil(t, resp.State.CPU)
	require.NotNil(t, resp.State.CPU.Numa, "state must preserve numa: false for state file")
	assert.False(t, *resp.State.CPU.Numa)
}

// TestVMCreatePreservesNumaFalse verifies that Create stores numa: false in the output
// state without calling Get (the old readCurrentOutput behavior which masked user input).
func TestVMCreatePreservesNumaFalse(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	mockOps := &mockVMOperations{
		createVMFunc: func(_ context.Context, _ proxmox.VMInputs) error {
			return nil
		},
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			t.Fatal("Create must not read VM state from API — that masked user inputs")
			return proxmox.VMInputs{}, nil
		},
	}

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps:  mockOps,
	}

	numaFalse := false
	createReq := infer.CreateRequest[proxmox.VMInputs]{
		Name: "numa-false-test",
		Inputs: proxmox.VMInputs{
			Name: "numa-false-test",
			Node: &nodeName,
			CPU: &proxmox.CPU{
				Cores:   testutils.Ptr(2),
				Sockets: testutils.Ptr(1),
				Numa:    &numaFalse,
			},
			Disks: []*proxmox.Disk{},
		},
	}

	createResp, err := vmRes.Create(context.Background(), createReq)
	require.NoError(t, err)

	// Output state must contain the user's numa: false, not nil
	require.NotNil(t, createResp.Output.CPU)
	require.NotNil(t, createResp.Output.CPU.Numa, "Create output must preserve numa: false")
	assert.False(t, *createResp.Output.CPU.Numa)
}
