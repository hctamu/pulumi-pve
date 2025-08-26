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

// Package storage provides Pulumi resources for managing files in Proxmox datastores.
package storage

import (
	"context"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Unimplemented fields are marked in the comments in the commit (hash): 2a127e9aaab17b21bebd027d3edf13e27b570cf9

var (
	_ = (infer.CustomResource[FileInputs, FileOutputs])((*File)(nil))
	_ = (infer.CustomDelete[FileOutputs])((*File)(nil))
	_ = (infer.CustomRead[FileInputs, FileOutputs])((*File)(nil))
	_ = (infer.CustomUpdate[FileInputs, FileOutputs])((*File)(nil))
)

// FileInputs represents the input properties required to manage a file resource.
type FileInputs struct {
	DataStoreID string        `pulumi:"datastoreId" provider:"replaceOnChanges"`
	ContentType string        `pulumi:"contentType" provider:"replaceOnChanges"`
	SourceRaw   FileSourceRaw `pulumi:"sourceRaw"`
}

// File represents a Pulumi custom resource for managing files in a Proxmox datastore.
type File struct{}

// FileSourceRaw represents the raw source data for a file upload.
type FileSourceRaw struct {
	FileData string `pulumi:"fileData" provider:"replaceOnChanges"`
	FileName string `pulumi:"fileName" provider:"replaceOnChanges"`
}

// FileOutputs represents the output properties of a file resource.
type FileOutputs struct {
	FileInputs
}

// Annotate is used to annotate the input and output properties of the resource.
// This is used to generate the schema for the resource and give default values.
func (args *FileInputs) Annotate(a infer.Annotator) {
	a.Describe(&args.DataStoreID, "The datastore to upload the file to.  (e.g:ceph-ha)")
	a.Describe(&args.ContentType, "The type of the file (e.g: snippets)")
	a.Describe(&args.SourceRaw, "The raw source data")
}

// Annotate is used to annotate the input and output properties of the resource.
// This is used to generate the schema for the resource and give default values.
func (args *FileSourceRaw) Annotate(a infer.Annotator) {
	a.Describe(&args.FileData, "The raw data in []byte")
	a.Describe(&args.FileName, "The name of the file")
}

// Create creates a new file resource
func (file *File) Create(ctx context.Context, request infer.CreateRequest[FileInputs]) (response infer.CreateResponse[FileOutputs], err error) {
	response = infer.CreateResponse[FileOutputs]{
		ID: request.Name,
		Output: FileOutputs{
			FileInputs: request.Inputs,
		},
	}

	inputs := request.Inputs

	if request.DryRun {
		return response, nil
	}

	p.GetLogger(ctx).Infof("getting ssh client")
	sc, err := client.GetSSHClient(ctx)
	if err != nil {
		return response, fmt.Errorf("error getting ssh client: %v", err)
	}

	p.GetLogger(ctx).Infof("sending data to %s", sc.TargetIP)

	fileName := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	fileData := inputs.SourceRaw.FileData
	if _, err = sc.Run(sc.Write(), fileName, fileData); err != nil {
		return response, fmt.Errorf("error sending data via SSH: %v", err)
	}

	response.Output.FileInputs = inputs
	response.ID = request.Name

	return response, err
}

// Delete deletes a file resource
func (file *File) Delete(ctx context.Context, request infer.DeleteRequest[FileOutputs]) (response infer.DeleteResponse, err error) {
	state := request.State

	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return response, fmt.Errorf("error getting ssh client: %v", err)
	}

	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		state.DataStoreID,
		state.ContentType,
		state.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(sshClient.Delete(), filePath); err != nil {
		return response, fmt.Errorf("error removing file via SSH: %v", err)
	}

	return response, err
}

// Read reads a file resource
func (file *File) Read(ctx context.Context, request infer.ReadRequest[FileInputs, FileOutputs]) (
	response infer.ReadResponse[FileInputs, FileOutputs],
	err error,
) {

	inputs := request.Inputs
	state := request.State

	response = infer.ReadResponse[FileInputs, FileOutputs]{
		ID:     request.ID,
		Inputs: inputs,
		State:  state,
	}

	// Get SSH client
	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return response, fmt.Errorf("error getting ssh client: %v", err)
	}

	// Construct the remote file path
	filePath := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	p.GetLogger(ctx).Infof("Reading file from path: %s", filePath)

	// Attempt to read the file content via SSH.
	fileContent, err := sshClient.Run(sshClient.Read(), filePath)
	if err != nil {
		return response, fmt.Errorf("error reading file via SSH: %v", err)
	}

	// Update the outputs with the read file content.
	response.State.FileInputs = inputs
	response.State.SourceRaw.FileData = fileContent

	p.GetLogger(ctx).Debugf("Read file state: %+v", response.State)
	return response, nil
}

// Update updates a file resource
func (file *File) Update(ctx context.Context, request infer.UpdateRequest[FileInputs, FileOutputs]) (
	response infer.UpdateResponse[FileOutputs],
	err error,
) {
	state := request.State
	inputs := request.Inputs

	response = infer.UpdateResponse[FileOutputs]{
		Output: request.State,
	}

	if request.DryRun {
		return response, nil
	}

	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return response, fmt.Errorf("error getting ssh client: %v", err)
	}

	// remove the file
	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		state.DataStoreID,
		state.ContentType,
		state.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(sshClient.Delete(), filePath); err != nil {
		return response, fmt.Errorf("error removing file via SSH: %v", err)
	}

	newFilePath := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	if _, err = sshClient.Run(sshClient.Write(), newFilePath, inputs.SourceRaw.FileData); err != nil {
		return response, fmt.Errorf("error creating file via SSH: %v", err)
	}

	response.Output.FileInputs = inputs

	return response, err
}
