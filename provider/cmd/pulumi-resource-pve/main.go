// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	p "github.com/pulumi/pulumi-go-provider"

	pve "github.com/hctamu/pulumi-pve/provider/pkg/provider"
)

// Serve the provider against Pulumi's Provider protocol.
func main() {
	provider := pve.Provider()

	p.RunProvider(pve.Name, pve.Version, provider)
}
