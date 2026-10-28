ARG BASE_IMAGE=cgr.dev/chainguard/wolfi-base@sha256:dbcc3a4a3b1b9e68420a4742940aa6267c97e61811736a5fef0918a06ade1827

FROM ${BASE_IMAGE}

ARG CNB_USER_ID=1001
ARG CNB_GROUP_ID=1001
ARG STACK_ID=io.github.taha2samy-3.stacks.wolfi

USER root
RUN apk add --no-cache \
      bash \
      ca-certificates-bundle \
      glibc \
      glibc-locale-posix \
      libgcc \
      libstdc++ \
      tzdata \
      zlib \
 && addgroup -g ${CNB_GROUP_ID} cnb \
 && adduser -D -u ${CNB_USER_ID} -G cnb -h /home/cnb -s /bin/bash cnb \
 && chmod 1777 /tmp

ENV CNB_USER_ID=${CNB_USER_ID} \
    CNB_GROUP_ID=${CNB_GROUP_ID} \
    LANG=C.UTF-8 \
    TZ=UTC

LABEL io.buildpacks.stack.id="${STACK_ID}" \
      io.buildpacks.base.distro.name="wolfi" \
      io.buildpacks.base.homepage="https://github.com/taha2samy-3/java_buildpack_wolfi" \
      io.buildpacks.base.maintainer="taha2samy-3" \
      org.opencontainers.image.title="Wolfi CNB build image" \
      org.opencontainers.image.source="https://github.com/taha2samy-3/java_buildpack_wolfi" \
      org.opencontainers.image.licenses="Apache-2.0"

USER ${CNB_USER_ID}:${CNB_GROUP_ID}
