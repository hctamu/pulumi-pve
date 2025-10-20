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
	"fmt"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// ACL represents a Proxmox ACL resource.
type ACL struct{}

var (
	_ = (infer.CustomResource[Inputs, Outputs])((*ACL)(nil))
	_ = (infer.CustomDelete[Outputs])((*ACL)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*ACL)(nil))
)

// ErrACLNotFound is returned when an ACL cannot be found in Proxmox.
var ErrACLNotFound = errors.New("acl not found")

// ErrInvalidACLType is returned when an ACL has an invalid type.
var ErrInvalidACLType = errors.New("invalid type (must be 'user', 'group', or 'token')")

// Inputs defines the input properties for a Proxmox ACL resource.
type Inputs struct {
	Path      string `pulumi:"path"               provider:"replaceOnChanges"`
	RoleID    string `pulumi:"roleid"             provider:"replaceOnChanges"`
	Type      string `pulumi:"type"               provider:"replaceOnChanges"`
	UGID      string `pulumi:"ugid"               provider:"replaceOnChanges"`
	Propagate bool   `pulumi:"propagate,optional" provider:"replaceOnChanges"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Path, "The path of the ACL.")
	a.Describe(&args.RoleID, "The role ID of the ACL.")
	a.Describe(&args.Type, "The type of the ACL. Must be 'user', 'group', or 'token'.")
	a.Describe(&args.UGID, "The user/group/token ID of the ACL.")
	a.Describe(&args.Propagate, "Whether the ACL should be propagated.")
}

// Outputs defines the output properties for a Proxmox acl resource.
type Outputs struct {
	Inputs
}

// Enum-like constants for ACL Type
const (
	ACLTypeUser  = "user"
	ACLTypeGroup = "group"
	ACLTypeToken = "token"
)

// Create is used to create a new ACL resource
func (acl *ACL) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	l := p.GetLogger(ctx)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	if err = validateInputs(request.Inputs); err != nil {
		l.Warning(err.Error())
		return response, err
	}

	response.ID = composeACLID(
		request.Inputs.Path,
		request.Inputs.RoleID,
		request.Inputs.Type,
		request.Inputs.UGID,
	)
	response.Output = Outputs{Inputs: request.Inputs}
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	if request.DryRun {
		return response, nil
	}

	newACL := &api.ACLOptions{
		Path:      request.Inputs.Path,
		Roles:     request.Inputs.RoleID,
		Propagate: api.IntOrBool(request.Inputs.Propagate),
	}
	switch request.Inputs.Type {
	// default: already handled above
	case ACLTypeGroup:
		if _, err = pxc.Group(ctx, request.Inputs.UGID); err != nil {
			err = fmt.Errorf("failed to find group %q for ACL %s: %w", request.Inputs.UGID, request.Name, err)
			l.Error(err.Error())
			return response, err
		}
		newACL.Groups = request.Inputs.UGID
	case ACLTypeUser:
		if _, err = pxc.User(ctx, request.Inputs.UGID); err != nil {
			err = fmt.Errorf("failed to find user %q for ACL %s: %w", request.Inputs.UGID, request.Name, err)
			l.Error(err.Error())
			return response, err
		}
		newACL.Users = request.Inputs.UGID
	case ACLTypeToken:
		newACL.Tokens = request.Inputs.UGID
		// No validation for tokens
	}
	if err = pxc.UpdateACL(ctx, *newACL); err != nil {
		err = fmt.Errorf("failed to create ACL %s (path=%s role=%s type=%s ugid=%s): %w",
			request.Name, request.Inputs.Path, request.Inputs.RoleID, request.Inputs.Type, request.Inputs.UGID, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
}

// Delete is used to delete an ACL resource
func (acl *ACL) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting ACL %v", request.State)

	// Always re-fetch to get the actual propagate flag (and verify existence)
	existing, getErr := GetACL(
		ctx,
		request.State.Path,
		request.State.RoleID,
		request.State.Type,
		request.State.UGID,
		pxc,
	)
	if getErr != nil {
		if errors.Is(getErr, ErrACLNotFound) {
			// Already gone: idempotent success
			return response, nil
		}
		return response, fmt.Errorf("failed to re-fetch ACL before delete: %w", getErr)
	}

	deletedACL := &api.ACLOptions{
		Path:      existing.Path,
		Roles:     existing.RoleID,
		Propagate: existing.Propagate,
		Delete:    api.IntOrBool(true),
	}
	switch request.State.Type {
	case ACLTypeGroup:
		deletedACL.Groups = request.State.UGID
	case ACLTypeUser:
		deletedACL.Users = request.State.UGID
	case ACLTypeToken:
		deletedACL.Tokens = request.State.UGID
	default:
		return response, ErrInvalidACLType
	}

	if err = pxc.UpdateACL(ctx, *deletedACL); err != nil {
		err = fmt.Errorf("failed to delete ACL %v: %w", request.State, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
}

// Read is used to read the state of an ACL resource
func (acl *ACL) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	response.ID = request.ID
	response.Inputs = request.Inputs
	response.State = request.State

	l := p.GetLogger(ctx)
	l.Debugf(
		"Read called for ACL with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	if request.ID == "" {
		l.Warningf("Missing ACL ID")
		err = errors.New("missing ACL ID")
		return response, err
	}

	var existingAPI *api.ACL
	path, roleid, typ, ugid, err := decomposeACLID(request.ID)
	if err != nil {
		l.Error(err.Error())
		return response, err
	}

	if existingAPI, err = GetACL(ctx, path, roleid, typ, ugid, pxc); err != nil {
		if errors.Is(err, ErrACLNotFound) {
			// Signal to engine that resource vanished; triggers replacement on next update
			l.Infof("ACL %s no longer exists in Proxmox; marking for recreation", request.ID)
			response.ID = ""
			return response, nil
		}
		err = fmt.Errorf("failed to get ACL %s: %w", request.ID, err)
		l.Error(err.Error())
		return response, err
	}

	response.State = Outputs{
		Inputs: Inputs{
			Path:      existingAPI.Path,
			RoleID:    existingAPI.RoleID,
			Type:      existingAPI.Type,
			UGID:      existingAPI.UGID,
			Propagate: bool(existingAPI.Propagate),
		},
	}

	response.Inputs = response.State.Inputs
	l.Debugf("Returning updated state: %+v", response.State)

	return response, nil
}

// validateInputs checks that the required inputs are provided and valid.
func validateInputs(inputs Inputs) error {
	if inputs.Path == "" {
		return errors.New("path must be specified")
	}
	if inputs.RoleID == "" {
		return errors.New("roleid must be specified")
	}
	if inputs.Type == "" ||
		(inputs.Type != ACLTypeGroup &&
			inputs.Type != ACLTypeUser &&
			inputs.Type != ACLTypeToken) {
		return ErrInvalidACLType
	}
	if inputs.UGID == "" {
		return errors.New("ugid must be specified")
	}
	return nil
}

// composeACLID builds a stable composite ID.
func composeACLID(path, roleID, typ, ugid string) string {
	// Assumes components do not contain '|'
	return strings.Join([]string{path, roleID, typ, ugid}, "|")
}

// decomposeACLID splits a composite ID into its components.
func decomposeACLID(id string) (path, roleID, typ, ugid string, err error) {
	parts := strings.Split(id, "|")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid ACL ID %q", id)
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

// GetACL is used to retrieve an ACL by its keys
func GetACL(
	ctx context.Context,
	path string,
	roleid string,
	typ string,
	ugid string,
	pxc *px.Client,
) (acl *api.ACL, err error) {
	l := p.GetLogger(ctx)
	l.Debugf("GetACL called for ACL with keys: %s, %s, %s, %s", path, roleid, typ, ugid)

	allACLs, err := pxc.ACL(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get ACLs: %v", err)
		l.Error(err.Error())
		return nil, err
	}

	for _, r := range allACLs {
		if r.Path == path && r.RoleID == roleid && r.Type == typ && r.UGID == ugid {
			return r, nil
		}
	}
	return nil, ErrACLNotFound
}
