package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/manifest"
)

type Options struct {
	Root, Out, Repository, ReleasesFile, Token string
	Offline                                    bool
}

type Release struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	BodyHTML    template.HTML `json:"-"`
}

func (r Release) Version() string { return strings.TrimPrefix(r.TagName, "v") }

type Page struct {
	Slug        string
	Title       string
	Nav         string
	Description string
	Order       int
	Content     template.HTML
	TOC         []Heading
}

type Heading struct {
	ID, Text string
}

type Runtime struct {
	Java     string
	JDK, JRE string
	LTS      bool
}

type Site struct {
	Pages      []Page
	Page       Page
	Releases   []Release
	Latest     string
	Repository string
	Owner      string
	Registry   string
	Build      []manifest.Configuration
	Launch     []manifest.Configuration
	Runtimes   []Runtime
	FIPS       []manifest.Dependency
	Groups     []string
	Year       int
	Generated  string
}

var funcs = template.FuncMap{
	"date": func(t time.Time) string { return t.Format("2 January 2006") },
}

var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Typographer),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

func Build(o Options) error {
	m, err := manifest.Load(filepath.Join(o.Root, "buildpacks", "jvm"))
	if err != nil {
		return err
	}
	owner, _, _ := strings.Cut(o.Repository, "/")
	s := Site{
		Repository: o.Repository,
		Owner:      owner,
		Registry:   "ghcr.io/" + strings.ToLower(owner),
		Year:       time.Now().UTC().Year(),
		Generated:  time.Now().UTC().Format("2006-01-02"),
		Latest:     "latest",
	}
	for _, c := range m.Configurations {
		if c.Build {
			s.Build = append(s.Build, c)
		} else {
			s.Launch = append(s.Launch, c)
		}
	}
	for _, java := range m.JavaVersions {
		r := Runtime{Java: java, LTS: true}
		if d, err := m.Find("jdk", java, "amd64"); err == nil {
			r.JDK = d.Version
		}
		if d, err := m.Find("jre", java, "amd64"); err == nil {
			r.JRE = d.Version
		}
		s.Runtimes = append(s.Runtimes, r)
	}
	s.FIPS = m.FindAll("bc-fips", "amd64")
	s.Groups, err = groups(filepath.Join(o.Root, "buildpacks", "java", "buildpack.toml"))
	if err != nil {
		return err
	}

	s.Releases, err = releases(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: releases unavailable: %v\n", err)
	}
	for i := range s.Releases {
		var buf bytes.Buffer
		if err := markdown.Convert([]byte(s.Releases[i].Body), &buf); err != nil {
			return err
		}
		s.Releases[i].BodyHTML = template.HTML(buf.String())
	}
	if len(s.Releases) > 0 {
		s.Latest = s.Releases[0].Version()
	}

	s.Pages, err = pages(filepath.Join(o.Root, "site", "content"), &s)
	if err != nil {
		return err
	}

	layout, err := template.New("layout.html").Funcs(funcs).ParseFiles(filepath.Join(o.Root, "site", "templates", "layout.html"))
	if err != nil {
		return err
	}

	if err := os.RemoveAll(o.Out); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(o.Root, "site", "static"), o.Out); err != nil {
		return err
	}
	for _, p := range s.Pages {
		s.Page = p
		file := filepath.Join(o.Out, p.Slug+".html")
		var buf bytes.Buffer
		if err := layout.Execute(&buf, s); err != nil {
			return fmt.Errorf("rendering %s: %w", p.Slug, err)
		}
		if err := os.WriteFile(file, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	feed, err := json.MarshalIndent(s.Releases, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(o.Out, "releases.json"), feed, 0o644)
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

var shortcode = regexp.MustCompile(`(?:<p>)?\[\[([a-z-]+)(?: ([a-z]+))?\]\](?:</p>)?`)

func pages(dir string, s *Site) ([]Page, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	partials, err := template.New("partials").Funcs(funcs).ParseGlob(filepath.Join(dir, "..", "templates", "partials", "*.html"))
	if err != nil {
		return nil, err
	}
	var out []Page
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		p, body := frontMatter(string(raw))
		p.Slug = strings.TrimSuffix(filepath.Base(f), ".md")

		var buf bytes.Buffer
		if err := markdown.Convert([]byte(body), &buf); err != nil {
			return nil, err
		}
		html := buf.String()
		var renderErr error
		html = shortcode.ReplaceAllStringFunc(html, func(match string) string {
			m := shortcode.FindStringSubmatch(match)
			if m[1] == "latest" {
				return strings.Replace(match, m[0], template.HTMLEscapeString(s.Latest), 1)
			}
			var b bytes.Buffer
			if err := partials.ExecuteTemplate(&b, m[1], map[string]any{"Site": s, "Arg": m[2]}); err != nil {
				renderErr = fmt.Errorf("%s: shortcode %s: %w", f, m[1], err)
			}
			return b.String()
		})
		if renderErr != nil {
			return nil, renderErr
		}
		p.Content = template.HTML(html)
		p.TOC = headings(html)
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}

var h2 = regexp.MustCompile(`<h2 id="([^"]+)">(.*?)</h2>`)
var tags = regexp.MustCompile(`<[^>]+>`)

func headings(html string) []Heading {
	var out []Heading
	for _, m := range h2.FindAllStringSubmatch(html, -1) {
		out = append(out, Heading{ID: m[1], Text: tags.ReplaceAllString(m[2], "")})
	}
	return out
}

func frontMatter(raw string) (Page, string) {
	var p Page
	head, body, ok := strings.Cut(raw, "\n---\n")
	if !ok {
		return p, raw
	}
	for _, line := range strings.Split(head, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "title":
			p.Title = v
		case "nav":
			p.Nav = v
		case "description":
			p.Description = v
		case "order":
			p.Order, _ = strconv.Atoi(v)
		}
	}
	if p.Nav == "" {
		p.Nav = p.Title
	}
	return p, body
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
