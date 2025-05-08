// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	gen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version string

const Name string = "pve"

func Provider() p.Provider {
	// We tell the provider what resources it needs to support.
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource[*pool.Pool, pool.PoolInput, pool.PoolOutput](),
			infer.Resource[*storage.File, storage.FileInput, storage.FileOutput](),
			infer.Resource[*ha.Ha, ha.HaInput, ha.HaOutput](),
			infer.Resource[*vm.Vm, vm.VmInput, vm.VmOutput](),
		},
		Config: infer.Config[config.Config](),
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
		Metadata: schema.Metadata{
			DisplayName: "pve",
			Description: "PVE Provider",
			LanguageMap: map[string]any{
				"go": gen.GoPackageInfo{
					Generics:       gen.GenericsSettingGenericsOnly,
					ImportBasePath: "github.com/hctamu/pulumi-pve/sdk/go/pve",
					ModulePath:     "github.com/hctamu/pulumi-pve/sdk",
				},
			},
			Repository: "github.com/hctamu/pulumi-pve",
		},
	})
}
