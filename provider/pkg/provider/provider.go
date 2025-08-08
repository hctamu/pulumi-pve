// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package provider implements the Pulumi PVE provider.
package provider

import (
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/ha"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/pool"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/storage"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version string

// Name is the name of the PVE provider.
const Name string = "pve"

// Provider returns a new instance of the PVE provider.
func Provider() p.Provider {
	// We tell the provider what resources it needs to support.
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*pool.Pool, pool.Input, pool.PoolOutput](),
			infer.Resource[*storage.File, storage.FileInput, storage.FileOutput](),
			infer.Resource[*ha.Ha, ha.Input, ha.Output](),
			infer.Resource[*vm.VM, vm.Input, vm.Output](),
		},
		Config: infer.Config[config.Config](),
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
		Metadata: schema.Metadata{
			License:     "Apache-2.0",
			DisplayName: "pve",
			Description: "PVE Provider",
			Namespace:   "hctamu",
			LanguageMap: map[string]any{
				"csharp": map[string]any{
					"respectSchemaVersion": true,
					"packageReferences": map[string]string{
						"Pulumi": "3.*",
					},
				},
				"go": map[string]any{
					"respectSchemaVersion":           true,
					"generateResourceContainerTypes": true,
					"importBasePath":                 "github.com/hctamu/pulumi-pve/sdk/go/pve",
				},
				"nodejs": map[string]any{
					"respectSchemaVersion": true,
					"packageName":          "@hctamu/pulumi-pve",
				},
				"python": map[string]any{
					"respectSchemaVersion": true,
					"pyproject": map[string]bool{
						"enabled": true,
					},
					"packageName": "pulumi_pve",
				},
				"java": map[string]any{
					"buildFiles":                      "gradle",
					"gradleNexusPublishPluginVersion": "2.0.0",
					"dependencies": map[string]any{
						"com.pulumi:pulumi":               "1.0.0",
						"com.google.code.gson:gson":       "2.8.9",
						"com.google.code.findbugs:jsr305": "3.0.2",
					},
				},
			},
			Repository: "https://github.com/hctamu/pulumi-pve",
		},
	})
}
