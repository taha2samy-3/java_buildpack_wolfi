package jvm

import (
	"testing"

	"github.com/taha2samy/java_buildpack_wolfi/internal/manifest"
)

func mustManifest(t *testing.T) manifest.Manifest {
	t.Helper()
	m, err := manifest.Load("../../buildpacks/jvm")
	if err != nil {
		t.Fatal(err)
	}
	return m
}
