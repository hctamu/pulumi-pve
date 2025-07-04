// *** WARNING: this file was generated by pulumi-language-nodejs. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as inputs from "../types/input";
import * as outputs from "../types/output";
import * as utilities from "../utilities";

export class File extends pulumi.CustomResource {
    /**
     * Get an existing File resource's state with the given name, ID, and optional extra
     * properties used to qualify the lookup.
     *
     * @param name The _unique_ name of the resulting resource.
     * @param id The _unique_ provider ID of the resource to lookup.
     * @param opts Optional settings to control the behavior of the CustomResource.
     */
    public static get(name: string, id: pulumi.Input<pulumi.ID>, opts?: pulumi.CustomResourceOptions): File {
        return new File(name, undefined as any, { ...opts, id: id });
    }

    /** @internal */
    public static readonly __pulumiType = 'pve:storage:File';

    /**
     * Returns true if the given object is an instance of File.  This is designed to work even
     * when multiple copies of the Pulumi SDK have been loaded into the same process.
     */
    public static isInstance(obj: any): obj is File {
        if (obj === undefined || obj === null) {
            return false;
        }
        return obj['__pulumiType'] === File.__pulumiType;
    }

    /**
     * The type of the file (e.g: snippets)
     */
    public readonly contentType!: pulumi.Output<string>;
    /**
     * The datastore to upload the file to.  (e.g:ceph-ha)
     */
    public readonly datastoreId!: pulumi.Output<string>;
    /**
     * The raw source data
     */
    public readonly sourceRaw!: pulumi.Output<outputs.storage.FileSourceRaw>;

    /**
     * Create a File resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the resource.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args: FileArgs, opts?: pulumi.CustomResourceOptions) {
        let resourceInputs: pulumi.Inputs = {};
        opts = opts || {};
        if (!opts.id) {
            if ((!args || args.contentType === undefined) && !opts.urn) {
                throw new Error("Missing required property 'contentType'");
            }
            if ((!args || args.datastoreId === undefined) && !opts.urn) {
                throw new Error("Missing required property 'datastoreId'");
            }
            if ((!args || args.sourceRaw === undefined) && !opts.urn) {
                throw new Error("Missing required property 'sourceRaw'");
            }
            resourceInputs["contentType"] = args ? args.contentType : undefined;
            resourceInputs["datastoreId"] = args ? args.datastoreId : undefined;
            resourceInputs["sourceRaw"] = args ? args.sourceRaw : undefined;
        } else {
            resourceInputs["contentType"] = undefined /*out*/;
            resourceInputs["datastoreId"] = undefined /*out*/;
            resourceInputs["sourceRaw"] = undefined /*out*/;
        }
        opts = pulumi.mergeOptions(utilities.resourceOptsDefaults(), opts);
        super(File.__pulumiType, name, resourceInputs, opts);
    }
}

/**
 * The set of arguments for constructing a File resource.
 */
export interface FileArgs {
    /**
     * The type of the file (e.g: snippets)
     */
    contentType: pulumi.Input<string>;
    /**
     * The datastore to upload the file to.  (e.g:ceph-ha)
     */
    datastoreId: pulumi.Input<string>;
    /**
     * The raw source data
     */
    sourceRaw: pulumi.Input<inputs.storage.FileSourceRawArgs>;
}
