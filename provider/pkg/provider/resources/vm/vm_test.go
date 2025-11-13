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

// Package vm_test contains comprehensive tests for VM resource operations.
package vm_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	vmResource "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Helper function to create a pointer to a string
func strPtr(s string) *string {
	return &s
}

// Helper function to create a pointer to an int
func intPtr(i int) *int {
	return &i
}

// Helper function to create a pointer to a bool
func boolPtr(b bool) *bool {
	return &b
}

func TestVMDiffDisksChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputDisks    []*vmResource.Disk
		stateDisks    []*vmResource.Disk
		expectChange  bool
		expectDiffKey string
	}{
		{
			name: "disk size changed",
			inputDisks: []*vmResource.Disk{
				{
					Size:      50,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
				{
					Size:      50,
					Interface: "scsi1",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks: []*vmResource.Disk{
				{
					Size:      40,
					Interface: "scsi0",
				},
			},
			stateDisks: []*vmResource.Disk{
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
			inputDisks:    []*vmResource.Disk{},
			stateDisks:    []*vmResource.Disk{},
			expectChange:  false,
			expectDiffKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  strPtr("test-vm"),
					Disks: tt.inputDisks,
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  strPtr("test-vm"),
						Disks: tt.stateDisks,
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
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

func TestVMDiffEfiDiskChange(t *testing.T) {
	t.Parallel()

	fileID1 := "vm-100-disk-efi"
	fileID2 := "vm-100-disk-efi-new"
	storage := "local-lvm"

	tests := []struct {
		name           string
		inputEfiDisk   *vmResource.EfiDisk
		stateEfiDisk   *vmResource.EfiDisk
		expectChange   bool
		expectDiffKeys []string // Changed to support multiple granular keys
		expectDiffKey  string   // Keep for backward compatibility (added/removed)
		description    string
	}{
		{
			name: "efi disk added",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk:  nil,
			expectChange:  true,
			expectDiffKey: "efidisk",
			description:   "Adding EFI disk should trigger diff",
		},
		{
			name:          "efi disk removed",
			inputEfiDisk:  nil,
			stateEfiDisk:  &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			expectChange:  true,
			expectDiffKey: "efidisk",
			description:   "Removing EFI disk should trigger diff",
		},
		{
			name:           "efi disk type changed",
			inputEfiDisk:   &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			stateEfiDisk:   &vmResource.EfiDisk{EfiType: vmResource.EfiType2M},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.efitype"},
			description:    "Changing EFI type should trigger diff on efitype only",
		},
		{
			name:         "efi disk unchanged",
			inputEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			stateEfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
			expectChange: false,
			description:  "Identical EFI disk should not trigger diff",
		},
		{
			name:         "both nil",
			inputEfiDisk: nil,
			stateEfiDisk: nil,
			expectChange: false,
			description:  "Both nil should not trigger diff",
		},
		{
			name: "FileID nil in input, present in state - no change",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			expectChange: false,
			description:  "FileID computed by provider should not trigger diff",
		},
		{
			name: "FileID explicitly set in input, different from state - change",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.fileId"},
			description:    "Explicitly set FileID that differs should trigger diff on fileId only",
		},
		{
			name: "FileID same in both - no change",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			expectChange: false,
			description:  "Same FileID should not trigger diff",
		},
		{
			name: "PreEnrolledKeys changed from true to false",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(false),
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true),
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Changing PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys added",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true),
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Adding PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys removed",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true),
			},
			expectChange:   true,
			expectDiffKeys: []string{"efidisk.preEnrolledKeys"},
			description:    "Removing PreEnrolledKeys should trigger diff on preEnrolledKeys only",
		},
		{
			name: "PreEnrolledKeys unchanged",
			inputEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true),
			},
			stateEfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true),
			},
			expectChange: false,
			description:  "Same PreEnrolledKeys should not trigger diff",
		},
	}

	// Set up FileID scenarios after struct initialization
	// Test 5: FileID nil in input, present in state
	tests[5].inputEfiDisk.Storage = storage
	tests[5].inputEfiDisk.FileID = nil // User didn't specify
	tests[5].stateEfiDisk.Storage = storage
	tests[5].stateEfiDisk.FileID = &fileID1 // Computed from API

	// Test 6: Different FileIDs
	tests[6].inputEfiDisk.Storage = storage
	tests[6].inputEfiDisk.FileID = &fileID2 // User explicitly set it
	tests[6].stateEfiDisk.Storage = storage
	tests[6].stateEfiDisk.FileID = &fileID1

	// Test 7: Same FileIDs
	tests[7].inputEfiDisk.Storage = storage
	tests[7].inputEfiDisk.FileID = &fileID1
	tests[7].stateEfiDisk.Storage = storage
	tests[7].stateEfiDisk.FileID = &fileID1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:    strPtr("test-vm"),
					EfiDisk: tt.inputEfiDisk,
					Disks:   []*vmResource.Disk{}, // Empty disks to focus on EFI
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:    strPtr("test-vm"),
						EfiDisk: tt.stateEfiDisk,
						Disks:   []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, tt.description)
				if tt.expectDiffKey != "" {
					// For added/removed cases, check the single key
					assert.Contains(t, resp.DetailedDiff, tt.expectDiffKey, "Expected diff key to be present")
				}
				if len(tt.expectDiffKeys) > 0 {
					// For granular changes, verify each expected key is present
					for _, key := range tt.expectDiffKeys {
						assert.Contains(t, resp.DetailedDiff, key, "Expected granular diff key to be present: %s", key)
					}
				}
			} else {
				assert.False(t, resp.HasChanges, tt.description)
			}
		})
	}
}

func TestVMDiffComputedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		inputVMID         *int
		stateVMID         *int
		inputNode         *string
		stateNode         *string
		expectChange      bool
		expectReplace     bool
		expectedDiffField string
	}{
		{
			name:         "vmId nil in input, present in state (computed)",
			inputVMID:    nil,
			stateVMID:    intPtr(100),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "vmId changed - should trigger replace",
			inputVMID:         intPtr(200),
			stateVMID:         intPtr(100),
			expectChange:      true,
			expectReplace:     true,
			expectedDiffField: "vmId",
		},
		{
			name:         "vmId unchanged",
			inputVMID:    intPtr(100),
			stateVMID:    intPtr(100),
			expectChange: false,
		},
		{
			name:         "node nil in input, present in state (computed)",
			inputNode:    nil,
			stateNode:    strPtr("pve-node1"),
			expectChange: false, // Computed field, no change expected
		},
		{
			name:              "node changed",
			inputNode:         strPtr("pve-node2"),
			stateNode:         strPtr("pve-node1"),
			expectChange:      true,
			expectedDiffField: "node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:  strPtr("test-vm"),
					VMID:  tt.inputVMID,
					Node:  tt.inputNode,
					Disks: []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:  strPtr("test-vm"),
						VMID:  tt.stateVMID,
						Node:  tt.stateNode,
						Disks: []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			if tt.expectChange {
				assert.True(t, resp.HasChanges, "Expected changes to be detected")
				if tt.expectReplace {
					assert.Equal(t, p.UpdateReplace, resp.DetailedDiff[tt.expectedDiffField].Kind)
					assert.True(t, resp.DeleteBeforeReplace)
				} else if tt.expectedDiffField != "" {
					assert.Equal(t, p.Update, resp.DetailedDiff[tt.expectedDiffField].Kind)
				}
			} else {
				assert.False(t, resp.HasChanges, "Expected no changes")
			}
		})
	}
}

func TestVMDiffPointerFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputMemory  *int
		stateMemory  *int
		inputCores   *int
		stateCores   *int
		expectChange bool
	}{
		{
			name:         "memory changed",
			inputMemory:  intPtr(4096),
			stateMemory:  intPtr(2048),
			expectChange: true,
		},
		{
			name:         "memory unchanged",
			inputMemory:  intPtr(2048),
			stateMemory:  intPtr(2048),
			expectChange: false,
		},
		{
			name:         "memory cleared (set to nil)",
			inputMemory:  nil,
			stateMemory:  intPtr(2048),
			expectChange: true,
		},
		{
			name:         "memory set from nil",
			inputMemory:  intPtr(2048),
			stateMemory:  nil,
			expectChange: true,
		},
		{
			name:         "cores changed",
			inputCores:   intPtr(4),
			stateCores:   intPtr(2),
			expectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vm := &vmResource.VM{}
			req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
				ID: "100",
				Inputs: vmResource.Inputs{
					Name:   strPtr("test-vm"),
					Memory: tt.inputMemory,
					Cores:  tt.inputCores,
					Disks:  []*vmResource.Disk{},
				},
				State: vmResource.Outputs{
					Inputs: vmResource.Inputs{
						Name:   strPtr("test-vm"),
						Memory: tt.stateMemory,
						Cores:  tt.stateCores,
						Disks:  []*vmResource.Disk{},
					},
				},
			}

			resp, err := vm.Diff(context.Background(), req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectChange, resp.HasChanges)
		})
	}
}

func TestVMDiffMultipleChanges(t *testing.T) {
	t.Parallel()

	vm := &vmResource.VM{}
	req := infer.DiffRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			Name:   strPtr("new-name"),
			Memory: intPtr(4096),
			Cores:  intPtr(4),
			Disks: []*vmResource.Disk{
				{Size: 50, Interface: "scsi0"},
			},
			EfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType4M},
		},
		State: vmResource.Outputs{
			Inputs: vmResource.Inputs{
				Name:   strPtr("old-name"),
				Memory: intPtr(2048),
				Cores:  intPtr(2),
				Disks: []*vmResource.Disk{
					{Size: 40, Interface: "scsi0"},
				},
				EfiDisk: &vmResource.EfiDisk{EfiType: vmResource.EfiType2M},
			},
		},
	}

	resp, err := vm.Diff(context.Background(), req)
	require.NoError(t, err)

	assert.True(t, resp.HasChanges)
	assert.Contains(t, resp.DetailedDiff, "name")
	assert.Contains(t, resp.DetailedDiff, "memory")
	assert.Contains(t, resp.DetailedDiff, "cores")
	assert.Contains(t, resp.DetailedDiff, "disks")
	// EfiDisk now produces granular diffs
	assert.Contains(t, resp.DetailedDiff, "efidisk.efitype")

	// All should be updates, not replacements
	for key, diff := range resp.DetailedDiff {
		if key == "vmId" {
			assert.Equal(t, p.UpdateReplace, diff.Kind)
		} else {
			assert.Equal(t, p.Update, diff.Kind)
		}
	}
}

//nolint:paralleltest // uses global env + client seam
func TestVMUpdateEfiDiskSuccess(t *testing.T) {
	mock, cleanup := resources.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	// Mock GET /cluster/status (called by FindVirtualMachine -> Cluster())
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/status (called by Node())
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/status/current to check VM exists
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	//  Mock GET /nodes/{node}/qemu/{vmid}/config (called by node.VirtualMachine())
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(
				`{"data":{"vmid":100,"name":"test-vm"}}`,
			)),
	).Enable()

	// Mock POST /nodes/{node}/qemu/{vmid}/config for the update (go-proxmox uses POST not PUT)
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(`{"data":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:"}`)),
	).Enable()

	// Mock task status endpoint - return completed task
	// Use ReplyFunction instead of Reply when using Repeat (mocha bug workaround)
	taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	taskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(taskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.UpdateRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			VMID: intPtr(vmID),
			Name: strPtr("test-vm"),
			EfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M, // Changed from 2m
			},
		},
		State: vmResource.Outputs{
			Inputs: vmResource.Inputs{
				VMID: intPtr(vmID),
				Name: strPtr("test-vm"),
				Node: &nodeName,
				EfiDisk: &vmResource.EfiDisk{
					EfiType: vmResource.EfiType2M,
				},
			},
		},
	}

	// Set storage and FileID on diskBase (embedded struct)
	req.Inputs.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.FileID = strPtr("vm-100-disk-0")

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, vmResource.EfiType4M, resp.Output.EfiDisk.EfiType)
	// FileID should have been copied from state
	assert.Equal(t, "vm-100-disk-0", *resp.Output.EfiDisk.FileID)
	mock.AssertCalled(t)
}

//nolint:paralleltest // uses global env + client seam
func TestVMUpdateEfiDiskPreEnrolledKeysChange(t *testing.T) {
	mock, cleanup := resources.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(
				`{"data":{"vmid":100,"name":"test-vm"}}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(`{"data":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:"}`)),
	).Enable()

	// Mock task status endpoint - return completed task
	// Must include all Task fields to prevent copier.Copy from clearing them during unmarshal
	// Use ReplyFunction instead of Reply when using Repeat (mocha bug workaround)
	taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	taskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(taskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.UpdateRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			VMID: intPtr(vmID),
			Name: strPtr("test-vm"),
			EfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(true), // Changed from nil
			},
		},
		State: vmResource.Outputs{
			Inputs: vmResource.Inputs{
				VMID: intPtr(vmID),
				Name: strPtr("test-vm"),
				Node: &nodeName,
				EfiDisk: &vmResource.EfiDisk{
					EfiType: vmResource.EfiType4M,
				},
			},
		},
	}

	// Set storage and FileID on diskBase
	req.Inputs.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.Storage = "local-lvm"
	req.State.EfiDisk.FileID = strPtr("vm-100-disk-0")

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, *resp.Output.EfiDisk.PreEnrolledKeys)
	mock.AssertCalled(t)
}

//nolint:paralleltest // uses global env + client seam
func TestVMReadWithEfiDisk(t *testing.T) {
	mock, cleanup := resources.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	// Mock GET /cluster/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/status/current
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/config - VM with EFI disk
	vmConfigJSON := `{"data":{"vmid":100,"name":"test-vm","cores":2,"memory":2048,` +
		`"efidisk0":"local-lvm:vm-100-disk-0,efitype=4m,pre-enrolled-keys=1"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.ReadRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			VMID: intPtr(vmID),
			Node: &nodeName,
		},
	}

	resp, err := vm.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "100", resp.ID)
	assert.NotNil(t, resp.State.EfiDisk)
	assert.Equal(t, vmResource.EfiType4M, resp.State.EfiDisk.EfiType)
	assert.NotNil(t, resp.State.EfiDisk.PreEnrolledKeys)
	assert.True(t, *resp.State.EfiDisk.PreEnrolledKeys)
	assert.Equal(t, "local-lvm", resp.State.EfiDisk.Storage)
	assert.NotNil(t, resp.State.EfiDisk.FileID)
	assert.Equal(t, "vm-100-disk-0", *resp.State.EfiDisk.FileID)
}

//nolint:paralleltest // uses global env + client seam
func TestVMReadWithoutEfiDisk(t *testing.T) {
	mock, cleanup := resources.NewAPIMock(t)
	defer cleanup()

	vmID := 100
	nodeName := "pve-node"

	// Mock GET /cluster/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/status/current
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":100}}`,
			)),
	).Enable()

	// Mock GET /nodes/{node}/qemu/{vmid}/config - VM without EFI disk
	vmConfigJSON := `{"data":{"vmid":100,"name":"test-vm","cores":2,"memory":2048}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.ReadRequest[vmResource.Inputs, vmResource.Outputs]{
		ID: "100",
		Inputs: vmResource.Inputs{
			VMID: intPtr(vmID),
			Node: &nodeName,
		},
	}

	resp, err := vm.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "100", resp.ID)
	assert.Nil(t, resp.State.EfiDisk)
}

//nolint:paralleltest // uses global env + client seam
func TestVMCloneRemovesUnwantedEfiDisk(t *testing.T) {
	mock, cleanup := resources.NewAPIMock(t)
	defer cleanup()

	nodeName := "pve-node"
	sourceVMID := 999
	newVMID := 100

	// Mock GET /cluster/status (for getting next ID and finding VMs)
	// Format: array with type="cluster" and type="node" elements
	clusterStatusJSON := `{"data":[{"type":"cluster","id":"cluster","name":"testcluster","version":4,"quorate":1},` +
		`{"type":"node","name":"` + nodeName + `","online":1,"id":"node/` + nodeName + `"}]}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clusterStatusJSON)}, nil
			}),
	).Enable()

	// Mock GET /cluster/nextid
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/nextid")).
			Reply(reply.OK().BodyString(`{"data":"100"}`)),
	).Enable()

	// Mock GET /nodes/{node}/status
	nodeStatusJSON := `{"data":{"node":"` + nodeName + `","status":"online"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nodeStatusJSON)}, nil
			}).
			Repeat(10),
	).Enable()

	// Mock GET source VM status
	sourceVMStatusJSON := `{"data":{"status":"running","vmid":999}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/999/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(sourceVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock GET source VM config (has EFI disk)
	sourceVMConfigJSON := `{"data":{"vmid":999,"name":"source-vm","cores":2,"memory":2048,` +
		`"efidisk0":"local-lvm:vm-999-disk-0,efitype=4m"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/999/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(sourceVMConfigJSON)}, nil
			}),
	).Enable()

	// Mock POST /nodes/{node}/qemu/{vmid}/clone
	cloneResponseJSON := `{"data":"UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/999/clone")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(cloneResponseJSON)}, nil
			}),
	).Enable()

	// Mock clone task status (running then stopped)
	cloneTaskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(cloneTaskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:",` +
					`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmclone",` +
					`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	// Mock GET new VM status (after clone)
	newVMStatusJSON := `{"data":{"status":"stopped","vmid":100}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock GET new VM config (after clone)
	// First call returns with EFI disk, subsequent calls (after unlink) return without
	var configCallCount int
	clonedVMConfigWithEfi := `{"data":{"vmid":100,"name":"cloned-vm","cores":2,"memory":2048,` +
		`"efidisk0":"local-lvm:vm-100-disk-0,efitype=4m"}}`
	clonedVMConfigWithoutEfi := `{"data":{"vmid":100,"name":"cloned-vm","cores":2,"memory":2048}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				defer func() { configCallCount++ }()
				// First call: return WITH EFI, later calls: return WITHOUT EFI
				if configCallCount == 0 {
					return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clonedVMConfigWithEfi)}, nil
				}
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clonedVMConfigWithoutEfi)}, nil
			}),
	).Enable()

	// Mock PUT /nodes/{node}/qemu/{vmid}/unlink - for removing EFI disk
	unlinkResponseJSON := `{"data":"UPID:pve-node:00005678:00000000:00000000:qmunlink:100:root@pam:"}`
	mock.AddMocks(
		mocha.Put(expect.URLPath("/nodes/" + nodeName + "/qemu/100/unlink")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(unlinkResponseJSON)}, nil
			}),
	).Enable()

	// Mock unlink task status
	unlinkTaskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00005678:00000000:00000000:qmunlink:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(unlinkTaskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00005678:00000000:00000000:qmunlink:100:root@pam:",` +
					`"node":"pve-node","pid":5678,"pstart":0,"starttime":1699999999,"type":"qmunlink",` +
					`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	// Mock POST /nodes/{node}/qemu/{vmid}/config (finalizing clone without EFI disk)
	configResponseJSON := `{"data":"UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(configResponseJSON)}, nil
			}),
	).Enable()

	// Mock config task status
	configTaskStatusURL := "/nodes/pve-node/tasks/UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(configTaskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				taskStatusJSON := `{"data":{"upid":"UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:",` +
					`"node":"pve-node","pid":9999,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
					`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.CreateRequest[vmResource.Inputs]{
		Name: "cloned-vm",
		Inputs: vmResource.Inputs{
			Name: strPtr("cloned-vm"),
			Node: &nodeName,
			Clone: &vmResource.Clone{
				VMID:    sourceVMID,
				Timeout: 300,
			},
			// Note: No EfiDisk specified - should be removed from cloned VM
		},
	}

	resp, err := vm.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	// Verify EFI disk was removed
	assert.Nil(t, resp.Output.EfiDisk)
	mock.AssertCalled(t)
}

//nolint:paralleltest // Uses shared API mock server
func TestVMCloneAddsEfiDisk(t *testing.T) {
	nodeName := "pve-node"
	sourceVMID := 999
	newVMID := 100

	mock, _ := resources.NewAPIMock(t)

	// Mock cluster status
	clusterStatusJSON := `{"data":[{"type":"cluster","quorate":1,"nodes":1},{"type":"node","name":"pve-node","online":1}]}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clusterStatusJSON)}, nil
			}),
	).Enable()

	// Mock next VMID
	nextIDJSON := `{"data":"100"}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/nextid")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nextIDJSON)}, nil
			}),
	).Enable()

	// Mock node status
	nodeStatusJSON := `{"data":{"status":"online"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nodeStatusJSON)}, nil
			}),
	).Enable()

	// Mock source VM status
	sourceVMStatusJSON := `{"data":{"status":"running","vmid":999}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/999/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(sourceVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock source VM config - WITHOUT EFI disk
	sourceVMConfigJSON := `{"data":{"vmid":999,"name":"source-vm","cores":2,"memory":2048}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/999/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(sourceVMConfigJSON)}, nil
			}),
	).Enable()

	// Mock clone operation
	cloneResponseJSON := `{"data":"UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/999/clone")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(cloneResponseJSON)}, nil
			}),
	).Enable()

	// Mock clone task status
	cloneTaskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:",` +
		`"node":"pve-node","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmclone",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	cloneTaskURL := "/nodes/pve-node/tasks/UPID:pve-node:00001234:00000000:00000000:qmclone:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(cloneTaskURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(cloneTaskStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM status
	newVMStatusJSON := `{"data":{"status":"stopped","vmid":100}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM config - first call returns WITHOUT EFI (after clone),
	// subsequent calls return WITH EFI (after config update)
	var configCallCount int
	clonedVMConfigWithoutEfi := `{"data":{"vmid":100,"name":"cloned-vm","cores":2,"memory":2048}}`
	clonedVMConfigWithEfi := `{"data":{"vmid":100,"name":"cloned-vm","cores":2,"memory":2048,` +
		`"efidisk0":"local-lvm:vm-100-disk-0,efitype=4m"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				defer func() { configCallCount++ }()
				// First call: return WITHOUT EFI, later calls: return WITH EFI
				if configCallCount == 0 {
					return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clonedVMConfigWithoutEfi)}, nil
				}
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clonedVMConfigWithEfi)}, nil
			}),
	).Enable()

	// Mock POST config to add EFI disk
	configResponseJSON := `{"data":"UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(configResponseJSON)}, nil
			}),
	).Enable()

	// Mock config task status
	configTaskStatusJSON := `{"data":{"upid":"UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:",` +
		`"node":"pve-node","pid":9999,"pstart":0,"starttime":1699999999,"type":"qmconfig",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	configTaskURL := "/nodes/pve-node/tasks/UPID:pve-node:00009999:00000000:00000000:qmconfig:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(configTaskURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(configTaskStatusJSON)}, nil
			}),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.CreateRequest[vmResource.Inputs]{
		Name: "cloned-vm-with-efi",
		Inputs: vmResource.Inputs{
			Name: strPtr("cloned-vm"),
			Node: &nodeName,
			Clone: &vmResource.Clone{
				VMID:    sourceVMID,
				Timeout: 300,
			},
			// Add EFI disk even though source doesn't have one
			EfiDisk: &vmResource.EfiDisk{
				EfiType: vmResource.EfiType4M,
			},
		},
	}

	resp, err := vm.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	// Verify EFI disk was added
	assert.NotNil(t, resp.Output.EfiDisk)
	assert.Equal(t, vmResource.EfiType4M, resp.Output.EfiDisk.EfiType)
	mock.AssertCalled(t)
}

//nolint:paralleltest // Uses shared API mock server
func TestVMCreateWithEfiDisk(t *testing.T) {
	nodeName := "pve-node"
	newVMID := 100

	mock, _ := resources.NewAPIMock(t)

	// Mock cluster status
	clusterStatusJSON := `{"data":[{"type":"cluster","quorate":1,"nodes":1},{"type":"node","name":"pve-node","online":1}]}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clusterStatusJSON)}, nil
			}).
			Repeat(10),
	).Enable()

	// Mock next VMID
	nextIDJSON := `{"data":"100"}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/nextid")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nextIDJSON)}, nil
			}),
	).Enable()

	// Mock node status
	nodeStatusJSON := `{"data":{"status":"online"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(nodeStatusJSON)}, nil
			}),
	).Enable()

	// Mock VM creation POST - verify it includes EFI disk settings
	createResponseJSON := `{"data":"UPID:pve-node:00001111:00000000:00000000:qmcreate:100:root@pam:"}`
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				// Verify the request body contains EFI disk settings
				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)
				assert.Contains(t, bodyStr, "efidisk0")
				assert.Contains(t, bodyStr, "efitype=4m")
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(createResponseJSON)}, nil
			}),
	).Enable()

	// Mock create task status
	createTaskStatusJSON := `{"data":{"upid":"UPID:pve-node:00001111:00000000:00000000:qmcreate:100:root@pam:",` +
		`"node":"pve-node","pid":1111,"pstart":0,"starttime":1699999999,"type":"qmcreate",` +
		`"id":"100","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	createTaskURL := "/nodes/pve-node/tasks/UPID:pve-node:00001111:00000000:00000000:qmcreate:100:root@pam:/status"
	mock.AddMocks(
		mocha.Get(expect.URLPath(createTaskURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(createTaskStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM status
	newVMStatusJSON := `{"data":{"status":"stopped","vmid":100}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/status/current")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMStatusJSON)}, nil
			}),
	).Enable()

	// Mock new VM config - return with EFI disk
	newVMConfigJSON := `{"data":{"vmid":100,"name":"test-vm-with-efi","cores":2,"memory":2048,` +
		`"efidisk0":"local-lvm:vm-100-disk-0,efitype=4m,pre-enrolled-keys=0"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/100/config")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(newVMConfigJSON)}, nil
			}),
	).Enable()

	vm := &vmResource.VM{}
	req := infer.CreateRequest[vmResource.Inputs]{
		Name: "test-vm-with-efi",
		Inputs: vmResource.Inputs{
			Name:   strPtr("test-vm-with-efi"),
			Node:   &nodeName,
			Cores:  intPtr(2),
			Memory: intPtr(2048),
			// No Clone settings - creating a new VM from scratch
			EfiDisk: &vmResource.EfiDisk{
				EfiType:         vmResource.EfiType4M,
				PreEnrolledKeys: boolPtr(false),
			},
		},
	}

	resp, err := vm.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, req.Name, resp.ID)
	assert.Equal(t, newVMID, *resp.Output.VMID)
	// Verify EFI disk was created with correct settings
	assert.NotNil(t, resp.Output.EfiDisk)
	assert.Equal(t, vmResource.EfiType4M, resp.Output.EfiDisk.EfiType)
	assert.NotNil(t, resp.Output.EfiDisk.PreEnrolledKeys)
	assert.False(t, *resp.Output.EfiDisk.PreEnrolledKeys)
	mock.AssertCalled(t)
}
