package config

// Define some provider-level configuration
type Config struct {
	PveUrl   string `pulumi:"pveUrl"`
	PveUser  string `pulumi:"pveUser"`
	PveToken string `pulumi:"pveToken" provider:"secret"`
	SSHUser  string `pulumi:"sshUser"`
	SSHPass  string `pulumi:"sshPass" provider:"secret"`
}
