FROM pulumi/pulumi-provider-build-environment:3.232.0-amd64

# The base image hard-codes XDG_CACHE_HOME and XDG_CONFIG_HOME to /root in
# /etc/environment. Remove them so tools fall back to the running user's home
# directory — required when VS Code connects as the non-root vscode user.
RUN sed -i '/^XDG_CACHE_HOME=/d; /^XDG_CONFIG_HOME=/d' /etc/environment

RUN apt-get update && apt-get install -y \
    zip \
    vim \
    sudo

RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN go install mvdan.cc/gofumpt@latest
RUN go install github.com/segmentio/golines@latest
RUN go install golang.org/x/tools/gopls@latest
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
RUN chmod -R a+rwX /go/pkg

RUN groupadd --gid 1000 vscode \
    && useradd --uid 1000 --gid 1000 -m vscode \
    && echo 'vscode ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# Everything below runs as the vscode user — the default user for this devcontainer.
USER vscode
ENV GOCACHE=/tmp/go-build

# Install sdkman, Gradle 8.7, and JDK 11 as the vscode user so everything lives
# in /home/vscode/.sdkman and is fully readable/writable without sudo.
RUN curl -s "https://get.sdkman.io" | bash \
    && bash -c "source /home/vscode/.sdkman/bin/sdkman-init.sh \
        && sdk install gradle 8.7 \
        && sdk install java 11.0.27-zulu"

# Add sdkman candidates/bin to PATH so gradle and java are found by make and
# other non-login shells (e.g. the shell that runs `gradle --console=plain build`).
ENV PATH="/home/vscode/.sdkman/candidates/gradle/current/bin:/home/vscode/.sdkman/candidates/java/current/bin:${PATH}"

# Tell Gradle's JVM toolchain resolver where to find the JDK 11 installation.
RUN mkdir -p /home/vscode/.gradle \
    && echo "org.gradle.java.installations.paths=/home/vscode/.sdkman/candidates/java/current" \
       > /home/vscode/.gradle/gradle.properties

# Pre-create .local subdirectories as vscode so Docker bind-mounts don't
# cause them to be owned by root (which would block mkdir of sibling dirs).
RUN mkdir -p /home/vscode/.local/share /home/vscode/.local/state
