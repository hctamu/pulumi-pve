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
	"fmt"
	"regexp"
)

// sdnNameRegexp enforces the Proxmox SDN identifier rule: start with a letter, alphanumeric only, max 8 chars.
var sdnNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{0,7}$`)

// ValidateSDNName enforces the Proxmox SDN identifier rule used by VNet and zone names:
// start with a letter, alphanumeric only, 1-8 characters.
func ValidateSDNName(name string) error {
	if !sdnNameRegexp.MatchString(name) {
		return fmt.Errorf(
			"invalid name %q: must start with a letter, contain only letters and numbers, and be at most 8 characters",
			name,
		)
	}
	return nil
}
