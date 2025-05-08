package ha

import (
	"context"
	"fmt"
	"strconv"

	px2 "github.com/hctamu/pulumi-pve/provider/px"

	"github.com/hctamu/pulumi-pve/provider/pkg/client"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Ensure Ha implements the required interfaces
var _ = (infer.CustomResource[HaInput, HaOutput])((*Ha)(nil))
var _ = (infer.CustomDelete[HaOutput])((*Ha)(nil))
var _ = (infer.CustomUpdate[HaInput, HaOutput])((*Ha)(nil))
var _ = (infer.CustomRead[HaInput, HaOutput])((*Ha)(nil))
var _ = (infer.CustomDiff[HaInput, HaOutput])((*Ha)(nil))

// Ha represents a Proxmox HA resource
type Ha struct{}

// HaState represents the state of the HA resource
type HaState string

const (
	StateEnabled  HaState = "ignored"
	StateDisabled HaState = "started"
	StateUnknown  HaState = "stopped"
)

// ValidateState validates the HA state
func (state HaState) ValidateState(ctx context.Context) (err error) {
	switch state {
	case StateEnabled, StateDisabled, StateUnknown:
		return nil
	default:
		err = fmt.Errorf("invalid state: %s", state)
		p.GetLogger(ctx).Error(err.Error())
		return err
	}
}

// HaInput represents the input properties for the HA resource
type HaInput struct {
	Group      string  `pulumi:"group,optional"`
	State      HaState `pulumi:"state,optional"`
	ResourceId int     `pulumi:"resourceId"`
}

// HaOutput represents the output properties for the HA resource
type HaOutput struct {
	HaInput
}

// Create creates a new HA resource
func (ha *Ha) Create(ctx context.Context, id string, inputs HaInput, preview bool) (idRet string, output HaOutput, err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Creating ha resource: %v", inputs)
	output = HaOutput{HaInput: inputs}
	idRet = id

	if preview {
		return idRet, output, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return idRet, output, nil
	}

	err = pxc.CreateHA(ctx, &px2.HaResource{
		Group: inputs.Group,
		State: string(inputs.State),
		Sid:   strconv.Itoa(inputs.ResourceId),
	})

	return idRet, output, err
}

// Delete deletes an existing HA resource
func (ha *Ha) Delete(ctx context.Context, id string, output HaOutput) (err error) {
	logger := p.GetLogger(ctx)
	logger.Debugf("Deleting ha resource: %v", output)

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return err
	}

	if err = pxc.DeleteHA(ctx, output.ResourceId); err != nil {
		return err
	}
	logger.Debugf("HA resource %v deleted", id)

	return nil
}

// Update updates an existing HA resource
func (ha *Ha) Update(ctx context.Context, id string, haOutput HaOutput, haInput HaInput, preview bool) (
	haOutputRet HaOutput, err error) {

	logger := p.GetLogger(ctx)
	logger.Debugf("Updating ha resource: %v", id)
	haOutputRet = haOutput

	if preview {
		return haOutputRet, nil
	}

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return haOutputRet, err
	}

	haOutputRet.HaInput = haInput

	haResource := px2.HaResource{
		State: string(haInput.State),
	}

	if haInput.Group == "" && haOutput.Group != "" {
		haResource.Delete = []string{"group"}
	} else if haInput.Group != "" {
		haResource.Group = haInput.Group
	}
	err = pxc.UpdateHA(ctx, haOutput.ResourceId, &haResource)

	return haOutputRet, err
}

// Read reads the current state of an HA resource
func (ha *Ha) Read(ctx context.Context, id string, inputs HaInput, output HaOutput) (
	canonicalID string, normalizedInputs HaInput, normalizedOutputs HaOutput, err error) {

	logger := p.GetLogger(ctx)
	logger.Debugf("Reading ha resource: %v", id)
	normalizedInputs = inputs
	normalizedOutputs = output
	canonicalID = id

	var pxc *px2.Client
	if pxc, err = client.GetProxmoxClient(ctx); err != nil {
		return canonicalID, normalizedInputs, normalizedOutputs, err
	}

	var haResource *px2.HaResource

	if haResource, err = pxc.GetHA(ctx, inputs.ResourceId); err != nil {
		return canonicalID, normalizedInputs, normalizedOutputs, err
	}

	normalizedInputs.Group = haResource.Group
	normalizedInputs.State = HaState(haResource.State)
	normalizedInputs.ResourceId = output.ResourceId

	normalizedOutputs.Group = haResource.Group
	normalizedOutputs.State = HaState(haResource.State)
	normalizedOutputs.ResourceId = output.ResourceId

	return canonicalID, normalizedInputs, normalizedOutputs, nil
}

// Diff computes the difference between the desired and actual state of an HA resource
func (ha *Ha) Diff(ctx context.Context, id string, olds HaOutput, news HaInput) (response p.DiffResponse, err error) {
	diff := map[string]p.PropertyDiff{}
	if olds.ResourceId != news.ResourceId {
		diff["resourceId"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}

	if olds.Group != news.Group {
		diff["group"] = p.PropertyDiff{Kind: p.Update}
	}

	if olds.State != news.State {
		diff["state"] = p.PropertyDiff{Kind: p.Update}
	}

	response = p.DiffResponse{
		DeleteBeforeReplace: true,
		HasChanges:          len(diff) > 0,
		DetailedDiff:        diff,
	}

	return response, nil
}

// Annotate adds descriptions to the HA resource and its properties
func (ha *Ha) Annotate(a infer.Annotator) {
	a.Describe(ha, "A Proxmox HA resource that manages the HA configuration of a virtual machine in the Proxmox VE.")
}

func (inputs *HaInput) Annotate(a infer.Annotator) {
	a.Describe(&inputs.Group, "The HA group identifier.")
	a.Describe(&inputs.ResourceId, "The ID of the virtual machine that will be managed by HA (required).")
	a.Describe(&inputs.State, "The state of the HA resource (default: started).")
	a.SetDefault(&inputs.State, "started")
}
