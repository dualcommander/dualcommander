FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git python3 make gcc libc6-dev pkg-config \
    libgl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 developer \
    && useradd --uid 1000 --gid developer --create-home --shell /bin/bash developer \
    && mkdir -p /workspaces/dualcommander /home/developer/go \
    && chown -R developer:developer /workspaces /home/developer

ENV GOPATH=/home/developer/go
ENV PATH=/home/developer/go/bin:/usr/local/go/bin:${PATH}
USER developer
WORKDIR /workspaces/dualcommander
CMD ["sleep", "infinity"]