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

package adapters

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/pkg/proxmox"
)

// Ensure SSHAdapter implements the proxmox.SSHClient interface.
var _ proxmox.SSHClient = (*SSHAdapter)(nil)

// SSHAdapter implements proxmox.SSHClient using the ProxmoxAdapter for node discovery.
type SSHAdapter struct {
	proxmoxAdapter *ProxmoxAdapter
	sshConfig      *ssh.ClientConfig
	targetIP       string
	once           sync.Once
	initErr        error
	PVEConfig      *config.Config
}

// NewSSHAdapter creates a new SSHAdapter wrapping the given ProxmoxAdapter.
func NewSSHAdapter(proxmoxAdapter *ProxmoxAdapter, cfg *config.Config) *SSHAdapter {
	return &SSHAdapter{
		proxmoxAdapter: proxmoxAdapter,
		PVEConfig:      cfg,
	}
}

// nodeStatus represents a Proxmox node from the /nodes API response.
type nodeStatus struct {
	Node string `json:"node"`
}

// networkInterface represents a network interface from the /nodes/{node}/network API response.
type networkInterface struct {
	Iface   string `json:"iface"`
	Address string `json:"address"`
}

// Connect initializes the SSH client configuration and discovers a target host.
// It is safe to call multiple times; initialization only happens once.
func (sa *SSHAdapter) Connect(ctx context.Context) error {
	sa.once.Do(func() {
		p.GetLogger(ctx).Debugf("SSH Client is not initialized, initializing it now")

		var cfg config.Config
		if sa.PVEConfig == nil {
			cfg = infer.GetConfig[config.Config](ctx)
		} else {
			cfg = *sa.PVEConfig
		}

		hostKeyCallback, callbackErr := newHostKeyCallback(cfg.InsecureIgnoreHostKey)
		if callbackErr != nil {
			sa.initErr = callbackErr
			return
		}

		sa.sshConfig = &ssh.ClientConfig{
			User: cfg.SSHUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(cfg.SSHPass),
			},
			HostKeyCallback: hostKeyCallback,
		}

		sa.targetIP, sa.initErr = sa.generateSSHHost(ctx)
	})
	return sa.initErr
}

func newHostKeyCallback(insecureIgnoreHostKey bool) (ssh.HostKeyCallback, error) {
	if insecureIgnoreHostKey {
		//nolint:gosec // Opt-in behavior controlled by provider config.
		return ssh.InsecureIgnoreHostKey(), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user home directory for SSH known_hosts: %w", err)
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to initialize SSH known_hosts verification from %q: %w",
			knownHostsPath,
			err,
		)
	}

	return hostKeyCallback, nil
}

// Run executes a command on the remote host and returns its output.
func (sa *SSHAdapter) Run(command proxmox.SSHOperation, filePath string, data ...string) (string, error) {
	// Dial a new SSH connection
	client, err := ssh.Dial("tcp", sa.targetIP+":22", sa.sshConfig)
	if err != nil {
		return "", fmt.Errorf("error creating ssh client: %v", err)
	}

	// Create a new session for this operation.
	session, err := client.NewSession()
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
		out, err = session.CombinedOutput(fmt.Sprintf("%s %s", command, filePath))
		if err != nil {
			return "", fmt.Errorf("error executing command: %v, output: %s", err, string(out))
		}
	} else {
		// Obtain the stdin pipe to send data.
		stdin, err := session.StdinPipe()
		if err != nil {
			return "", fmt.Errorf("error obtaining the stdin pipe: %v", err)
		}
		// Start the command on the remote host.
		if err := session.Start(fmt.Sprintf("%s %s", command, filePath)); err != nil {
			return "", fmt.Errorf("error starting session: %v", err)
		}

		// Write data to the remote process.
		if _, err := fmt.Fprintf(stdin, "%s", data[0]); err != nil {
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

// generateSSHHost discovers a random Proxmox node and returns its IP address
// by querying the Proxmox API through the ProxmoxAdapter.
func (sa *SSHAdapter) generateSSHHost(ctx context.Context) (string, error) {
	if err := sa.proxmoxAdapter.Connect(ctx); err != nil {
		return "", fmt.Errorf("error connecting proxmox adapter: %v", err)
	}

	var nodes []nodeStatus
	if err := sa.proxmoxAdapter.Get(ctx, "/nodes", &nodes); err != nil {
		return "", fmt.Errorf("error getting nodes: %v", err)
	}

	if len(nodes) == 0 {
		return "", errors.New("no nodes found")
	}

	nodeCount := big.NewInt(int64(len(nodes)))
	nBig, err := rand.Int(rand.Reader, nodeCount)
	if err != nil {
		return "", fmt.Errorf("error generating secure random index: %v", err)
	}
	selectedNode := nodes[nBig.Int64()]

	var networks []networkInterface
	if err := sa.proxmoxAdapter.Get(ctx, fmt.Sprintf("/nodes/%s/network", selectedNode.Node), &networks); err != nil {
		return "", fmt.Errorf("error getting networks for %s: %v", selectedNode.Node, err)
	}

	for _, nic := range networks {
		if nic.Iface == "vmbr1.606" {
			return nic.Address, nil
		}
	}
	return "", nil
}
