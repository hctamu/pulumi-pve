// *** WARNING: this file was generated by pulumi-language-java. ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

package com.hctamu.pve.storage.outputs;

import com.pulumi.core.annotations.CustomType;
import com.pulumi.exceptions.MissingRequiredPropertyException;
import java.lang.String;
import java.util.Objects;

@CustomType
public final class FileSourceRaw {
    /**
     * @return The raw data in []byte
     * 
     */
    private String fileData;
    /**
     * @return The name of the file
     * 
     */
    private String fileName;

    private FileSourceRaw() {}
    /**
     * @return The raw data in []byte
     * 
     */
    public String fileData() {
        return this.fileData;
    }
    /**
     * @return The name of the file
     * 
     */
    public String fileName() {
        return this.fileName;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static Builder builder(FileSourceRaw defaults) {
        return new Builder(defaults);
    }
    @CustomType.Builder
    public static final class Builder {
        private String fileData;
        private String fileName;
        public Builder() {}
        public Builder(FileSourceRaw defaults) {
    	      Objects.requireNonNull(defaults);
    	      this.fileData = defaults.fileData;
    	      this.fileName = defaults.fileName;
        }

        @CustomType.Setter
        public Builder fileData(String fileData) {
            if (fileData == null) {
              throw new MissingRequiredPropertyException("FileSourceRaw", "fileData");
            }
            this.fileData = fileData;
            return this;
        }
        @CustomType.Setter
        public Builder fileName(String fileName) {
            if (fileName == null) {
              throw new MissingRequiredPropertyException("FileSourceRaw", "fileName");
            }
            this.fileName = fileName;
            return this;
        }
        public FileSourceRaw build() {
            final var _resultValue = new FileSourceRaw();
            _resultValue.fileData = fileData;
            _resultValue.fileName = fileName;
            return _resultValue;
        }
    }
}
