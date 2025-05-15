# Pulumi PVE Provider

The Pulumi PVE Provider enables you to manage Proxmox Virtual Environment (PVE) resources using Pulumi. This document provides an overview of the installation and configuration process for the provider.

## Installation

To use the Pulumi PVE Provider, install the Pulumi CLI and ensure it is configured for your environment. Then, add the provider to your Pulumi project.

### Prerequisites

- Pulumi CLI installed ([installation guide](https://www.pulumi.com/docs/get-started/install/))
- Go programming language installed (if using the Go SDK)
- Access to a Proxmox VE instance

### Installing the Provider

#### Go

If you are using the Go SDK, add the provider to your `go.mod` file:

```bash
# Navigate to your Pulumi project directory
cd examples/go

# Add the Pulumi PVE provider
go get github.com/hctamu/pulumi-pve/sdk/go/pve
```

#### YAML

For YAML-based Pulumi projects, include the provider in your `Pulumi.yaml` file. Below is an example configuration:

```yaml
name: provider-pve
runtime: yaml
plugins:
  providers:
    - name: pve
      path: ../../bin

resources:
  myFile:
    type: pve:storage:File
    properties:
      datastoreId: cephfs
      contentType: snippets
      sourceRaw:
        fileData: |
          hello world
          this is the updated file content
        fileName: testfile03.yaml

  myVM:
    type: pve:vm:Vm
    properties:
      name: "testVMEE"
      description: "test VM EE, changed"
      cpu: "EPYC-v3"
      memory: 32
      disks:
        - storage: "ceph-ha"
          size: 20
          interface: "scsi0"
        - storage: "ceph-ha"
          size: 17
          interface: "scsi1"
        - storage: "ceph-ha"
          size: 20
          interface: "sata0"
      clone:
        vmId: 102
        timeout: 360
        fullClone: true
```

### Implemented Resources

The Pulumi PVE Provider currently supports the following resources:

- **pve:storage:File**: Manage file-based storage in Proxmox VE (needs SSH access).
- **pve:vm:Vm**: Manage virtual machines in Proxmox VE.
- **pve:ha:Ha**: Manage high availability (HA) configurations in Proxmox VE.
- **pve:pool:Pool**: Manage resource pools in Proxmox VE.

These resources allow you to define and manage Proxmox VE infrastructure programmatically using Pulumi.

### Examples

{{< chooser language "javascript,typescript,python,go,csharp,yaml,java" >}}

{{% choosable language go %}}
```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/hctamu/pulumi-pve/sdk/go/pve/pool"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := pool.NewPool(ctx, "myPool", &pool.PoolArgs{
			Name:    pulumi.String("myPool"),
			Comment: pulumi.String("myPool").ToStringPtrOutput(),
		})

		if err != nil {
			return err
		}

		return nil
	})
}
```

{{% /choosable %}}

{{% choosable language yaml %}}
```yaml
name: provider-pve
runtime: yaml
plugins:
  providers:
    - name: pve
      path: ../../bin

resources:
  myFile:
    type: pve:storage:File
    properties:
      datastoreId: cephfs
      contentType: snippets
      sourceRaw:
        fileData: |
          hello world
          this is the updated file content
        fileName: testfile03.yaml

  myVM:
    type: pve:vm:Vm
    properties:
      name: "testVMEE"
      description: "test VM EE, changed"
      cpu: "EPYC-v3"
      memory: 32
      disks:
        - storage: "ceph-ha"
          size: 20
          interface: "scsi0"
        - storage: "ceph-ha"
          size: 17
          interface: "scsi1"
        - storage: "ceph-ha"
          size: 20
          interface: "sata0"
      clone:
        vmId: 102
        timeout: 360
        fullClone: true
```

{{% /choosable %}}

{{< /chooser >}}

### Choosable Parts

The Pulumi PVE Provider documentation includes the following sections to help you get started:

- [Installation](#installation): Learn how to install the Pulumi PVE Provider.
- [Configuration](./installation-configuration.mdonfiguration): Understand how to configure the provider for your Proxmox VE instance.
- [Implemented Resources](#implemented-resources): Explore the resources currently supported by the provider.
- [Examples](#examples): See examples of how to use the provider in your Pulumi projects.

## Next Steps

- Explore the [examples](../examples/) directory for sample projects.
- Refer to the [Pulumi documentation](https://www.pulumi.com/docs/) for more details on using Pulumi.
- Check the [Proxmox VE API documentation](https://pve.proxmox.com/pve-docs/api-viewer/index.html) for available resources and operations.