// Package config provides configuration structures for the PVE provider.
package config

// Config holds the configuration for the PVE provider.
type Config struct {
	PveURL   string `pulumi:"pveUrl"`
	PveUser  string `pulumi:"pveUser"`
	PveToken string `pulumi:"pveToken" provider:"secret"`
	SSHUser  string `pulumi:"sshUser"`
	SSHPass  string `pulumi:"sshPass" provider:"secret"`
}
