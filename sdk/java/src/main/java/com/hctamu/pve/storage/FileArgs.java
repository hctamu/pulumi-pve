// *** WARNING: this file was generated by pulumi-language-java. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

package com.hctamu.pve.storage;

import com.hctamu.pve.storage.inputs.FileSourceRawArgs;
import com.pulumi.core.Output;
import com.pulumi.core.annotations.Import;
import com.pulumi.exceptions.MissingRequiredPropertyException;
import java.lang.String;
import java.util.Objects;


public final class FileArgs extends com.pulumi.resources.ResourceArgs {

    public static final FileArgs Empty = new FileArgs();

    /**
     * The type of the file (e.g: snippets)
     * 
     */
    @Import(name="contentType", required=true)
    private Output<String> contentType;

    /**
     * @return The type of the file (e.g: snippets)
     * 
     */
    public Output<String> contentType() {
        return this.contentType;
    }

    /**
     * The datastore to upload the file to.  (e.g:ceph-ha)
     * 
     */
    @Import(name="datastoreId", required=true)
    private Output<String> datastoreId;

    /**
     * @return The datastore to upload the file to.  (e.g:ceph-ha)
     * 
     */
    public Output<String> datastoreId() {
        return this.datastoreId;
    }

    /**
     * The raw source data
     * 
     */
    @Import(name="sourceRaw", required=true)
    private Output<FileSourceRawArgs> sourceRaw;

    /**
     * @return The raw source data
     * 
     */
    public Output<FileSourceRawArgs> sourceRaw() {
        return this.sourceRaw;
    }

    private FileArgs() {}

    private FileArgs(FileArgs $) {
        this.contentType = $.contentType;
        this.datastoreId = $.datastoreId;
        this.sourceRaw = $.sourceRaw;
    }

    public static Builder builder() {
        return new Builder();
    }
    public static Builder builder(FileArgs defaults) {
        return new Builder(defaults);
    }

    public static final class Builder {
        private FileArgs $;

        public Builder() {
            $ = new FileArgs();
        }

        public Builder(FileArgs defaults) {
            $ = new FileArgs(Objects.requireNonNull(defaults));
        }

        /**
         * @param contentType The type of the file (e.g: snippets)
         * 
         * @return builder
         * 
         */
        public Builder contentType(Output<String> contentType) {
            $.contentType = contentType;
            return this;
        }

        /**
         * @param contentType The type of the file (e.g: snippets)
         * 
         * @return builder
         * 
         */
        public Builder contentType(String contentType) {
            return contentType(Output.of(contentType));
        }

        /**
         * @param datastoreId The datastore to upload the file to.  (e.g:ceph-ha)
         * 
         * @return builder
         * 
         */
        public Builder datastoreId(Output<String> datastoreId) {
            $.datastoreId = datastoreId;
            return this;
        }

        /**
         * @param datastoreId The datastore to upload the file to.  (e.g:ceph-ha)
         * 
         * @return builder
         * 
         */
        public Builder datastoreId(String datastoreId) {
            return datastoreId(Output.of(datastoreId));
        }

        /**
         * @param sourceRaw The raw source data
         * 
         * @return builder
         * 
         */
        public Builder sourceRaw(Output<FileSourceRawArgs> sourceRaw) {
            $.sourceRaw = sourceRaw;
            return this;
        }

        /**
         * @param sourceRaw The raw source data
         * 
         * @return builder
         * 
         */
        public Builder sourceRaw(FileSourceRawArgs sourceRaw) {
            return sourceRaw(Output.of(sourceRaw));
        }

        public FileArgs build() {
            if ($.contentType == null) {
                throw new MissingRequiredPropertyException("FileArgs", "contentType");
            }
            if ($.datastoreId == null) {
                throw new MissingRequiredPropertyException("FileArgs", "datastoreId");
            }
            if ($.sourceRaw == null) {
                throw new MissingRequiredPropertyException("FileArgs", "sourceRaw");
            }
            return $;
        }
    }

}
