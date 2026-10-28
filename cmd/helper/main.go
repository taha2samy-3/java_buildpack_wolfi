package main

import (
	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy/java_buildpack_wolfi/internal/helper"
	"github.com/taha2samy/java_buildpack_wolfi/internal/jvm"
)

func main() {
	libcnb.RunExecD(map[string]libcnb.ExecD{
		jvm.ExecCACerts:    helper.NewCACertificates(),
		jvm.ExecMemory:     helper.NewMemoryCalculator(),
		jvm.ExecJVMOptions: helper.NewJVMOptions(),
	})
}
