// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// 	http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package px provides a client wrapper for interacting with Proxmox VE via the go-proxmox API.
package px

import (
	api "github.com/luthermonson/go-proxmox"
)

// Client wraps the go-proxmox API client to provide additional functionality for interacting with Proxmox VE.
type Client struct {
	*api.Client
}
