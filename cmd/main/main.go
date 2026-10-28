package main

import (
	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/jvm"
)

func main() {
	libcnb.BuildpackMain(jvm.Detect, jvm.Build)
}
