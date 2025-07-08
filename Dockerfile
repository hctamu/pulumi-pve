FROM pulumi/pulumi-provider-build-environment:3.159.0-amd64

RUN apt-get update && apt-get install -y \
    zip \
    vim

RUN go install github.com/go-delve/delve/cmd/dlv@latest

RUN curl -s "https://get.sdkman.io" | bash
test
RUN chmod a+x "$HOME/.sdkman/bin/sdkman-init.sh"

RUN ["/bin/bash", "-c", "source $HOME/.sdkman/bin/sdkman-init.sh && \
    sdk install gradle 7.6 && \
    sdk install java 11.0.27-zulu"]
