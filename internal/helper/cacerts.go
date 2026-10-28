package helper

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/certs"
	"github.com/taha2samy-3/java_buildpack_wolfi/internal/jvm"
)

type CACertificates struct {
	Getenv func(string) string
	Log    io.Writer
}

func NewCACertificates() CACertificates {
	return CACertificates{Getenv: os.Getenv, Log: os.Stderr}
}

func (c CACertificates) sources() ([]string, error) {
	var paths []string
	if root := c.Getenv(libcnb.EnvServiceBindings); root != "" {
		bindings, err := libcnb.NewBindingsFromPath(root)
		if err != nil {
			return nil, err
		}
		p, err := certs.FromBindings(bindings)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p...)
	}
	for _, p := range filepath.SplitList(c.Getenv("BPL_JVM_CA_CERTIFICATES")) {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (c CACertificates) Execute() (map[string]string, error) {
	paths, err := c.sources()
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	list, err := certs.Load(paths)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}

	home := c.Getenv("JAVA_HOME")
	base := c.Getenv("BPI_JVM_TRUSTSTORE")
	if home == "" || base == "" {
		return nil, fmt.Errorf("JAVA_HOME and BPI_JVM_TRUSTSTORE must be set to add CA certificates")
	}
	storeType := c.Getenv("BPI_JVM_TRUSTSTORE_TYPE")

	dir, err := os.MkdirTemp("", "jvm-truststore-")
	if err != nil {
		return nil, fmt.Errorf("creating writable truststore directory (is /tmp writable?): %w", err)
	}
	dest := filepath.Join(dir, filepath.Base(base))
	if err := jvm.CopyFile(base, dest, 0o600); err != nil {
		return nil, err
	}
	p12 := filepath.Join(dir, "extra.p12")
	if err := certs.WritePKCS12(list, jvm.TruststorePassword, p12); err != nil {
		return nil, err
	}
	defer os.Remove(p12)

	if err := jvm.ImportTruststore(home, p12, dest, storeType, filepath.SplitList(c.Getenv("BPI_JVM_FIPS_JARS"))); err != nil {
		return nil, err
	}
	fmt.Fprintf(c.Log, "Added %d CA certificate(s) to the JVM truststore\n", len(list))

	flags := []string{"-Djavax.net.ssl.trustStore=" + dest, "-Djavax.net.ssl.trustStorePassword=" + jvm.TruststorePassword}
	if storeType != "" {
		flags = append(flags, "-Djavax.net.ssl.trustStoreType="+storeType)
	}
	return map[string]string{"JAVA_TOOL_OPTIONS": Join(c.Getenv("JAVA_TOOL_OPTIONS"), flags...)}, nil
}
