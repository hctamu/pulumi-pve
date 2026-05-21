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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
	"github.com/hctamu/pulumi-pve/provider/pkg/testutils"
)

// mockErrorClient is a proxmox.Client that returns configurable errors for
// ResolveNode and NextVMID, allowing error-path testing of the VM resource.
type mockErrorClient struct {
	resolveNodeErr error
	nextVMIDErr    error
}

func (mock *mockErrorClient) Get(_ context.Context, _ string, _ any) error     { return nil }
func (mock *mockErrorClient) Post(_ context.Context, _ string, _, _ any) error { return nil }
func (mock *mockErrorClient) Put(_ context.Context, _ string, _, _ any) error  { return nil }
func (mock *mockErrorClient) Delete(_ context.Context, _ string, _ any) error  { return nil }

func (mock *mockErrorClient) ResolveNode(_ context.Context, _ *string) (string, error) {
	return "", mock.resolveNodeErr
}

func (mock *mockErrorClient) NextVMID(_ context.Context) (int, error) {
	return 0, mock.nextVMIDErr
}

func TestVMCreateConfigurationErrors(t *testing.T) {
	t.Parallel()

	nodeName := "pve-node"
	resolveErr := errors.New("cluster unreachable")
	nextIDErr := errors.New("no VMID available")

	tests := []struct {
		name           string
		client         proxmox.Client
		vmOps          proxmox.VMOperations
		inputs         proxmox.VMInputs
		wantErrContain string
		wantErr        error // non-nil to check exact error identity
	}{
		{
			name:           "VMOperations not configured",
			client:         &testutils.MockProxmoxClient{DefaultNode: "pve-node", DefaultVMID: 100},
			vmOps:          nil,
			inputs:         proxmox.VMInputs{},
			wantErrContain: "VMOperations not configured",
		},
		{
			name:           "Client not configured",
			client:         nil,
			vmOps:          &mockVMOperations{},
			inputs:         proxmox.VMInputs{},
			wantErrContain: "client not configured",
		},
		{
			name:    "ResolveNode failure is propagated",
			client:  &mockErrorClient{resolveNodeErr: resolveErr},
			vmOps:   &mockVMOperations{},
			inputs:  proxmox.VMInputs{},
			wantErr: resolveErr,
		},
		{
			name:   "NextVMID failure is propagated",
			client: &mockErrorClient{nextVMIDErr: nextIDErr},
			vmOps:  &mockVMOperations{},
			inputs: proxmox.VMInputs{
				Name: "test-vm",
				Node: &nodeName,
				// VMID is nil so NextVMID is called
			},
			wantErr: nextIDErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vmRes := &vm.VM{Client: tt.client, VMOps: tt.vmOps}
			req := infer.CreateRequest[proxmox.VMInputs]{Name: "test", Inputs: tt.inputs}

			_, err := vmRes.Create(context.Background(), req)
			require.Error(t, err)

			if tt.wantErr != nil {
				assert.Equal(t, tt.wantErr, err)
			}
			if tt.wantErrContain != "" {
				assert.Contains(t, err.Error(), tt.wantErrContain)
			}
		})
	}
}
