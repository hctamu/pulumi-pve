// *** WARNING: this file was generated by pulumi-language-java. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

package com.hctamu.pve.vm;

import com.hctamu.pve.Utilities;
import com.hctamu.pve.vm.VmArgs;
import com.hctamu.pve.vm.outputs.Disk;
import com.hctamu.pve.vm.outputs.VmClone;
import com.pulumi.core.Output;
import com.pulumi.core.annotations.Export;
import com.pulumi.core.annotations.ResourceType;
import com.pulumi.core.internal.Codegen;
import java.lang.Integer;
import java.lang.String;
import java.util.List;
import java.util.Optional;
import javax.annotation.Nullable;

@ResourceType(type="pve:vm:Vm")
public class Vm extends com.pulumi.resources.CustomResource {
    @Export(name="acpi", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> acpi;

    public Output<Optional<Integer>> acpi() {
        return Codegen.optional(this.acpi);
    }
    @Export(name="affinity", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> affinity;

    public Output<Optional<String>> affinity() {
        return Codegen.optional(this.affinity);
    }
    @Export(name="agent", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> agent;

    public Output<Optional<String>> agent() {
        return Codegen.optional(this.agent);
    }
    @Export(name="audio0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> audio0;

    public Output<Optional<String>> audio0() {
        return Codegen.optional(this.audio0);
    }
    @Export(name="autostart", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> autostart;

    public Output<Optional<Integer>> autostart() {
        return Codegen.optional(this.autostart);
    }
    @Export(name="balloon", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> balloon;

    public Output<Optional<Integer>> balloon() {
        return Codegen.optional(this.balloon);
    }
    @Export(name="bios", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> bios;

    public Output<Optional<String>> bios() {
        return Codegen.optional(this.bios);
    }
    @Export(name="boot", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> boot;

    public Output<Optional<String>> boot() {
        return Codegen.optional(this.boot);
    }
    @Export(name="cicustom", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> cicustom;

    public Output<Optional<String>> cicustom() {
        return Codegen.optional(this.cicustom);
    }
    @Export(name="cipassword", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> cipassword;

    public Output<Optional<String>> cipassword() {
        return Codegen.optional(this.cipassword);
    }
    @Export(name="citype", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> citype;

    public Output<Optional<String>> citype() {
        return Codegen.optional(this.citype);
    }
    @Export(name="ciupgrade", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> ciupgrade;

    public Output<Optional<Integer>> ciupgrade() {
        return Codegen.optional(this.ciupgrade);
    }
    @Export(name="ciuser", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> ciuser;

    public Output<Optional<String>> ciuser() {
        return Codegen.optional(this.ciuser);
    }
    @Export(name="clone", refs={VmClone.class}, tree="[0]")
    private Output</* @Nullable */ VmClone> clone;

    public Output<Optional<VmClone>> clone_() {
        return Codegen.optional(this.clone);
    }
    @Export(name="cores", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> cores;

    public Output<Optional<Integer>> cores() {
        return Codegen.optional(this.cores);
    }
    @Export(name="cpu", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> cpu;

    public Output<Optional<String>> cpu() {
        return Codegen.optional(this.cpu);
    }
    @Export(name="cpulimit", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> cpulimit;

    public Output<Optional<String>> cpulimit() {
        return Codegen.optional(this.cpulimit);
    }
    @Export(name="cpuunits", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> cpuunits;

    public Output<Optional<Integer>> cpuunits() {
        return Codegen.optional(this.cpuunits);
    }
    @Export(name="description", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> description;

    public Output<Optional<String>> description() {
        return Codegen.optional(this.description);
    }
    @Export(name="digest", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> digest;

    public Output<Optional<String>> digest() {
        return Codegen.optional(this.digest);
    }
    @Export(name="disks", refs={List.class,Disk.class}, tree="[0,1]")
    private Output<List<Disk>> disks;

    public Output<List<Disk>> disks() {
        return this.disks;
    }
    @Export(name="efidisk0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> efidisk0;

    public Output<Optional<String>> efidisk0() {
        return Codegen.optional(this.efidisk0);
    }
    @Export(name="hookscript", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> hookscript;

    public Output<Optional<String>> hookscript() {
        return Codegen.optional(this.hookscript);
    }
    @Export(name="hostpci0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> hostpci0;

    public Output<Optional<String>> hostpci0() {
        return Codegen.optional(this.hostpci0);
    }
    @Export(name="hotplug", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> hotplug;

    public Output<Optional<String>> hotplug() {
        return Codegen.optional(this.hotplug);
    }
    @Export(name="hugepages", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> hugepages;

    public Output<Optional<String>> hugepages() {
        return Codegen.optional(this.hugepages);
    }
    @Export(name="ipconfig0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> ipconfig0;

    public Output<Optional<String>> ipconfig0() {
        return Codegen.optional(this.ipconfig0);
    }
    @Export(name="kvm", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> kvm;

    public Output<Optional<Integer>> kvm() {
        return Codegen.optional(this.kvm);
    }
    @Export(name="lock", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> lock;

    public Output<Optional<String>> lock() {
        return Codegen.optional(this.lock);
    }
    @Export(name="machine", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> machine;

    public Output<Optional<String>> machine() {
        return Codegen.optional(this.machine);
    }
    @Export(name="memory", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> memory;

    public Output<Optional<Integer>> memory() {
        return Codegen.optional(this.memory);
    }
    @Export(name="name", refs={String.class}, tree="[0]")
    private Output<String> name;

    public Output<String> name() {
        return this.name;
    }
    @Export(name="nameserver", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> nameserver;

    public Output<Optional<String>> nameserver() {
        return Codegen.optional(this.nameserver);
    }
    @Export(name="net0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> net0;

    public Output<Optional<String>> net0() {
        return Codegen.optional(this.net0);
    }
    @Export(name="node", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> node;

    public Output<Optional<String>> node() {
        return Codegen.optional(this.node);
    }
    @Export(name="numa", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> numa;

    public Output<Optional<Integer>> numa() {
        return Codegen.optional(this.numa);
    }
    @Export(name="numa0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> numa0;

    public Output<Optional<String>> numa0() {
        return Codegen.optional(this.numa0);
    }
    @Export(name="onboot", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> onboot;

    public Output<Optional<Integer>> onboot() {
        return Codegen.optional(this.onboot);
    }
    @Export(name="ostype", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> ostype;

    public Output<Optional<String>> ostype() {
        return Codegen.optional(this.ostype);
    }
    @Export(name="parallel0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> parallel0;

    public Output<Optional<String>> parallel0() {
        return Codegen.optional(this.parallel0);
    }
    @Export(name="protection", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> protection;

    public Output<Optional<Integer>> protection() {
        return Codegen.optional(this.protection);
    }
    @Export(name="rng0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> rng0;

    public Output<Optional<String>> rng0() {
        return Codegen.optional(this.rng0);
    }
    @Export(name="scsihw", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> scsihw;

    public Output<Optional<String>> scsihw() {
        return Codegen.optional(this.scsihw);
    }
    @Export(name="searchdomain", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> searchdomain;

    public Output<Optional<String>> searchdomain() {
        return Codegen.optional(this.searchdomain);
    }
    @Export(name="serial0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> serial0;

    public Output<Optional<String>> serial0() {
        return Codegen.optional(this.serial0);
    }
    @Export(name="smbios1", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> smbios1;

    public Output<Optional<String>> smbios1() {
        return Codegen.optional(this.smbios1);
    }
    @Export(name="sockets", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> sockets;

    public Output<Optional<Integer>> sockets() {
        return Codegen.optional(this.sockets);
    }
    @Export(name="sshkeys", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> sshkeys;

    public Output<Optional<String>> sshkeys() {
        return Codegen.optional(this.sshkeys);
    }
    @Export(name="tablet", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> tablet;

    public Output<Optional<Integer>> tablet() {
        return Codegen.optional(this.tablet);
    }
    @Export(name="tags", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> tags;

    public Output<Optional<String>> tags() {
        return Codegen.optional(this.tags);
    }
    @Export(name="template", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> template;

    public Output<Optional<Integer>> template() {
        return Codegen.optional(this.template);
    }
    @Export(name="tpmstate0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> tpmstate0;

    public Output<Optional<String>> tpmstate0() {
        return Codegen.optional(this.tpmstate0);
    }
    @Export(name="usb0", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> usb0;

    public Output<Optional<String>> usb0() {
        return Codegen.optional(this.usb0);
    }
    @Export(name="vcpus", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> vcpus;

    public Output<Optional<Integer>> vcpus() {
        return Codegen.optional(this.vcpus);
    }
    @Export(name="vga", refs={String.class}, tree="[0]")
    private Output</* @Nullable */ String> vga;

    public Output<Optional<String>> vga() {
        return Codegen.optional(this.vga);
    }
    @Export(name="vmId", refs={Integer.class}, tree="[0]")
    private Output</* @Nullable */ Integer> vmId;

    public Output<Optional<Integer>> vmId() {
        return Codegen.optional(this.vmId);
    }

    /**
     *
     * @param name The _unique_ name of the resulting resource.
     */
    public Vm(java.lang.String name) {
        this(name, VmArgs.Empty);
    }
    /**
     *
     * @param name The _unique_ name of the resulting resource.
     * @param args The arguments to use to populate this resource's properties.
     */
    public Vm(java.lang.String name, VmArgs args) {
        this(name, args, null);
    }
    /**
     *
     * @param name The _unique_ name of the resulting resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param options A bag of options that control this resource's behavior.
     */
    public Vm(java.lang.String name, VmArgs args, @Nullable com.pulumi.resources.CustomResourceOptions options) {
        super("pve:vm:Vm", name, makeArgs(args, options), makeResourceOptions(options, Codegen.empty()), false);
    }

    private Vm(java.lang.String name, Output<java.lang.String> id, @Nullable com.pulumi.resources.CustomResourceOptions options) {
        super("pve:vm:Vm", name, null, makeResourceOptions(options, id), false);
    }

    private static VmArgs makeArgs(VmArgs args, @Nullable com.pulumi.resources.CustomResourceOptions options) {
        if (options != null && options.getUrn().isPresent()) {
            return null;
        }
        return args == null ? VmArgs.Empty : args;
    }

    private static com.pulumi.resources.CustomResourceOptions makeResourceOptions(@Nullable com.pulumi.resources.CustomResourceOptions options, @Nullable Output<java.lang.String> id) {
        var defaultOptions = com.pulumi.resources.CustomResourceOptions.builder()
            .version(Utilities.getVersion())
            .build();
        return com.pulumi.resources.CustomResourceOptions.merge(defaultOptions, options, id);
    }

    /**
     * Get an existing Host resource's state with the given name, ID, and optional extra
     * properties used to qualify the lookup.
     *
     * @param name The _unique_ name of the resulting resource.
     * @param id The _unique_ provider ID of the resource to lookup.
     * @param options Optional settings to control the behavior of the CustomResource.
     */
    public static Vm get(java.lang.String name, Output<java.lang.String> id, @Nullable com.pulumi.resources.CustomResourceOptions options) {
        return new Vm(name, id, options);
    }
}
