FROM pulumi/pulumi-provider-build-environment:3.232.0-amd64

RUN apt-get update && apt-get install -y \
    zip \
    vim

RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN go install mvdan.cc/gofumpt@latest
RUN go install github.com/segmentio/golines@latest
RUN go install golang.org/x/tools/gopls@latest
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

RUN curl -s "https://get.sdkman.io" | bash

RUN chmod a+x "$HOME/.sdkman/bin/sdkman-init.sh"

RUN ["/bin/bash", "-c", "source $HOME/.sdkman/bin/sdkman-init.sh && \
    sdk install gradle 7.6 && \
    sdk install java 11.0.27-zulu"]


RUN groupadd --gid 1000 vscode \
    && useradd --uid 1000 --gid 1000 -m vscode \
    && apt-get install -y sudo \
    && echo 'vscode ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

ENV GOCACHE=/tmp/go-build
