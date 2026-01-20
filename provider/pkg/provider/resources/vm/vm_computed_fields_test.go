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

package vm

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
	api "github.com/luthermonson/go-proxmox"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"
)

// createMockVMWithConfig creates a minimal api.VirtualMachine with basic fields populated
// for testing ConvertVMConfigToInputs and preservation logic.
func createMockVMWithConfig(node string, vmid uint64, diskConfigs map[string]string, efi string) *api.VirtualMachine {
	cfg := &api.VirtualMachineConfig{
		Name:        "test-vm",
		Description: "desc",
	}

	// Populate disk fields based on interface name
	for iface, conf := range diskConfigs {
		switch iface {
		case "scsi0":
			cfg.SCSI0 = conf
		case "scsi1":
			cfg.SCSI1 = conf
		case "virtio0":
			cfg.VirtIO0 = conf
		case "ide2":
			cfg.IDE2 = conf
		case "sata0":
			cfg.SATA0 = conf
		}
	}
	if efi != "" {
		cfg.EFIDisk0 = efi
	}

	return &api.VirtualMachine{
		Node:                 node,
		VMID:                 api.StringOrUint64(vmid),
		VirtualMachineConfig: cfg,
	}
}

func TestConvertAndPreserve_NoVMIDOrNodeInPrev(t *testing.T) {
	t.Parallel()

	// Prev inputs: user did not provide vmId/node and omitted disk/efi file IDs
	prev := Inputs{
		Disks: []*Disk{{
			DiskBase:  DiskBase{Storage: "local-lvm"},
			Interface: "scsi0",
			Size:      32,
		}},
		// No EfiDisk in prev
	}

	// API VM returns concrete vmId/node and file IDs
	vm := createMockVMWithConfig(
		"pve-node1",
		100,
		map[string]string{
			"scsi0": "local-lvm:vm-100-disk-0,size=32G",
		},
		"local-lvm:vm-100-efidisk,size=1G,efitype=4m,pre-enrolled-keys=0",
	)

	computed, err := ConvertVMConfigToInputs(vm, prev)
	require.NoError(t, err)

	// Computed should carry values from API
	require.NotNil(t, computed.VMID)
	assert.Equal(t, 100, *computed.VMID)
	require.NotNil(t, computed.Node)
	assert.Equal(t, "pve-node1", *computed.Node)
	require.NotNil(t, computed.Disks)
	require.Len(t, computed.Disks, 1)
	require.NotNil(t, computed.Disks[0].FileID)
	assert.Equal(t, "vm-100-disk-0", *computed.Disks[0].FileID)
	// Efi present in computed
	require.NotNil(t, computed.EfiDisk)
	require.NotNil(t, computed.EfiDisk.FileID)

	// Preserve emptiness based on prev
	preserved := preserveComputedInputEmptiness(prev, computed)

	// VMID and Node must remain nil in Inputs
	assert.Nil(t, preserved.VMID)
	assert.Nil(t, preserved.Node)
	// Disk FileID cleared because prev omitted it
	require.Len(t, preserved.Disks, 1)
	assert.Nil(t, preserved.Disks[0].FileID)
	// EfiDisk removed entirely because prev did not have it
	assert.Nil(t, preserved.EfiDisk)
}

func TestConvertAndPreserve_WithVMIDAndNodeInPrev(t *testing.T) {
	t.Parallel()

	vmid := 100
	node := "pve-node1"
	prev := Inputs{
		VMID: &vmid,
		Node: &node,
		EfiDisk: &EfiDisk{ // present but without FileID
			DiskBase: DiskBase{Storage: "local-lvm"},
			EfiType:  EfiType4M,
		},
		Disks: []*Disk{{
			DiskBase:  DiskBase{Storage: "local-lvm"},
			Interface: "scsi0",
			Size:      32,
		}},
	}

	vm := createMockVMWithConfig(
		node,
		100,
		map[string]string{
			"scsi0": "local-lvm:vm-100-disk-0,size=32G",
		},
		"local-lvm:vm-100-efidisk,size=1G,efitype=4m,pre-enrolled-keys=1",
	)

	computed, err := ConvertVMConfigToInputs(vm, prev)
	require.NoError(t, err)

	preserved := preserveComputedInputEmptiness(prev, computed)

	// VMID and Node should remain set
	require.NotNil(t, preserved.VMID)
	assert.Equal(t, vmid, *preserved.VMID)
	require.NotNil(t, preserved.Node)
	assert.Equal(t, node, *preserved.Node)

	// Disk FileID still nil because prev omitted it
	require.Len(t, preserved.Disks, 1)
	assert.Nil(t, preserved.Disks[0].FileID)

	// EfiDisk remains present but FileID stays nil because prev omitted it
	require.NotNil(t, preserved.EfiDisk)
	assert.Nil(t, preserved.EfiDisk.FileID)
}

// --- Update IO handling tests ---

func TestUpdateCopiesVMIDAndNodeFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 123
	stateNode := "pve-node1"

	req := infer.UpdateRequest[Inputs, Outputs]{
		ID:     "vm-123",
		DryRun: true,
		Inputs: Inputs{ /* VMID & Node omitted */ },
		State:  Outputs{Inputs: Inputs{VMID: &stateVMID, Node: &stateNode}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// response.Output should carry VMID and Node copied from prior state
	require.NotNil(t, resp.Output.VMID)
	assert.Equal(t, stateVMID, *resp.Output.VMID)
	require.NotNil(t, resp.Output.Node)
	assert.Equal(t, stateNode, *resp.Output.Node)
}

func TestUpdateCopiesDiskFileIDsFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 200
	stateNode := "pve-node2"

	req := infer.UpdateRequest[Inputs, Outputs]{
		ID:     "vm-200",
		DryRun: true,
		Inputs: Inputs{
			// Disks omit FileID but have same interfaces
			Disks: []*Disk{
				{Interface: "scsi0", DiskBase: DiskBase{Storage: "local-lvm"}, Size: 32},
				{Interface: "scsi1", DiskBase: DiskBase{Storage: "local-lvm"}, Size: 64},
			},
		},
		State: Outputs{Inputs: Inputs{
			VMID: &stateVMID,
			Node: &stateNode,
			Disks: []*Disk{
				{
					Interface: "scsi0",
					DiskBase:  DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-200-disk-0")},
					Size:      32,
				},
				{
					Interface: "scsi1",
					DiskBase:  DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-200-disk-1")},
					Size:      64,
				},
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// VMID/Node should be copied as well
	require.NotNil(t, resp.Output.VMID)
	require.NotNil(t, resp.Output.Node)

	// FileIDs should be copied for both disks
	require.Len(t, resp.Output.Disks, 2)
	if assert.NotNil(t, resp.Output.Disks[0].FileID) {
		assert.Equal(t, "vm-200-disk-0", *resp.Output.Disks[0].FileID)
	}
	if assert.NotNil(t, resp.Output.Disks[1].FileID) {
		assert.Equal(t, "vm-200-disk-1", *resp.Output.Disks[1].FileID)
	}
}

func TestUpdateCopiesEfiFileIDFromState(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 300
	stateNode := "pve-node3"

	req := infer.UpdateRequest[Inputs, Outputs]{
		ID:     "vm-300",
		DryRun: true,
		Inputs: Inputs{
			// EfiDisk present but fileId omitted
			EfiDisk: &EfiDisk{DiskBase: DiskBase{Storage: "local-lvm"}, EfiType: EfiType4M},
		},
		State: Outputs{Inputs: Inputs{
			VMID: &stateVMID,
			Node: &stateNode,
			EfiDisk: &EfiDisk{
				DiskBase: DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-300-efidisk")},
				EfiType:  EfiType4M,
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// EFI disk FileID should be copied from state
	require.NotNil(t, resp.Output.EfiDisk)
	require.NotNil(t, resp.Output.EfiDisk.FileID)
	assert.Equal(t, "vm-300-efidisk", *resp.Output.EfiDisk.FileID)
}

func TestUpdateDoesNotOverwriteUserProvidedFileIDs(t *testing.T) {
	t.Parallel()

	vm := &VM{}
	stateVMID := 400
	stateNode := "pve-node4"

	req := infer.UpdateRequest[Inputs, Outputs]{
		ID:     "vm-400",
		DryRun: true,
		Inputs: Inputs{
			Disks: []*Disk{
				{Interface: "scsi0", DiskBase: DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("custom-file")}, Size: 32},
			},
		},
		State: Outputs{Inputs: Inputs{
			VMID: &stateVMID,
			Node: &stateNode,
			Disks: []*Disk{
				{
					Interface: "scsi0",
					DiskBase:  DiskBase{Storage: "local-lvm", FileID: testutils.Ptr("vm-400-disk-0")},
					Size:      32,
				},
			},
		}},
	}

	resp, err := vm.Update(context.Background(), req)
	require.NoError(t, err)

	// User-provided FileID should remain unchanged
	require.Len(t, resp.Output.Disks, 1)
	if assert.NotNil(t, resp.Output.Disks[0].FileID) {
		assert.Equal(t, "custom-file", *resp.Output.Disks[0].FileID)
	}
}

// --- Read IO handling tests (end-to-end with API mock) ---

//nolint:paralleltest // uses global env + client seam via testutils.NewAPIMock
func TestVMReadComputedAndPreserved_NoPrevIDs(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	vmID := 200
	nodeName := "pve-node"

	// Cluster and node status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			Repeat(3).
			Reply(reply.OK().BodyString(
				`{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`,
			)),
	).Enable()

	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/status")).
			Repeat(2).
			Reply(reply.OK().BodyString(
				`{"data":{"node":"` + nodeName + `","status":"online"}}`,
			)),
	).Enable()

	// VM status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/200/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":200}}`,
			)),
	).Enable()

	// VM config with disks and EFI
	vmConfigJSON := `{"data":{"vmid":200,"name":"test-vm","scsi0":"local-lvm:vm-200-disk-0,size=32G","efidisk0":"local-lvm:vm-200-efidisk,size=1G,efitype=4m"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/200/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vmRes := &VM{}
	req := infer.ReadRequest[Inputs, Outputs]{
		ID: "200",
		Inputs: Inputs{
			// User previously omitted VMID and disk/efi FileIDs; provide node for lookup
			Node: &nodeName,
			Disks: []*Disk{{
				DiskBase:  DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
			// No EfiDisk in prev inputs
		},
		// Provide VMID via state so Read can locate the VM
		State: Outputs{Inputs: Inputs{VMID: testutils.Ptr(vmID)}},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "200", resp.ID)

	// Outputs (state) carry computed values from API
	require.NotNil(t, resp.State.VMID)
	assert.Equal(t, vmID, *resp.State.VMID)
	// Node may not be set by API client; focus on disks/efi
	require.Len(t, resp.State.Disks, 1)
	require.NotNil(t, resp.State.EfiDisk)

	// Inputs preserve emptiness where user previously omitted
	assert.Nil(t, resp.Inputs.VMID)
	// Node was provided by user; preserved as-is
	require.NotNil(t, resp.Inputs.Node)
	assert.Equal(t, nodeName, *resp.Inputs.Node)
	require.Len(t, resp.Inputs.Disks, 1)
	assert.Nil(t, resp.Inputs.Disks[0].FileID)
	assert.Nil(t, resp.Inputs.EfiDisk)
}

//nolint:paralleltest // uses global env + client seam via testutils.NewAPIMock
func TestVMReadComputedAndPreserved_WithPrevIDs(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	vmID := 300
	nodeName := "pve-node"

	// Cluster and node status
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

	// VM status
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/300/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":300}}`,
			)),
	).Enable()

	// VM config with disks and EFI
	vmConfigJSON := `{"data":{"vmid":300,"name":"test-vm","scsi0":"local-lvm:vm-300-disk-0,size=32G","efidisk0":"local-lvm:vm-300-efidisk,size=1G,efitype=4m"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/300/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vmRes := &VM{}
	req := infer.ReadRequest[Inputs, Outputs]{
		ID: "300",
		Inputs: Inputs{
			VMID: testutils.Ptr(vmID),
			Node: &nodeName,
			Disks: []*Disk{{
				DiskBase:  DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
			EfiDisk: &EfiDisk{DiskBase: DiskBase{Storage: "local-lvm"}, EfiType: EfiType4M},
		},
	}

	resp, err := vmRes.Read(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "300", resp.ID)

	// Outputs (state) carry computed values from API
	require.NotNil(t, resp.State.VMID)
	assert.Equal(t, vmID, *resp.State.VMID)
	// Node may not be set by API client; focus on disks/efi
	require.Len(t, resp.State.Disks, 1)
	require.NotNil(t, resp.State.EfiDisk)

	// Inputs preserve user-provided VMID/Node and keep FileIDs nil when omitted
	require.NotNil(t, resp.Inputs.VMID)
	assert.Equal(t, vmID, *resp.Inputs.VMID)
	require.NotNil(t, resp.Inputs.Node)
	assert.Equal(t, nodeName, *resp.Inputs.Node)
	require.Len(t, resp.Inputs.Disks, 1)
	assert.Nil(t, resp.Inputs.Disks[0].FileID)
	require.NotNil(t, resp.Inputs.EfiDisk)
	assert.Nil(t, resp.Inputs.EfiDisk.FileID)
}

// --- Create IO handling test ---

//nolint:paralleltest // uses global env + client seam via testutils.NewAPIMock
func TestVMCreateOutputsContainComputedValues(t *testing.T) {
	mock, cleanup := testutils.NewAPIMock(t)
	defer cleanup()

	nodeName := "pve-node"
	nextID := 500

	// Mock GET /cluster/status
	clusterStatusJSON := `{"data":[{"type":"cluster","nodes":[{"name":"` + nodeName + `","status":"online"}]}]}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/status")).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(clusterStatusJSON)}, nil
			}).
			Repeat(10),
	).Enable()

	// Mock GET /cluster/nextid
	mock.AddMocks(
		mocha.Get(expect.URLPath("/cluster/nextid")).
			Reply(reply.OK().BodyString(`{"data":"` + strconv.Itoa(nextID) + `"}`)),
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

	// Mock POST /nodes/{node}/qemu to create VM
	mock.AddMocks(
		mocha.Post(expect.URLPath("/nodes/" + nodeName + "/qemu")).
			Reply(reply.OK().BodyString(`{"data":"UPID:pve-node:0000cafe:00000000:00000000:qmcreate:` + strconv.Itoa(nextID) + `:root@pam:"}`)),
	).Enable()

	// Mock task status endpoint
	taskStatusURL := "/nodes/" + nodeName + "/tasks/UPID:pve-node:0000cafe:00000000:00000000:qmcreate:" + strconv.Itoa(
		nextID,
	) + ":root@pam:/status"
	taskStatusJSON := `{"data":{"upid":"UPID:pve-node:0000cafe:00000000:00000000:qmcreate:` + strconv.Itoa(
		nextID,
	) + `:root@pam:","node":"` + nodeName + `","pid":1234,"pstart":0,"starttime":1699999999,"type":"qmcreate","id":"` + strconv.Itoa(
		nextID,
	) + `","user":"root@pam","status":"stopped","exitstatus":"OK"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath(taskStatusURL)).
			ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
				return &reply.Response{Status: http.StatusOK, Body: strings.NewReader(taskStatusJSON)}, nil
			}),
	).Enable()

	// Subsequent Read after creation: VM status + config
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/" + strconv.Itoa(nextID) + "/status/current")).
			Reply(reply.OK().BodyString(
				`{"data":{"status":"running","vmid":` + strconv.Itoa(nextID) + `}}`,
			)),
	).Enable()

	vmConfigJSON := `{"data":{"vmid":` + strconv.Itoa(
		nextID,
	) + `,"name":"test-vm","scsi0":"local-lvm:vm-` + strconv.Itoa(
		nextID,
	) + `-disk-0,size=32G","efidisk0":"local-lvm:vm-` + strconv.Itoa(
		nextID,
	) + `-efidisk,size=1G,efitype=4m"}}`
	mock.AddMocks(
		mocha.Get(expect.URLPath("/nodes/" + nodeName + "/qemu/" + strconv.Itoa(nextID) + "/config")).
			Reply(reply.OK().BodyString(vmConfigJSON)),
	).Enable()

	vmRes := &VM{}
	req := infer.CreateRequest[Inputs]{
		Name: strconv.Itoa(nextID),
		Inputs: Inputs{
			Name: testutils.Ptr("test-vm"),
			// Provide node explicitly to bypass cluster node discovery
			Node: &nodeName,
			Disks: []*Disk{{
				DiskBase:  DiskBase{Storage: "local-lvm"},
				Interface: "scsi0",
				Size:      32,
			}},
			// Provide EfiDisk without FileID to verify computed value appears in outputs
			EfiDisk: &EfiDisk{DiskBase: DiskBase{Storage: "local-lvm"}, EfiType: EfiType4M},
		},
	}

	resp, err := vmRes.Create(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(nextID), resp.ID)

	// Outputs contain computed VMID
	require.NotNil(t, resp.Output.VMID)
	assert.Equal(t, nextID, *resp.Output.VMID)

	// Inputs should not contain computed VMID (user omitted it)
	assert.Nil(t, req.Inputs.VMID)

	mock.AssertCalled(t)
}
