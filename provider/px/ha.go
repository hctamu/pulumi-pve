// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package px

import (
	"context"
	"fmt"
)

// HaResource represents a high availability resource in the Proxmox cluster.
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
	deleteURL := fmt.Sprintf("/cluster/ha/resources/%v", id)
	if err = client.Delete(ctx, deleteURL, nil); err != nil {
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
