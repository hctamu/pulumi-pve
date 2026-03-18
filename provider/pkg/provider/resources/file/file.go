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

// Package file provides Pulumi resources for managing files in Proxmox datastores.
package file

import (
	"context"
	"errors"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

var (
	_ = (infer.CustomResource[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomDelete[proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomRead[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomUpdate[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
	_ = infer.Annotated((*File)(nil))
)

// File represents a Pulumi custom resource for managing files in a Proxmox datastore.
type File struct {
	FileOps proxmox.FileOperations
}

// Create creates a new file resource
func (file *File) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.FileInputs],
) (infer.CreateResponse[proxmox.FileOutputs], error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	response := infer.CreateResponse[proxmox.FileOutputs]{
		ID: request.Name,
		Output: proxmox.FileOutputs{
			FileInputs: request.Inputs,
		},
	}
	logger.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	if preview {
		return response, nil
	}

	if file.FileOps == nil {
		return response, errors.New("FileOperations not configured")
	}

	if err := file.FileOps.Create(ctx, inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Delete is used to delete a file resource
func (file *File) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.FileOutputs],
) (infer.DeleteResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting file resource: %v", request.State)

	var response infer.DeleteResponse

	if file.FileOps == nil {
		return response, errors.New("FileOperations not configured")
	}

	if err := file.FileOps.Delete(ctx, request.State); err != nil {
		return response, err
	}

	return response, nil
}

// Read is used to read the state of a file resource
func (file *File) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.FileInputs, proxmox.FileOutputs],
) (infer.ReadResponse[proxmox.FileInputs, proxmox.FileOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for File with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response := infer.ReadResponse[proxmox.FileInputs, proxmox.FileOutputs](request)

	if file.FileOps == nil {
		return response, errors.New("FileOperations not configured")
	}

	// if resource does not exist, pulumi will invoke Create
	if request.ID == "" {
		return response, nil
	}

	outputs, err := file.FileOps.Get(ctx, request.Inputs)
	if err != nil {
		return response, err
	}

	response.Inputs = outputs.FileInputs
	response.State = *outputs

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Update is used to update a file resource
func (file *File) Update(
	ctx context.Context,
	request infer.UpdateRequest[proxmox.FileInputs, proxmox.FileOutputs],
) (infer.UpdateResponse[proxmox.FileOutputs], error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Update called for File with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response := infer.UpdateResponse[proxmox.FileOutputs]{
		Output: request.State,
	}

	if request.DryRun {
		return response, nil
	}

	if file.FileOps == nil {
		return response, errors.New("FileOperations not configured")
	}

	// log the differences between inputs and state
	if request.Inputs.DataStoreID != request.State.DataStoreID {
		logger.Infof("Updating DataStoreID from %q to %q", request.State.DataStoreID, request.Inputs.DataStoreID)
	}
	if request.Inputs.ContentType != request.State.ContentType {
		logger.Infof("Updating ContentType from %q to %q", request.State.ContentType, request.Inputs.ContentType)
	}
	if request.Inputs.SourceRaw.FileData != request.State.SourceRaw.FileData {
		logger.Infof("Updating FileData from %q to %q", request.State.SourceRaw.FileData, request.Inputs.SourceRaw.FileData)
	}
	if request.Inputs.SourceRaw.FileName != request.State.SourceRaw.FileName {
		logger.Infof("Updating FileName from %q to %q", request.State.SourceRaw.FileName, request.Inputs.SourceRaw.FileName)
	}

	response.Output.FileInputs = request.Inputs

	if err := file.FileOps.Update(ctx, request.State.FileInputs, request.Inputs); err != nil {
		return response, err
	}

	return response, nil
}

// Annotate is used to annotate the file resource
// This is used to provide documentation for the resource in the Pulumi schema
// and to provide default values for the resource properties.
func (file *File) Annotate(a infer.Annotator) {
	a.Describe(
		file,
		"A Proxmox file resource that represents a file in a Proxmox datastore.",
	)
}
