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

package proxmox

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// FileOperations defines the interface for File resource operations.
type FileOperations interface {
	// Create creates a new File resource.
	Create(ctx context.Context, inputs FileInputs) error

	// Get retrieves an existing File resource.
	Get(ctx context.Context, inputs FileInputs) (*FileOutputs, error)

	// Delete deletes an existing File resource.
	Delete(ctx context.Context, FileOutputs FileOutputs) error
}

// FileInputs represents the input properties required to manage a file resource.
type FileInputs struct {
	DataStoreID string        `pulumi:"datastoreId" provider:"replaceOnChanges"`
	ContentType string        `pulumi:"contentType" provider:"replaceOnChanges"`
	SourceRaw   FileSourceRaw `pulumi:"sourceRaw"   provider:"replaceOnChanges"`
}

// FileSourceRaw represents the raw source data for a file upload.
type FileSourceRaw struct {
	FileData string `pulumi:"fileData" provider:"replaceOnChanges"`
	FileName string `pulumi:"fileName" provider:"replaceOnChanges"`
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

// FileOutputs represents the output properties of a File resource.
type FileOutputs struct {
	FileInputs
}
