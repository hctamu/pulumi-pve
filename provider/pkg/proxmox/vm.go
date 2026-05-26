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
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// VMOperations defines the interface for interacting with Proxmox VM resources.
// Methods are granular API primitives; orchestration logic belongs in the resource layer.
type VMOperations interface {
	// CreateVM creates a new (non-clone) virtual machine.
	// inputs.Node and inputs.VMID must already be set by the caller.
	CreateVM(ctx context.Context, inputs VMInputs) error

	// CloneVM clones a source VM to create a new virtual machine.
	// inputs.Clone, inputs.Node, and inputs.VMID must already be set by the caller.
	CloneVM(ctx context.Context, inputs VMInputs) error

	// ApplyConfig applies full configuration to an existing VM.
	ApplyConfig(ctx context.Context, vmID int, node *string, inputs VMInputs, timeout time.Duration) error

	// GetCurrentDisks retrieves the current disk configuration from a live VM.
	// Returns regular disks keyed by interface name and the EFI disk (nil if none).
	GetCurrentDisks(ctx context.Context, vmID int, node *string) (disks map[string]Disk, efiDisk *EfiDisk, err error)

	// ResizeDisk resizes a specific disk on a VM.
	ResizeDisk(ctx context.Context, vmID int, node *string, diskInterface string, sizeGB int) error

	// RemoveDisk unlinks/removes a specific disk from a VM.
	RemoveDisk(ctx context.Context, vmID int, node *string, diskInterface string) error

	// RemoveEfiDisk removes the EFI disk from a VM.
	RemoveEfiDisk(ctx context.Context, vmID int, node *string) error

	// Get retrieves the current state of a virtual machine from the API.
	// userDisks is used as an ordering hint so that the returned disk slice
	// follows the same order as the user's prior inputs.
	// Input preservation (clearing computed fields the user did not supply) is
	// the responsibility of the caller (resource layer).
	Get(
		ctx context.Context,
		vmID int,
		node *string,
		userDisks []*Disk,
	) (VMInputs, error)

	// UpdateConfig applies configuration changes to an existing virtual machine.
	UpdateConfig(ctx context.Context, vmID int, node *string, inputs VMInputs, stateInputs VMInputs) error

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
	a.Describe(&cpu, "CPU configuration for the virtual machine.")
	a.Describe(&cpu.Type, "CPU type (e.g., host, kvm64, x86-64-v2-AES).")
	a.Describe(&cpu.FlagsEnabled, "List of CPU flags to enable (e.g., pcid, spec-ctrl).")
	a.Describe(&cpu.FlagsDisabled, "List of CPU flags to disable.")
	a.Describe(&cpu.Hidden, "Hide VM CPU type from the guest operating system.")
	a.Describe(&cpu.HVVendorID, "Hyper-V vendor ID presented to the guest (up to 12 characters).")
	a.Describe(&cpu.PhysBits, "Number of physical address bits exposed to the guest (e.g., 36, 40, 48).")
	a.Describe(&cpu.Cores, "Number of CPU cores per socket.")
	a.Describe(&cpu.Sockets, "Number of CPU sockets.")
	a.Describe(&cpu.Limit, "CPU usage limit as a fraction of one core (e.g., 1.5 caps at 150%).")
	a.Describe(&cpu.Units, "CPU weight for the scheduler relative to other VMs (higher = more CPU time).")
	a.Describe(&cpu.Vcpus, "Number of hotplugged vCPUs (must be <= cores * sockets).")
	a.Describe(&cpu.Numa, "Enable NUMA topology.")
	a.Describe(&cpu.NumaNodes, "NUMA node topology configuration.")
	a.SetDefault(&cpu.Cores, 1, "Number of CPU cores")
}

// Annotate provides documentation for the NumaNode type.
func (numaNode *NumaNode) Annotate(a infer.Annotator) {
	a.Describe(&numaNode, "NUMA node topology configuration for the virtual machine.")
	a.Describe(&numaNode.Cpus, "CPUs (and optionally threads) assigned to this NUMA node (e.g., 0-3).")
	a.Describe(&numaNode.HostNodes, "Host NUMA nodes to map to this virtual NUMA node (e.g., 0-1).")
	a.Describe(&numaNode.Memory, "Memory in megabytes allocated to this NUMA node.")
	a.Describe(&numaNode.Policy, "NUMA memory allocation policy (preferred, bind, interleave, or mpol).")
}

// Clone represents the configuration for cloning a virtual machine.
type Clone struct {
	VMID        int     `pulumi:"vmId"`
	DataStoreID *string `pulumi:"dataStoreId,optional"`
	FullClone   *bool   `pulumi:"fullClone,optional"`
	NodeID      *string `pulumi:"node,optional"`
	Timeout     int     `pulumi:"timeout,optional"`
}

// Annotate provides documentation for the Clone type.
func (clone *Clone) Annotate(a infer.Annotator) {
	a.Describe(&clone, "Configuration for cloning a source virtual machine.")
	a.Describe(&clone.VMID, "Source VM ID to clone from.")
	a.Describe(&clone.DataStoreID, "Target storage pool for the cloned disks.")
	a.Describe(&clone.FullClone, "Create a full independent clone instead of a linked clone.")
	a.Describe(&clone.NodeID, "Target Proxmox node for the clone operation.")
	a.Describe(&clone.Timeout, "Timeout in seconds for the clone operation.")
}

// DiskBandwidth holds the I/O throttle limits.
// All fields are optional; omitting a field leaves the Proxmox default (no limit).
type DiskBandwidth struct {
	MBpsRd    *float64 `pulumi:"mbpsRd,optional"`    // Read limit in MB/s.
	MBpsRdMax *float64 `pulumi:"mbpsRdMax,optional"` // Read burst limit in MB/s.
	MBpsWr    *float64 `pulumi:"mbpsWr,optional"`    // Write limit in MB/s.
	MBpsWrMax *float64 `pulumi:"mbpsWrMax,optional"` // Write burst limit in MB/s.
	IOPSRd    *int     `pulumi:"iopsRd,optional"`    // Read limit in operations/s.
	IOPSRdMax *int     `pulumi:"iopsRdMax,optional"` // Read burst limit in operations/s.
	IOPSWr    *int     `pulumi:"iopsWr,optional"`    // Write limit in operations/s.
	IOPSWrMax *int     `pulumi:"iopsWrMax,optional"` // Write burst limit in operations/s.
}

// Annotate provides documentation for the DiskBandwidth type.
func (bw *DiskBandwidth) Annotate(a infer.Annotator) {
	a.Describe(&bw, "I/O throttle limits for the disk")
	a.Describe(&bw.MBpsRd, "Read throughput limit in MB/s (0 = unlimited).")
	a.Describe(&bw.MBpsRdMax, "Read burst throughput limit in MB/s; allows temporary bursts above MBpsRd.")
	a.Describe(&bw.MBpsWr, "Write throughput limit in MB/s (0 = unlimited).")
	a.Describe(&bw.MBpsWrMax, "Write burst throughput limit in MB/s; allows temporary bursts above MBpsWr.")
	a.Describe(&bw.IOPSRd, "Read I/O operations per second limit (0 = unlimited).")
	a.Describe(&bw.IOPSRdMax, "Read burst I/O operations per second limit.")
	a.Describe(&bw.IOPSWr, "Write I/O operations per second limit (0 = unlimited).")
	a.Describe(&bw.IOPSWrMax, "Write burst I/O operations per second limit.")
}

// DiskBase contains common fields shared between Disk and EfiDisk.
type DiskBase struct {
	Storage string  `pulumi:"storage"`
	FileID  *string `pulumi:"filename,optional"` // Optional, computed if not provided
}

// Disk represents a virtual machine disk configuration.
type Disk struct {
	DiskBase
	Size      int            `pulumi:"size"`               // Size in Gigabytes (required for regular disks).
	Interface string         `pulumi:"interface"`          // Disk interface: "scsi0", "ide1", "virtio", etc.
	Cache     *string        `pulumi:"cache,optional"`     // Cache mode: none, writethrough, writeback, unsafe, directsync
	Aio       *string        `pulumi:"aio,optional"`       // Async I/O mode: native, threads, io_uring.
	Discard   *string        `pulumi:"discard,optional"`   // Discard/TRIM: ignore, on.
	IOThread  *bool          `pulumi:"iothread,optional"`  // Enable per-disk I/O thread (virtio/scsi only).
	SSD       *bool          `pulumi:"ssd,optional"`       // Emulate SSD for the guest OS.
	Backup    *bool          `pulumi:"backup,optional"`    // Include disk in backups (Proxmox default: true).
	Replicate *bool          `pulumi:"replicate,optional"` // Include disk in replication (Proxmox default: true).
	ReadOnly  *bool          `pulumi:"ro,optional"`        // Mount disk read-only.
	Bandwidth *DiskBandwidth `pulumi:"bandwidth,optional"` // I/O throttle limits (Proxmox GUI "Bandwidth" section).
	Format    *string        `pulumi:"format,optional"`    // Disk image format: raw, qcow2, vmdk, etc.
	Serial    *string        `pulumi:"serial,optional"`    // Disk serial number exposed to the guest.
	WWN       *string        `pulumi:"wwn,optional"`       // World Wide Name (16 hex digits, e.g. 0x500a0000deadbeef).
	Media     *string        `pulumi:"media,optional"`     // Media type: disk or cdrom.
	Queues    *int           `pulumi:"queues,optional"`    // Number of I/O queues (virtio/scsi only).
	Snapshot  *bool          `pulumi:"snapshot,optional"`  // Disk is part of a snapshot chain (managed by Proxmox).
	Shared    *bool          `pulumi:"shared,optional"`    // Mark disk as shared across cluster nodes.
	RError    *string        `pulumi:"rerror,optional"`    // Read error policy: ignore, report, or stop.
	WError    *string        `pulumi:"werror,optional"`    // Write error policy: enospc, ignore, report, or stop.
	ScsiBlock *bool          `pulumi:"scsiblock,optional"` // Use scsi-block I/O path instead of virtio-scsi (scsi only).
}

// Annotate provides documentation for the Disk type.
func (disk *Disk) Annotate(a infer.Annotator) {
	a.Describe(&disk, "Disk configuration for the virtual machine.")
	a.Describe(&disk.Storage, "Target storage pool for the disk (e.g., local-lvm, ceph-pool).")
	a.Describe(&disk.FileID, "File name of the disk image (computed by Proxmox if not provided).")
	a.Describe(&disk.Size, "Disk size in gigabytes.")
	a.Describe(
		&disk.Interface,
		"Disk interface type and slot (e.g., scsi0, virtio0, ide1, sata2). "+
			"This field is the stable identity key for the disk: changing it is treated as "+
			"removing the old disk (permanently deleting the image) and adding a new empty disk. "+
			"To move data between slots, perform the migration manually in Proxmox.",
	)
	a.Describe(&disk.Cache, "Cache mode for the disk: none, writethrough, writeback, unsafe, or directsync. "+
		"Omit to use the Proxmox default (no explicit cache setting).")
	a.Describe(&disk.Aio, "Asynchronous I/O mode: native, threads, or io_uring. "+
		"Omit to use the Proxmox default.")
	a.Describe(&disk.Discard, "Discard/TRIM support: ignore (default) or on. "+
		"Enable for thin-provisioned storage and SSDs to reclaim freed blocks.")
	a.Describe(&disk.IOThread, "Enable a dedicated I/O thread for this disk. "+
		"Only supported on scsi and virtio interfaces.")
	a.Describe(&disk.SSD, "Emulate a solid-state drive for the guest OS (affects rotation rate hints). "+
		"Supported on ide, sata, and scsi interfaces; not valid for virtio.")
	a.Describe(&disk.Backup, "Include this disk in Proxmox backups. "+
		"Defaults to true when omitted; set to false to exclude the disk from backups.")
	a.Describe(&disk.Replicate, "Include this disk in Proxmox storage replication. "+
		"Defaults to true when omitted; set to false to exclude the disk from replication.")
	a.Describe(&disk.ReadOnly, "Mount this disk as read-only inside the guest. "+
		"Only supported on scsi and virtio interfaces.")
	a.Describe(&disk.Bandwidth, "I/O throttle limits for this disk (Proxmox GUI 'Bandwidth' section). "+
		"Omit to apply no throttling.")
	a.Describe(&disk.Format, "Disk image format: raw, qcow2, vmdk, etc. "+
		"Relevant primarily for file-based storage (local, NFS); block-based storage (LVM, Ceph) "+
		"ignores this field and may not return it on read. "+
		"Changing the format of an existing disk is not supported by Proxmox.")
	a.Describe(&disk.Serial, "Serial number string exposed to the guest OS. "+
		"Up to 60 characters; alphanumeric characters, hyphens, underscores, and dots are accepted. "+
		"Commas and equals signs are rejected by Proxmox. "+
		"Validated and enforced by the provider.")
	a.Describe(&disk.WWN, "World Wide Name (unique disk identifier). "+
		"Must be exactly 16 lowercase hex digits prefixed with '0x', e.g. 0x500a0000deadbeef. "+
		"Proxmox enforces the format with a regex; invalid values are rejected at apply time.")
	a.Describe(&disk.Media, "Media type: 'disk' (default) or 'cdrom'. "+
		"Supported on all disk interfaces.")
	a.Describe(&disk.Queues, "Number of I/O queues for this disk. "+
		"Only supported on scsi and virtio interfaces. "+
		"Minimum value is 2 (enforced by Proxmox); there is no enforced upper bound.")
	a.Describe(&disk.Snapshot, "Disk is part of a Proxmox snapshot chain. "+
		"This field is normally managed by Proxmox and should not be set manually.")
	a.Describe(&disk.Shared, "Mark this disk as shared across cluster nodes. "+
		"Required for live migration with local storage.")
	a.Describe(&disk.RError, "Action on read I/O errors: 'ignore', 'report', or 'stop'. "+
		"Proxmox default is 'report'. Supported on all disk interfaces.")
	a.Describe(&disk.WError, "Action on write I/O errors: 'enospc', 'ignore', 'report', or 'stop'. "+
		"Proxmox default is 'enospc'. Supported on all disk interfaces.")
	a.Describe(&disk.ScsiBlock, "Use the scsi-block I/O path instead of virtio-scsi. "+
		"Only supported on scsi interfaces. May improve performance for some workloads.")
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
	a.Describe(&efiDisk, "EFI disk configuration for the virtual machine.")
	a.Describe(&efiDisk.Storage, "Target storage pool for the EFI disk (e.g., local-lvm).")
	a.Describe(&efiDisk.FileID, "File name of the EFI disk image (computed by Proxmox if not provided).")
	a.Describe(&efiDisk.EfiType, "EFI firmware size: '2m' (2 MB, legacy) or '4m' (4 MB, supports Secure Boot).")
	a.Describe(&efiDisk.PreEnrolledKeys, "Pre-enroll Microsoft and standard UEFI keys into the EFI firmware.")
}

// ValidateEfiType checks if the EfiType is valid.
func (efiDisk EfiDisk) ValidateEfiType() error {
	switch efiDisk.EfiType {
	case EfiType2M, EfiType4M:
		return nil
	default:
		return fmt.Errorf("invalid EFI type: %v", efiDisk.EfiType)
	}
}

// DiskChangeType describes the kind of change detected between desired and current disk state.
type DiskChangeType int

const (
	// DiskUnchanged means no change detected. It is the zero value so a zero-value DiskChange is safe by default.
	DiskUnchanged DiskChangeType = iota
	// DiskAdded means the disk is present in desired but absent from current.
	DiskAdded
	// DiskRemoved means the disk is present in current but absent from desired.
	DiskRemoved
	// DiskResized means the disk size increased.
	DiskResized
	// DiskShrunk means the disk size decreased. Proxmox does not support shrinking.
	DiskShrunk
	// DiskStorageChanged means the disk was moved to a different storage pool.
	DiskStorageChanged
	// DiskFlagsChanged means one or more performance/management flag fields changed
	// (cache, aio, discard, iothread, ssd, backup, replicate, ro, bandwidth,
	// format, serial, wwn, media, queues, snapshot, shared, rerror, werror, scsiblock).
	// No direct Proxmox API call is required before UpdateConfig; the updated config
	// string is re-emitted by BuildVMOptionsDiff.
	DiskFlagsChanged
	// DiskFileIDChanged means both desired and current have a non-nil FileID but they differ.
	DiskFileIDChanged
)

// DiskChange describes a single detected change between desired and current disk state.
type DiskChange struct {
	// Interface is the Proxmox disk interface (e.g., "scsi0") — the stable identity key.
	Interface string
	// Type is the kind of change detected.
	Type DiskChangeType
	// Desired is the desired disk state (nil for DiskRemoved).
	Desired *Disk
	// Current is the current disk state (nil for DiskAdded).
	Current *Disk
}

// diskIfaceType returns the bus prefix of a Proxmox disk interface string.
// For example "scsi0" → "scsi", "virtio2" → "virtio", "ide3" → "ide", "sata1" → "sata".
// Returns an empty string for unrecognised values.
func diskIfaceType(iface string) string {
	for _, prefix := range []string{"scsi", "virtio", "sata", "ide"} {
		if len(iface) > len(prefix) && iface[:len(prefix)] == prefix {
			return prefix
		}
	}
	return ""
}

// ValidateDiskFlags returns an error if any flag on disk is incompatible with its
// interface type or violates a known Proxmox API constraint. Validated here so
// that `pulumi preview` fails fast rather than propagating to the API call.
//
// Interface constraints (Proxmox schema):
//   - iothread:   scsi and virtio only
//   - ro:         scsi and virtio only
//   - ssd:        ide, sata, and scsi only (not virtio)
//   - queues:     scsi and virtio only; minimum value 2
//   - scsiblock:  scsi only
//
// Field constraints (confirmed by Proxmox API integration tests):
//   - serial:     max 60 characters
func ValidateDiskFlags(disk *Disk) error {
	if disk == nil {
		return nil
	}
	iface := diskIfaceType(disk.Interface)

	if disk.IOThread != nil {
		if iface != "scsi" && iface != "virtio" {
			return fmt.Errorf(
				"disk %s: iothread is only supported on scsi and virtio interfaces",
				disk.Interface,
			)
		}
	}
	if disk.ReadOnly != nil {
		if iface != "scsi" && iface != "virtio" {
			return fmt.Errorf(
				"disk %s: ro (read-only) is only supported on scsi and virtio interfaces",
				disk.Interface,
			)
		}
	}
	if disk.SSD != nil {
		if iface == "virtio" {
			return fmt.Errorf(
				"disk %s: ssd emulation is not supported on virtio interfaces",
				disk.Interface,
			)
		}
	}
	if disk.Queues != nil {
		if iface != "scsi" && iface != "virtio" {
			return fmt.Errorf(
				"disk %s: queues is only supported on scsi and virtio interfaces",
				disk.Interface,
			)
		}
		if *disk.Queues < 2 {
			return fmt.Errorf(
				"disk %s: queues must be at least 2 (got %d)",
				disk.Interface, *disk.Queues,
			)
		}
	}
	if disk.ScsiBlock != nil {
		if iface != "scsi" {
			return fmt.Errorf(
				"disk %s: scsiblock is only supported on scsi interfaces",
				disk.Interface,
			)
		}
	}
	if disk.Serial != nil && len(*disk.Serial) > 60 {
		return fmt.Errorf(
			"disk %s: serial must be at most 60 characters (got %d)",
			disk.Interface, len(*disk.Serial),
		)
	}
	return nil
}

// CompareDisksByInterface compares desired and current disk lists keyed by Interface name
// and returns a DiskChange entry for every interface seen in either list.
//
// Priority order when multiple fields change on the same disk:
// StorageChanged > Resized/Shrunk > FileIDChanged > Unchanged
//
// FileID comparison rule: only compare FileID when desired.FileID != nil.
// A nil desired FileID means the user did not set it (it is computed by Proxmox) — not a change.
func CompareDisksByInterface(desired, current []*Disk) []DiskChange {
	desiredByIface := make(map[string]*Disk, len(desired))
	for _, d := range desired {
		if d != nil {
			desiredByIface[d.Interface] = d
		}
	}

	currentByIface := make(map[string]*Disk, len(current))
	for _, d := range current {
		if d != nil {
			currentByIface[d.Interface] = d
		}
	}

	var changes []DiskChange

	// Disks present in current but absent from desired → removed.
	for iface, cur := range currentByIface {
		if _, ok := desiredByIface[iface]; !ok {
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      DiskRemoved,
				Current:   cur,
			})
		}
	}

	// Disks present in desired — either added or compared against current.
	for iface, des := range desiredByIface {
		cur, exists := currentByIface[iface]
		if !exists {
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      DiskAdded,
				Desired:   des,
			})
			continue
		}

		// Storage change has highest priority.
		if des.Storage != cur.Storage {
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      DiskStorageChanged,
				Desired:   des,
				Current:   cur,
			})
			continue
		}

		// Size change is next.
		if des.Size != cur.Size {
			changeType := DiskResized
			if des.Size < cur.Size {
				changeType = DiskShrunk
			}
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      changeType,
				Desired:   des,
				Current:   cur,
			})
			continue
		}

		// Flag fields (cache, aio, discard, iothread, ssd, backup, replicate, ro).
		if diskFlagsChanged(des, cur) {
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      DiskFlagsChanged,
				Desired:   des,
				Current:   cur,
			})
			continue
		}

		// Compare FileID only when desired is non-nil (nil means "let Proxmox assign it").
		if des.FileID != nil && cur.FileID != nil && *des.FileID != *cur.FileID {
			changes = append(changes, DiskChange{
				Interface: iface,
				Type:      DiskFileIDChanged,
				Desired:   des,
				Current:   cur,
			})
			continue
		}

		changes = append(changes, DiskChange{
			Interface: iface,
			Type:      DiskUnchanged,
			Desired:   des,
			Current:   cur,
		})
	}

	return changes
}

// diskFlagsChanged reports whether any performance or data-management flag field
// differs between desired and current. nil is treated as "not set" (Proxmox default);
// a non-nil value means the user explicitly configured the option.
//
// Format is compared only when both sides are non-nil: Proxmox omits the format
// field for block-based storage (LVM, Ceph), so a nil current value means
// "not returned", not "changed to nil".
func diskFlagsChanged(des, cur *Disk) bool {
	boolDiff := func(a, b *bool) bool {
		if a == nil && b == nil {
			return false
		}
		if a == nil || b == nil {
			return true
		}
		return *a != *b
	}
	strDiff := func(a, b *string) bool {
		if a == nil && b == nil {
			return false
		}
		if a == nil || b == nil {
			return true
		}
		return *a != *b
	}
	intDiff := func(a, b *int) bool {
		if a == nil && b == nil {
			return false
		}
		if a == nil || b == nil {
			return true
		}
		return *a != *b
	}
	// Format: only compare when both sides are non-nil to avoid false diffs on
	// block storage that omits the format key in its config string.
	formatChanged := des.Format != nil && cur.Format != nil && *des.Format != *cur.Format
	return strDiff(des.Cache, cur.Cache) ||
		strDiff(des.Aio, cur.Aio) ||
		strDiff(des.Discard, cur.Discard) ||
		boolDiff(des.IOThread, cur.IOThread) ||
		boolDiff(des.SSD, cur.SSD) ||
		boolDiff(des.Backup, cur.Backup) ||
		boolDiff(des.Replicate, cur.Replicate) ||
		boolDiff(des.ReadOnly, cur.ReadOnly) ||
		formatChanged ||
		strDiff(des.Serial, cur.Serial) ||
		strDiff(des.WWN, cur.WWN) ||
		strDiff(des.Media, cur.Media) ||
		intDiff(des.Queues, cur.Queues) ||
		boolDiff(des.Snapshot, cur.Snapshot) ||
		boolDiff(des.Shared, cur.Shared) ||
		strDiff(des.RError, cur.RError) ||
		strDiff(des.WError, cur.WError) ||
		boolDiff(des.ScsiBlock, cur.ScsiBlock) ||
		bandwidthChanged(des.Bandwidth, cur.Bandwidth)
}

// bandwidthChanged reports whether any DiskBandwidth field differs between desired and current.
// nil bandwidth and a zero-value DiskBandwidth (all fields nil) are treated as equivalent.
func bandwidthChanged(des, cur *DiskBandwidth) bool {
	f64Diff := func(a, b *float64) bool {
		if a == nil && b == nil {
			return false
		}
		if a == nil || b == nil {
			return true
		}
		return *a != *b
	}
	intDiff := func(a, b *int) bool {
		if a == nil && b == nil {
			return false
		}
		if a == nil || b == nil {
			return true
		}
		return *a != *b
	}
	// Treat nil as empty struct for comparison purposes.
	var d, c DiskBandwidth
	if des != nil {
		d = *des
	}
	if cur != nil {
		c = *cur
	}
	return f64Diff(d.MBpsRd, c.MBpsRd) ||
		f64Diff(d.MBpsRdMax, c.MBpsRdMax) ||
		f64Diff(d.MBpsWr, c.MBpsWr) ||
		f64Diff(d.MBpsWrMax, c.MBpsWrMax) ||
		intDiff(d.IOPSRd, c.IOPSRd) ||
		intDiff(d.IOPSRdMax, c.IOPSRdMax) ||
		intDiff(d.IOPSWr, c.IOPSWr) ||
		intDiff(d.IOPSWrMax, c.IOPSWrMax)
}

// VMInputs represents the input configuration for a virtual machine.
type VMInputs struct {
	Name        string   `pulumi:"name"`
	Description *string  `pulumi:"description,optional"`
	Node        *string  `pulumi:"node,optional"`
	VMID        *int     `pulumi:"vmId,optional"        provider:"replaceOnChanges"`
	Hotplug     *string  `pulumi:"hotplug,optional"`
	Template    *int     `pulumi:"template,optional"`
	Autostart   *int     `pulumi:"autostart,optional"`
	Tags        []string `pulumi:"tags,optional"`
	OSType      *string  `pulumi:"ostype,optional"`
	Machine     *string  `pulumi:"machine,optional"`
	EfiDisk     *EfiDisk `pulumi:"efidisk,optional"`
	CPU         *CPU     `pulumi:"cpu,optional"`
	Memory      *int     `pulumi:"memory,optional"`
	Balloon     *int     `pulumi:"balloon,optional"`
	Disks       []*Disk  `pulumi:"disks"`
	Clone       *Clone   `pulumi:"clone,optional"`
}

// Annotate adds descriptions to the VMInputs resource and its properties.
func (inputs *VMInputs) Annotate(a infer.Annotator) {
	a.Describe(inputs, "A Proxmox Virtual Machine (VM) resource that manages virtual machines in the Proxmox VE.")
	a.Describe(&inputs.Name, "Name of the virtual machine.")
	a.Describe(&inputs.Description, "Description or notes for the virtual machine.")
	a.Describe(&inputs.Node, "Proxmox node where the VM resides.")
	a.Describe(&inputs.VMID, "Unique numeric identifier for the virtual machine (auto-assigned if omitted).")
	a.Describe(&inputs.Hotplug, "Comma-separated list of hotplug features (network, disk, cpu, memory, usb).")
	a.Describe(&inputs.Template, "Mark the VM as a template (1) or a regular VM (0).")
	a.Describe(&inputs.Autostart, "Automatically start the VM when the host boots (1 to enable, 0 to disable).")
	a.Describe(&inputs.Tags, "Tags associated with the virtual machine.")
	a.Describe(&inputs.OSType, "Guest operating system type (e.g., l26, win11, other).")
	a.Describe(&inputs.Machine, "Machine type for the VM (e.g., pc, q35, pc-i440fx-8.1).")
	a.Describe(&inputs.EfiDisk, "EFI disk configuration (required when bios is set to ovmf).")
	a.Describe(&inputs.CPU, "CPU configuration including type, topology, and feature flags.")
	a.Describe(&inputs.Memory, "Memory size in megabytes.")
	a.Describe(&inputs.Balloon, "Minimum memory for ballooning in megabytes (0 disables the balloon device).")
	a.Describe(
		&inputs.Disks,
		"List of disk configurations attached to the virtual machine. "+
			"Each disk is identified by its interface slot (e.g., scsi0). "+
			"Disks can be added or removed freely, and sizes can only be increased. "+
			"Changing the interface field of an existing disk is data-destructive: "+
			"the old disk image is permanently deleted and a new empty disk is provisioned.",
	)
	a.Describe(&inputs.Clone, "Clone configuration for creating the VM from a source template or VM.")
}

// VMOutputs represents the output state of a Proxmox virtual machine resource.
type VMOutputs struct {
	VMInputs
}
