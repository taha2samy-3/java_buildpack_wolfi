package jvm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy/java_buildpack_wolfi/internal/manifest"
)

type entry struct {
	name, body, link string
	typ              byte
}

func tarball(t *testing.T, entries []entry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o755, Linkname: e.link, Size: int64(len(e.body))}
		if e.typ != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractStripsAndKeepsLinks(t *testing.T) {
	a := tarball(t, []entry{
		{name: "jdk-21/", typ: tar.TypeDir},
		{name: "jdk-21/bin/java", body: "bin", typ: tar.TypeReg},
		{name: "jdk-21/legal/java.base/LICENSE", link: "../../LICENSE", typ: tar.TypeSymlink},
		{name: "jdk-21/LICENSE", body: "gpl", typ: tar.TypeReg},
	})
	dest := t.TempDir()
	if err := ExtractTarGz(a, dest, 1); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "bin", "java")); err != nil || string(b) != "bin" {
		t.Fatalf("bin/java: %v %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "legal", "java.base", "LICENSE")); err != nil || string(b) != "gpl" {
		t.Fatalf("symlink: %v %q", err, b)
	}
	info, _ := os.Stat(filepath.Join(dest, "bin", "java"))
	if info.Mode().Perm()&0o022 != 0 {
		t.Fatalf("group/world writable bits kept: %v", info.Mode())
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	cases := [][]entry{
		{{name: "root/../../evil", body: "x", typ: tar.TypeReg}},
		{{name: "root/link", link: "/etc/passwd", typ: tar.TypeSymlink}},
		{{name: "root/link", link: "../../../etc", typ: tar.TypeSymlink}},
		{{name: "root/dev", typ: tar.TypeChar}},
	}
	for i, c := range cases {
		dest := t.TempDir()
		err := ExtractTarGz(tarball(t, c), filepath.Join(dest, "out"), 1)
		if _, statErr := os.Stat(filepath.Join(dest, "evil")); statErr == nil {
			t.Fatalf("case %d wrote outside destination", i)
		}
		if err == nil && i != 0 {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestFetchVerifiesChecksumAndMirror(t *testing.T) {
	body := []byte("payload")
	sum := sha256.Sum256(body)
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Write(body)
	}))
	defer srv.Close()

	f := NewFetcher(map[string]string{"BP_DEPENDENCY_MIRROR": srv.URL + "/mirror/"}, nil)
	f.TempDir = t.TempDir()
	d := manifest.Dependency{Name: "x", URI: "https://example.com/a/b.tar.gz", SHA256: hex.EncodeToString(sum[:])}
	file, err := f.Fetch(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != "payload" {
		t.Fatalf("content %q", got)
	}
	if paths[0] != "/mirror/a/b.tar.gz" {
		t.Fatalf("mirror path %s", paths[0])
	}

	d.SHA256 = strings.Repeat("0", 64)
	f.Attempts = 3
	paths = nil
	if _, err := f.Fetch(context.Background(), d); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected checksum error, got %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("checksum mismatch must not retry, got %d attempts", len(paths))
	}
	entries, _ := os.ReadDir(f.TempDir)
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %d", len(entries))
	}
}

func TestFetchDependencyMapping(t *testing.T) {
	body := []byte("offline")
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	local := filepath.Join(t.TempDir(), "dep")
	os.WriteFile(local, body, 0o644)

	f := NewFetcher(nil, libcnb.Bindings{{Type: "dependency-mapping", Secret: map[string]string{sha: "file://" + local}}})
	f.TempDir = t.TempDir()
	if _, err := f.Fetch(context.Background(), manifest.Dependency{URI: "https://unreachable.invalid/x", SHA256: sha}); err != nil {
		t.Fatal(err)
	}
}
