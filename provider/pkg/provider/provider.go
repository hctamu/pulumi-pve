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

// Package provider implements the Pulumi PVE provider.
package provider

import (
	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/group"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/ha"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/pool"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/role"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/storage"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/user"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version = "0.0.6"

// Name is the name of the PVE provider.
const Name = "pve"

// newHAResourceWithConfig creates a new HA resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newHAResourceWithConfig(cfg *config.Config) *ha.HA {
	// Create ProxmoxAdapter with config
	proxmoxClient := adapters.NewProxmoxAdapter(cfg)

	// Create HAAdapter using the ProxmoxClient interface
	haAdapter := adapters.NewHAAdapter(proxmoxClient)

	return &ha.HA{HAOps: haAdapter}
}

// newPoolResourceWithConfig creates a new Pool resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newPoolResourceWithConfig(cfg *config.Config) *pool.Pool {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create PoolAdapter using the ProxmoxAdapter
	poolAdapter := adapters.NewPoolAdapter(proxmoxAdapter)

	return &pool.Pool{PoolOps: poolAdapter}
}

// newACLResourceWithConfig creates a new ACL resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newACLResourceWithConfig(cfg *config.Config) *acl.ACL {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create ACLAdapter using the ProxmoxAdapter
	aclAdapter := adapters.NewACLAdapter(proxmoxAdapter)

	return &acl.ACL{ACLOps: aclAdapter}
}

// newUserResourceWithConfig creates a new User resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newUserResourceWithConfig(cfg *config.Config) *user.User {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create UserAdapter using the ProxmoxAdapter
	userAdapter := adapters.NewUserAdapter(proxmoxAdapter)

	return &user.User{UserOps: userAdapter}
}

// NewProvider returns a new instance of the PVE provider.
func NewProvider() p.Provider {
	return NewProviderWithConfig(nil)
}

// NewProviderWithConfig returns a new instance of the PVE provider with a specific config (for testing).
func NewProviderWithConfig(cfg *config.Config) p.Provider {
	// We tell the provider what resources it needs to support.
	return infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource(newPoolResourceWithConfig(cfg)),
			infer.Resource(&storage.File{}),
			infer.Resource(newHAResourceWithConfig(cfg)),
			infer.Resource(&vm.VM{}),
			infer.Resource(&group.Group{}),
			infer.Resource(&role.Role{}),
			infer.Resource(newACLResourceWithConfig(cfg)),
			infer.Resource(newUserResourceWithConfig(cfg)),
		},
		Config: infer.Config(config.Config{}),
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
					"basePackage":                     "io.github.hctamu",
				},
			},
			Repository: "https://github.com/hctamu/pulumi-pve",
		},
	})
}
