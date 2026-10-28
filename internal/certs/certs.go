package certs

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/buildpacks/libcnb/v2"
	"software.sslmate.com/src/go-pkcs12"
)

func FromBindings(bindings libcnb.Bindings) ([]string, error) {
	var paths []string
	for _, b := range bindings {
		if !strings.EqualFold(b.Type, "ca-certificates") {
			continue
		}
		for name := range b.Secret {
			if p, ok := b.SecretFilePath(name); ok {
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func Load(paths []string) ([]*x509.Certificate, error) {
	seen := map[[32]byte]bool{}
	var out []*x509.Certificate
	for _, p := range paths {
		files, err := expand(p)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			for {
				var block *pem.Block
				block, data = pem.Decode(data)
				if block == nil {
					break
				}
				if block.Type != "CERTIFICATE" {
					continue
				}
				c, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", f, err)
				}
				sum := sha256.Sum256(c.Raw)
				if !seen[sum] {
					seen[sum] = true
					out = append(out, c)
				}
			}
		}
	}
	return out, nil
}

func expand(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() || e.Type()&os.ModeSymlink != 0 {
			full := filepath.Join(p, e.Name())
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				files = append(files, full)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func WritePKCS12(certs []*x509.Certificate, password, dest string) error {
	entries := make([]pkcs12.TrustStoreEntry, len(certs))
	for i, c := range certs {
		sum := sha256.Sum256(c.Raw)
		entries[i] = pkcs12.TrustStoreEntry{Cert: c, FriendlyName: fmt.Sprintf("extra-%x", sum[:8])}
	}
	data, err := pkcs12.Modern2023.EncodeTrustStoreEntries(entries, password)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o600)
}
