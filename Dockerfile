FROM pulumi/pulumi-provider-build-environment:3.239.0-amd64

USER root

RUN apt-get update && apt-get install -y \
    zip \
    vim \
    sudo \
    ca-certificates \
    npm

RUN update-ca-certificates --fresh
RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN go install mvdan.cc/gofumpt@latest
RUN go install github.com/segmentio/golines@latest
RUN go install golang.org/x/tools/gopls@latest
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

ENV GOCACHE=/tmp/go-build

# Install sdkman, Gradle 8.7, and JDK 11 under root's home directory.
RUN curl -s "https://get.sdkman.io" | bash \
    && bash -c "source /root/.sdkman/bin/sdkman-init.sh \
        && sdk install gradle 8.7 \
        && sdk install java 11.0.27-zulu"

# Add sdkman candidates/bin to PATH so gradle and java are found by make and
# other non-login shells (e.g. the shell that runs `gradle --console=plain build`).
ENV PATH="/root/.sdkman/candidates/gradle/current/bin:/root/.sdkman/candidates/java/current/bin:${PATH}"

# Tell Gradle's JVM toolchain resolver where to find the JDK 11 installation.
RUN mkdir -p /root/.gradle \
    && echo "org.gradle.java.installations.paths=/root/.sdkman/candidates/java/current" \
       > /root/.gradle/gradle.properties

RUN mkdir -p /root/.local/share /root/.local/state

CMD [ "bash" ]
