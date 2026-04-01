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

package adapters

import (
	"context"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure FileAdapter implements the FileOperations interface
var _ proxmox.FileOperations = (*FileAdapter)(nil)

// FileAdapter implements proxmox.FileOperations using the ProxmoxAdapter.
type FileAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
	sshClient      proxmox.GetSSHClientFunc
}

// NewFileAdapter creates a new FileAdapter wrapping the given ProxmoxAdapter.
func NewFileAdapter(proxmoxAdapter *ProxmoxAdapter, sshClient proxmox.GetSSHClientFunc) *FileAdapter {
	return &FileAdapter{
		proxmoxAdapter: proxmoxAdapter,
		sshClient:      sshClient,
	}
}

// Create creates a new file resource.
func (file *FileAdapter) Create(ctx context.Context, inputs proxmox.FileInputs) error {
	if err := file.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	sc, err := file.sshClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting ssh client: %v", err)
	}

	fileName := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		inputs.DataStoreID,
		inputs.ContentType,
		inputs.SourceRaw.FileName,
	)
	fileData := inputs.SourceRaw.FileData
	if _, err = sc.Run(proxmox.SSHOperationWrite, fileName, fileData); err != nil {
		return fmt.Errorf("error sending data via SSH: %v", err)
	}

	return nil
}

// Get retrieves an existing file resource.
func (file *FileAdapter) Get(ctx context.Context, inputs proxmox.FileInputs) (*proxmox.FileOutputs, error) {
	if err := file.proxmoxAdapter.Connect(ctx); err != nil {
		return nil, err
	}

	sc, err := file.sshClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting ssh client: %v", err)
	}

	fileName := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		inputs.DataStoreID,
		inputs.ContentType,
		inputs.SourceRaw.FileName,
	)

	fileContent, err := sc.Run(proxmox.SSHOperationRead, fileName)
	if err != nil {
		return nil, fmt.Errorf("error reading file via SSH: %v", err)
	}

	return &proxmox.FileOutputs{
		FileInputs: proxmox.FileInputs{
			DataStoreID: inputs.DataStoreID,
			ContentType: inputs.ContentType,
			SourceRaw: proxmox.FileSourceRaw{
				FileData: fileContent,
				FileName: inputs.SourceRaw.FileName,
			},
		},
	}, nil
}

// Update updates an existing file resource.
func (file *FileAdapter) Update(
	ctx context.Context,
	state proxmox.FileInputs,
	inputs proxmox.FileInputs,
) error {
	if err := file.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	sshClient, err := file.sshClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting ssh client: %v", err)
	}

	// remove the file
	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		state.DataStoreID,
		state.ContentType,
		state.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(proxmox.SSHOperationDelete, filePath); err != nil {
		return fmt.Errorf("error removing file via SSH: %v", err)
	}

	newFilePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		inputs.DataStoreID,
		inputs.ContentType,
		inputs.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(proxmox.SSHOperationWrite, newFilePath, inputs.SourceRaw.FileData); err != nil {
		return fmt.Errorf("error creating file via SSH: %v", err)
	}

	return nil
}

// Delete deletes an existing file resource.
func (file *FileAdapter) Delete(ctx context.Context, outputs proxmox.FileOutputs) error {
	if err := file.proxmoxAdapter.Connect(ctx); err != nil {
		return err
	}

	sc, err := file.sshClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting ssh client: %v", err)
	}

	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		outputs.DataStoreID,
		outputs.ContentType,
		outputs.SourceRaw.FileName,
	)
	if _, err = sc.Run(proxmox.SSHOperationDelete, filePath); err != nil {
		return fmt.Errorf("error removing file via SSH: %v", err)
	}

	return nil
}
