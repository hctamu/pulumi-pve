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
			// Simulate what Proxmox returns: no numa nodes and numa=0 (or not set)
			return proxmox.VMInputs{
				VMID: testutils.Ptr(vmID),
				Name: "numa-false-test",
				CPU: &proxmox.CPU{
					Cores:   testutils.Ptr(2),
					Sockets: testutils.Ptr(1),
					Numa:    nil, // API returns nil when numa is not set
				},
			}, nil
		},
	}

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps:  mockOps,
	}

	// User specified numa: false in their input
	numaFalse := false
	req := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
			CPU: &proxmox.CPU{
				Cores:   testutils.Ptr(2),
				Sockets: testutils.Ptr(1),
				Numa:    &numaFalse,
			},
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)

	// Verify that numa: false is preserved in the Inputs (not changed to nil)
	require.NotNil(t, resp.Inputs.CPU)
	require.NotNil(t, resp.Inputs.CPU.Numa)
	assert.False(t, *resp.Inputs.CPU.Numa, "numa should be preserved as false, not changed to nil")

	// Also verify in the State - since preserveInputs only affects Inputs,
	// State will have what the API returned, but Inputs should match user intent
	require.NotNil(t, resp.State.CPU)
}

// TestVMCreatePreservesNumaFalseThenRead verifies the full lifecycle:
// Create with numa: false, then read back and verify no spurious diff.
func TestVMCreatePreservesNumaFalseThenRead(t *testing.T) {
	t.Parallel()

	vmID := 100
	nodeName := "pve-node"

	mockOps := &mockVMOperations{
		createVMFunc: func(_ context.Context, inputs proxmox.VMInputs) error {
			return nil
		},
		getFunc: func(_ context.Context, _ int, _ *string, _ []*proxmox.Disk) (proxmox.VMInputs, error) {
			// After creation, Proxmox returns the VM state
			// Numa field is nil because it was set to 0 (false) and there are no numa nodes
			return proxmox.VMInputs{
				VMID: testutils.Ptr(vmID),
				Name: "numa-false-test",
				CPU: &proxmox.CPU{
					Cores:   testutils.Ptr(2),
					Sockets: testutils.Ptr(1),
					Numa:    nil, // This would previously be nil, not false
				},
			}, nil
		},
	}

	vmRes := &vm.VM{
		Client: &testutils.MockProxmoxClient{DefaultNode: nodeName, DefaultVMID: vmID},
		VMOps:  mockOps,
	}

	// Create with numa: false
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

	// Verify creation captured numa: false
	require.NotNil(t, createResp.Output.CPU)
	require.NotNil(t, createResp.Output.CPU.Numa)
	assert.False(t, *createResp.Output.CPU.Numa)

	// Now do a read to simulate a subsequent pulumi up
	readReq := infer.ReadRequest[proxmox.VMInputs, proxmox.VMOutputs]{
		ID: "100",
		Inputs: proxmox.VMInputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
			CPU: &proxmox.CPU{
				Cores:   testutils.Ptr(2),
				Sockets: testutils.Ptr(1),
				Numa:    &numaFalse, // User still has numa: false
			},
		},
		State: createResp.Output,
	}

	readResp, err := vmRes.Read(context.Background(), readReq)
	require.NoError(t, err)

	// Key verification: numa: false should be preserved
	require.NotNil(t, readResp.Inputs.CPU)
	require.NotNil(t, readResp.Inputs.CPU.Numa)
	assert.False(t, *readResp.Inputs.CPU.Numa, "numa: false should be preserved through Read, not become nil")

	// This ensures that a subsequent Update/Diff won't detect a spurious change
}
