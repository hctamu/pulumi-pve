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

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// standard

// default

// pulumi

// TestVMCustomDiffComputedVMID ensures vmId difference is ignored when user didn't set it.
func TestVMCustomDiffComputedVMID(t *testing.T) {
	t.Parallel()
	v := &vm.VM{}
	vmid := 101
	state := vm.Outputs{Inputs: vm.Inputs{VMID: &vmid}}
	// User inputs omit vmId (nil) -> should not produce diff for vmId
	req := infer.DiffRequest[vm.Inputs, vm.Outputs]{
		Inputs: vm.Inputs{},
		State:  state,
	}
	resp, err := v.Diff(context.Background(), req)
	require.NoError(t, err)
	// No changes expected because only difference is computed vmId
	assert.False(t, resp.HasChanges)
}

// TestVMCustomDiffVMIDReplace ensures replacement when vmId explicitly changes.
func TestVMCustomDiffVMIDReplace(t *testing.T) {
	t.Parallel()
	v := &vm.VM{}
	oldID := 200
	newID := 300
	req := infer.DiffRequest[vm.Inputs, vm.Outputs]{
		Inputs: vm.Inputs{VMID: &newID},
		State:  vm.Outputs{Inputs: vm.Inputs{VMID: &oldID}},
	}
	resp, err := v.Diff(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.Equal(t, p.UpdateReplace, resp.DetailedDiff["vmId"].Kind)
}

// TestVMCustomDiffRegularUpdate ensures normal update diff for a regular field.
func TestVMCustomDiffRegularUpdate(t *testing.T) {
	t.Parallel()
	v := &vm.VM{}
	nameOld := "vm-old"
	nameNew := "vm-new"
	state := vm.Outputs{Inputs: vm.Inputs{Name: &nameOld}}
	req := infer.DiffRequest[vm.Inputs, vm.Outputs]{
		Inputs: vm.Inputs{Name: &nameNew},
		State:  state,
	}
	resp, err := v.Diff(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.HasChanges)
	assert.Equal(t, p.Update, resp.DetailedDiff["name"].Kind)
}
