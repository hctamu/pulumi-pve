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

// Package config provides configuration structures for the PVE provider.
package config

// Config holds the configuration for the PVE provider.
type Config struct {
	// PveURL is the URL of the Proxmox VE API server (e.g., https://pve.example.com:8006).
	PveURL string `pulumi:"pveUrl"`

	// PveUser is the Proxmox VE user for authentication (e.g., root@pam or user@pve).
	PveUser string `pulumi:"pveUser"`

	// PveToken is the API token for Proxmox VE authentication.
	// This should be a token generated in the Proxmox VE UI and is marked as secret.
	PveToken string `pulumi:"pveToken" provider:"secret"`

	// SSHUser is the SSH user for connecting to Proxmox VE nodes (e.g., root).
	SSHUser string `pulumi:"sshUser"`

	// SSHPass is the SSH password for authenticating to Proxmox VE nodes.
	// This is marked as secret and should not be logged or exposed.
	SSHPass string `pulumi:"sshPass" provider:"secret"`

	// InsecureSkipVerify disables TLS certificate verification for HTTPS connections to the Proxmox VE API.
	// Only use this for testing with self-signed certificates.
	// Defaults to false (certificates are verified).
	InsecureSkipVerify bool `pulumi:"insecureSkipVerify,optional"`

	// InsecureIgnoreHostKey disables SSH host key verification when connecting to Proxmox VE nodes.
	// Only use this for testing environments. In production, ensure ~/.ssh/known_hosts is properly configured.
	// Defaults to false (host keys are verified against ~/.ssh/known_hosts).
	InsecureIgnoreHostKey bool `pulumi:"insecureIgnoreHostKey,optional"`
}
