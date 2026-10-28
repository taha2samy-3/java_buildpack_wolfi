# Changelog

Release notes for every version are published automatically on [GitHub Releases](https://github.com/taha2samy-3/java_buildpack_wolfi/releases) and on the [documentation site](https://taha2samy-3.github.io/java_buildpack_wolfi/releases).

## 2.0.0 (unreleased)

Complete rewrite.

- Buildpack rewritten in Go with libcnb v2; no Python in the build image.
- Java version selection fixed: `BP_JVM_VERSION`, version manager files, Maven, Gradle and JAR manifests are honoured; Java 8, 11, 17, 21 and 25 supported.
- Dependencies pinned in `buildpack.toml` by SHA-256 instead of resolving "latest" at build time; offline mirrors and dependency mappings supported.
- FIPS mode is now optional (`BP_JVM_FIPS_ENABLED`), uses the CMVP validated `bc-fips` 2.1.1, removes non-FIPS JCE providers, keeps JDK security defaults and is verified by a build-time self-test.
- `BPL_*` variables are evaluated at launch instead of being frozen at build time.
- New memory calculator that budgets metaspace, threads, code cache and direct memory; the original percentage mode is still available.
- Tar extraction hardened against path traversal; failed downloads and keystore conversions now fail the build.
- New distroless run image built from scratch, Platform API compliant builder configuration, multi-arch images.
- SLSA Build Level 3 provenance, Sigstore signatures, SBOM attestations, CodeQL, Scorecard and Dependabot.
- Documentation site with automatically generated configuration and release pages.

## 1.0.1

Python implementation with a PyInstaller-based memory calculator.
