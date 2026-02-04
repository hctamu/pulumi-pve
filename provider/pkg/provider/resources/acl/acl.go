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

// Package acl provides resources for managing Proxmox ACLs.
package acl

import (
	"context"
	"errors"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure ACL implements the required interfaces
var (
	_ = (infer.CustomResource[proxmox.ACLInputs, proxmox.ACLOutputs])((*ACL)(nil))
	_ = (infer.CustomDelete[proxmox.ACLOutputs])((*ACL)(nil))
	_ = (infer.CustomRead[proxmox.ACLInputs, proxmox.ACLOutputs])((*ACL)(nil))
	_ = infer.Annotated((*ACL)(nil))
)

// ACL represents a Proxmox ACL resource
type ACL struct {
	ACLOps proxmox.ACLOperations
}

// Create is used to create an ACL resource
func (acl *ACL) Create(
	ctx context.Context,
	request infer.CreateRequest[proxmox.ACLInputs],
) (response infer.CreateResponse[proxmox.ACLOutputs], err error) {
	inputs := request.Inputs
	preview := request.DryRun

	logger := p.GetLogger(ctx)
	logger.Debugf("Creating acl resource: %v", inputs)

	response = infer.CreateResponse[proxmox.ACLOutputs]{
		ID:     composeACLID(inputs),
		Output: proxmox.ACLOutputs{ACLInputs: inputs},
	}

	if preview {
		return response, nil
	}

	if acl.ACLOps == nil {
		err = errors.New("ACLOperations not configured")
		return response, err
	}

	err = acl.ACLOps.Create(ctx, inputs)

	return response, err
}

// Read is used to read an existing ACL resource
func (acl *ACL) Read(
	ctx context.Context,
	request infer.ReadRequest[proxmox.ACLInputs, proxmox.ACLOutputs],
) (response infer.ReadResponse[proxmox.ACLInputs, proxmox.ACLOutputs], err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf(
		"Read called for ACL with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	response.ID = request.ID
	response.Inputs = request.Inputs
	response.State = request.State

	if acl.ACLOps == nil {
		err = errors.New("ACLOperations not configured")
		return response, err
	}

	if request.ID == "" {
		logger.Warningf("Missing ACL ID")
		err = errors.New("missing acl ID")
		return response, err
	}

	var outputs *proxmox.ACLOutputs

	if outputs, err = acl.ACLOps.Get(ctx, request.ID); err != nil {
		return response, err
	}

	response.Inputs = outputs.ACLInputs
	response.State = *outputs

	logger.Debugf("Returning updated state: %+v", response.State)
	return response, nil
}

// Delete is used to delete an acl resource
func (acl *ACL) Delete(
	ctx context.Context,
	request infer.DeleteRequest[proxmox.ACLOutputs],
) (response infer.DeleteResponse, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting acl resource: %v", request.State)

	if acl.ACLOps == nil {
		return response, errors.New("ACLOperations not configured")
	}

	if err := acl.ACLOps.Delete(ctx, request.State); err != nil {
		return response, err
	}
	aclid := composeACLID(request.State.ACLInputs)
	logger.Debugf("ACL resource %v deleted", aclid)

	return response, nil
}

// Annotate is used to annotate the acl resource
// This is used to provide documentation for the resource in the Pulumi schema
// and to provide default values for the resource properties.
func (acl *ACL) Annotate(a infer.Annotator) {
	a.Describe(
		acl,
		"A Proxmox ACL resource that controls access to Proxmox objects.",
	)
}

// composeACLID builds a stable composite ID.
func composeACLID(acl proxmox.ACLInputs) string {
	// Assumes components do not contain '|'
	return strings.Join([]string{acl.Path, acl.RoleID, acl.Type, acl.UGID}, "|")
}
