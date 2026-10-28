package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/manifest"
)

type Options struct {
	Root, Out, Repository, ReleasesFile, Token string
	Offline                                    bool
}

type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

type ReleaseData struct {
	Tag        string    `json:"tag"`
	Version    string    `json:"version"`
	URL        string    `json:"url"`
	Published  time.Time `json:"published"`
	Prerelease bool      `json:"prerelease"`
	HTML       string    `json:"html"`
}

type Runtime struct {
	Java string `json:"java"`
	JDK  string `json:"jdk"`
	JRE  string `json:"jre"`
}

type Variable struct {
	Name        string `json:"name"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

type Module struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	PURL    string `json:"purl"`
}

type Data struct {
	Repository string     `json:"repository"`
	Registry   string     `json:"registry"`
	Latest     string     `json:"latest"`
	Generated  string     `json:"generated"`
	Build      []Variable `json:"build"`
	Launch     []Variable `json:"launch"`
	Runtimes   []Runtime  `json:"runtimes"`
	FIPS       []Module   `json:"fips"`
	Groups     []string   `json:"groups"`
}

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

func Export(o Options) error {
	m, err := manifest.Load(filepath.Join(o.Root, "buildpacks", "jvm"))
	if err != nil {
		return err
	}
	owner, _, _ := strings.Cut(o.Repository, "/")
	d := Data{
		Repository: o.Repository,
		Registry:   "ghcr.io/" + strings.ToLower(owner),
		Latest:     "latest",
		Generated:  time.Now().UTC().Format("2006-01-02"),
	}
	for _, c := range m.Configurations {
		v := Variable{c.Name, c.Default, c.Description}
		if c.Build {
			d.Build = append(d.Build, v)
		} else {
			d.Launch = append(d.Launch, v)
		}
	}
	for _, java := range m.JavaVersions {
		r := Runtime{Java: java}
		if dep, err := m.Find("jdk", java, "amd64"); err == nil {
			r.JDK = dep.Version
		}
		if dep, err := m.Find("jre", java, "amd64"); err == nil {
			r.JRE = dep.Version
		}
		d.Runtimes = append(d.Runtimes, r)
	}
	for _, dep := range m.FindAll("bc-fips", "amd64") {
		d.FIPS = append(d.FIPS, Module{dep.Name, dep.Version, dep.SHA256, dep.PURL})
	}
	if d.Groups, err = groups(filepath.Join(o.Root, "buildpacks", "java", "buildpack.toml")); err != nil {
		return err
	}

	list, err := releases(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: releases unavailable: %v\n", err)
	}
	var out []ReleaseData
	for _, r := range list {
		var buf bytes.Buffer
		if err := markdown.Convert([]byte(r.Body), &buf); err != nil {
			return err
		}
		out = append(out, ReleaseData{
			Tag:        r.TagName,
			Version:    strings.TrimPrefix(r.TagName, "v"),
			URL:        r.HTMLURL,
			Published:  r.PublishedAt,
			Prerelease: r.Prerelease,
			HTML:       buf.String(),
		})
	}
	for _, r := range out {
		if !r.Prerelease {
			d.Latest = r.Version
			break
		}
	}

	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(o.Out, "buildpack.json"), d); err != nil {
		return err
	}
	if out == nil {
		out = []ReleaseData{}
	}
	return writeJSON(filepath.Join(o.Out, "releases.json"), out)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func groups(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range regexp.MustCompile(`id = "paketo-buildpacks/([a-z-]+)"`).FindAllStringSubmatch(string(raw), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out, nil
}

func releases(o Options) ([]Release, error) {
	var data []byte
	switch {
	case o.ReleasesFile != "":
		var err error
		if data, err = os.ReadFile(o.ReleasesFile); err != nil {
			return nil, err
		}
	case o.Offline:
		return nil, nil
	default:
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+o.Repository+"/releases?per_page=50", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if o.Token != "" {
			req.Header.Set("Authorization", "Bearer "+o.Token)
		}
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub API: %s", resp.Status)
		}
		if data, err = io.ReadAll(io.LimitReader(resp.Body, 20<<20)); err != nil {
			return nil, err
		}
	}
	var all []Release
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	var out []Release
	for _, r := range all {
		if !r.Draft {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}
