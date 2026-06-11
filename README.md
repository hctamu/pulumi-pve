# Pulumi Proxmox VE Provider

This repository contains a Pulumi Provider for managing Proxmox VE resources. It allows you to define and manage Proxmox VE resources using Pulumi's infrastructure-as-code approach.

## Getting Started

### Prerequisites

To work with this repository, you need to use the provided development container (`devcontainer`). The devcontainer includes all the necessary tools and dependencies pre-installed.

### Setting Up the Devcontainer

1. Open this repository in Visual Studio Code.
2. Install the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension.
3. Reopen the repository in the devcontainer by selecting **Reopen in Container** from the Command Palette (`Ctrl+Shift+P`).

Once the devcontainer is up and running, you can start developing and testing the provider.

#### A brief repository overview

You now have:

1. A `provider/` folder containing the building and implementation logic
    - `cmd/pulumi-resource-pve/main.go` - holds the provider's sample implementation logic.
2. `sdk` - holds the generated code libraries created by `pulumi-gen-pve/main.go`
3. `examples` a folder of Pulumi programs to try locally and/or use in CI.
4. A `Makefile` and this README.

##### Additional Details

This repository depends on the pulumi-go-provider library. For more details on building providers, please check the [Pulumi Go Provider](https://github.com/pulumi/pulumi-go-provider) docs.

NPM repository: <https://www.npmjs.com/settings/hctamu/packages>
Nuget repository: <https://www.nuget.org/packages/Hctamu.Pve>
PyPi repository: <https://pypi.org/project/pulumi-pve/>

### Release new version

To release new version create a new release on Github, with the following tag syntax: v*.\*.\*

A pipeline will automatically release the provider with the given version.


### Build the provider and install the plugin

```bash
make build install
```

This will:

1. Create the SDK codegen binary and place it in a ./bin folder (gitignored)
2. Create the provider binary and place it in the ./bin folder (gitignored)
3. Generate the ~~dotnet~~, Go, ~~Node, and Python~~ SDKs and place them in the ./sdk folder
4. Install the provider on your machine.

## Configuration

The Pulumi Proxmox VE provider requires the following configuration settings:

### Required Settings

- **`pve:pveUrl`** - The URL of the Proxmox VE API server (e.g., `https://pve.example.com:8006`)
- **`pve:pveUser`** - The Proxmox VE user for authentication (e.g., `root@pam` or `user@pve`)
- **`pve:pveToken`** - An API token generated in the Proxmox VE UI (marked as secret)
- **`pve:sshUser`** - The SSH user for connecting to Proxmox VE nodes (e.g., `root`)
- **`pve:sshPass`** - The SSH password for authenticating to Proxmox VE nodes (marked as secret)

### Optional Settings

- **`pve:insecureSkipVerify`** - Disable TLS certificate verification for HTTPS connections. Defaults to `false`. ⚠️ Only use for testing with self-signed certificates.
- **`pve:insecureIgnoreHostKey`** - Disable SSH host key verification when connecting to nodes. Defaults to `false`. ⚠️ Only use for testing environments. In production, ensure `~/.ssh/known_hosts` is properly configured.

### Configuration Example

Set these in your Pulumi stack configuration (e.g., `Pulumi.dev.yaml`):

```yaml
config:
  pve:pveUrl: https://pve.local:8006
  pve:pveUser: root@pam
  pve:pveToken:
    secure: AQAAAA...  # Use `pulumi config set --secret` for token
  pve:sshUser: root
  pve:sshPass:
    secure: AQAAAA...  # Use `pulumi config set --secret` for password
  pve:insecureSkipVerify: false
  pve:insecureIgnoreHostKey: false
```

Or set via environment variables:

```bash
export PULUMI_CONFIG_PVE_PVEURL=https://pve.local:8006
export PULUMI_CONFIG_PVE_PVEUSER=root@pam
export PULUMI_CONFIG_PVE_PVETOKEN=your-token-here
export PULUMI_CONFIG_PVE_SSHUSER=root
export PULUMI_CONFIG_PVE_SSHPASS=your-password-here
```
