// Package px provides a client wrapper for interacting with Proxmox VE via the go-proxmox API.
package px

import (
	api "github.com/luthermonson/go-proxmox"
)

// Client wraps the go-proxmox API client to provide additional functionality for interacting with Proxmox VE.
type Client struct {
	*api.Client
}
