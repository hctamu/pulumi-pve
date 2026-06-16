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
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// SDNOperations defines the interface for SDN apply operations.
type SDNOperations interface {
	// Lock acquires the SDN cluster lock and returns a lock token.
	// retryTimeout controls how long lock acquisition retries should continue.
	// When allowPending is true, the lock request includes allow-pending so that
	// Proxmox accepts the lock even when there are pending changes.
	Lock(ctx context.Context, retryTimeout time.Duration, allowPending bool) (string, error)

	// Apply applies pending SDN configuration changes via PUT /cluster/sdn.
	// lockToken must be the token returned by Lock.
	// applyTimeout controls how long to wait for the apply task to complete.
	Apply(ctx context.Context, lockToken string, applyTimeout time.Duration) error
}

// SDNApplyInputs represents the input properties for the SDNApply resource.
type SDNApplyInputs struct {
	Triggers            map[string]any `pulumi:"triggers,optional"`
	LockTimeoutSeconds  int            `pulumi:"lockTimeoutSeconds,optional"`
	ApplyTimeoutSeconds int            `pulumi:"applyTimeoutSeconds,optional"`
	AllowPending        bool           `pulumi:"allowPending,optional"`
}

// Annotate adds descriptions to the SDNApply input properties.
func (inputs *SDNApplyInputs) Annotate(a infer.Annotator) {
	a.Describe(
		&inputs.Triggers,
		"Arbitrary key-value pairs that can include resource outputs or complex objects. "+
			"When any trigger value changes, the SDN apply is re-executed.",
	)
	a.Describe(
		&inputs.LockTimeoutSeconds,
		"How long to keep retrying SDN lock acquisition before failing, in seconds. Defaults to 60.",
	)
	a.SetDefault(&inputs.LockTimeoutSeconds, 60)
	a.Describe(
		&inputs.ApplyTimeoutSeconds,
		"How long to wait for the SDN apply task to complete, in seconds. Defaults to 60.",
	)
	a.SetDefault(&inputs.ApplyTimeoutSeconds, 60)
	a.Describe(
		&inputs.AllowPending,
		"When true, allows acquiring the SDN lock even when there are pending changes. Defaults to false.",
	)
}

// SDNApplyOutputs represents the output properties for the SDNApply resource.
type SDNApplyOutputs struct {
	SDNApplyInputs
}

// SDNApplyBody is the request body for the SDN apply PUT request (API level).
type SDNApplyBody struct {
	Lock        string `json:"lock-token,omitempty"`
	ReleaseLock int    `json:"release-lock,omitempty"`
}

// SDNLockBody is the request body for the SDN lock POST request (API level).
type SDNLockBody struct {
	AllowPending int `json:"allow-pending,omitempty"`
}
