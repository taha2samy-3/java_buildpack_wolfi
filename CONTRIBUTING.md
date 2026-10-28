# Contributing

Thanks for helping. This document explains how to work on the project.

## Workflow

1. Open an issue for anything larger than a small fix, so the approach can be agreed first.
2. Create a branch from `main` and keep pull requests focused.
3. Use [Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `deps:`, `docs:`, `ci:`, `refactor:`, `test:`.
4. Run `task check` before pushing. CI also runs `golangci-lint`, `govulncheck`, CodeQL and the integration scenarios.

## Layout

| Path | Contents |
| --- | --- |
| `buildpacks/jvm` | JVM buildpack manifest, dependency pins and the FIPS self-test source |
| `buildpacks/java` | Composite buildpack order and package configuration |
| `builder` | Builder configuration template |
| `stacks` | Build and run image Dockerfiles |
| `cmd/main` | Buildpack `detect` and `build` entry point |
| `cmd/helper` | `exec.d` launch helpers |
| `cmd/update-deps` | Temurin dependency updater |
| `cmd/site` | Documentation data export and release notes |
| `internal` | Buildpack logic and unit tests |
| `integration` | End-to-end scenarios (`-tags integration`) |
| `samples` | Sample applications used by the integration tests |
| `site` | Astro documentation site |

## Dependencies

- Temurin releases are updated by `go run ./cmd/update-deps`, which also runs weekly in CI.
- `bc-fips` stays on the CMVP validated release. Change it only when NIST lists a newer version on a certificate, and update the documentation in the same pull request.
- GitHub Actions are pinned by commit SHA and updated by Dependabot.

## Releases

Maintainers push a `vX.Y.Z` tag on `main`. The release workflow builds, signs and attests every image, publishes the GitHub release and redeploys the documentation, which picks up the new release automatically.
