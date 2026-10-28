ARG BASE_IMAGE=cgr.dev/chainguard/wolfi-base@sha256:dbcc3a4a3b1b9e68420a4742940aa6267c97e61811736a5fef0918a06ade1827

FROM ${BASE_IMAGE} AS rootfs

ARG CNB_USER_ID=1001
ARG CNB_GROUP_ID=1001

USER root
RUN mkdir -p /rootfs/etc/apk \
 && cp /etc/apk/repositories /rootfs/etc/apk/repositories \
 && cp -r /etc/apk/keys /rootfs/etc/apk/keys \
 && apk add --no-cache --initdb --no-scripts --root /rootfs \
      --keys-dir /etc/apk/keys --repositories-file /etc/apk/repositories \
      wolfi-baselayout \
      ca-certificates-bundle \
      glibc \
      glibc-locale-posix \
      ld-linux \
      libgcc \
      libstdc++ \
      tzdata \
      zlib \
 && ldconfig -r /rootfs \
 && rm -rf /rootfs/etc/apk/keys /rootfs/var/cache/apk \
 && grep -q '^cnb:' /rootfs/etc/passwd || echo "cnb:x:${CNB_USER_ID}:${CNB_GROUP_ID}:cnb:/home/cnb:/sbin/nologin" >> /rootfs/etc/passwd \
 && grep -q '^cnb:' /rootfs/etc/group || echo "cnb:x:${CNB_GROUP_ID}:" >> /rootfs/etc/group \
 && mkdir -p /rootfs/home/cnb /rootfs/tmp \
 && chown ${CNB_USER_ID}:${CNB_GROUP_ID} /rootfs/home/cnb \
 && chmod 1777 /rootfs/tmp

FROM scratch

ARG CNB_USER_ID=1001
ARG CNB_GROUP_ID=1001
ARG STACK_ID=io.github.taha2samy.stacks.wolfi

COPY --from=rootfs /rootfs /

ENV CNB_USER_ID=${CNB_USER_ID} \
    CNB_GROUP_ID=${CNB_GROUP_ID} \
    LANG=C.UTF-8 \
    TZ=UTC \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

LABEL io.buildpacks.stack.id="${STACK_ID}" \
      io.buildpacks.base.distro.name="wolfi" \
      io.buildpacks.base.homepage="https://github.com/taha2samy/java_buildpack_wolfi" \
      io.buildpacks.base.maintainer="taha2samy" \
      org.opencontainers.image.title="Wolfi CNB run image (distroless)" \
      org.opencontainers.image.source="https://github.com/taha2samy/java_buildpack_wolfi" \
      org.opencontainers.image.licenses="Apache-2.0"

USER ${CNB_USER_ID}:${CNB_GROUP_ID}
WORKDIR /home/cnb
