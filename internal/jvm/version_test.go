package jvm

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

var supported = []string{"8", "11", "17", "21", "25"}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name, file, content, env, want, source string
	}{
		{"default", "", "", "", "21", "default"},
		{"env wins", "pom.xml", "<java.version>17</java.version>", "11", "11", "BP_JVM_VERSION"},
		{"env legacy form", "", "", "1.8", "8", "BP_JVM_VERSION"},
		{"env wildcard", "", "", "17.*", "17", "BP_JVM_VERSION"},
		{"pom java.version", "pom.xml", "<properties><java.version>17</java.version></properties>", "", "17", "pom.xml"},
		{"pom release", "pom.xml", "<maven.compiler.release>11</maven.compiler.release>", "", "11", "pom.xml"},
		{"pom 1.8", "pom.xml", "<maven.compiler.source>1.8</maven.compiler.source>", "", "8", "pom.xml"},
		{"pom non-lts rounds up", "pom.xml", "<java.version>22</java.version>", "", "25", "pom.xml"},
		{"gradle toolchain", "build.gradle", "java { toolchain { languageVersion = JavaLanguageVersion.of(25) } }", "", "25", "build.gradle"},
		{"gradle compatibility", "build.gradle", "sourceCompatibility = '17'", "", "17", "build.gradle"},
		{"gradle enum", "build.gradle.kts", "java { sourceCompatibility = JavaVersion.VERSION_1_8 }", "", "8", "build.gradle.kts"},
		{"sdkmanrc", ".sdkmanrc", "java=21.0.4-tem\n", "", "21", ".sdkmanrc"},
		{"java-version", ".java-version", "temurin-17.0.2\n", "", "17", ".java-version"},
		{"tool-versions", ".tool-versions", "java temurin-11.0.20\n", "", "11", ".tool-versions"},
		{"manifest", "META-INF/MANIFEST.MF", "Manifest-Version: 1.0\nBuild-Jdk-Spec: 17\n", "", "17", "META-INF/MANIFEST.MF"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.file != "" {
				write(t, dir, tc.file, tc.content)
			}
			env := map[string]string{}
			if tc.env != "" {
				env["BP_JVM_VERSION"] = tc.env
			}
			r, err := Resolve(dir, env, supported, "21")
			if err != nil {
				t.Fatal(err)
			}
			if r.Version != tc.want || r.Source != tc.source {
				t.Fatalf("got %+v, want %s from %s", r, tc.want, tc.source)
			}
		})
	}
}

func TestResolveRejectsUnsupportedEnv(t *testing.T) {
	for _, v := range []string{"22", "abc", "7"} {
		if _, err := Resolve(t.TempDir(), map[string]string{"BP_JVM_VERSION": v}, supported, "21"); err == nil {
			t.Fatalf("expected error for %s", v)
		}
	}
}

func TestResolveFromJar(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "app.jar"))
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("META-INF/MANIFEST.MF")
	w.Write([]byte("Manifest-Version: 1.0\r\nBuild-Jdk-Spec: 11\r\n"))
	z.Close()
	f.Close()

	r, err := Resolve(dir, nil, supported, "21")
	if err != nil || r.Version != "11" {
		t.Fatalf("got %+v %v", r, err)
	}
}
