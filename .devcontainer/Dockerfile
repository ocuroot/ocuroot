FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

# Install Go and templ
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends golang-go \
    && go install github.com/a-h/templ/cmd/templ@latest \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Add Go bin to PATH
ENV PATH="/home/vscode/go/bin:${PATH}"
