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

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"golang.org/x/crypto/ssh"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

var (
	sshClient *SSHClient
	sshOnce   sync.Once
)

// SSHCommand represents a command that can be executed over SSH.
type SSHCommand struct {
	string
}

// SSHClient represents an SSH client connection and its configuration.
type SSHClient struct {
	Client   *ssh.Client
	Config   *ssh.ClientConfig
	TargetIP string
}

// Delete returns an SSHCommand for removing files.
func (sc SSHClient) Delete() SSHCommand {
	return SSHCommand{"rm"}
}

// Read returns an SSHCommand for reading files.
func (sc SSHClient) Read() SSHCommand {
	return SSHCommand{"cat"}
}

// Write returns an SSHCommand for writing to files.
func (sc SSHClient) Write() SSHCommand {
	return SSHCommand{"cat >"}
}

// Run executes a command on the remote host and returns its output.
func (sc *SSHClient) Run(command SSHCommand, filePath string, data ...string) (string, error) {
	// Dial a new SSH connection
	client, err := ssh.Dial("tcp", sc.TargetIP+":22", sc.Config)
	if err != nil {
		return "", fmt.Errorf("error creating ssh client: %v", err)
	}
	sc.Client = client

	// Create a new session for this operation.
	session, err := sc.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("error creating new ssh session: %v", err)
	}
	defer func() {
		if cerr := session.Close(); cerr != nil {
			fmt.Printf("error closing SSH session: %v\n", cerr)
		}
	}()

	var out []byte
	if len(data) < 1 {
		// Execute the command and capture its combined output.
		out, err = session.CombinedOutput(fmt.Sprintf("%s %s", command.string, filePath))
		if err != nil {
			return "", fmt.Errorf("error executing command: %v, output: %s", err, string(out))
		}
	} else {
		// If there is data

		// Obtain the stdin pipe to send data.
		stdin, err := session.StdinPipe()
		if err != nil {
			return "", fmt.Errorf("error obtaining the stdin pipe: %v", err)
		}
		// Start the command on the remote host.
		if err := session.Start(fmt.Sprintf("%s %s", command.string, filePath)); err != nil {
			return "", fmt.Errorf("error starting session: %v", err)
		}

		// Write data to the remote process.
		if _, err := fmt.Fprintf(stdin, "%s\n", data[0]); err != nil {
			return "", fmt.Errorf("error writing string: %v", err)
		}

		// Closing stdin signals EOF to the remote command.
		if closeErr := stdin.Close(); closeErr != nil {
			return "", fmt.Errorf("error closing stdin: %v", closeErr)
		}

		if err := session.Wait(); err != nil {
			return "", err
		}

	}

	return string(out), nil
}

// newSSHClient creates a new SSH client with the provided username and password.
func newSSHClient(ctx context.Context, sshUser, sshPass string) (*SSHClient, error) {
	//nolint:gosec // Ignoring host key for internal infrastructure
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPass), // Use public key authentication for better security
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshIP, err := generateSSHHost(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting random host: %v", err)
	}

	client := &SSHClient{
		Config:   sshConfig,
		TargetIP: sshIP,
	}

	return client, nil
}

// GetSSHClient returns a singleton instance of the SSHClient.
// It initializes the SSH client if it hasn't been created yet.
func GetSSHClient(ctx context.Context) (*SSHClient, error) {
	var err error
	sshOnce.Do(func() {
		p.GetLogger(ctx).Debugf("SSH Client is not initialized, initializing it now")
		pveConfig := infer.GetConfig[config.Config](ctx)
		sshClient, err = newSSHClient(ctx, pveConfig.SSHUser, pveConfig.SSHPass)
	})

	return sshClient, err
}

// generateSSHHost generates a random SSH host from the Proxmox nodes.

func generateSSHHost(ctx context.Context) (string, error) {
	proxmoxClient, err := GetProxmoxClient(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting proxmox client: %v", err)
	}

	nodes, err := proxmoxClient.Nodes(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting nodes: %v", err)
	}

	nodeCount := big.NewInt(int64(len(nodes)))
	nBig, err := rand.Int(rand.Reader, nodeCount)
	if err != nil {
		return "", fmt.Errorf("error generating secure random index: %v", err)
	}
	selectedNodeStatus := nodes[nBig.Int64()]
	nodeObject, err := proxmoxClient.Node(ctx, selectedNodeStatus.Node)
	if err != nil {
		return "", fmt.Errorf("error getting selected node %s: %v", selectedNodeStatus.ID, err)
	}

	nodeNetworks, err := nodeObject.Networks(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting networks for %s: %v", nodeObject.Name, err)
	}

	for _, nic := range nodeNetworks {
		if nic.Iface == "vmbr1.606" {
			return nic.Address, nil
		}
	}
	return "", nil
}
