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

package proxmox

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// HAOperations defines the interface for High Availability resource operations.
type HAOperations interface {
	// Create creates a new HA resource.
	Create(ctx context.Context, inputs HAInputs) error

	// Get retrieves an existing HA resource by its ID.
	Get(ctx context.Context, id int) (*HAOutputs, error)

	// Update updates an existing HA resource.
	Update(ctx context.Context, id int, inputs HAInputs, oldOutputs HAOutputs) error

	// Delete deletes an existing HA resource by its ID.
	Delete(ctx context.Context, id int) error
}

// HaResource represents a high availability resource in the Proxmox cluster (API level).
// This is used for API communication only.
type HaResource struct {
	Group  string   `json:"group,omitempty"`
	State  string   `json:"state,omitempty"`
	Sid    string   `json:"sid"`
	Delete []string `json:"delete,omitempty"`
}

// HAState represents the state of the HA resource
type HAState string

const (
	// HAStateIgnored represents the "ignored" state for HA.
	HAStateIgnored HAState = "ignored"
	// HAStateStarted represents the "started" state for HA (default).
	HAStateStarted HAState = "started"
	// HAStateStopped represents the "stopped" state for HA.
	HAStateStopped HAState = "stopped"
)

// ValidateState validates the HA state
func (state HAState) ValidateState(ctx context.Context) error {
	switch state {
	case HAStateIgnored, HAStateStarted, HAStateStopped:
		return nil
	default:
		err := fmt.Errorf("invalid state: %s", state)
		p.GetLogger(ctx).Error(err.Error())
		return err
	}
}

// HAInputs represents the input properties for the HA resource
type HAInputs struct {
	Group      string  `pulumi:"group,optional"`
	State      HAState `pulumi:"state,optional"`
	ResourceID int     `pulumi:"resourceId"     provider:"replaceOnChanges"`
}

// Annotate adds descriptions to the Input properties for documentation and schema generation.
func (inputs *HAInputs) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Group, "The HA group identifier.")
	a.Describe(
		&inputs.ResourceID,
		"The ID of the virtual machine that will be managed by HA (required).",
	)
	a.Describe(&inputs.State, "The state of the HA resource (default: started).")
	a.SetDefault(&inputs.State, "started")
}

// HAOutputs represents the output properties for the HA resource
type HAOutputs struct {
	HAInputs
}
