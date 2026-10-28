title: Get started
order: 10
description: Build and run your first image in two commands.
---

## Requirements

- [pack](https://buildpacks.io/docs/for-platform-operators/how-to/integrate-ci/pack/) 0.36 or newer, or any platform that runs Cloud Native Buildpacks (kpack, Tekton, GitLab Auto DevOps, Spring Boot `bootBuildImage`).
- A Docker or Podman daemon, or a registry when using `--publish`.

## Build an application

Run `pack` from the root of a Maven, Gradle or sbt project, or point `--path` at a built JAR or WAR:

```sh
pack build my-app --builder ghcr.io/taha2samy-3/wolfi-java-builder:[[latest]]
```

Run it with a memory limit so the memory calculator can size the JVM:

```sh
docker run --rm -p 8080:8080 --memory 1g my-app
```

## Turn on FIPS mode

```sh
pack build my-app \
  --builder ghcr.io/taha2samy-3/wolfi-java-builder:[[latest]] \
  --env BP_JVM_FIPS_ENABLED=true
```

The build fails if the FIPS self-test does not pass, so an image that builds is an image whose crypto configuration was verified. See [FIPS 140-3 mode](fips.html).

## Pick the Java version

The version is resolved in this order:

1. `BP_JVM_VERSION`, for example `--env BP_JVM_VERSION=17`.
2. `.java-version`, `.sdkmanrc` or `.tool-versions`.
3. `pom.xml` (`maven.compiler.release`, `release`, `java.version`, `maven.compiler.target`, `maven.compiler.source`).
4. `build.gradle` or `build.gradle.kts` (toolchain `JavaLanguageVersion.of(..)`, `sourceCompatibility`, `targetCompatibility`).
5. `Build-Jdk-Spec` in `META-INF/MANIFEST.MF` of a prebuilt JAR.
6. The default, Java 21.

Hints from project files round up to the next supported LTS: a project targeting Java 22 gets Java 25. An unsupported value in `BP_JVM_VERSION` fails the build instead of silently picking another version.

## Use it from your build tool

Spring Boot Maven plugin:

```xml
<plugin>
  <groupId>org.springframework.boot</groupId>
  <artifactId>spring-boot-maven-plugin</artifactId>
  <configuration>
    <image>
      <builder>ghcr.io/taha2samy-3/wolfi-java-builder:[[latest]]</builder>
      <env>
        <BP_JVM_FIPS_ENABLED>true</BP_JVM_FIPS_ENABLED>
      </env>
    </image>
  </configuration>
</plugin>
```

Spring Boot Gradle plugin:

```groovy
tasks.named('bootBuildImage') {
    builder = 'ghcr.io/taha2samy-3/wolfi-java-builder:[[latest]]'
    environment = ['BP_JVM_FIPS_ENABLED': 'true']
}
```

## Use it in Kubernetes

The images run as user `1001` with no shell and work with a read-only root filesystem. Give the JVM a writable `/tmp`:

```yaml
securityContext:
  runAsNonRoot: true
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: RuntimeDefault
volumeMounts:
  - name: tmp
    mountPath: /tmp
```

## Supported application types

The builder detects the application type and combines the JVM buildpack with these Paketo buildpacks:

[[groups]]
