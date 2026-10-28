#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
JDK_IMAGE="docker.io/library/eclipse-temurin:21-jdk@sha256:5d90b26c0784ccf55af55f97cbcf7f3969c66a06299c93bdc72649219942a48f"

usage() {
  cat >&2 <<EOF
usage:
  $0 jvm <version> <out-dir> [archs...]
  $0 render <template> <out-file> KEY=VALUE...
EOF
  exit 2
}

compile_selftest() {
  local out="$1"
  mkdir -p "$out"
  if command -v javac >/dev/null 2>&1; then
    javac --release 8 -Xlint:-options -d "$out" "$ROOT/buildpacks/jvm/selftest/FipsSelfTest.java"
  else
    docker run --rm --user "$(id -u):$(id -g)" \
      -v "$ROOT/buildpacks/jvm/selftest:/src:ro" -v "$out:/out" \
      "$JDK_IMAGE" javac --release 8 -Xlint:-options -d /out /src/FipsSelfTest.java
  fi
}

package_jvm() {
  local version="$1" out="$2"
  shift 2
  local archs=("$@")
  [[ ${#archs[@]} -gt 0 ]] || archs=(amd64 arm64)

  rm -rf "$out"
  mkdir -p "$out"
  out="$(cd "$out" && pwd)"

  sed "s/{{VERSION}}/${version}/g" "$ROOT/buildpacks/jvm/buildpack.toml" >"$out/buildpack.toml"
  local classes
  classes="$(mktemp -d)"
  compile_selftest "$classes"

  for arch in "${archs[@]}"; do
    local platform="$out/linux/$arch"
    local bin="$platform/bin"
    mkdir -p "$bin" "$platform/selftest"
    cp "$ROOT/LICENSE" "$ROOT/README.md" "$platform/"
    cp "$classes/FipsSelfTest.class" "$platform/selftest/"
    for cmd in main helper; do
      (cd "$ROOT" && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -mod=readonly -trimpath \
        -ldflags="-s -w -buildid=" -o "$bin/$cmd" "./cmd/$cmd")
    done
    ln -sf main "$bin/detect"
    ln -sf main "$bin/build"
  done

  rm -rf "$classes"
  find "$out" -exec touch -h -d @0 {} +
}

render() {
  local template="$1" out="$2"
  shift 2
  local content
  content="$(cat "$template")"
  for kv in "$@"; do
    content="${content//\{\{${kv%%=*}\}\}/${kv#*=}}"
  done
  if grep -q '{{[A-Z_]*}}' <<<"$content"; then
    echo "unresolved placeholders in $template:" >&2
    grep -o '{{[A-Z_]*}}' <<<"$content" | sort -u >&2
    exit 1
  fi
  mkdir -p "$(dirname "$out")"
  printf '%s\n' "$content" >"$out"
}

[[ $# -ge 1 ]] || usage
case "$1" in
  jvm)
    [[ $# -ge 3 ]] || usage
    shift
    package_jvm "$@"
    ;;
  render)
    [[ $# -ge 3 ]] || usage
    shift
    render "$@"
    ;;
  *) usage ;;
esac
