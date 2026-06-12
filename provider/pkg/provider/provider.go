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
	"context"
	"reflect"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/middleware/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/hctamu/pulumi-pve/provider/pkg/adapters"
	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/acl"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/file"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/group"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/ha"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/pool"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/role"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/sdnapply"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/user"
	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/vm"
)

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

// newGroupResourceWithConfig creates a new Group resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newGroupResourceWithConfig(cfg *config.Config) *group.Group {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create GroupAdapter using the ProxmoxAdapter
	groupAdapter := adapters.NewGroupAdapter(proxmoxAdapter)

	return &group.Group{GroupOps: groupAdapter}
}

// newRoleResourceWithConfig creates a new Role resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newRoleResourceWithConfig(cfg *config.Config) *role.Role {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create RoleAdapter using the ProxmoxAdapter
	roleAdapter := adapters.NewRoleAdapter(proxmoxAdapter)

	return &role.Role{RoleOps: roleAdapter}
}

func newFileWithConfig(cfg *config.Config) *file.File {
	// Create ProxmoxAdapter with config
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)

	// Create SSHAdapter with the ProxmoxAdapter and config
	sshAdapter := adapters.NewSSHAdapter(proxmoxAdapter, cfg)

	// Create FileAdapter using the ProxmoxAdapter and SSHAdapter
	fileAdapter := adapters.NewFileAdapter(proxmoxAdapter, sshAdapter)

	return &file.File{FileOps: fileAdapter}
}

// newSDNApplyResourceWithConfig creates a new SDNApply resource with a specific config.
// Passing nil will cause it to fetch config from context when Connect() is called.
func newSDNApplyResourceWithConfig(cfg *config.Config) *sdnapply.SDNApply {
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
	sdnAdapter := adapters.NewSDNAdapter(proxmoxAdapter)
	return &sdnapply.SDNApply{SDNOps: sdnAdapter}
}

// newVMResourceWithConfig creates a new VM resource with a specific config.
func newVMResourceWithConfig(cfg *config.Config) *vm.VM {
	proxmoxAdapter := adapters.NewProxmoxAdapter(cfg)
	return &vm.VM{
		Client: proxmoxAdapter,
		VMOps:  adapters.NewVMAdapter(proxmoxAdapter),
	}
}

// NewProvider returns a new instance of the PVE provider.
func NewProvider() p.Provider {
	return NewProviderWithConfig(nil)
}

// NewProviderWithConfig returns a new instance of the PVE provider with a specific config (for testing).
func NewProviderWithConfig(cfg *config.Config) p.Provider {
	// We tell the provider what resources it needs to support.
	prov := infer.Provider(infer.Options{
		Resources: []infer.InferredResource{
			infer.Resource(newPoolResourceWithConfig(cfg)),
			infer.Resource(newFileWithConfig(cfg)),
			infer.Resource(newHAResourceWithConfig(cfg)),
			infer.Resource(newVMResourceWithConfig(cfg)),
			infer.Resource(newGroupResourceWithConfig(cfg)),
			infer.Resource(newRoleResourceWithConfig(cfg)),
			infer.Resource(newACLResourceWithConfig(cfg)),
			infer.Resource(newUserResourceWithConfig(cfg)),
			infer.Resource(newSDNApplyResourceWithConfig(cfg)),
		},
		Config: infer.Config(config.Config{}),
		ModuleMap: map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		},
		Metadata: schema.Metadata{
			License:           "Apache-2.0",
			DisplayName:       "pve",
			Description:       "PVE Provider",
			Namespace:         "hctamu",
			PluginDownloadURL: "github://api.github.com/hctamu/pulumi-pve",
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
	prov.DiffConfig = diffConfig
	return prov
}

// diffConfig reports which provider config properties changed without triggering resource
// replacement. The infer framework's default marks every config change as UpdateReplace,
// which would force replacement of all managed resources on any config update.
func diffConfig(_ context.Context, req p.DiffRequest) (p.DiffResponse, error) {
	inputs := req.Inputs
	for _, key := range req.IgnoreChanges {
		if v, ok := req.State.GetOk(key); ok {
			inputs = inputs.Set(key, v)
		}
	}

	diff := map[string]p.PropertyDiff{}
	for key, newVal := range inputs.All {
		if key == "version" || strings.HasPrefix(key, "__") {
			continue
		}
		oldVal, exists := req.State.GetOk(key)
		if !exists {
			diff[key] = p.PropertyDiff{Kind: p.Add}
		} else if !reflect.DeepEqual(oldVal, newVal) {
			diff[key] = p.PropertyDiff{Kind: p.Update}
		}
	}
	for key := range req.State.All {
		if _, exists := inputs.GetOk(key); !exists {
			diff[key] = p.PropertyDiff{Kind: p.Delete}
		}
	}

	return p.DiffResponse{
		HasChanges:   len(diff) > 0,
		DetailedDiff: diff,
	}, nil
}
