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

// Package vm provides virtual machine resource management for Proxmox VE.
package vm

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources/utils"
	api "github.com/luthermonson/go-proxmox"
	"golang.org/x/exp/slices"
)

const (
	efiDiskID = "efidisk0"

	// Efi disk size is constant because it is ignored by the API anyway
	efiDiskSize = "1"

	efiDiskInputName = "efidisk"
)

// NumaNode represents a single NUMA node topology configuration.
type NumaNode struct {
	Cpus      string  `pulumi:"cpus"`
	HostNodes *string `pulumi:"hostNodes,optional"`
	Memory    *int    `pulumi:"memory,optional"`
	Policy    *string `pulumi:"policy,optional"`
}

// Cpu represents the structured CPU configuration.
type Cpu struct {
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
	// TODO: Affinity is currently buggy in Proxmox VE - requires root permissions and has permission issues
	// Affinity      *string  `pulumi:"affinity,optional"`
}

// ToProxmoxNumaString converts a NumaNode to Proxmox format.
func (n *NumaNode) ToProxmoxNumaString() string {
	parts := make([]string, 0, 4)
	parts = append(parts, "cpus="+n.Cpus)
	if n.HostNodes != nil {
		parts = append(parts, "hostnodes="+*n.HostNodes)
	}
	if n.Memory != nil {
		parts = append(parts, fmt.Sprintf("memory=%d", *n.Memory))
	}
	if n.Policy != nil {
		parts = append(parts, "policy="+*n.Policy)
	}
	return strings.Join(parts, ",")
}

// ParseNumaNode parses a Proxmox NUMA node config string.
func ParseNumaNode(value string) (node *NumaNode, err error) {
	if value == "" {
		return nil, nil
	}
	node = &NumaNode{}
	segments := strings.Split(value, ",")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "cpus":
			node.Cpus = val
		case "hostnodes":
			node.HostNodes = &val
		case "memory":
			mem, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("invalid memory value '%s': %w", val, err)
			}
			node.Memory = &mem
		case "policy":
			node.Policy = &val
		}
	}
	if node.Cpus == "" {
		return nil, fmt.Errorf("NUMA node missing required 'cpus' field")
	}
	return node, nil
}

// ToProxmoxString converts the Cpu config to Proxmox format.
func (c *Cpu) ToProxmoxString() string {
	if c == nil {
		return ""
	}
	parts := make([]string, 0, 6)
	if c.Type != nil && *c.Type != "" {
		parts = append(parts, *c.Type)
	}
	if len(c.FlagsEnabled) > 0 || len(c.FlagsDisabled) > 0 {
		flags := make([]string, 0, len(c.FlagsEnabled)+len(c.FlagsDisabled))
		for _, f := range c.FlagsEnabled {
			if f == "" {
				continue
			}
			flags = append(flags, "+"+f)
		}
		for _, f := range c.FlagsDisabled {
			if f == "" {
				continue
			}
			flags = append(flags, "-"+f)
		}
		if len(flags) > 0 {
			parts = append(parts, "flags="+strings.Join(flags, ";"))
		}
	}
	if c.Hidden != nil {
		if *c.Hidden {
			parts = append(parts, "hidden=1")
		} else {
			parts = append(parts, "hidden=0")
		}
	}
	if c.HVVendorID != nil {
		parts = append(parts, "hv-vendor-id="+*c.HVVendorID)
	}
	if c.PhysBits != nil {
		parts = append(parts, "phys-bits="+*c.PhysBits)
	}
	return strings.Join(parts, ",")
}

// ParseCpu parses a Proxmox CPU config string into Cpu.
func ParseCpu(value string) (cfg *Cpu, err error) {
	if value == "" {
		return nil, nil
	}
	cfg = &Cpu{}
	segments := strings.Split(value, ",")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			// First segment without '=' is the CPU type
			if i == 0 {
				cfg.Type = &seg
			}
			continue
		}
		key, val := kv[0], kv[1]
		switch key {
		case "cputype":
			cfg.Type = &val
		case "flags":
			if val != "" {
				flags := strings.Split(val, ";")
				for _, f := range flags {
					if f == "" {
						continue
					}
					switch f[0] {
					case '+':
						cfg.FlagsEnabled = append(cfg.FlagsEnabled, f[1:])
					case '-':
						cfg.FlagsDisabled = append(cfg.FlagsDisabled, f[1:])
					default:
						cfg.FlagsEnabled = append(cfg.FlagsEnabled, f)
					}
				}
			}
		case "hidden":
			if val == "1" {
				b := true
				cfg.Hidden = &b
			} else if val == "0" {
				b := false
				cfg.Hidden = &b
			}
		case "hv-vendor-id":
			cfg.HVVendorID = &val
		case "phys-bits":
			cfg.PhysBits = &val
			// other keys ignored
		}
	}
	return cfg, nil
}

// Inputs represents the input configuration for a virtual machine.
type Inputs struct {
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

	Cpu       *Cpu    `pulumi:"cpu,optional"`
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

// ValidateEfiType checks if the EfiType is valid.
func (efi EfiDisk) ValidateEfiType() error {
	switch efi.EfiType {
	case EfiType2M, EfiType4M:
		return nil
	default:
		return fmt.Errorf("invalid EFI type: %v", efi.EfiType)
	}
}

// ToProxmoxEfiDiskConfig converts the EfiDisk struct to Proxmox EFI disk config string.
func (efi EfiDisk) ToProxmoxEfiDiskConfig() string {
	var fullDiskPath string
	if efi.FileID == nil || *efi.FileID == "" {
		// No file Id means we are creating the disk now, so we use the storage:size format to create the disk
		fullDiskPath = fmt.Sprintf("%v:%v", efi.Storage, efiDiskSize)
	} else {
		// We already have a disk file, so we use the storage:file_id format
		fullDiskPath = fmt.Sprintf("%v:%v", efi.Storage, *efi.FileID)
	}

	config := fmt.Sprintf("file=%v", fullDiskPath)
	if efi.EfiType != "" {
		config += fmt.Sprintf(",efitype=%v", efi.EfiType)
	}
	if efi.PreEnrolledKeys != nil {
		if *efi.PreEnrolledKeys {
			config += ",pre-enrolled-keys=1"
		} else {
			config += ",pre-enrolled-keys=0"
		}
	}
	return config
}

// parsedDiskBase contains the result of parsing common disk configuration.
type parsedDiskBase struct {
	DiskBase
	Size   *int              // Size is optional for EFI disks but required for regular disks
	Extras map[string]string // Additional key-value pairs not in diskBase
}

// parseDiskBase parses common disk configuration fields shared by Disk and EfiDisk.
func parseDiskBase(diskConfig string) (parsedDiskBase, error) {
	result := parsedDiskBase{
		Extras: make(map[string]string),
	}

	parts := strings.Split(diskConfig, ",")

	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			// Handle storage:fileID format (no key)
			if strings.Contains(kv[0], ":") {
				diskFile := strings.Split(kv[0], ":")
				result.Storage = diskFile[0]
				if len(diskFile) > 1 {
					fileID := diskFile[1]
					result.FileID = &fileID
				}
			}
			continue
		}

		key, value := kv[0], kv[1]
		switch key {
		case "file":
			diskFile := strings.Split(value, ":")
			result.Storage = diskFile[0]
			if len(diskFile) > 1 {
				fileID := diskFile[1]
				result.FileID = &fileID
			}
		case "size":
			size, err := parseDiskSize(value)
			if err != nil {
				return parsedDiskBase{}, err
			}
			result.Size = &size
		default:
			// Store any other key-value pairs
			result.Extras[key] = value
		}
	}

	if result.Storage == "" {
		return parsedDiskBase{}, fmt.Errorf("failed to parse disk config: missing storage in %s", diskConfig)
	}

	return result, nil
}

// ConvertVMConfigToInputs converts a VirtualMachine configuration to Args.
func ConvertVMConfigToInputs(vm *api.VirtualMachine, currentInput Inputs) (inputs Inputs, err error) {
	vmConfig := vm.VirtualMachineConfig
	diskMap := vmConfig.MergeDisks()

	var parsedCPU *Cpu
	if vmConfig.CPU != "" {
		cpuCfg, parseErr := ParseCpu(vmConfig.CPU)
		if parseErr != nil {
			err = fmt.Errorf("failed to parse CPU config '%s': %w", vmConfig.CPU, parseErr)
			return
		}
		parsedCPU = cpuCfg
	}

	if parsedCPU == nil {
		parsedCPU = &Cpu{}
	}

	if vmConfig.Cores > 0 {
		c := vmConfig.Cores
		parsedCPU.Cores = &c
	}
	if vmConfig.Sockets > 0 {
		s := vmConfig.Sockets
		parsedCPU.Sockets = &s
	}
	if vmConfig.CPULimit > 0 {
		limit := float64(vmConfig.CPULimit)
		parsedCPU.Limit = &limit
	}
	if vmConfig.CPUUnits > 0 {
		parsedCPU.Units = &vmConfig.CPUUnits
	}
	if vmConfig.Vcpus > 0 {
		parsedCPU.Vcpus = &vmConfig.Vcpus
	}
	// TODO: Affinity is currently buggy in Proxmox VE
	// if vmConfig.Affinity != "" {
	// 	parsedCPU.Affinity = &vmConfig.Affinity
	// }

	if vmConfig.Numa > 0 {
		numaEnabled := vmConfig.Numa > 0
		parsedCPU.Numa = &numaEnabled
	}

	numaStrings := []string{
		vmConfig.Numa0, vmConfig.Numa1, vmConfig.Numa2, vmConfig.Numa3, vmConfig.Numa4,
		vmConfig.Numa5, vmConfig.Numa6, vmConfig.Numa7, vmConfig.Numa8, vmConfig.Numa9,
	}
	var numaNodes []NumaNode
	for _, numaStr := range numaStrings {
		if numaStr != "" {
			node, parseErr := ParseNumaNode(numaStr)
			if parseErr != nil {
				err = fmt.Errorf("failed to parse NUMA node '%s': %w", numaStr, parseErr)
				return
			}
			if node != nil {
				numaNodes = append(numaNodes, *node)
			}
		}
	}
	if len(numaNodes) > 0 {
		parsedCPU.NumaNodes = numaNodes
	}

	// Sort disk interfaces to ensure consistent ordering
	disks := []*Disk{}
	var checkedDisks []string

	for _, currentDisk := range currentInput.Disks {
		// check if current input disk is in the read config
		if _, exists := diskMap[currentDisk.Interface]; exists {
			disk := &Disk{Interface: currentDisk.Interface}
			checkedDisks = append(checkedDisks, currentDisk.Interface)
			if err := disk.ParseDiskConfig(diskMap[currentDisk.Interface]); err != nil {
				return Inputs{}, err
			}
			disks = append(disks, disk)
		}
	}

	for diskInterface, diskParams := range diskMap {
		if slices.Contains(checkedDisks, diskInterface) {
			continue
		}
		disk := Disk{Interface: diskInterface}
		if err := disk.ParseDiskConfig(diskParams); err != nil {
			return Inputs{}, err
		}
		disks = append(disks, &disk)
	}

	var efiDisk *EfiDisk
	if vmConfig.EFIDisk0 != "" {
		efiDisk = &EfiDisk{}
		if err := efiDisk.ParseEfiDiskConfig(vmConfig.EFIDisk0); err != nil {
			return Inputs{}, err
		}
	}

	var vmID int
	if vm.VMID > math.MaxInt {
		return Inputs{}, fmt.Errorf("VMID %d overflows int", vm.VMID)
	}
	vmID = int(vm.VMID) // #nosec G115 - overflow checked above

	return Inputs{
		Name:        strOrNil(vmConfig.Name),
		Description: strOrNil(vmConfig.Description),
		VMID:        &vmID,
		Hookscript:  strOrNil(vmConfig.Hookscript),
		Hotplug:     strOrNil(vmConfig.Hotplug),
		Template:    intOrNil(vmConfig.Template),
		Autostart:   intOrNil(vmConfig.Autostart),
		Tablet:      intOrNil(vmConfig.Tablet),
		KVM:         intOrNil(vmConfig.KVM),
		Protection:  intOrNil(vmConfig.Protection),
		Lock:        strOrNil(vmConfig.Lock),

		EfiDisk: efiDisk,

		// Boot:   strOrNil(vmConfig.Boot),
		// OnBoot: intOrNil(vmConfig.OnBoot),

		OSType:  strOrNil(vmConfig.OSType),
		Machine: strOrNil(vmConfig.Machine),
		Bio:     strOrNil(vmConfig.Bios),

		Acpi: intOrNil(vmConfig.Acpi),

		Cpu:       parsedCPU,
		Memory:    intOrNil(int(vmConfig.Memory)), // MB (no conversion)
		Hugepages: strOrNil(vmConfig.Hugepages),
		Balloon:   intOrNil(vmConfig.Balloon),

		VGA: strOrNil(vmConfig.VGA),
		// SCSIHW:    strOrNil(vmConfig.SCSIHW),
		TPMState0: strOrNil(vmConfig.TPMState0),
		Rng0:      strOrNil(vmConfig.Rng0),
		Audio0:    strOrNil(vmConfig.Audio0),

		Disks: disks,

		// Net0: strOrNil(vmConfig.Net0),

		HostPCI0: strOrNil(vmConfig.HostPCI0),

		Serial0: strOrNil(vmConfig.Serial0),

		USB0: strOrNil(vmConfig.USB0),

		Parallel0: strOrNil(vmConfig.Parallel0),

		CIType:       strOrNil(vmConfig.CIType),
		CIUser:       strOrNil(vmConfig.CIUser),
		CIPassword:   strOrNil(vmConfig.CIPassword),
		Nameserver:   strOrNil(vmConfig.Nameserver),
		Searchdomain: strOrNil(vmConfig.Searchdomain),
		SSHKeys:      strOrNil(vmConfig.SSHKeys),
		CICustom:     strOrNil(vmConfig.CICustom),
		CIUpgrade:    intOrNil(vmConfig.CIUpgrade),

		IPConfig0: strOrNil(vmConfig.IPConfig0),
		Node:      strOrNil(vm.Node),
	}, nil
}

// BuildOptionsDiff builds a list of VirtualMachineOption that represent the differences between the
// current and new Args.
func (inputs *Inputs) BuildOptionsDiff(
	vmID int,
	currentInputs *Inputs,
) (options []api.VirtualMachineOption) {
	// Memory already stored in MB; no conversion required.
	compareAndAddOption("name", &options, inputs.Name, currentInputs.Name)
	compareAndAddOption("memory", &options, inputs.Memory, currentInputs.Memory)
	compareAndAddOption("description", &options, inputs.Description, currentInputs.Description)
	compareAndAddOption("autostart", &options, inputs.Autostart, currentInputs.Autostart)
	compareAndAddOption("protection", &options, inputs.Protection, currentInputs.Protection)
	compareAndAddOption("lock", &options, inputs.Lock, currentInputs.Lock)
	compareAndAddOption("hugepages", &options, inputs.Hugepages, currentInputs.Hugepages)
	compareAndAddOption("balloon", &options, inputs.Balloon, currentInputs.Balloon)
	compareAndAddOption("vga", &options, inputs.VGA, currentInputs.VGA)
	compareAndAddOption("ostype", &options, inputs.OSType, currentInputs.OSType)
	compareAndAddOption("sshkeys", &options, inputs.SSHKeys, currentInputs.SSHKeys)
	compareAndAddOption("cicustom", &options, inputs.CICustom, currentInputs.CICustom)
	compareAndAddOption("ciupgrade", &options, inputs.CIUpgrade, currentInputs.CIUpgrade)
	addCpuDiff(&options, inputs, currentInputs)
	// Handle EFI disk changes
	if !reflect.DeepEqual(inputs.EfiDisk, currentInputs.EfiDisk) {
		if inputs.EfiDisk != nil {
			efiConfig := inputs.EfiDisk.ToProxmoxEfiDiskConfig()
			options = append(options, api.VirtualMachineOption{Name: "efidisk0", Value: efiConfig})
		}
	}

	if !slices.Equal(inputs.Disks, currentInputs.Disks) {
		for _, disk := range inputs.Disks {
			diskKey, diskConfig := disk.ToProxmoxDiskKeyConfig()
			options = append(options, api.VirtualMachineOption{Name: diskKey, Value: diskConfig})
		}
	}

	return options
}

// BuildOptions builds a list of VirtualMachineOption from the Inputs.
func (inputs *Inputs) BuildOptions(vmID int) (options []api.VirtualMachineOption) {
	// Memory already stored in MB; no conversion required.

	addOption("name", &options, inputs.Name)
	addOption("memory", &options, inputs.Memory)
	addOption("description", &options, inputs.Description)
	addOption("autostart", &options, inputs.Autostart)
	addOption("protection", &options, inputs.Protection)
	addOption("lock", &options, inputs.Lock)
	addOption("hugepages", &options, inputs.Hugepages)
	addOption("balloon", &options, inputs.Balloon)
	addOption("vga", &options, inputs.VGA)
	addOption("ostype", &options, inputs.OSType)
	addOption("citype", &options, inputs.CIType)
	addOption("ciuser", &options, inputs.CIUser)
	addOption("cipassword", &options, inputs.CIPassword)
	addOption("nameserver", &options, inputs.Nameserver)
	addOption("searchdomain", &options, inputs.Searchdomain)
	addOption("sshkeys", &options, inputs.SSHKeys)
	addOption("cicustom", &options, inputs.CICustom)
	addOption("ciupgrade", &options, inputs.CIUpgrade)
	if inputs.Cpu != nil {
		cpuStr := inputs.Cpu.ToProxmoxString()
		if cpuStr != "" {
			options = append(options, api.VirtualMachineOption{Name: "cpu", Value: cpuStr})
		}
		if inputs.Cpu.Cores != nil {
			options = append(options, api.VirtualMachineOption{Name: "cores", Value: inputs.Cpu.Cores})
		}
		if inputs.Cpu.Sockets != nil {
			options = append(options, api.VirtualMachineOption{Name: "sockets", Value: inputs.Cpu.Sockets})
		}
		if inputs.Cpu.Limit != nil {
			options = append(options, api.VirtualMachineOption{Name: "cpulimit", Value: inputs.Cpu.Limit})
		}
		if inputs.Cpu.Units != nil {
			options = append(options, api.VirtualMachineOption{Name: "cpuunits", Value: inputs.Cpu.Units})
		}
		if inputs.Cpu.Vcpus != nil {
			options = append(options, api.VirtualMachineOption{Name: "vcpus", Value: inputs.Cpu.Vcpus})
		}
		if inputs.Cpu.Numa != nil {
			numaValue := 0
			if *inputs.Cpu.Numa {
				numaValue = 1
			}
			options = append(options, api.VirtualMachineOption{Name: "numa", Value: numaValue})
		}
		for i, node := range inputs.Cpu.NumaNodes {
			numaKey := fmt.Sprintf("numa%d", i)
			numaValue := node.ToProxmoxNumaString()
			options = append(options, api.VirtualMachineOption{Name: numaKey, Value: numaValue})
		}
		// TODO: Affinity is currently buggy in Proxmox VE
		// if inputs.Cpu.Affinity != nil {
		// 	options = append(options, api.VirtualMachineOption{Name: "affinity", Value: inputs.Cpu.Affinity})
		// }
	}

	// Add EFI disk if configured
	if inputs.EfiDisk != nil {
		efiConfig := inputs.EfiDisk.ToProxmoxEfiDiskConfig()
		options = append(options, api.VirtualMachineOption{Name: efiDiskID, Value: efiConfig})
	}

	for _, disk := range inputs.Disks {
		diskKey, diskConfig := disk.ToProxmoxDiskKeyConfig()
		options = append(options, api.VirtualMachineOption{Name: diskKey, Value: diskConfig})
	}

	return options
}

// addCpuDiff appends VirtualMachineOption entries for cpu string, cores, and sockets when they differ.
func addCpuDiff(options *[]api.VirtualMachineOption, newInputs, currentInputs *Inputs) {
	if newInputs.Cpu != nil || currentInputs.Cpu != nil {
		// Diff CPU string
		var newCPU, oldCPU string
		if newInputs.Cpu != nil {
			newCPU = newInputs.Cpu.ToProxmoxString()
		}
		if currentInputs.Cpu != nil {
			oldCPU = currentInputs.Cpu.ToProxmoxString()
		}
		if newCPU != oldCPU && newCPU != "" {
			*options = append(*options, api.VirtualMachineOption{Name: "cpu", Value: newCPU})
		}

		// Diff cores
		var newCores, oldCores *int
		if newInputs.Cpu != nil {
			newCores = newInputs.Cpu.Cores
		}
		if currentInputs.Cpu != nil {
			oldCores = currentInputs.Cpu.Cores
		}
		if utils.DifferPtr(newCores, oldCores) {
			if newCores != nil { // skip clear operations
				*options = append(*options, api.VirtualMachineOption{Name: "cores", Value: newCores})
			}
		}

		// Diff sockets
		var newSockets, oldSockets *int
		if newInputs.Cpu != nil {
			newSockets = newInputs.Cpu.Sockets
		}
		if currentInputs.Cpu != nil {
			oldSockets = currentInputs.Cpu.Sockets
		}
		if utils.DifferPtr(newSockets, oldSockets) {
			if newSockets != nil {
				*options = append(*options, api.VirtualMachineOption{Name: "sockets", Value: newSockets})
			}
		}

		// Diff cpulimit
		var newLimit, oldLimit *float64
		if newInputs.Cpu != nil {
			newLimit = newInputs.Cpu.Limit
		}
		if currentInputs.Cpu != nil {
			oldLimit = currentInputs.Cpu.Limit
		}
		if utils.DifferPtr(newLimit, oldLimit) {
			if newLimit != nil {
				*options = append(*options, api.VirtualMachineOption{Name: "cpulimit", Value: newLimit})
			}
		}

		// Diff cpuunits
		var newUnits, oldUnits *int
		if newInputs.Cpu != nil {
			newUnits = newInputs.Cpu.Units
		}
		if currentInputs.Cpu != nil {
			oldUnits = currentInputs.Cpu.Units
		}
		if utils.DifferPtr(newUnits, oldUnits) {
			if newUnits != nil {
				*options = append(*options, api.VirtualMachineOption{Name: "cpuunits", Value: newUnits})
			}
		}

		// Diff vcpus
		var newVcpus, oldVcpus *int
		if newInputs.Cpu != nil {
			newVcpus = newInputs.Cpu.Vcpus
		}
		if currentInputs.Cpu != nil {
			oldVcpus = currentInputs.Cpu.Vcpus
		}
		if utils.DifferPtr(newVcpus, oldVcpus) {
			if newVcpus != nil {
				*options = append(*options, api.VirtualMachineOption{Name: "vcpus", Value: newVcpus})
			}
		}

		// Diff NUMA enabled
		var newNuma, oldNuma *bool
		if newInputs.Cpu != nil {
			newNuma = newInputs.Cpu.Numa
		}
		if currentInputs.Cpu != nil {
			oldNuma = currentInputs.Cpu.Numa
		}
		if utils.DifferPtr(newNuma, oldNuma) {
			if newNuma != nil {
				numaValue := 0
				if *newNuma {
					numaValue = 1
				}
				*options = append(*options, api.VirtualMachineOption{Name: "numa", Value: numaValue})
			}
		}

		// Diff NUMA nodes
		var newNodes, oldNodes []NumaNode
		if newInputs.Cpu != nil {
			newNodes = newInputs.Cpu.NumaNodes
		}
		if currentInputs.Cpu != nil {
			oldNodes = currentInputs.Cpu.NumaNodes
		}
		if !numaNodesEqual(newNodes, oldNodes) {
			for i, node := range newNodes {
				numaKey := fmt.Sprintf("numa%d", i)
				numaValue := node.ToProxmoxNumaString()
				*options = append(*options, api.VirtualMachineOption{Name: numaKey, Value: numaValue})
			}
		}

		// TODO: Affinity is currently buggy in Proxmox VE
		// var newAffinity, oldAffinity *string
		// if newInputs.Cpu != nil {
		// 	newAffinity = newInputs.Cpu.Affinity
		// }
		// if currentInputs.Cpu != nil {
		// 	oldAffinity = currentInputs.Cpu.Affinity
		// }
		// if utils.DifferPtr(newAffinity, oldAffinity) {
		// 	if newAffinity != nil {
		// 		*options = append(*options, api.VirtualMachineOption{Name: "affinity", Value: newAffinity})
		// 	}
		// }
	}
}

// numaNodesEqual checks if two NumaNode slices are equal.
func numaNodesEqual(a, b []NumaNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Cpus != b[i].Cpus {
			return false
		}
		if !ptrEqual(a[i].HostNodes, b[i].HostNodes) {
			return false
		}
		if !ptrEqual(a[i].Memory, b[i].Memory) {
			return false
		}
		if !ptrEqual(a[i].Policy, b[i].Policy) {
			return false
		}
	}
	return true
}

// ptrEqual compares two pointers of any comparable type.
func ptrEqual[T comparable](a, b *T) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// getDiskOption returns the disk option with the specified interface.
func getDiskOption(
	options []api.VirtualMachineOption,
	diskInterface string,
) (diskOption *api.VirtualMachineOption) {
	for index := range options {
		if options[index].Name == diskInterface {
			diskOption = &options[index]
			return diskOption
		}
	}
	return nil
}

// strOrNil returns a pointer to the string value if it is not empty, otherwise returns nil.
func strOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// intOrNil returns a pointer to the int value if it is not zero, otherwise returns nil.
func intOrNil(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

// ParseDiskConfig parses the disk configuration string and sets the Disk fields accordingly.
func (disk *Disk) ParseDiskConfig(diskConfig string) error {
	parsed, err := parseDiskBase(diskConfig)
	if err != nil {
		return err
	}

	// Size is required for regular disks
	if parsed.Size == nil {
		return fmt.Errorf("size is required for disk: %s", diskConfig)
	}

	disk.DiskBase = parsed.DiskBase
	disk.Size = *parsed.Size
	return nil
}

// ParseEfiDiskConfig parses the EFI disk configuration string and sets the EfiDisk fields.
func (efi *EfiDisk) ParseEfiDiskConfig(diskConfig string) error {
	parsed, err := parseDiskBase(diskConfig)
	if err != nil {
		return err
	}

	efi.DiskBase = parsed.DiskBase

	// Extract efitype if present in extras
	if efitype, ok := parsed.Extras["efitype"]; ok {
		efi.EfiType = EfiType(efitype)
	}

	// Extract pre-enrolled-keys if present in extras
	if preEnrolledKeys, ok := parsed.Extras["pre-enrolled-keys"]; ok {
		value := preEnrolledKeys == "1"
		efi.PreEnrolledKeys = &value
	}

	return nil
}

// parseDiskSize parses the disk size string and returns the size in gigabytes.
func parseDiskSize(value string) (size int, err error) {
	if utils.EndsWithLetter(value) {
		unit := value[len(value)-1]
		size, err = strconv.Atoi(value[:len(value)-1])
		if err != nil {
			return 0, fmt.Errorf("failed to parse disk size: %v", value)
		}

		switch string(unit) {
		case "G", "g":
			return size, nil
		case "M", "m":
			return size / 1024, nil
		case "T", "t":
			return size * 1024, nil
		default:
			return 0, fmt.Errorf("unknown size unit: %v", unit)
		}
	}

	return strconv.Atoi(value)
}

// compareAndAddOption compares the new value with the current value and adds the option if they differ.
func compareAndAddOption[T comparable](
	name string,
	options *[]api.VirtualMachineOption,
	newValue, currentValue *T,
) {
	if utils.DifferPtr(newValue, currentValue) {
		// Only add option if newValue is not nil - we don't try to "clear" fields
		// by sending nil or empty values as this can cause validation errors
		if newValue != nil {
			*options = append(*options, api.VirtualMachineOption{Name: name, Value: newValue})
		}
	}
}

// addOption adds the option to the list if the value is not nil.
func addOption[T comparable](name string, options *[]api.VirtualMachineOption, value *T) {
	if value != nil {
		*options = append(*options, api.VirtualMachineOption{Name: name, Value: value})
	}
}
