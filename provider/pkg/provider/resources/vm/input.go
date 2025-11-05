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
	"strconv"
	"strings"

	"github.com/hctamu/pulumi-pve/provider/pkg/provider/resources"
	api "github.com/luthermonson/go-proxmox"
	"golang.org/x/exp/slices"
)

// Disk represents a virtual machine disk configuration.
type Disk struct {
	Storage   string `pulumi:"storage"`
	Size      int    `pulumi:"size"`      // Size in Gigabytes.
	Interface string `pulumi:"interface"` // Disk interface: "scsi0", "ide1", "virtio", etc.
	FileID    string `pulumi:"filename,optional"`
}

// ToProxmoxDiskKeyConfig converts the Disk struct to Proxmox disk key and config strings.
func (disk Disk) ToProxmoxDiskKeyConfig() (diskKey, diskConfig string) {
	fullDiskPath := fmt.Sprintf("%v:%v", disk.Storage, disk.Size)
	if disk.FileID != "" {
		fullDiskPath = fmt.Sprintf("%v:%v", disk.Storage, disk.FileID)
	}

	diskKey = disk.Interface
	diskConfig = fmt.Sprintf("file=%v,size=%v", fullDiskPath, disk.Size)
	return
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

	OSType   *string `pulumi:"ostype,optional"`
	Machine  *string `pulumi:"machine,optional"`
	Bio      *string `pulumi:"bios,optional"`
	EFIDisk0 *string `pulumi:"efidisk0,optional"`
	// SMBios1  *string `pulumi:"smbios1,optional"`
	Acpi *int `pulumi:"acpi,optional"`

	// Sockets  *int    `pulumi:"sockets,optional"`
	Cores    *int    `pulumi:"cores,optional"`
	CPU      *string `pulumi:"cpu,optional"`
	CPULimit *string `pulumi:"cpulimit,optional"`
	CPUUnits *int    `pulumi:"cpuunits,optional"`
	Vcpus    *int    `pulumi:"vcpus,optional"`
	Affinity *string `pulumi:"affinity,optional"`

	Numa      *int    `pulumi:"numa,optional"`
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

	Numa0 *string `pulumi:"numa0,optional"`

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

// ConvertVMConfigToInputs converts a VirtualMachine configuration to Args.
func ConvertVMConfigToInputs(vm *api.VirtualMachine, currentInput Inputs) (Inputs, error) {
	vmConfig := vm.VirtualMachineConfig
	diskMap := vmConfig.MergeDisks()

	// Sort disk interfaces to ensure consistent ordering
	diskInterfaces := resources.GetSortedMapKeys(diskMap)

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
		// Agent:       strOrNil(vmConfig.Agent),
		Autostart: intOrNil(vmConfig.Autostart),
		Tablet:    intOrNil(vmConfig.Tablet),
		KVM:       intOrNil(vmConfig.KVM),
		// Tags:       strOrNil(vmConfig.Tags),
		Protection: intOrNil(vmConfig.Protection),
		Lock:       strOrNil(vmConfig.Lock),

		// Boot:   strOrNil(vmConfig.Boot),
		// OnBoot: intOrNil(vmConfig.OnBoot),

		OSType:   strOrNil(vmConfig.OSType),
		Machine:  strOrNil(vmConfig.Machine),
		Bio:      strOrNil(vmConfig.Bios),
		EFIDisk0: strOrNil(vmConfig.EFIDisk0),
		// SMBios1:  strOrNil(vmConfig.SMBios1),
		Acpi: intOrNil(vmConfig.Acpi),

		// Sockets:  intOrNil(vmConfig.Sockets),
		Cores:    intOrNil(vmConfig.Cores),
		CPU:      strOrNil(vmConfig.CPU),
		CPUUnits: intOrNil(vmConfig.CPUUnits),
		Vcpus:    intOrNil(vmConfig.Vcpus),
		Affinity: strOrNil(vmConfig.Affinity),

		Numa:      intOrNil(vmConfig.Numa),
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

		Numa0: strOrNil(vmConfig.Numa0),

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
	compareAndAddOption("cores", &options, inputs.Cores, currentInputs.Cores)
	compareAndAddOption("description", &options, inputs.Description, currentInputs.Description)
	compareAndAddOption("autostart", &options, inputs.Autostart, currentInputs.Autostart)
	compareAndAddOption("protection", &options, inputs.Protection, currentInputs.Protection)
	compareAndAddOption("lock", &options, inputs.Lock, currentInputs.Lock)
	compareAndAddOption("cpu", &options, inputs.CPU, currentInputs.CPU)
	compareAndAddOption("cpulimit", &options, inputs.CPULimit, currentInputs.CPULimit)
	compareAndAddOption("cpuunits", &options, inputs.CPUUnits, currentInputs.CPUUnits)
	compareAndAddOption("vcpus", &options, inputs.Vcpus, currentInputs.Vcpus)
	compareAndAddOption("hugepages", &options, inputs.Hugepages, currentInputs.Hugepages)
	compareAndAddOption("balloon", &options, inputs.Balloon, currentInputs.Balloon)
	compareAndAddOption("vga", &options, inputs.VGA, currentInputs.VGA)
	compareAndAddOption("ostype", &options, inputs.OSType, currentInputs.OSType)
	compareAndAddOption("citype", &options, inputs.CIType, currentInputs.CIType)
	compareAndAddOption("ciuser", &options, inputs.CIUser, currentInputs.CIUser)
	compareAndAddOption("cipassword", &options, inputs.CIPassword, currentInputs.CIPassword)
	compareAndAddOption("nameserver", &options, inputs.Nameserver, currentInputs.Nameserver)
	compareAndAddOption("searchdomain", &options, inputs.Searchdomain, currentInputs.Searchdomain)
	compareAndAddOption("sshkeys", &options, inputs.SSHKeys, currentInputs.SSHKeys)
	compareAndAddOption("cicustom", &options, inputs.CICustom, currentInputs.CICustom)
	compareAndAddOption("ciupgrade", &options, inputs.CIUpgrade, currentInputs.CIUpgrade)
	//nolint:gocritic // commentedOutCode
	// compareAndAddOption("boot", &options, inputs.Boot, currentInputs.Boot)
	// compareAndAddOption("onboot", &options, inputs.OnBoot, currentInputs.OnBoot)
	// compareAndAddOption("scsihw", &options, inputs.SCSIHW, currentInputs.SCSIHW)
	// compareAndAddOption("net0", &options, inputs.Net0, currentInputs.Net0)
	// compareAndAddOption("tags", &options, inputs.Tags, currentInputs.Tags)

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
	addOption("cores", &options, inputs.Cores)
	addOption("description", &options, inputs.Description)
	addOption("autostart", &options, inputs.Autostart)
	addOption("protection", &options, inputs.Protection)
	addOption("lock", &options, inputs.Lock)
	addOption("cpu", &options, inputs.CPU)
	addOption("cpulimit", &options, inputs.CPULimit)
	addOption("cpuunits", &options, inputs.CPUUnits)
	addOption("vcpus", &options, inputs.Vcpus)
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
	//nolint:gocritic // commentedOutCode
	// addOption("net0", &options, inputs.Net0)
	// addOption("boot", &options, inputs.Boot)
	// addOption("onboot", &options, inputs.OnBoot)
	// addOption("tags", &options, inputs.Tags)
	// addOption("scsihw", &options, inputs.SCSIHW)

	for _, disk := range inputs.Disks {
		diskKey, diskConfig := disk.ToProxmoxDiskKeyConfig()
		options = append(options, api.VirtualMachineOption{Name: diskKey, Value: diskConfig})
	}

	return options
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
func (disk *Disk) ParseDiskConfig(diskConfig string) (err error) {
	parts := strings.Split(diskConfig, ",")
	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			if !strings.Contains(kv[0], ":") {
				return fmt.Errorf("invalid disk config part: %s", part)
			}

			// Handle disk file configuration
			diskFile := strings.Split(kv[0], ":")
			disk.Storage = diskFile[0]
			disk.FileID = diskFile[1]
		} else {
			key, value := kv[0], kv[1]
			switch key {
			case "file":
				// Handle file configuration
				diskFile := strings.Split(value, ":")
				disk.Storage = diskFile[0]
				disk.FileID = diskFile[1]
			case "size":
				// Parse and set disk size
				var size int
				size, err = parseDiskSize(value)
				if err != nil {
					return err
				}
				disk.Size = size
			}
		}
	}

	if disk.Storage == "" || disk.Size == 0 {
		return fmt.Errorf("failed to parse disk config: %s", diskConfig)
	}

	return nil
}

// parseDiskSize parses the disk size string and returns the size in gigabytes.
func parseDiskSize(value string) (size int, err error) {
	if resources.EndsWithLetter(value) {
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
	if resources.DifferPtr(newValue, currentValue) {
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
