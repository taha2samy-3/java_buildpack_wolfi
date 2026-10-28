package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/taha2samy/java_buildpack_wolfi/internal/manifest"
)

const marker = "[[metadata.dependencies]]"

var (
	javaVersionsLine = regexp.MustCompile(`(?m)^(\s*java-versions = )\[.*\]`)
	supportedList    = regexp.MustCompile(`(Java major version to install \()[0-9, ]+\)`)
)

var archs = map[string]string{"amd64": "x64", "arm64": "aarch64"}

type asset struct {
	Binary struct {
		Package struct {
			Checksum     string `json:"checksum"`
			ChecksumLink string `json:"checksum_link"`
			Link         string `json:"link"`
		} `json:"package"`
	} `json:"binary"`
	Version struct {
		Major    int    `json:"major"`
		Minor    int    `json:"minor"`
		Security int    `json:"security"`
		Semver   string `json:"semver"`
		OpenJDK  string `json:"openjdk_version"`
	} `json:"version"`
	ReleaseName string `json:"release_name"`
}

func main() {
	path := flag.String("buildpack", "buildpacks/jvm", "buildpack directory")
	flag.Parse()

	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	m, err := manifest.Load(dir)
	if err != nil {
		return err
	}

	var releases struct {
		LTS []int `json:"available_lts_releases"`
	}
	if err := getJSON(ctx, "https://api.adoptium.net/v3/info/available_releases", &releases); err != nil {
		return err
	}
	var versions []string
	for _, v := range releases.LTS {
		if v >= 8 {
			versions = append(versions, strconv.Itoa(v))
		}
	}
	if len(versions) == 0 {
		return fmt.Errorf("adoptium returned no LTS releases")
	}
	m.JavaVersions = versions

	var deps []manifest.Dependency
	for _, d := range m.Dependencies {
		if d.ID != "jdk" && d.ID != "jre" {
			deps = append(deps, d)
		}
	}

	for _, java := range m.JavaVersions {
		for _, kind := range []string{"jdk", "jre"} {
			for _, arch := range []string{"amd64", "arm64"} {
				d, err := resolve(ctx, java, kind, arch)
				if err != nil {
					return err
				}
				deps = append(deps, d)
			}
		}
	}

	file := dir + "/buildpack.toml"
	raw, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	head, _, _ := strings.Cut(string(raw), marker)
	quoted := make([]string, len(versions))
	for i, v := range versions {
		quoted[i] = strconv.Quote(v)
	}
	head = javaVersionsLine.ReplaceAllString(head, "${1}["+strings.Join(quoted, ", ")+"]")
	head = supportedList.ReplaceAllString(head, "${1}"+strings.Join(versions, ", ")+")")

	var b strings.Builder
	b.WriteString(strings.TrimRight(head, "\n"))
	b.WriteString("\n")
	for _, d := range deps {
		b.WriteString("\n")
		write(&b, d)
	}
	return os.WriteFile(file, []byte(b.String()), 0o644)
}

func resolve(ctx context.Context, java, kind, arch string) (manifest.Dependency, error) {
	q := url.Values{}
	q.Set("os", "linux")
	q.Set("architecture", archs[arch])
	q.Set("image_type", kind)
	q.Set("vendor", "eclipse")
	api := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/%s/hotspot?%s", java, q.Encode())

	var assets []asset
	if err := getJSON(ctx, api, &assets); err != nil {
		return manifest.Dependency{}, err
	}
	if len(assets) == 0 {
		return manifest.Dependency{}, fmt.Errorf("no Temurin %s %s for %s", kind, java, arch)
	}
	a := assets[0]
	pkg := a.Binary.Package

	if !strings.HasPrefix(pkg.Link, "https://github.com/adoptium/") {
		return manifest.Dependency{}, fmt.Errorf("unexpected download origin %s", pkg.Link)
	}
	published, err := getText(ctx, pkg.ChecksumLink)
	if err != nil {
		return manifest.Dependency{}, err
	}
	fields := strings.Fields(published)
	if len(fields) == 0 || !strings.EqualFold(fields[0], pkg.Checksum) {
		return manifest.Dependency{}, fmt.Errorf("checksum mismatch between Adoptium API and GitHub release for %s", pkg.Link)
	}

	version := strings.TrimSuffix(strings.TrimSuffix(a.Version.Semver, ".LTS"), ".0")
	cpeVersion := fmt.Sprintf("%d.%d.%d", a.Version.Major, a.Version.Minor, a.Version.Security)
	if a.Version.Major == 8 {
		cpeVersion = fmt.Sprintf("1.8.0:update%d", a.Version.Security)
	}

	name := "Eclipse Temurin JRE"
	if kind == "jdk" {
		name = "Eclipse Temurin JDK"
	}

	return manifest.Dependency{
		ID:       kind,
		Name:     name,
		Version:  version,
		Java:     java,
		Arch:     arch,
		URI:      pkg.Link,
		SHA256:   strings.ToLower(pkg.Checksum),
		PURL:     fmt.Sprintf("pkg:generic/eclipse-temurin-%s@%s?arch=%s&download_url=%s", kind, url.QueryEscape(a.ReleaseName), arch, url.QueryEscape(pkg.Link)),
		CPEs:     []string{fmt.Sprintf("cpe:2.3:a:eclipse:temurin:%s:*:*:*:*:*:*:*", cpeVersion), fmt.Sprintf("cpe:2.3:a:oracle:openjdk:%s:*:*:*:*:*:*:*", cpeVersion)},
		Licenses: []string{"GPL-2.0-only WITH Classpath-exception-2.0"},
	}, nil
}

func write(b *strings.Builder, d manifest.Dependency) {
	q := func(s string) string { out, _ := json.Marshal(s); return string(out) }
	list := func(items []string) string {
		out := make([]string, len(items))
		for i, s := range items {
			out[i] = q(s)
		}
		sort.Strings(out)
		return "[" + strings.Join(out, ", ") + "]"
	}

	b.WriteString(marker + "\n")
	fmt.Fprintf(b, "  id = %s\n", q(d.ID))
	fmt.Fprintf(b, "  name = %s\n", q(d.Name))
	fmt.Fprintf(b, "  version = %s\n", q(d.Version))
	if d.Java != "" {
		fmt.Fprintf(b, "  java = %s\n", q(d.Java))
	}
	fmt.Fprintf(b, "  arch = %s\n", q(d.Arch))
	fmt.Fprintf(b, "  uri = %s\n", q(d.URI))
	fmt.Fprintf(b, "  sha256 = %s\n", q(d.SHA256))
	fmt.Fprintf(b, "  purl = %s\n", q(d.PURL))
	if len(d.CPEs) > 0 {
		fmt.Fprintf(b, "  cpes = %s\n", list(d.CPEs))
	}
	fmt.Fprintf(b, "  licenses = %s\n", list(d.Licenses))
}

func get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "taha2samy-wolfi-jvm-update-deps")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

func getJSON(ctx context.Context, u string, v any) error {
	body, err := get(ctx, u)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

func getText(ctx context.Context, u string) (string, error) {
	body, err := get(ctx, u)
	return string(body), err
}
