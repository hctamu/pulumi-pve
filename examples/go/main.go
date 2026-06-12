package main

import (
	"github.com/hctamu/pulumi-pve/sdk/go/pve/proxmox"
	"github.com/hctamu/pulumi-pve/sdk/go/pve/sdnapply"
	"github.com/hctamu/pulumi-pve/sdk/go/pve/vm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		vmArgs := &vm.VMArgs{
			Name:        pulumi.String("aTestVM"),
			Description: pulumi.StringPtr("test"),
			Tags:        pulumi.StringArray{pulumi.String("tag2"), pulumi.String("tag1")},
			Memory:      pulumi.IntPtr(27875),
			Cpu: proxmox.CPUArgs{
				Type:          pulumi.StringPtr("EPYC-v4"),
				FlagsEnabled:  pulumi.StringArray{pulumi.String("virt-ssbd")},
				FlagsDisabled: pulumi.StringArray{pulumi.String("spec-ctrl")},
				Hidden:        pulumi.BoolPtr(true),
				HvVendorId:    pulumi.StringPtr("AuthenticAMD"),
				PhysBits:      pulumi.StringPtr("48"),
				Cores:         pulumi.IntPtr(4),
				Sockets:       pulumi.IntPtr(2),
				Limit:         pulumi.Float64Ptr(3.0),
				Units:         pulumi.IntPtr(2048),
				Vcpus:         pulumi.IntPtr(3),
				Numa:          pulumi.BoolPtr(true),
				NumaNodes: proxmox.NumaNodeArray{
					proxmox.NumaNodeArgs{
						Cpus:      pulumi.String("0-1"),
						HostNodes: pulumi.StringPtr("0"),
						Memory:    pulumi.IntPtr(2048),
						Policy:    pulumi.StringPtr("preferred"),
					},
					proxmox.NumaNodeArgs{
						Cpus:      pulumi.String("2-3"),
						HostNodes: pulumi.StringPtr("1"),
						Policy:    pulumi.StringPtr("interleave"),
					},
				},
			}.ToCPUPtrOutput(),
			Efidisk: proxmox.EfiDiskArgs{
				Storage:         pulumi.String("ceph-ha"),
				Efitype:         pulumi.String("4m"),
				PreEnrolledKeys: pulumi.BoolPtr(false),
			}.ToEfiDiskPtrOutput(),
			Disks: proxmox.DiskArray{
				proxmox.DiskArgs{
					Storage:   pulumi.String("ceph-ha"),
					Size:      pulumi.Int(20),
					Interface: pulumi.String("scsi0"),
				},
				proxmox.DiskArgs{
					Storage:   pulumi.String("ceph-ha"),
					Size:      pulumi.Int(17),
					Interface: pulumi.String("scsi1"),
				},
				proxmox.DiskArgs{
					Storage:   pulumi.String("ceph-ha"),
					Size:      pulumi.Int(21),
					Interface: pulumi.String("sata0"),
				},
			},
		}

		myVm, err := vm.NewVM(ctx, "myVM", vmArgs)
		if err != nil {
			return err
		}

		_, err = sdnapply.NewSDNApply(ctx, "mySdnApply", &sdnapply.SDNApplyArgs{
			Triggers: pulumi.Map{
				"vm": vmArgs,
			},
		}, pulumi.DependsOn([]pulumi.Resource{myVm}))
		if err != nil {
			return err
		}

		return nil
	})
}
