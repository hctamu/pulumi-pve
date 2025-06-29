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
from .. import _utilities
from . import outputs
from ._inputs import *

__all__ = ['FileArgs', 'File']

@pulumi.input_type
class FileArgs:
    def __init__(__self__, *,
                 content_type: pulumi.Input[str],
                 datastore_id: pulumi.Input[str],
                 source_raw: pulumi.Input['FileSourceRawArgs']):
        """
        The set of arguments for constructing a File resource.
        :param pulumi.Input[str] content_type: The type of the file (e.g: snippets)
        :param pulumi.Input[str] datastore_id: The datastore to upload the file to.  (e.g:ceph-ha)
        :param pulumi.Input['FileSourceRawArgs'] source_raw: The raw source data
        """
        pulumi.set(__self__, "content_type", content_type)
        pulumi.set(__self__, "datastore_id", datastore_id)
        pulumi.set(__self__, "source_raw", source_raw)

    @property
    @pulumi.getter(name="contentType")
    def content_type(self) -> pulumi.Input[str]:
        """
        The type of the file (e.g: snippets)
        """
        return pulumi.get(self, "content_type")

    @content_type.setter
    def content_type(self, value: pulumi.Input[str]):
        pulumi.set(self, "content_type", value)

    @property
    @pulumi.getter(name="datastoreId")
    def datastore_id(self) -> pulumi.Input[str]:
        """
        The datastore to upload the file to.  (e.g:ceph-ha)
        """
        return pulumi.get(self, "datastore_id")

    @datastore_id.setter
    def datastore_id(self, value: pulumi.Input[str]):
        pulumi.set(self, "datastore_id", value)

    @property
    @pulumi.getter(name="sourceRaw")
    def source_raw(self) -> pulumi.Input['FileSourceRawArgs']:
        """
        The raw source data
        """
        return pulumi.get(self, "source_raw")

    @source_raw.setter
    def source_raw(self, value: pulumi.Input['FileSourceRawArgs']):
        pulumi.set(self, "source_raw", value)


class File(pulumi.CustomResource):
    @overload
    def __init__(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 content_type: Optional[pulumi.Input[str]] = None,
                 datastore_id: Optional[pulumi.Input[str]] = None,
                 source_raw: Optional[pulumi.Input[Union['FileSourceRawArgs', 'FileSourceRawArgsDict']]] = None,
                 __props__=None):
        """
        Create a File resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param pulumi.ResourceOptions opts: Options for the resource.
        :param pulumi.Input[str] content_type: The type of the file (e.g: snippets)
        :param pulumi.Input[str] datastore_id: The datastore to upload the file to.  (e.g:ceph-ha)
        :param pulumi.Input[Union['FileSourceRawArgs', 'FileSourceRawArgsDict']] source_raw: The raw source data
        """
        ...
    @overload
    def __init__(__self__,
                 resource_name: str,
                 args: FileArgs,
                 opts: Optional[pulumi.ResourceOptions] = None):
        """
        Create a File resource with the given unique name, props, and options.
        :param str resource_name: The name of the resource.
        :param FileArgs args: The arguments to use to populate this resource's properties.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        ...
    def __init__(__self__, resource_name: str, *args, **kwargs):
        resource_args, opts = _utilities.get_resource_args_opts(FileArgs, pulumi.ResourceOptions, *args, **kwargs)
        if resource_args is not None:
            __self__._internal_init(resource_name, opts, **resource_args.__dict__)
        else:
            __self__._internal_init(resource_name, *args, **kwargs)

    def _internal_init(__self__,
                 resource_name: str,
                 opts: Optional[pulumi.ResourceOptions] = None,
                 content_type: Optional[pulumi.Input[str]] = None,
                 datastore_id: Optional[pulumi.Input[str]] = None,
                 source_raw: Optional[pulumi.Input[Union['FileSourceRawArgs', 'FileSourceRawArgsDict']]] = None,
                 __props__=None):
        opts = pulumi.ResourceOptions.merge(_utilities.get_resource_opts_defaults(), opts)
        if not isinstance(opts, pulumi.ResourceOptions):
            raise TypeError('Expected resource options to be a ResourceOptions instance')
        if opts.id is None:
            if __props__ is not None:
                raise TypeError('__props__ is only valid when passed in combination with a valid opts.id to get an existing resource')
            __props__ = FileArgs.__new__(FileArgs)

            if content_type is None and not opts.urn:
                raise TypeError("Missing required property 'content_type'")
            __props__.__dict__["content_type"] = content_type
            if datastore_id is None and not opts.urn:
                raise TypeError("Missing required property 'datastore_id'")
            __props__.__dict__["datastore_id"] = datastore_id
            if source_raw is None and not opts.urn:
                raise TypeError("Missing required property 'source_raw'")
            __props__.__dict__["source_raw"] = source_raw
        super(File, __self__).__init__(
            'pve:storage:File',
            resource_name,
            __props__,
            opts)

    @staticmethod
    def get(resource_name: str,
            id: pulumi.Input[str],
            opts: Optional[pulumi.ResourceOptions] = None) -> 'File':
        """
        Get an existing File resource's state with the given name, id, and optional extra
        properties used to qualify the lookup.

        :param str resource_name: The unique name of the resulting resource.
        :param pulumi.Input[str] id: The unique provider ID of the resource to lookup.
        :param pulumi.ResourceOptions opts: Options for the resource.
        """
        opts = pulumi.ResourceOptions.merge(opts, pulumi.ResourceOptions(id=id))

        __props__ = FileArgs.__new__(FileArgs)

        __props__.__dict__["content_type"] = None
        __props__.__dict__["datastore_id"] = None
        __props__.__dict__["source_raw"] = None
        return File(resource_name, opts=opts, __props__=__props__)

    @property
    @pulumi.getter(name="contentType")
    def content_type(self) -> pulumi.Output[str]:
        """
        The type of the file (e.g: snippets)
        """
        return pulumi.get(self, "content_type")

    @property
    @pulumi.getter(name="datastoreId")
    def datastore_id(self) -> pulumi.Output[str]:
        """
        The datastore to upload the file to.  (e.g:ceph-ha)
        """
        return pulumi.get(self, "datastore_id")

    @property
    @pulumi.getter(name="sourceRaw")
    def source_raw(self) -> pulumi.Output['outputs.FileSourceRaw']:
        """
        The raw source data
        """
        return pulumi.get(self, "source_raw")

