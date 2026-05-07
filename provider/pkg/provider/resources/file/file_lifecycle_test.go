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

package file_test

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/file"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

const (
	dataStoreID     = "local-lvm"
	contentType     = "iso"
	fileName        = "test.iso"
	fileData        = "test-data"
	updatedFileData = "updated-test-data"
	resourceName    = "pve:file:File"
)

// MockFileOperations is a mock implementation of the FileOperations interface.
type MockFileOperations struct {
	CreateCalls []proxmox.FileInputs
	GetCalls    []proxmox.FileInputs
	DeleteCalls []proxmox.FileOutputs
}

func (mock *MockFileOperations) Create(ctx context.Context, inputs proxmox.FileInputs) error {
	mock.CreateCalls = append(mock.CreateCalls, inputs)
	return nil
}

func (mock *MockFileOperations) Get(
	ctx context.Context,
	inputs proxmox.FileInputs,
) (*proxmox.FileOutputs, error) {
	mock.GetCalls = append(mock.GetCalls, inputs)
	return &proxmox.FileOutputs{FileInputs: inputs}, nil
}

func (mock *MockFileOperations) Delete(ctx context.Context, outputs proxmox.FileOutputs) error {
	mock.DeleteCalls = append(mock.DeleteCalls, outputs)
	return nil
}

func TestFileHealthyLifeCycle(t *testing.T) {
	t.Parallel()

	mockOps := &MockFileOperations{}
	fileResource := &file.File{FileOps: mockOps}

	// We create a provider that is configured to use our mock-injected resource.
	provider := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource(fileResource),
		},
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
	})

	pulumiServer, err := integration.NewServer(
		t.Context(),
		"pve",
		semver.Version{Minor: 1},
		integration.WithProvider(provider),
	)
	require.NoError(t, err)

	// Define expected output after update
	expected := property.NewMap(map[string]property.Value{
		"datastoreId": property.New(dataStoreID),
		"contentType": property.New(contentType),
		"sourceRaw": property.New(property.NewMap(map[string]property.Value{
			"fileName": property.New(fileName),
			"fileData": property.New(updatedFileData),
		})),
	})

	// Run lifecycle test
	integration.LifeCycleTest{
		Resource: resourceName,
		Create: integration.Operation{
			Inputs: property.NewMap(map[string]property.Value{
				"datastoreId": property.New(dataStoreID),
				"contentType": property.New(contentType),
				"sourceRaw": property.New(property.NewMap(map[string]property.Value{
					"fileName": property.New(fileName),
					"fileData": property.New(fileData),
				})),
			}),
			Hook: func(in, out property.Map) {
				assert.Equal(t, dataStoreID, out.Get("datastoreId").AsString())
				assert.Equal(t, contentType, out.Get("contentType").AsString())
				sourceRaw := out.Get("sourceRaw").AsMap()
				assert.Equal(t, fileName, sourceRaw.Get("fileName").AsString())
				assert.Equal(t, fileData, sourceRaw.Get("fileData").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: property.NewMap(map[string]property.Value{
				"datastoreId": property.New(dataStoreID),
				"contentType": property.New(contentType),
				"sourceRaw": property.New(property.NewMap(map[string]property.Value{
					"fileName": property.New(fileName),
					"fileData": property.New(updatedFileData),
				})),
			}),
			ExpectedOutput: &expected,
		}},
	}.Run(t, pulumiServer)

	// Verify mock calls
	require.Len(t, mockOps.CreateCalls, 2, "Create should be called twice (initial + replace)")
	createCall := mockOps.CreateCalls[0]
	assert.Equal(t, dataStoreID, createCall.DataStoreID)
	assert.Equal(t, contentType, createCall.ContentType)
	assert.Equal(t, fileName, createCall.SourceRaw.FileName)
	assert.Equal(t, fileData, createCall.SourceRaw.FileData)

	replaceCreateCall := mockOps.CreateCalls[1]
	assert.Equal(t, dataStoreID, replaceCreateCall.DataStoreID)
	assert.Equal(t, contentType, replaceCreateCall.ContentType)
	assert.Equal(t, fileName, replaceCreateCall.SourceRaw.FileName)
	assert.Equal(t, updatedFileData, replaceCreateCall.SourceRaw.FileData)

	require.Len(t, mockOps.DeleteCalls, 2, "Delete should be called twice (replace + teardown)")
	replaceDeleteCall := mockOps.DeleteCalls[0]
	assert.Equal(t, dataStoreID, replaceDeleteCall.DataStoreID)
}
