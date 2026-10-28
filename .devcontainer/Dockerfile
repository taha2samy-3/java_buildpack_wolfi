FROM mcr.microsoft.com/devcontainers/go:1.26-trixie@sha256:3f36febaf3729a34fc796e4126899d64baa1271a9d99f2224c18679abc69efa1

ARG PACK_VERSION=0.40.10
ARG PACK_SHA256=687c3616e21824a449b56d4f989613ca68c921c6cce670b4de4459b025233806
ARG TASK_VERSION=3.54.0
ARG TASK_SHA256=680859dbb4d881a9c72d4d9a8f510825450849af8567deacd7302c01124416fb

RUN set -eux; \
    cd /tmp; \
    curl -fsSLo pack.tgz "https://github.com/buildpacks/pack/releases/download/v${PACK_VERSION}/pack-v${PACK_VERSION}-linux.tgz"; \
    echo "${PACK_SHA256}  pack.tgz" | sha256sum -c -; \
    tar -xzf pack.tgz -C /usr/local/bin pack; \
    curl -fsSLo task.tgz "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_amd64.tar.gz"; \
    echo "${TASK_SHA256}  task.tgz" | sha256sum -c -; \
    tar -xzf task.tgz -C /usr/local/bin task; \
    rm -f pack.tgz task.tgz
