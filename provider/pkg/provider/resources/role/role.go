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

// Package role provides resources for managing Proxmox roles.
package role

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Role represents a Proxmox role resource.
type Role struct{}

// ErrRoleNotFound is returned when a role cannot be found in Proxmox.
var ErrRoleNotFound = errors.New("role not found")

// Inputs defines the input properties for a Proxmox role resource.
type Inputs struct {
	Name       string   `pulumi:"name"`
	Privileges []string `pulumi:"privileges,optional"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The name of the Proxmox role.")
	a.Describe(
		&args.Privileges,
		"A list of privileges assigned to this role. Each privilege should be a string identifier (e.g., 'VM.PowerMgmt').",
	)
}

// Outputs defines the output properties for a Proxmox role resource.
type Outputs struct {
	Inputs
}

// Create is used to create a new role resource
func (role *Role) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	response.ID = request.Name
	response.Output = Outputs{Inputs: Inputs{
		Name:       request.Inputs.Name,
		Privileges: stringToPrivileges(privilegesToString(request.Inputs.Privileges)),
	}}

	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)
	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	privsStr := privilegesToString(request.Inputs.Privileges)

	err = pxc.NewRole(ctx, request.Inputs.Name, privsStr)
	if err != nil {
		return response, fmt.Errorf("failed to create role %s: %w", request.Inputs.Name, err)
	}

	return response, nil
}

// Delete is used to delete a role resource
func (role *Role) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting role %v", request.State.Name)

	var existingRole *api.Role
	if existingRole, err = GetRole(ctx, request.State.Name, pxc); err != nil {
		err = fmt.Errorf("failed to get role: %v", err)
		return response, err
	}

	if err = existingRole.Delete(ctx); err != nil {
		err = fmt.Errorf("failed to delete role %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
}

// GetRole is used to retrieve a role by its ID
func GetRole(
	ctx context.Context,
	roleid string,
	pxc *px.Client,
) (role *api.Role, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("GetRole called for Role with ID: %s", roleid)

	allRoles, err := pxc.Roles(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get roles: %v", err)
		l.Error(err.Error())
		return nil, err
	}

	for _, r := range allRoles {
		if r.RoleID == roleid {
			return r, nil
		}
	}
	return nil, ErrRoleNotFound
}

// privilegesToString is used to convert a slice of privilege strings to a comma-separated string.
func privilegesToString(privs []string) string {
	if len(privs) == 0 {
		return ""
	}
	// Sort for consistent output and easier comparison later
	sortedPrivs := make([]string, len(privs))
	copy(sortedPrivs, privs)
	sort.Strings(sortedPrivs)
	return strings.Join(sortedPrivs, ",")
}

// stringToPrivileges is used to convert a comma-separated privilege string to a slice of strings.
func stringToPrivileges(privsStr string) []string {
	if privsStr == "" {
		return []string{}
	}
	parts := strings.Split(privsStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	// Sort for consistent output and easier comparison
	sort.Strings(result)
	return result
}
