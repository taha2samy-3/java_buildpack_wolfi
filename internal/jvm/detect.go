package jvm

import "github.com/buildpacks/libcnb/v2"

const (
	PlanJDK = "jdk"
	PlanJRE = "jre"
)

func Detect(libcnb.DetectContext) (libcnb.DetectResult, error) {
	return libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{Provides: []libcnb.BuildPlanProvide{{Name: PlanJDK}, {Name: PlanJRE}}},
			{Provides: []libcnb.BuildPlanProvide{{Name: PlanJDK}}},
			{Provides: []libcnb.BuildPlanProvide{{Name: PlanJRE}}},
		},
	}, nil
}
