package manifest

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

type Dependency struct {
	ID       string   `toml:"id"`
	Name     string   `toml:"name"`
	Version  string   `toml:"version"`
	Java     string   `toml:"java,omitempty"`
	Arch     string   `toml:"arch"`
	URI      string   `toml:"uri"`
	SHA256   string   `toml:"sha256"`
	PURL     string   `toml:"purl"`
	CPEs     []string `toml:"cpes,omitempty"`
	Licenses []string `toml:"licenses"`
}

type Configuration struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Default     string `toml:"default"`
	Build       bool   `toml:"build"`
	Launch      bool   `toml:"launch"`
}

type Manifest struct {
	Dependencies   []Dependency    `toml:"dependencies"`
	Configurations []Configuration `toml:"configurations"`
	JavaVersions   []string        `toml:"java-versions"`
}

type buildpackTOML struct {
	Metadata Manifest `toml:"metadata"`
}

func Load(buildpackDir string) (Manifest, error) {
	var b buildpackTOML
	path := filepath.Join(buildpackDir, "buildpack.toml")
	if _, err := toml.DecodeFile(path, &b); err != nil {
		return Manifest{}, fmt.Errorf("decoding %s: %w", path, err)
	}
	return b.Metadata, nil
}

func (m Manifest) Find(id, java, arch string) (Dependency, error) {
	for _, d := range m.Dependencies {
		if d.ID == id && d.Java == java && (d.Arch == arch || d.Arch == "noarch") {
			return d, nil
		}
	}
	return Dependency{}, fmt.Errorf("no dependency %q for java %q on %s in buildpack.toml", id, java, arch)
}

func (m Manifest) FindAll(id, arch string) []Dependency {
	var out []Dependency
	for _, d := range m.Dependencies {
		if d.ID == id && (d.Arch == arch || d.Arch == "noarch") {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (m Manifest) Default(name string) string {
	for _, c := range m.Configurations {
		if c.Name == name {
			return c.Default
		}
	}
	return ""
}

func (m Manifest) Supports(java string) bool {
	for _, v := range m.JavaVersions {
		if v == java {
			return true
		}
	}
	return false
}
