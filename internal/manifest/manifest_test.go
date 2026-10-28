package manifest

import "testing"

func TestBuildpackManifest(t *testing.T) {
	m, err := Load("../../buildpacks/jvm")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.JavaVersions) == 0 {
		t.Fatal("no java versions")
	}
	for _, java := range m.JavaVersions {
		for _, id := range []string{"jdk", "jre"} {
			for _, arch := range []string{"amd64", "arm64"} {
				d, err := m.Find(id, java, arch)
				if err != nil {
					t.Fatal(err)
				}
				if len(d.SHA256) != 64 || d.PURL == "" || len(d.Licenses) == 0 {
					t.Fatalf("incomplete dependency %+v", d)
				}
			}
		}
	}
	bc := m.FindAll("bc-fips", "amd64")
	if len(bc) != 3 {
		t.Fatalf("expected 3 Bouncy Castle jars, got %d", len(bc))
	}
	for _, d := range bc {
		if d.Name == "Bouncy Castle FIPS Java API" && d.Version != "2.1.1" {
			t.Fatalf("bc-fips must stay on the CMVP validated release, got %s", d.Version)
		}
	}
	if m.Default("BP_JVM_FIPS_ENABLED") != "false" || !m.Supports("8") {
		t.Fatal("unexpected defaults")
	}
}
