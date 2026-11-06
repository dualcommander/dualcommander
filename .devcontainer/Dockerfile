FROM golang:1.27-trixie

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git python3 make gcc libc6-dev pkg-config \
    libgl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols \
    rpm cpio desktop-file-utils xz-utils \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 developer \
    && useradd --uid 1000 --gid developer --create-home --shell /bin/bash developer \
    && mkdir -p /workspaces/project /home/developer/.cline/data/settings /home/developer/go \
    && chown -R developer:developer /workspaces /home/developer

ENV GOPATH=/home/developer/go
ENV PATH=/home/developer/go/bin:/usr/local/go/bin:${PATH}
# soft Go heap target for small Docker VMs; override for larger workloads.
ENV GOMEMLIMIT=2GiB
COPY .devcontainer/vscode-memory.cjs /usr/local/lib/vscode-memory.cjs
USER developer
RUN go install golang.org/x/tools/gopls@v0.24.0 \
    && go install github.com/go-delve/delve/cmd/dlv@v1.27.2
WORKDIR /workspaces/project
CMD ["sleep", "infinity"]
