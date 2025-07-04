# coding=utf-8
# *** WARNING: this file was generated by pulumi-language-python. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

import copy
import warnings
import sys
import pulumi
import pulumi.runtime
from typing import Any, Mapping, Optional, Sequence, Union, overload
if sys.version_info >= (3, 11):
    from typing import NotRequired, TypedDict, TypeAlias
else:
    from typing_extensions import NotRequired, TypedDict, TypeAlias
from . import _utilities

__all__ = ['ProviderArgs', 'Provider']

@pulumi.input_type
class ProviderArgs:
    def __init__(__self__, *,
                 pve_token: pulumi.Input[str],
                 pve_url: pulumi.Input[str],
                 pve_user: pulumi.Input[str],
                 ssh_pass: pulumi.Input[str],
                 ssh_user: pulumi.Input[str]):
        """
        The set of arguments for constructing a Provider resource.
        """
        pulumi.set(__self__, "pve_token", pve_token)
        pulumi.set(__self__, "pve_url", pve_url)
        pulumi.set(__self__, "pve_user", pve_user)
        pulumi.set(__self__, "ssh_pass", ssh_pass)
        pulumi.set(__self__, "ssh_user", ssh_user)

    @property
    @pulumi.getter(name="pveToken")
    def pve_token(self) -> pulumi.Input[str]:
        return pulumi.get(self, "pve_token")

    @pve_token.setter
    def pve_token(self, value: pulumi.Input[str]):
        pulumi.set(self, "pve_token", value)

    @property
    @pulumi.getter(name="pveUrl")
    def pve_url(self) -> pulumi.Input[str]:
        return pulumi.get(self, "pve_url")

    @pve_url.setter
    def pve_url(self, value: pulumi.Input[str]):
        pulumi.set(self, "pve_url", value)

    @property
    @pulumi.getter(name="pveUser")
    def pve_user(self) -> pulumi.Input[str]:
        return pulumi.get(self, "pve_user")

    @pve_user.setter
    def pve_user(self, value: pulumi.Input[str]):
        pulumi.set(self, "pve_user", value)

    @property
    @pulumi.getter(name="sshPass")
    def ssh_pass(self) -> pulumi.Input[str]:
        return pulumi.get(self, "ssh_pass")

    @ssh_pass.setter
    def ssh_pass(self, value: pulumi.Input[str]):
        pulumi.set(self, "ssh_pass", value)

    @property
    @pulumi.getter(name="sshUser")
    def ssh_user(self) -> pulumi.Input[str]:
        return pulumi.get(self, "ssh_user")

    @ssh_user.setter
    def ssh_user(self, value: pulumi.Input[str]):
        pulumi.set(self, "ssh_user", value)


class Provider(pulumi.ProviderResource):
    @overload
    def __init__(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 pve_token: Optional[pulumi.Input[str]] = None,
                 pve_url: Optional[pulumi.Input[str]] = None,
                 pve_user: Optional[pulumi.Input[str]] = None,
                 ssh_pass: Optional[pulumi.Input[str]] = None,
                 ssh_user: Optional[pulumi.Input[str]] = None,
                 __props__=None):
        """
        Create a Pve resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        ...
    @overload
    def __init__(__self__,
                 resource_name: str,
                 args: ProviderArgs,
                 opts: Optional[pulumi.ResourceOptions] = None):
        """
        Create a Pve resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param ProviderArgs args: The arguments to use to populate this resource's properties.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        ...
    def __init__(__self__, resource_name: str, *args, **kwargs):
        resource_args, opts = _utilities.get_resource_args_opts(ProviderArgs, pulumi.ResourceOptions, *args, **kwargs)
        if resource_args is not None:
            __self__._internal_init(resource_name, opts, **resource_args.__dict__)
        else:
            __self__._internal_init(resource_name, *args, **kwargs)

    def _internal_init(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 pve_token: Optional[pulumi.Input[str]] = None,
                 pve_url: Optional[pulumi.Input[str]] = None,
                 pve_user: Optional[pulumi.Input[str]] = None,
                 ssh_pass: Optional[pulumi.Input[str]] = None,
                 ssh_user: Optional[pulumi.Input[str]] = None,
                 __props__=None):
        opts = pulumi.ResourceOptions.merge(_utilities.get_resource_opts_defaults(), opts)
        if not isinstance(opts, pulumi.ResourceOptions):
            raise TypeError('Expected resource options to be a ResourceOptions instance')
        if opts.id is None:
            if __props__ is not None:
                raise TypeError('__props__ is only valid when passed in combination with a valid opts.id to get an existing resource')
            __props__ = ProviderArgs.__new__(ProviderArgs)

            if pve_token is None and not opts.urn:
                raise TypeError("Missing required property 'pve_token'")
            __props__.__dict__["pve_token"] = None if pve_token is None else pulumi.Output.secret(pve_token)
            if pve_url is None and not opts.urn:
                raise TypeError("Missing required property 'pve_url'")
            __props__.__dict__["pve_url"] = pve_url
            if pve_user is None and not opts.urn:
                raise TypeError("Missing required property 'pve_user'")
            __props__.__dict__["pve_user"] = pve_user
            if ssh_pass is None and not opts.urn:
                raise TypeError("Missing required property 'ssh_pass'")
            __props__.__dict__["ssh_pass"] = None if ssh_pass is None else pulumi.Output.secret(ssh_pass)
            if ssh_user is None and not opts.urn:
                raise TypeError("Missing required property 'ssh_user'")
            __props__.__dict__["ssh_user"] = ssh_user
        secret_opts = pulumi.ResourceOptions(additional_secret_outputs=["pveToken", "sshPass"])
        opts = pulumi.ResourceOptions.merge(opts, secret_opts)
        super(Provider, __self__).__init__(
            'pve',
            resource_name,
            __props__,
            opts)

    @property
    @pulumi.getter(name="pveToken")
    def pve_token(self) -> pulumi.Output[str]:
        return pulumi.get(self, "pve_token")

    @property
    @pulumi.getter(name="pveUrl")
    def pve_url(self) -> pulumi.Output[str]:
        return pulumi.get(self, "pve_url")

    @property
    @pulumi.getter(name="pveUser")
    def pve_user(self) -> pulumi.Output[str]:
        return pulumi.get(self, "pve_user")

    @property
    @pulumi.getter(name="sshPass")
    def ssh_pass(self) -> pulumi.Output[str]:
        return pulumi.get(self, "ssh_pass")

    @property
    @pulumi.getter(name="sshUser")
    def ssh_user(self) -> pulumi.Output[str]:
        return pulumi.get(self, "ssh_user")

