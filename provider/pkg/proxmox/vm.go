/* Copyright 2025, Pulumi Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxmox

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// VMOperations defines the interface for interacting with Proxmox VM resources.
type VMOperations interface {
	// Create creates a new virtual machine and returns its assigned VM ID and node name.
	Create(ctx context.Context, inputs VMInputs) (vmID int, node string, err error)

	// Get retrieves the current state of a virtual machine.
	// It returns both computed inputs (full API state) and preserved inputs
	// (user-visible values with computed fields cleared when not provided by the user).
	Get(
		ctx context.Context,
		vmID int,
		node *string,
		existingInputs VMInputs,
	) (computed VMInputs, preserved VMInputs, err error)

	// Update applies configuration changes to an existing virtual machine.
	Update(ctx context.Context, vmID int, node *string, inputs VMInputs, stateInputs VMInputs) error

	// Delete deletes an existing virtual machine.
	Delete(ctx context.Context, vmID int, node *string) error
}

const (
	// EfiDiskID is the Proxmox key name for the EFI disk.
	EfiDiskID = "efidisk0"

	// EfiDiskInputName is the pulumi property name used for EFI disk diff tracking.
	EfiDiskInputName = "efidisk"

	// EfiDiskSize is constant because it is ignored by the API anyway.
	EfiDiskSize = "1"
)

// NumaNode represents a single NUMA node topology configuration.
type NumaNode struct {
	Cpus      string  `pulumi:"cpus"`
	HostNodes *string `pulumi:"hostNodes,optional"`
	Memory    *int    `pulumi:"memory,optional"`
	Policy    *string `pulumi:"policy,optional"`
}

// CPU represents the structured CPU configuration.
type CPU struct {
	Type          *string    `pulumi:"type,optional"`
	FlagsEnabled  []string   `pulumi:"flagsEnabled,optional"`
	FlagsDisabled []string   `pulumi:"flagsDisabled,optional"`
	Hidden        *bool      `pulumi:"hidden,optional"`
	HVVendorID    *string    `pulumi:"hvVendorId,optional"`
	PhysBits      *string    `pulumi:"physBits,optional"`
	Cores         *int       `pulumi:"cores,optional"`
	Sockets       *int       `pulumi:"sockets,optional"`
	Limit         *float64   `pulumi:"limit,optional"`
	Units         *int       `pulumi:"units,optional"`
	Vcpus         *int       `pulumi:"vcpus,optional"`
	Numa          *bool      `pulumi:"numa,optional"`
	NumaNodes     []NumaNode `pulumi:"numaNodes,optional"`
}

// Annotate provides documentation for the CPU type.
func (cpu *CPU) Annotate(a infer.Annotator) {
	a.Describe(
		&cpu,
		"CPU configuration for the virtual machine.",
	)
	a.SetDefault(&cpu.Cores, 1, "Number of CPU cores")
}

// Clone represents the configuration for cloning a virtual machine.
type Clone struct {
	VMID        int     `pulumi:"vmId"`
	DataStoreID *string `pulumi:"dataStoreId,optional"`
	FullClone   *bool   `pulumi:"fullClone,optional"`
	NodeID      *string `pulumi:"node,optional"`
	Timeout     int     `pulumi:"timeout,optional"`
}

// DiskBase contains common fields shared between Disk and EfiDisk.
type DiskBase struct {
	Storage string  `pulumi:"storage"`
	FileID  *string `pulumi:"filename,optional"` // Optional, computed if not provided
}

// Disk represents a virtual machine disk configuration.
type Disk struct {
	DiskBase
	Size      int    `pulumi:"size"`      // Size in Gigabytes (required for regular disks).
	Interface string `pulumi:"interface"` // Disk interface: "scsi0", "ide1", "virtio", etc.
}

// ToProxmoxDiskKeyConfig converts the Disk struct to Proxmox disk key and config strings.
func (disk Disk) ToProxmoxDiskKeyConfig() (diskKey, diskConfig string) {
	var fullDiskPath string

	if disk.FileID == nil || *disk.FileID == "" {
		// No file Id means we are creating the disk now, so we use the storage:size format to create the disk
		fullDiskPath = fmt.Sprintf("%v:%v", disk.Storage, disk.Size)
	} else {
		// We already have a disk file, so we use the storage:file_id format
		fullDiskPath = fmt.Sprintf("%v:%v", disk.Storage, *disk.FileID)
	}

	diskKey = disk.Interface
	diskConfig = fmt.Sprintf("file=%v,size=%v", fullDiskPath, disk.Size)
	return
}

// EfiType represents the EFI type for an EFI disk.
type EfiType string

// EFI type constants.
const (
	EfiType2M EfiType = "2m"
	EfiType4M EfiType = "4m"
)

// EfiDisk represents an EFI disk configuration.
type EfiDisk struct {
	DiskBase
	EfiType         EfiType `pulumi:"efitype"`
	PreEnrolledKeys *bool   `pulumi:"preEnrolledKeys,optional"`
}

// Annotate provides documentation for the EfiDisk type.
func (efiDisk *EfiDisk) Annotate(a infer.Annotator) {
	a.Describe(
		&efiDisk,
		"EFI disk configuration for the virtual machine.",
	)
}

// ValidateEfiType checks if the EfiType is valid.
func (efi EfiDisk) ValidateEfiType() error {
	switch efi.EfiType {
	case EfiType2M, EfiType4M:
		return nil
	default:
		return fmt.Errorf("invalid EFI type: %v", efi.EfiType)
	}
}

// VMInputs represents the input configuration for a virtual machine.
type VMInputs struct {
	Name        *string `pulumi:"name"`
	Description *string `pulumi:"description,optional"`
	Node        *string `pulumi:"node,optional"`
	VMID        *int    `pulumi:"vmId,optional"`
	Hookscript  *string `pulumi:"hookscript,optional"`
	Hotplug     *string `pulumi:"hotplug,optional"`
	Template    *int    `pulumi:"template,optional"`
	// Agent       *string `pulumi:"agent,optional"`
	Autostart *int `pulumi:"autostart,optional"`
	Tablet    *int `pulumi:"tablet,optional"`
	KVM       *int `pulumi:"kvm,optional"`
	// Tags       *string `pulumi:"tags,optional"`
	Protection *int    `pulumi:"protection,optional"`
	Lock       *string `pulumi:"lock,optional"`

	// Boot   *string `pulumi:"boot,optional"`
	// OnBoot *int    `pulumi:"onboot,optional"`

	OSType  *string `pulumi:"ostype,optional"`
	Machine *string `pulumi:"machine,optional"`
	Bio     *string `pulumi:"bios,optional"`

	EfiDisk *EfiDisk `pulumi:"efidisk,optional"`

	// SMBios1 *string `pulumi:"smbios1,optional"`
	Acpi *int `pulumi:"acpi,optional"`

	// Sockets  *int    `pulumi:"sockets,optional"`

	CPU       *CPU    `pulumi:"cpu,optional"`
	Memory    *int    `pulumi:"memory,optional"`
	Hugepages *string `pulumi:"hugepages,optional"`
	Balloon   *int    `pulumi:"balloon,optional"`

	VGA *string `pulumi:"vga,optional"`
	// SCSIHW    *string `pulumi:"scsihw,optional"`
	TPMState0 *string `pulumi:"tpmstate0,optional"`
	Rng0      *string `pulumi:"rng0,optional"`
	Audio0    *string `pulumi:"audio0,optional"`

	Disks []*Disk `pulumi:"disks"`

	// Net0 *string `pulumi:"net0,optional"`

	HostPCI0 *string `pulumi:"hostpci0,optional"`

	Serial0 *string `pulumi:"serial0,optional"`

	USB0 *string `pulumi:"usb0,optional"`

	Parallel0 *string `pulumi:"parallel0,optional"`

	CIType       *string `pulumi:"citype,optional"`
	CIUser       *string `pulumi:"ciuser,optional"`
	CIPassword   *string `pulumi:"cipassword,optional"`
	Nameserver   *string `pulumi:"nameserver,optional"`
	Searchdomain *string `pulumi:"searchdomain,optional"`
	SSHKeys      *string `pulumi:"sshkeys,optional"`
	CICustom     *string `pulumi:"cicustom,optional"`
	CIUpgrade    *int    `pulumi:"ciupgrade,optional"`

	IPConfig0 *string `pulumi:"ipconfig0,optional"`

	Clone *Clone `pulumi:"clone,optional"`
}

// Annotate adds descriptions to the VMInputs resource and its properties.
func (inputs *VMInputs) Annotate(a infer.Annotator) {
	a.Describe(
		inputs,
		"A Proxmox Virtual Machine (VM) resource that manages virtual machines in the Proxmox VE.",
	)
}

// VMOutputs represents the output state of a Proxmox virtual machine resource.
type VMOutputs struct {
	VMInputs
}
