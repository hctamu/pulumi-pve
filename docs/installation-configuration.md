---
title: PVE Provider Installation & Configuration
meta_desc: Information on how to install the Pulumi PVE provider.
layout: package
---

## Installation

The Pulumi PVE provider is available as a package in the following Pulumi languages:

* Go: [`github.com/hctamu/pulumi-pve/sdk/go/pve`](https://pkg.go.dev/github.com/hctamu/pulumi-pve/sdk)

### Go

To install the Pulumi PVE provider for Go, add the `github.com/hctamu/pulumi-pve/sdk/go/pve` to your `go.mod` file:

## Configuration

The Pulumi PVE Provider requires configuration to connect to your Proxmox VE instance. You can set these values using Pulumi configuration commands or environment variables.

### Required Configuration

* `pve:apiUrl`: The URL of the Proxmox VE API endpoint.
* `pve:username`: The username for authentication.
* `pve:password`: The password for authentication.

### Current Configuration Options

The Pulumi PVE Provider supports the following configuration options:

* `pve:pveUrl`: The URL of the Proxmox VE API endpoint.
* `pve:pveUser`: The username for authentication.
* `pve:pveToken`: The API token for authentication (marked as secret).
* `pve:sshUser`: The SSH username for connecting to Proxmox VE.
* `pve:sshPass`: The SSH password for connecting to Proxmox VE (marked as secret).

These configurations can currently only be set using Pulumi's configuration system. Environment variable support is planned for a future release.

### Adding Secrets to Configuration

The Pulumi PVE Provider allows you to securely store sensitive information, such as API tokens or passwords, as secrets in the configuration. Secrets are encrypted and managed securely by Pulumi.

To add a secret to the configuration, use the `--secret` flag with the `pulumi config set` command. For example:

```bash
pulumi config set --secret pve:pveToken your-api-token
pulumi config set --secret pve:sshPass your-ssh-password
```

These secrets will be encrypted and stored securely in your Pulumi stack configuration. When accessed in your Pulumi program, they will remain encrypted in logs and outputs unless explicitly decrypted.