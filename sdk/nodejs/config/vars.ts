// *** WARNING: this file was generated by pulumi-language-nodejs. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

import * as pulumi from "@pulumi/pulumi";
import * as utilities from "../utilities";

declare var exports: any;
const __config = new pulumi.Config("pve");

export declare const pveToken: string | undefined;
Object.defineProperty(exports, "pveToken", {
    get() {
        return __config.get("pveToken");
    },
    enumerable: true,
});

export declare const pveUrl: string | undefined;
Object.defineProperty(exports, "pveUrl", {
    get() {
        return __config.get("pveUrl");
    },
    enumerable: true,
});

export declare const pveUser: string | undefined;
Object.defineProperty(exports, "pveUser", {
    get() {
        return __config.get("pveUser");
    },
    enumerable: true,
});

export declare const sshPass: string | undefined;
Object.defineProperty(exports, "sshPass", {
    get() {
        return __config.get("sshPass");
    },
    enumerable: true,
});

export declare const sshUser: string | undefined;
Object.defineProperty(exports, "sshUser", {
    get() {
        return __config.get("sshUser");
    },
    enumerable: true,
});

