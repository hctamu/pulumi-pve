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
