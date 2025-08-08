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

package client

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"golang.org/x/crypto/ssh"
)

var sshClient *SSHClient
var sshOnce sync.Once

type SSHCommand struct {
	string
}

// SSHClient represents an SSH client connection and its configuration.
type SSHClient struct {
	Client   *ssh.Client
	Config   *ssh.ClientConfig
	TargetIP string
}

func (sc SSHClient) Delete() SSHCommand {
	return SSHCommand{"rm"}
}

func (sc SSHClient) Read() SSHCommand {
	return SSHCommand{"cat"}
}

func (sc SSHClient) Write() SSHCommand {
	return SSHCommand{"cat >"}
}

// Run executes a command on the remote host and returns its output.
func (sc *SSHClient) Run(command SSHCommand, filePath string, data ...string) (output string, err error) {
	// Dial a new SSH connection
	sc.Client, err = ssh.Dial("tcp", sc.TargetIP+":22", sc.Config)
	if err != nil {
		return output, fmt.Errorf("error creating ssh client: %v", err)
	}

	// Create a new session for this operation.
	var session *ssh.Session
	session, err = sc.Client.NewSession()
	if err != nil {
		return output, fmt.Errorf("error creating new ssh session: %v", err)
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
			return output, fmt.Errorf("error executing command: %v, output: %s", err, string(out))
		}
	} else {
		// If there is data

		// Obtain the stdin pipe to send data.
		stdin, err := session.StdinPipe()
		if err != nil {
			return output, fmt.Errorf("error obtaining the stdin pipe: %v", err)
		}
		// Start the command on the remote host.
		if err := session.Start(fmt.Sprintf("%s %s", command.string, filePath)); err != nil {
			return output, fmt.Errorf("error starting session: %v", err)
		}

		// Write data to the remote process.
		_, err = fmt.Fprintf(stdin, "%s\n", data[0])
		if err != nil {
			return output, fmt.Errorf("error writing string: %v", err)
		}

		// Closing stdin signals EOF to the remote command.
		if closeErr := stdin.Close(); closeErr != nil {
			return output, fmt.Errorf("error closing stdin: %v", closeErr)
		}

		err = session.Wait()
		if err != nil {
			return output, err
		}

	}

	output = string(out)
	return output, err
}

// newSSHClient creates a new SSH client with the provided username and password.
func newSSHClient(ctx context.Context, sshUser, sshPass string) (client *SSHClient, err error) {
	sshConfig := &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(sshPass), // Use public key authentication for better security
		},
		// WARNING: Using InsecureIgnoreHostKey is insecure and should be replaced with proper host key verification in production.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshIP, err := generateSSHHost(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting random host: %v", err)
	}

	client = &SSHClient{
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
func generateSSHHost(ctx context.Context) (host string, err error) {
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
			host = nic.Address
			break
		}
	}
	return host, nil
}
