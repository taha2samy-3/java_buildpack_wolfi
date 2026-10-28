package classcount

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func jar(t *testing.T, path string, entries map[string][]byte, method uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		w.Write(body)
	}
	z.Close()
	if path != "" {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, buf.Bytes(), 0o644)
	}
	return buf.Bytes()
}

func TestTree(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "classes", "a"), 0o755)
	os.WriteFile(filepath.Join(dir, "classes", "a", "A.class"), nil, 0o644)
	inner := jar(t, "", map[string][]byte{"x/X.class": nil, "x/Y.class": nil}, zip.Store)
	jar(t, filepath.Join(dir, "lib", "dep.jar"), map[string][]byte{"d/D.class": nil, "d/res.txt": nil}, zip.Deflate)
	jar(t, filepath.Join(dir, "app.jar"), map[string][]byte{"BOOT-INF/lib/inner.jar": inner, "B.class": nil}, zip.Store)

	if got := Tree(dir); got != 5 {
		t.Fatalf("got %d", got)
	}
}

func TestJVMJimage(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, "lib"), 0o755)
	header := make([]byte, 28)
	binary.LittleEndian.PutUint32(header[0:], jimageMagic)
	binary.LittleEndian.PutUint32(header[12:], 31337)
	os.WriteFile(filepath.Join(home, "lib", "modules"), header, 0o644)
	if got := JVM(home); got != 31337 {
		t.Fatalf("got %d", got)
	}
}
