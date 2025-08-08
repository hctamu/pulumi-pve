package client

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"sync"

	"github.com/hctamu/pulumi-pve/provider/pkg/config"
	"github.com/hctamu/pulumi-pve/provider/px"
	api "github.com/luthermonson/go-proxmox"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

var client *px.Client
var once sync.Once

// newClient creates a new Proxmox client
func newClient(PveURL string, pveUser string, pveToken string) (client *px.Client, err error) {
	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient := http.DefaultClient
	httpClient.Transport = transport

	apiClient := api.NewClient(PveURL,
		api.WithAPIToken(pveUser, pveToken),
		api.WithHTTPClient(httpClient),
	)

	client = &px.Client{Client: apiClient}

	return client, nil
}

// GetProxmoxClient returns a Proxmox client dik
func GetProxmoxClient(ctx context.Context) (ret *px.Client, err error) {
	once.Do(func() {
		p.GetLogger(ctx).Debugf("Client is not initialized, initializing now")
		pveConfig := infer.GetConfig[config.Config](ctx)
		pveURL := os.Getenv("PVE_API_URL")
		if pveURL != "" {
			pveConfig.PveURL = pveURL
		}
		client, err = newClient(pveConfig.PveURL, pveConfig.PveUser, pveConfig.PveToken)
	})

	if err != nil {
		p.GetLogger(ctx).Errorf("Error creating Proxmox client: %v", err)
		return nil, err
	}

	return client, nil
}
