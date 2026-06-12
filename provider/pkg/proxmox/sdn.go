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

	"github.com/pulumi/pulumi-go-provider/infer"
)

// SdnOperations defines the interface for SDN apply operations.
type SdnOperations interface {
	// Apply applies pending SDN configuration changes via PUT /cluster/sdn.
	Apply(ctx context.Context) error
}

// SdnApplyInputs represents the input properties for the SdnApply resource.
type SdnApplyInputs struct {
	Triggers map[string]any `pulumi:"triggers,optional"`
}

// Annotate adds descriptions to the SdnApply input properties.
func (inputs *SdnApplyInputs) Annotate(a infer.Annotator) {
	a.Describe(
		&inputs.Triggers,
		"Arbitrary key-value pairs that can include resource outputs or complex objects. "+
			"When any trigger value changes, the SDN apply is re-executed.",
	)
}

// SdnApplyOutputs represents the output properties for the SdnApply resource.
type SdnApplyOutputs struct {
	SdnApplyInputs
}
