package jvm

import (
	"testing"

	"github.com/buildpacks/libcnb/v2"
)

func buildContext(t *testing.T, env map[string]string, entries ...libcnb.BuildpackPlanEntry) libcnb.BuildContext {
	return libcnb.BuildContext{
		ApplicationPath: t.TempDir(),
		Buildpack:       libcnb.Buildpack{Path: "../../buildpacks/jvm"},
		Platform:        libcnb.Platform{Environment: env},
		Plan:            libcnb.BuildpackPlan{Entries: entries},
		TargetInfo:      libcnb.TargetInfo{Arch: "arm64"},
	}
}

func TestConfigure(t *testing.T) {
	m := mustManifest(t)
	cases := []struct {
		name    string
		env     map[string]string
		entries []libcnb.BuildpackPlanEntry
		check   func(Config) bool
	}{
		{"maven", nil, []libcnb.BuildpackPlanEntry{{Name: "jdk"}, {Name: "jre", Metadata: map[string]any{"launch": true}}},
			func(c Config) bool {
				return c.NeedJDK && c.Launch && c.Type == "JRE" && !c.FIPS && c.Java.Version == "21" && c.Arch == "arm64"
			}},
		{"build only", nil, []libcnb.BuildpackPlanEntry{{Name: "jdk"}},
			func(c Config) bool { return c.NeedJDK && !c.Launch }},
		{"jre without metadata launches", nil, []libcnb.BuildpackPlanEntry{{Name: "jre"}},
			func(c Config) bool { return !c.NeedJDK && c.Launch }},
		{"jre build", nil, []libcnb.BuildpackPlanEntry{{Name: "jre", Metadata: map[string]any{"launch": true, "build": true}}},
			func(c Config) bool { return c.JREBuild }},
		{"fips jdk", map[string]string{"BP_JVM_FIPS_ENABLED": "true", "BP_JVM_TYPE": "jdk", "BP_JVM_VERSION": "8"}, []libcnb.BuildpackPlanEntry{{Name: "jre"}},
			func(c Config) bool { return c.FIPS && c.Type == "JDK" && c.Java.Version == "8" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Configure(buildContext(t, tc.env, tc.entries...), m)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.check(c) {
				t.Fatalf("%+v", c)
			}
		})
	}
}

func TestConfigureValidation(t *testing.T) {
	m := mustManifest(t)
	for _, env := range []map[string]string{{"BP_JVM_TYPE": "JVM"}, {"BP_JVM_FIPS_ENABLED": "maybe"}, {"BP_JVM_VERSION": "23"}} {
		if _, err := Configure(buildContext(t, env), m); err == nil {
			t.Fatalf("expected error for %v", env)
		}
	}
}

func TestDetect(t *testing.T) {
	r, err := Detect(libcnb.DetectContext{})
	if err != nil || !r.Pass || len(r.Plans) != 3 {
		t.Fatalf("%+v %v", r, err)
	}
	for _, p := range r.Plans {
		if len(p.Requires) != 0 {
			t.Fatal("detect must not require its own provisions")
		}
	}
}
