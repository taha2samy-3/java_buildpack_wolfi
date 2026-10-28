package jvm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/taha2samy/java_buildpack_wolfi/internal/manifest"
)

type cdxHash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

type cdxLicense struct {
	Expression string `json:"expression"`
}

type cdxProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type cdxComponent struct {
	BOMRef     string        `json:"bom-ref"`
	Type       string        `json:"type"`
	Name       string        `json:"name"`
	Version    string        `json:"version"`
	PURL       string        `json:"purl"`
	CPE        string        `json:"cpe,omitempty"`
	Hashes     []cdxHash     `json:"hashes"`
	Licenses   []cdxLicense  `json:"licenses,omitempty"`
	Properties []cdxProperty `json:"properties,omitempty"`
}

type cdxBOM struct {
	BOMFormat    string         `json:"bomFormat"`
	SpecVersion  string         `json:"specVersion"`
	SerialNumber string         `json:"serialNumber"`
	Version      int            `json:"version"`
	Metadata     map[string]any `json:"metadata"`
	Components   []cdxComponent `json:"components"`
}

func WriteSBOM(path, buildpackID, buildpackVersion string, deps []manifest.Dependency) error {
	bom := cdxBOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: map[string]any{
			"tools": map[string]any{
				"components": []map[string]string{{"type": "application", "name": buildpackID, "version": buildpackVersion}},
			},
		},
	}
	seed := sha256.New()
	for _, d := range deps {
		seed.Write([]byte(d.SHA256))
		c := cdxComponent{
			BOMRef:  d.PURL,
			Type:    "library",
			Name:    d.Name,
			Version: d.Version,
			PURL:    d.PURL,
			Hashes:  []cdxHash{{"SHA-256", d.SHA256}},
			Properties: []cdxProperty{
				{"io.buildpacks.dependency.id", d.ID},
				{"io.buildpacks.dependency.uri", d.URI},
			},
		}
		if len(d.CPEs) > 0 {
			c.CPE = d.CPEs[0]
		}
		for _, l := range d.Licenses {
			c.Licenses = append(c.Licenses, cdxLicense{l})
		}
		bom.Components = append(bom.Components, c)
	}
	s := seed.Sum(nil)
	s[6] = (s[6] & 0x0f) | 0x50
	s[8] = (s[8] & 0x3f) | 0x80
	bom.SerialNumber = fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", s[0:4], s[4:6], s[6:8], s[8:10], s[10:16])

	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
