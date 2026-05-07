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

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

var (
	_ = (infer.CustomResource[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomDelete[proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomRead[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
	_ = (infer.CustomDiff[proxmox.FileInputs, proxmox.FileOutputs])((*File)(nil))
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

// Diff computes a custom diff for the File resource. All fields are replace-on-change,
// so any change triggers an UpdateReplace diff with DeleteBeforeReplace set to true.
func (file *File) Diff(
	ctx context.Context,
	request infer.DiffRequest[proxmox.FileInputs, proxmox.FileOutputs],
) (p.DiffResponse, error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Diff called for File with ID: %s", request.ID)

	diff := map[string]p.PropertyDiff{}

	if request.Inputs.DataStoreID != request.State.DataStoreID {
		diff["datastoreId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if request.Inputs.ContentType != request.State.ContentType {
		diff["contentType"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if request.Inputs.SourceRaw.FileData != request.State.SourceRaw.FileData {
		diff["sourceRaw"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if request.Inputs.SourceRaw.FileName != request.State.SourceRaw.FileName {
		diff["sourceRaw"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	return p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}, nil
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
