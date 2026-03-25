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

package client

import "context"

// SSHOperation represents a type of SSH command to be executed.
type SSHOperation string

const (
	// SSHOperationWrite is used for writing data to a file on the remote host.
	SSHOperationWrite SSHOperation = "cat >"
	// SSHOperationRead is used for reading data from a file on the remote host.
	SSHOperationRead SSHOperation = "cat"
	// SSHOperationDelete is used for deleting a file on the remote host.
	SSHOperationDelete SSHOperation = "rm"
)

// SSHClientInterface defines the operations our SSH client performs.
type SSHClientInterface interface {
	// Run executes a command on the remote host and returns its output.
	// The 'command' parameter will be one of SSHOperationWrite, SSHOperationRead, or SSHOperationDelete.
	// The 'filePath' is the target file path on the remote system.
	// The 'data' parameter is optional and used for write operations.
	Run(command SSHOperation, filePath string, data ...string) (string, error)
}

// GetSSHClientFunc is a function type that returns an SSHClientInterface.
type GetSSHClientFunc func(ctx context.Context) (SSHClientInterface, error)
