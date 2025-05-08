FROM pulumi/pulumi-go:3.159.0

RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | tee /etc/apt/sources.list.d/goreleaser.list && \
    apt-get update && apt-get install -y \
    wget \
    build-essential \
    goreleaser \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/go-delve/delve/cmd/dlv@latest

RUN curl -fsSL https://get.pulumi.com | sh -s -- --version 3.159.0 --install-root /pulumi

RUN wget https://github.com/pulumi/pulumictl/releases/download/v0.0.48/pulumictl-v0.0.48-linux-amd64.tar.gz && \
    tar -xvzf pulumictl-v0.0.48-linux-amd64.tar.gz && \
    mv pulumictl /usr/local/bin && \
    chmod +x /usr/local/bin/pulumictl && \
    rm pulumictl-v0.0.48-linux-amd64.tar.gz


RUN pulumi version && pulumictl version && goreleaser --version