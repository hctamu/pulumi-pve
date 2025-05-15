package px

import (
	"context"
	"fmt"
)

type HaResource struct {
	Group  string   `json:"group,omitempty"`
	State  string   `json:"state,omitempty"`
	Sid    string   `json:"sid"`
	Delete []string `json:"delete,omitempty"`
}

// CreateHA creates a new HA resource
func (client *Client) CreateHA(ctx context.Context, ha *HaResource) (err error) {
	if err = client.Post(ctx, "/cluster/ha/resources/", ha, nil); err != nil {
		err = fmt.Errorf("failed to create HA resource %v ", err.Error())
	}
	return err
}

// DeleteHA deletes an existing HA resource
func (client *Client) DeleteHA(ctx context.Context, id int) (err error) {
	deleteUrl := fmt.Sprintf("/cluster/ha/resources/%v", id)
	if err = client.Delete(ctx, deleteUrl, nil); err != nil {
		err = fmt.Errorf("failed to delete HA resource %v ", err.Error())
	}
	return err
}

// UpdateHA updates an existing HA resource
func (client *Client) UpdateHA(ctx context.Context, id int, ha *HaResource) (err error) {
	url := fmt.Sprintf("/cluster/ha/resources/%v", id)
	if err = client.Put(ctx, url, ha, nil); err != nil {
		err = fmt.Errorf("failed to update HA resource %v ", err.Error())
	}
	return err
}

// GetHA retrieves an existing HA resource
func (client *Client) GetHA(ctx context.Context, id int) (ha *HaResource, err error) {
	url := fmt.Sprintf("/cluster/ha/resources/%v", id)
	if err = client.Get(ctx, url, &ha); err != nil {
		err = fmt.Errorf("failed to get HA resource %v ", err.Error())
	}
	return ha, err
}
