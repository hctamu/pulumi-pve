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
	"sort"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
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
	Password  string   `pulumi:"password,optional"  provider:"replaceOnChanges"`
	// Realm_type       string   `pulumi:"realm-type"`
	// TFA_locked_until int      `pulumi:"tfa-locked-until,optional"`
	// Tokens           []string `pulumi:"tokens,optional"`
	// Totp_locked      bool     `pulumi:"totp-locked,optional"`
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
	response.ID = request.Name
	response.Output = Outputs{Inputs: request.Inputs}
	l := p.GetLogger(ctx)
	l.Debugf("Create: %v, %v, %v", request.Name, request.Inputs, response.Output)
	if request.DryRun {
		return response, nil
	}

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	newUser := &api.NewUser{
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

	if err = pxc.NewUser(ctx, newUser); err != nil {
		err = fmt.Errorf("failed to create user: %v", err)
		l.Error(err.Error())
	}

	return response, err
}

// Delete is used to delete a user resource
func (user *User) Delete(
	ctx context.Context,
	request infer.DeleteRequest[Outputs],
) (response infer.DeleteResponse, err error) {
	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	l := p.GetLogger(ctx)
	l.Debugf("Deleting user %v", request.State.Name)

	var existingUser *api.User
	if existingUser, err = pxc.User(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get user: %v", err)
		return response, err
	}

	if err = existingUser.Delete(ctx); err != nil {
		err = fmt.Errorf("failed to delete user %s: %v", request.State.Name, err)
		l.Error(err.Error())
		return response, err
	}

	return response, nil
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

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	var existingUser *api.User
	if existingUser, err = pxc.User(ctx, request.ID); err != nil {
		err = fmt.Errorf("failed to get user: %v", err)
		return response, err
	}

	response.State = Outputs{
		Inputs: Inputs{
			Name:      existingUser.UserID,
			Comment:   existingUser.Comment,
			Email:     existingUser.Email,
			Enable:    bool(existingUser.Enable),
			Expire:    existingUser.Expire,
			Firstname: existingUser.Firstname,
			Groups:    existingUser.Groups,
			Keys:      stringToSlice(existingUser.Keys),
			Lastname:  existingUser.Lastname,
			// Password:  existingUser.Password,
		},
	}

	response.Inputs = response.State.Inputs

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

	var pxc *px.Client
	if pxc, err = client.GetProxmoxClientFn(ctx); err != nil {
		return response, err
	}

	var existingUser *api.User
	if existingUser, err = pxc.User(ctx, request.State.Name); err != nil {
		err = fmt.Errorf("failed to get user: %v", err)
		return response, err
	}
	l.Infof("'%v'", sliceToString(existingUser.Groups))
	l.Infof("'%v'", sliceToString(request.Inputs.Groups))

	if request.Inputs.Comment != existingUser.Comment {
		l.Infof("Updating comment from %q to %q", existingUser.Comment, request.Inputs.Comment)
		existingUser.Comment = request.Inputs.Comment
	}
	if request.Inputs.Email != existingUser.Email {
		l.Infof("Updating email from %q to %q", existingUser.Email, request.Inputs.Email)
		existingUser.Email = request.Inputs.Email
	}
	if request.Inputs.Enable != bool(existingUser.Enable) {
		l.Infof("Updating enable from %v to %v", existingUser.Enable, request.Inputs.Enable)
		existingUser.Enable = api.IntOrBool(request.Inputs.Enable)
	}
	if request.Inputs.Expire != existingUser.Expire {
		l.Infof("Updating expire from %d to %d", existingUser.Expire, request.Inputs.Expire)
		existingUser.Expire = request.Inputs.Expire
	}
	if request.Inputs.Firstname != existingUser.Firstname {
		l.Infof("Updating firstname from %q to %q", existingUser.Firstname, request.Inputs.Firstname)
		existingUser.Firstname = request.Inputs.Firstname
	}
	if sliceToString(request.Inputs.Groups) != sliceToString(existingUser.Groups) {
		sort.Strings(request.Inputs.Groups)
		l.Infof("Updating groups from %q to %q", existingUser.Groups, request.Inputs.Groups)
		existingUser.Groups = request.Inputs.Groups
	}
	if sliceToString(request.Inputs.Keys) != existingUser.Keys {
		l.Infof("Updating keys from %q to %q", existingUser.Keys, request.Inputs.Keys)
		existingUser.Keys = sliceToString(request.Inputs.Keys)
	}
	// if !equalOrdered(request.Inputs.Groups, existingUser.Groups) {
	// 	l.Infof("Updating groups from %q to %q", existingUser.Groups, request.Inputs.Groups)
	// 	existingUser.Groups = append([]string(nil), request.Inputs.Groups...)
	// }
	// if !equalOrdered(request.Inputs.Keys, stringToSlice(existingUser.Keys)) {
	// 	l.Infof("Updating keys from %q to %q", existingUser.Keys, request.Inputs.Keys)
	// 	existingUser.Keys = sliceToString(request.Inputs.Keys)
	// }
	if request.Inputs.Lastname != existingUser.Lastname {
		l.Infof("Updating lastname from %q to %q", existingUser.Lastname, request.Inputs.Lastname)
		existingUser.Lastname = request.Inputs.Lastname
	}

	if err = existingUser.Update(ctx); err != nil {
		err = fmt.Errorf("failed to update user %s: %v", request.State.Name, err)
		l.Infof("%v", err.Error())
		l.Error(err.Error())
		return response, err
	}

	response.Output = Outputs{Inputs: request.Inputs}

	return response, nil
}

func sliceToString(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	// Sort for consistent output and easier comparison later
	// sortedSlice := make([]string, len(slice))
	// copy(sortedSlice, slice)
	// sort.Strings(sortedSlice)
	// return strings.Join(sortedSlice, ",")
	sort.Strings(slice)
	return strings.Join(slice, ",")
}

func stringToSlice(str string) []string {
	if str == "" {
		return []string{}
	}
	parts := strings.Split(str, ",")
	slice := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			slice = append(slice, trimmed)
		}
	}
	// Sort for consistent output and easier comparison
	sort.Strings(slice)
	return slice
}
