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

// Package user provides resources for managing Proxmox users.
package user

import (
	"context"
	"fmt"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	utils "github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// User represents a Proxmox user resource.
type User struct{}

var (
	_ = (infer.CustomResource[Inputs, Outputs])((*User)(nil))
	_ = (infer.CustomDelete[Outputs])((*User)(nil))
	_ = (infer.CustomRead[Inputs, Outputs])((*User)(nil))
	_ = (infer.CustomUpdate[Inputs, Outputs])((*User)(nil))
)

// Inputs defines the input properties for a Proxmox user resource.
type Inputs struct {
	Name      string   `pulumi:"userid"             provider:"replaceOnChanges"` // contains realm (e.g., user@pve)
	Comment   string   `pulumi:"comment,optional"`
	Email     string   `pulumi:"email,optional"`
	Enable    bool     `pulumi:"enable,optional"`
	Expire    int      `pulumi:"expire,optional"`
	Firstname string   `pulumi:"firstname,optional"`
	Groups    []string `pulumi:"groups,optional"`
	Keys      []string `pulumi:"keys,optional"`
	Lastname  string   `pulumi:"lastname,optional"`
	Password  string   `pulumi:"password,optional"  provider:"secret,replaceOnChanges"`
}

// Annotate is used to annotate the input and output properties of the resource.
func (args *Inputs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "The user ID of the Proxmox user, including the realm (e.g., 'user@pve').")
	a.Describe(&args.Comment, "An optional comment for the user.")
	a.Describe(&args.Email, "An optional email address for the user.")
	a.SetDefault(&args.Enable, true)
	a.Describe(&args.Enable, "Whether the user is enabled. Defaults to true.")
	a.Describe(&args.Expire, "The expiration time for the user as a Unix timestamp.")
	a.Describe(&args.Firstname, "The first name of the user.")
	a.Describe(&args.Groups, "A list of groups the user belongs to.")
	a.Describe(&args.Keys, "A list of SSH keys associated with the user.")
	a.Describe(&args.Lastname, "The last name of the user.")
	a.Describe(&args.Password, "The password for the user. This field is treated as a secret.")
}

// Outputs defines the output properties for a Proxmox user resource.
type Outputs struct {
	Inputs
}

// Create is used to create a new user resource
func (user *User) Create(
	ctx context.Context,
	request infer.CreateRequest[Inputs],
) (response infer.CreateResponse[Outputs], err error) {
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)

	// set provider id to resource primary key
	response.ID = request.Inputs.Name

	// set output properties
	response.Output = Outputs{Inputs: request.Inputs}

	if request.DryRun {
		return response, nil
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// create resource
	newUser := api.NewUser{
		UserID:    request.Inputs.Name,
		Comment:   request.Inputs.Comment,
		Email:     request.Inputs.Email,
		Enable:    request.Inputs.Enable,
		Expire:    request.Inputs.Expire,
		Firstname: request.Inputs.Firstname,
		Groups:    request.Inputs.Groups,
		Keys:      request.Inputs.Keys,
		Lastname:  request.Inputs.Lastname,
		Password:  request.Inputs.Password,
	}

	// api.User and api.NewUser inconsistency
	if len(newUser.Keys) == 0 {
		newUser.Keys = nil
	}

	// perform create
	if err = pxc.NewUser(ctx, &newUser); err != nil {
		return response, fmt.Errorf("failed to create user %s: %v", request.Inputs.Name, err)
	}

	// fetch created resource to confirm
	if _, err = pxc.User(ctx, request.Inputs.Name); err != nil {
		return response, fmt.Errorf("failed to fetch user %s: %w", request.Inputs.Name, err)
	}

	l.Debugf("Successfully created user %s", request.Inputs.Name)

	return response, nil
}

// Delete is used to delete a user resource 103-124
func (user *User) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	response, err = utils.DeleteResource(utils.DeletedResource{
		Ctx:          ctx,
		ResourceID:   request.State.Name,
		URL:          "/access/users/" + request.State.Name,
		ResourceType: "user",
	})
	return response, err
}

// Read is used to read the state of a user resource
func (user *User) Read(
	ctx context.Context,
	request infer.ReadRequest[Inputs, Outputs],
) (response infer.ReadResponse[Inputs, Outputs], err error) {
	response.ID = request.ID
	response.Inputs = request.Inputs

	l := p.GetLogger(ctx)
	l.Debugf(
		"Read called for User with ID: %s, Inputs: %+v, State: %+v",
		request.ID,
		request.Inputs,
		request.State,
	)

	// if resource does not exist, pulumi will invoke Create
	if request.ID == "" {
		return response, nil
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// fetch existing resource from server
	var existingUser *api.User
	if existingUser, err = pxc.User(ctx, request.ID); err != nil {
		if utils.IsNotFound(err) {
			response.ID = ""
			return response, nil
		}
		err = fmt.Errorf("failed to get user %s: %w", request.ID, err)
		return response, err
	}

	l.Debugf("Successfully fetched user: %+v", existingUser.UserID)

	// set state from fetched resource
	response.State = Outputs{
		Inputs: Inputs{
			Name:      existingUser.UserID,
			Comment:   existingUser.Comment,
			Email:     existingUser.Email,
			Enable:    bool(existingUser.Enable),
			Expire:    existingUser.Expire,
			Firstname: existingUser.Firstname,
			Groups:    existingUser.Groups,
			Keys:      utils.StringToSlice(existingUser.Keys),
			Lastname:  existingUser.Lastname,
			Password:  request.Inputs.Password,
		},
	}

	// api.User and api.NewUser inconsistency
	if len(response.State.Keys) == 0 {
		response.State.Keys = nil
	}

	// update inputs to match state
	response.Inputs = response.State.Inputs

	l.Debugf("Returning updated user: %+v", response.State)
	return response, nil
}

// Update is used to update a user resource
func (user *User) Update(
	ctx context.Context,
	request infer.UpdateRequest[Inputs, Outputs],
) (response infer.UpdateResponse[Outputs], err error) {
	response.Output = request.State

	l := p.GetLogger(ctx)
	l.Debugf("Update called for User with ID: %s, Inputs: %+v, State: %+v",
		request.State.Name,
		request.Inputs,
		request.State,
	)

	if request.DryRun {
		return response, nil
	}

	// compare and update fields
	if request.Inputs.Comment != request.State.Comment {
		l.Infof("Updating comment from %q to %q", request.State.Comment, request.Inputs.Comment)
		response.Output.Comment = request.Inputs.Comment
	}
	if request.Inputs.Email != request.State.Email {
		l.Infof("Updating email from %q to %q", request.State.Email, request.Inputs.Email)
		response.Output.Email = request.Inputs.Email
	}
	if request.Inputs.Enable != request.State.Enable {
		l.Infof("Updating enable from %v to %v", request.State.Enable, request.Inputs.Enable)
		response.Output.Enable = request.Inputs.Enable
	}
	if request.Inputs.Expire != request.State.Expire {
		l.Infof("Updating expire from %d to %d", request.State.Expire, request.Inputs.Expire)
		response.Output.Expire = request.Inputs.Expire
	}
	if request.Inputs.Firstname != request.State.Firstname {
		l.Infof("Updating firstname from %q to %q", request.State.Firstname, request.Inputs.Firstname)
		response.Output.Firstname = request.Inputs.Firstname
	}
	if utils.SliceToString(request.Inputs.Groups) != utils.SliceToString(request.State.Groups) {
		l.Infof("Updating groups from %q to %q", request.State.Groups, request.Inputs.Groups)
		response.Output.Groups = request.Inputs.Groups
	}
	if utils.SliceToString(request.Inputs.Keys) != utils.SliceToString(request.State.Keys) {
		l.Infof("Updating keys from %q to %q", request.State.Keys, request.Inputs.Keys)
		response.Output.Keys = request.Inputs.Keys
	}
	if request.Inputs.Lastname != request.State.Lastname {
		l.Infof("Updating lastname from %q to %q", request.State.Lastname, request.Inputs.Lastname)
		response.Output.Lastname = request.Inputs.Lastname
	}

	// prepare updated resource
	updatedUser := &api.User{
		UserID:    response.Output.Name,
		Comment:   response.Output.Comment,
		Email:     response.Output.Email,
		Enable:    api.IntOrBool(response.Output.Enable),
		Expire:    response.Output.Expire,
		Firstname: response.Output.Firstname,
		Groups:    response.Output.Groups,
		Keys:      utils.SliceToString(response.Output.Keys),
		Lastname:  response.Output.Lastname,
	}

	// get client
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	// perform update (avoid fmt.Sprintf for simple concatenation to satisfy perfsprint linter)
	if err = pxc.Put(ctx, "/access/users/"+updatedUser.UserID, updatedUser, nil); err != nil {
		return response, fmt.Errorf("failed to update user %s: %w", request.State.Name, err)
	}

	l.Debugf("Successfully updated user %s", request.State.Name)
	return response, nil
}
