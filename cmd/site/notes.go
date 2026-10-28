package main

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/manifest"
)

func Notes(w io.Writer, root, repo, version string, images []string) error {
	if version == "" {
		return fmt.Errorf("--version is required")
	}
	m, err := manifest.Load(filepath.Join(root, "buildpacks", "jvm"))
	if err != nil {
		return err
	}

	refs := map[string]string{}
	var names []string
	for _, i := range images {
		k, v, ok := strings.Cut(i, "=")
		if !ok {
			return fmt.Errorf("invalid --image %q", i)
		}
		refs[k] = v
		names = append(names, k)
	}
	sort.Strings(names)

	builder := refs["builder"]
	builderTag := builder
	if repoPart, _, ok := strings.Cut(builder, "@"); ok {
		builderTag = repoPart + ":" + version
	}

	fmt.Fprintf(w, "## Wolfi Java Buildpack %s\n\n", version)
	fmt.Fprintf(w, "Build any Java application into a distroless Wolfi image with Eclipse Temurin, optionally in FIPS 140-3 mode.\n\n")

	fmt.Fprintf(w, "### Quick start\n\n```sh\npack build my-app --builder %s\npack build my-app --builder %s --env BP_JVM_FIPS_ENABLED=true\n```\n\n", builderTag, builderTag)

	fmt.Fprintf(w, "### Artifacts\n\n| Artifact | Reference |\n| --- | --- |\n")
	for _, n := range names {
		fmt.Fprintf(w, "| %s | `%s` |\n", n, refs[n])
	}

	fmt.Fprintf(w, "\n### Java runtimes\n\n| Java | Temurin JDK | Temurin JRE |\n| --- | --- | --- |\n")
	for _, java := range m.JavaVersions {
		jdk, _ := m.Find("jdk", java, "amd64")
		jre, _ := m.Find("jre", java, "amd64")
		fmt.Fprintf(w, "| %s | %s | %s |\n", java, jdk.Version, jre.Version)
	}

	fmt.Fprintf(w, "\n### FIPS 140-3 module\n\n| Component | Version | Note |\n| --- | --- | --- |\n")
	for _, d := range m.FindAll("bc-fips", "amd64") {
		note := "outside the cryptographic boundary"
		if strings.Contains(d.PURL, "/bc-fips@") {
			note = "CMVP certificate #4943"
		}
		fmt.Fprintf(w, "| %s | %s | %s |\n", d.Name, d.Version, note)
	}

	identity := fmt.Sprintf("https://github.com/%s/.github/workflows/release.yml@refs/tags/v%s", repo, version)
	fmt.Fprintf(w, "\n### Verify\n\nEvery image is signed with Sigstore keyless signing, carries a CycloneDX SBOM attestation and SLSA Build Level 3 provenance.\n\n```sh\n")
	fmt.Fprintf(w, "slsa-verifier verify-image %s \\\n  --source-uri github.com/%s --source-tag v%s\n\n", builder, repo, version)
	fmt.Fprintf(w, "cosign verify %s \\\n  --certificate-identity %s \\\n  --certificate-oidc-issuer https://token.actions.githubusercontent.com\n\n", builder, identity)
	fmt.Fprintf(w, "cosign verify-attestation --type cyclonedx %s \\\n  --certificate-identity %s \\\n  --certificate-oidc-issuer https://token.actions.githubusercontent.com\n```\n", builder, identity)

	if changes := changelog(root, "v"+version); changes != "" {
		fmt.Fprintf(w, "\n### Changes\n\n%s\n", changes)
	}
	return nil
}

func changelog(root, tag string) string {
	prev, err := exec.Command("git", "-C", root, "describe", "--tags", "--abbrev=0", tag+"^").Output()
	rng := tag
	if err == nil {
		rng = strings.TrimSpace(string(prev)) + ".." + tag
	}
	out, err := exec.Command("git", "-C", root, "log", "--no-merges", "--pretty=format:- %s (%h)", rng).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
