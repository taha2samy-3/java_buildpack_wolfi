package jvm

import (
	"archive/zip"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Resolution struct {
	Version string
	Source  string
}

type hint struct {
	file    string
	pattern *regexp.Regexp
}

var hints = []hint{
	{".java-version", regexp.MustCompile(`(?m)^\s*(?:[a-z-]+-)?(1\.8|\d+)`)},
	{".sdkmanrc", regexp.MustCompile(`(?m)^\s*java\s*=\s*(1\.8|\d+)`)},
	{".tool-versions", regexp.MustCompile(`(?m)^\s*java\s+(?:[a-z]+-)*?(1\.8|\d+)`)},
	{"pom.xml", regexp.MustCompile(`<(?:maven\.compiler\.release|release|java\.version|maven\.compiler\.target|maven\.compiler\.source)>\s*(1\.8|\d+)\s*<`)},
	{"build.gradle", regexp.MustCompile(`(?:JavaLanguageVersion\.of\(\s*|(?:source|target)Compatibility\s*=\s*(?:JavaVersion\.VERSION_)?['"]?)(1[._]8|\d+)`)},
	{"build.gradle.kts", regexp.MustCompile(`(?:JavaLanguageVersion\.of\(\s*|(?:source|target)Compatibility\s*=\s*(?:JavaVersion\.VERSION_)?['"]?)(1[._]8|\d+)`)},
}

func Resolve(appDir string, env map[string]string, supported []string, fallback string) (Resolution, error) {
	if v := strings.TrimSpace(env["BP_JVM_VERSION"]); v != "" {
		major, err := normalize(v)
		if err != nil {
			return Resolution{}, fmt.Errorf("BP_JVM_VERSION=%q: %w", v, err)
		}
		if !contains(supported, major) {
			return Resolution{}, fmt.Errorf("BP_JVM_VERSION=%s is not supported, choose one of %s", major, strings.Join(supported, ", "))
		}
		return Resolution{major, "BP_JVM_VERSION"}, nil
	}

	for _, h := range hints {
		raw, err := os.ReadFile(filepath.Join(appDir, h.file))
		if err != nil {
			continue
		}
		m := h.pattern.FindSubmatch(raw)
		if m == nil {
			continue
		}
		if r, ok := atLeast(string(m[1]), supported); ok {
			return Resolution{r, h.file}, nil
		}
	}

	if spec := manifestSpec(appDir); spec != "" {
		if r, ok := atLeast(spec, supported); ok {
			return Resolution{r, "META-INF/MANIFEST.MF"}, nil
		}
	}

	return Resolution{fallback, "default"}, nil
}

func normalize(v string) (string, error) {
	v = strings.TrimSuffix(strings.TrimSpace(v), ".*")
	v = strings.TrimPrefix(v, "1.")
	if i := strings.IndexAny(v, ".+_-"); i > 0 {
		v = v[:i]
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return "", fmt.Errorf("not a Java version")
	}
	return strconv.Itoa(n), nil
}

func atLeast(requested string, supported []string) (string, bool) {
	r, err := normalize(strings.ReplaceAll(requested, "_", "."))
	if err != nil {
		return "", false
	}
	want, _ := strconv.Atoi(r)
	versions := make([]int, 0, len(supported))
	for _, s := range supported {
		if n, err := strconv.Atoi(s); err == nil {
			versions = append(versions, n)
		}
	}
	sort.Ints(versions)
	for _, v := range versions {
		if v >= want {
			return strconv.Itoa(v), true
		}
	}
	return "", false
}

func manifestSpec(appDir string) string {
	if f, err := os.Open(filepath.Join(appDir, "META-INF", "MANIFEST.MF")); err == nil {
		defer f.Close()
		return scanSpec(bufio.NewScanner(f))
	}
	jars, _ := filepath.Glob(filepath.Join(appDir, "*.jar"))
	if len(jars) != 1 {
		return ""
	}
	z, err := zip.OpenReader(jars[0])
	if err != nil {
		return ""
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name == "META-INF/MANIFEST.MF" {
			r, err := f.Open()
			if err != nil {
				return ""
			}
			defer r.Close()
			return scanSpec(bufio.NewScanner(r))
		}
	}
	return ""
}

func scanSpec(s *bufio.Scanner) string {
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), "Build-Jdk-Spec") {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
