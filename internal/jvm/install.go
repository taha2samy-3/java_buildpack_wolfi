package jvm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy/java_buildpack_wolfi/internal/manifest"
)

type Fetcher struct {
	Mirror   string
	Mappings map[string]string
	Client   *http.Client
	TempDir  string
	Attempts int
}

func NewFetcher(env map[string]string, bindings libcnb.Bindings) Fetcher {
	f := Fetcher{
		Mirror:   strings.TrimRight(env["BP_DEPENDENCY_MIRROR"], "/"),
		Mappings: map[string]string{},
		Client:   &http.Client{Timeout: 15 * time.Minute},
		TempDir:  os.TempDir(),
		Attempts: 3,
	}
	for _, b := range bindings {
		if !strings.EqualFold(b.Type, "dependency-mapping") {
			continue
		}
		for sha, uri := range b.Secret {
			f.Mappings[strings.ToLower(sha)] = uri
		}
	}
	return f
}

func (f Fetcher) source(d manifest.Dependency) (string, error) {
	if uri, ok := f.Mappings[d.SHA256]; ok {
		return uri, nil
	}
	if f.Mirror == "" {
		return d.URI, nil
	}
	u, err := url.Parse(d.URI)
	if err != nil {
		return "", err
	}
	m, err := url.Parse(f.Mirror)
	if err != nil {
		return "", fmt.Errorf("BP_DEPENDENCY_MIRROR: %w", err)
	}
	m.Path = path.Join(m.Path, u.Path)
	return m.String(), nil
}

func (f Fetcher) Fetch(ctx context.Context, d manifest.Dependency) (string, error) {
	src, err := f.source(d)
	if err != nil {
		return "", err
	}
	var last error
	for attempt := 1; attempt <= f.Attempts; attempt++ {
		file, err := f.fetchOnce(ctx, src, d.SHA256)
		if err == nil {
			return file, nil
		}
		last = err
		var mismatch checksumError
		if errors.As(err, &mismatch) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * 2 * time.Second):
		}
	}
	return "", fmt.Errorf("downloading %s %s: %w", d.Name, d.Version, last)
}

type checksumError struct{ want, got string }

func (e checksumError) Error() string {
	return fmt.Sprintf("sha256 mismatch: expected %s, got %s", e.want, e.got)
}

func (f Fetcher) fetchOnce(ctx context.Context, src, want string) (string, error) {
	out, err := os.CreateTemp(f.TempDir, "dependency-*")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			os.Remove(out.Name())
		}
	}()

	var body io.ReadCloser
	if strings.HasPrefix(src, "file://") {
		body, err = os.Open(strings.TrimPrefix(src, "file://"))
		if err != nil {
			return "", err
		}
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "taha2samy-wolfi-jvm-buildpack")
		resp, err := f.Client.Do(req)
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("GET %s: %s", src, resp.Status)
		}
		body = resp.Body
	}
	defer body.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), body); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", checksumError{want, got}
	}
	ok = true
	return out.Name(), nil
}

func ExtractTarGz(archive, dest string, strip int) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := stripComponents(hdr.Name, strip)
		if name == "" {
			continue
		}
		target, err := within(root, name)
		if err != nil {
			return err
		}
		mode := os.FileMode(hdr.Mode).Perm() &^ 0o022

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, mode|0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			linkTarget := hdr.Linkname
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("absolute symlink %s -> %s rejected", hdr.Name, linkTarget)
			}
			if _, err := within(root, filepath.Join(filepath.Dir(name), linkTarget)); err != nil {
				return fmt.Errorf("symlink %s escapes destination", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				return err
			}
		case tar.TypeLink:
			src, err := within(root, stripComponents(hdr.Linkname, strip))
			if err != nil {
				return err
			}
			if err := os.Link(src, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported entry type %q for %s", hdr.Typeflag, hdr.Name)
		}
	}
}

func stripComponents(name string, n int) string {
	parts := strings.Split(strings.TrimPrefix(path.Clean("/"+name), "/"), "/")
	if len(parts) <= n {
		return ""
	}
	return path.Join(parts[n:]...)
}

func within(root, name string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes %s", name, root)
	}
	return target, nil
}

func writeFile(target string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeFile(dst, in, mode)
}
