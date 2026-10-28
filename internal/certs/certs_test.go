package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buildpacks/libcnb/v2"
	"software.sslmate.com/src/go-pkcs12"
)

func pemCert(t *testing.T, cn string) []byte {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestLoadAndEncode(t *testing.T) {
	dir := t.TempDir()
	a, b := pemCert(t, "a"), pemCert(t, "b")
	os.WriteFile(filepath.Join(dir, "bundle.pem"), append(append([]byte{}, a...), b...), 0o644)
	sub := filepath.Join(dir, "d")
	os.Mkdir(sub, 0o755)
	os.WriteFile(filepath.Join(sub, "dup.crt"), a, 0o644)
	os.WriteFile(filepath.Join(sub, "notes.txt"), []byte("not a cert"), 0o644)

	list, err := Load([]string{filepath.Join(dir, "bundle.pem"), sub})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 unique certificates, got %d", len(list))
	}

	p12 := filepath.Join(dir, "out.p12")
	if err := WritePKCS12(list, "changeit", p12); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p12)
	decoded, err := pkcs12.DecodeTrustStore(data, "changeit")
	if err != nil || len(decoded) != 2 {
		t.Fatalf("%d %v", len(decoded), err)
	}
}

func TestFromBindings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ca.pem"), pemCert(t, "x"), 0o644)
	bindings := libcnb.Bindings{
		libcnb.NewBinding("certs", dir, map[string]string{"type": "ca-certificates", "ca.pem": "..."}),
		libcnb.NewBinding("other", "/nowhere", map[string]string{"type": "dependency-mapping", "x": "y"}),
	}
	paths, err := FromBindings(bindings)
	if err != nil || len(paths) != 1 || paths[0] != filepath.Join(dir, "ca.pem") {
		t.Fatalf("%v %v", paths, err)
	}
}
