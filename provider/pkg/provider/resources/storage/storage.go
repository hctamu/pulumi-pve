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
	_ = (infer.CustomResource[FileInput, FileOutput])((*File)(nil))
	_ = (infer.CustomDelete[FileOutput])((*File)(nil))
	_ = (infer.CustomRead[FileInput, FileOutput])((*File)(nil))
	_ = (infer.CustomUpdate[FileInput, FileOutput])((*File)(nil))
	_ = (infer.CustomDiff[FileInput, FileOutput])((*File)(nil))
)

// FileInput represents the input properties required to manage a file resource.
type FileInput struct {
	DataStoreID string        `pulumi:"datastoreId"`
	ContentType string        `pulumi:"contentType"`
	SourceRaw   FileSourceRaw `pulumi:"sourceRaw"`
}

// File represents a Pulumi custom resource for managing files in a Proxmox datastore.
type File struct{}

// FileSourceRaw represents the raw source data for a file upload.
type FileSourceRaw struct {
	FileData string `pulumi:"fileData"`
	FileName string `pulumi:"fileName"`
}

// FileOutput represents the output properties of a file resource.
type FileOutput struct {
	FileInput
}

// Annotate is used to annotate the input and output properties of the resource.
// This is used to generate the schema for the resource and give default values.
func (args *FileInput) Annotate(a infer.Annotator) {
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
func (file *File) Create(
	ctx context.Context,
	id string,
	inputs FileInput,
	preview bool,
) (idRet string, output FileOutput, err error) {
	if preview {
		return idRet, output, nil
	}

	p.GetLogger(ctx).Infof("getting ssh client")
	sc, err := client.GetSSHClient(ctx)
	if err != nil {
		return "", FileOutput{}, fmt.Errorf("error getting ssh client: %v", err)
	}

	p.GetLogger(ctx).Infof("sending data to %s", sc.TargetIP)

	fileName := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	fileData := inputs.SourceRaw.FileData
	if _, err = sc.Run(sc.Write(), fileName, fileData); err != nil {
		return "", output, fmt.Errorf("error sending data via SSH: %v", err)
	}

	output.FileInput = inputs
	idRet = id

	return idRet, output, err
}

// Delete deletes a file resource
func (file *File) Delete(ctx context.Context, id string, output FileOutput) (err error) {
	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return fmt.Errorf("error getting ssh client: %v", err)
	}

	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		output.DataStoreID,
		output.ContentType,
		output.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(sshClient.Delete(), filePath); err != nil {
		return fmt.Errorf("error removing file via SSH: %v", err)
	}

	return err
}

// Read reads a file resource
func (file *File) Read(
	ctx context.Context,
	id string,
	inputs FileInput,
	outputs FileOutput,
) (
	canonicalID string,
	normalizedInputs FileInput,
	normalizedOutputs FileOutput,
	err error,
) {
	// Get SSH client
	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return "", inputs, outputs, fmt.Errorf("error getting ssh client: %v", err)
	}

	// Construct the remote file path
	filePath := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	p.GetLogger(ctx).Infof("Reading file from path: %s", filePath)

	// Attempt to read the file content via SSH.
	fileContent, err := sshClient.Run(sshClient.Read(), filePath)
	if err != nil {
		return "", inputs, outputs, fmt.Errorf("error reading file via SSH: %v", err)
	}

	// Update the outputs with the read file content.
	outputs.FileInput = inputs
	outputs.SourceRaw.FileData = fileContent

	// Return the canonical ID (unchanged) and the normalized inputs/outputs.
	canonicalID = id
	normalizedInputs = inputs
	normalizedOutputs = outputs
	p.GetLogger(ctx).Debugf("Read file state: %+v", normalizedOutputs)
	return
}

// Update updates a file resource
func (file *File) Update(
	ctx context.Context,
	id string,
	outputs FileOutput,
	inputs FileInput,
	preview bool,
) (outputsRet FileOutput, err error) {
	if preview {
		return outputsRet, nil
	}

	sshClient, err := client.GetSSHClient(ctx)
	if err != nil {
		return outputsRet, fmt.Errorf("error getting ssh client: %v", err)
	}

	// remove the file
	filePath := fmt.Sprintf(
		"/mnt/pve/%s/%s/%s",
		outputs.DataStoreID,
		outputs.ContentType,
		outputs.SourceRaw.FileName,
	)
	if _, err = sshClient.Run(sshClient.Delete(), filePath); err != nil {
		return outputsRet, fmt.Errorf("error removing file via SSH: %v", err)
	}

	newFilePath := fmt.Sprintf("/mnt/pve/%s/%s/%s", inputs.DataStoreID, inputs.ContentType, inputs.SourceRaw.FileName)
	if _, err = sshClient.Run(sshClient.Write(), newFilePath, inputs.SourceRaw.FileData); err != nil {
		return outputsRet, fmt.Errorf("error creating file via SSH: %v", err)
	}

	outputsRet.FileInput = inputs

	return outputsRet, err
}

// Diff computes the differences between the old and new state of a file resource.
func (file *File) Diff(
	ctx context.Context,
	id string,
	olds FileOutput,
	news FileInput,
) (response p.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}

	if news.DataStoreID != olds.DataStoreID {
		diff["FileInput.dataStoreId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if news.ContentType != olds.ContentType {
		diff["FileInput.contentType"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if news.SourceRaw.FileName != olds.SourceRaw.FileName {
		diff["FileInput.sourceRaw.fileName"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if news.SourceRaw.FileData != olds.SourceRaw.FileData {
		diff["FileInput.sourceRaw.fileData"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	response.DetailedDiff = diff
	return response, err
}
