package jvm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/buildpacks/libcnb/v2"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/certs"
	"github.com/taha2samy-3/java_buildpack_wolfi/internal/classcount"
	"github.com/taha2samy-3/java_buildpack_wolfi/internal/manifest"
)

const (
	HelperName      = "helper"
	ExecCACerts     = "0-ca-certificates"
	ExecMemory      = "1-memory-calculator"
	ExecJVMOptions  = "2-jvm-options"
	fipsLayerName   = "fips"
	helperLayerName = "helper"
)

var ExecDHelpers = []string{ExecCACerts, ExecMemory, ExecJVMOptions}

type Config struct {
	Java      Resolution
	Type      string
	FIPS      bool
	Arch      string
	NeedJDK   bool
	Launch    bool
	JREBuild  bool
	ExtraCAs  []string
	CAHash    string
	Env       map[string]string
	Bindings  libcnb.Bindings
	AppPath   string
	BPPath    string
	BPID      string
	BPVersion string
}

type Builder struct {
	Out io.Writer
}

func Build(ctx libcnb.BuildContext) (libcnb.BuildResult, error) {
	return Builder{Out: os.Stdout}.Build(ctx)
}

func (b Builder) logf(format string, a ...any) {
	fmt.Fprintf(b.Out, "  "+format+"\n", a...)
}

func environment(platform map[string]string) map[string]string {
	env := map[string]string{}
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}
	for k, v := range platform {
		env[k] = v
	}
	return env
}

func Configure(ctx libcnb.BuildContext, m manifest.Manifest) (Config, error) {
	env := environment(ctx.Platform.Environment)
	c := Config{
		Env:       env,
		Bindings:  ctx.Platform.Bindings,
		AppPath:   ctx.ApplicationPath,
		BPPath:    ctx.Buildpack.Path,
		BPID:      ctx.Buildpack.Info.ID,
		BPVersion: ctx.Buildpack.Info.Version,
		Arch:      ctx.TargetInfo.Arch,
	}
	if c.Arch == "" {
		c.Arch = runtime.GOARCH
	}

	r, err := Resolve(ctx.ApplicationPath, env, m.JavaVersions, m.Default("BP_JVM_VERSION"))
	if err != nil {
		return Config{}, err
	}
	c.Java = r

	c.Type = strings.ToUpper(strings.TrimSpace(env["BP_JVM_TYPE"]))
	if c.Type == "" {
		c.Type = strings.ToUpper(m.Default("BP_JVM_TYPE"))
	}
	if c.Type != "JRE" && c.Type != "JDK" {
		return Config{}, fmt.Errorf("BP_JVM_TYPE must be JRE or JDK, got %q", env["BP_JVM_TYPE"])
	}

	fips := strings.TrimSpace(env["BP_JVM_FIPS_ENABLED"])
	if fips == "" {
		fips = m.Default("BP_JVM_FIPS_ENABLED")
	}
	c.FIPS, err = strconv.ParseBool(fips)
	if err != nil {
		return Config{}, fmt.Errorf("BP_JVM_FIPS_ENABLED must be true or false, got %q", fips)
	}

	for _, e := range ctx.Plan.Entries {
		switch e.Name {
		case PlanJDK:
			c.NeedJDK = true
			if flag(e.Metadata, "launch") {
				c.Launch = true
			}
		case PlanJRE:
			c.Launch = c.Launch || !hasFalse(e.Metadata, "launch")
			if flag(e.Metadata, "build") {
				c.JREBuild = true
			}
		}
	}

	c.ExtraCAs, err = certs.FromBindings(ctx.Platform.Bindings)
	if err != nil {
		return Config{}, err
	}
	c.CAHash, err = hashFiles(c.ExtraCAs)
	return c, err
}

func flag(m map[string]any, key string) bool {
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

func hasFalse(m map[string]any, key string) bool {
	if _, ok := m[key]; !ok {
		return false
	}
	return !flag(m, key)
}

func hashFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	h := sha256.New()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		h.Write([]byte(p))
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (b Builder) Build(ctx libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

	m, err := manifest.Load(ctx.Buildpack.Path)
	if err != nil {
		return result, err
	}
	c, err := Configure(ctx, m)
	if err != nil {
		return result, err
	}

	fmt.Fprintf(b.Out, "%s %s\n", ctx.Buildpack.Info.Name, ctx.Buildpack.Info.Version)
	b.logf("Java %s (from %s), runtime %s, FIPS %t, arch %s", c.Java.Version, c.Java.Source, c.Type, c.FIPS, c.Arch)
	if !c.NeedJDK && !c.Launch {
		b.logf("No buildpack requires a JDK or JRE, skipping")
		return result, nil
	}

	fetcher := NewFetcher(c.Env, c.Bindings)
	bgctx := context.Background()

	var runtimeHome string
	var runtimeDep manifest.Dependency

	if c.NeedJDK || (c.Launch && c.Type == "JDK") {
		dep, err := m.Find(PlanJDK, c.Java.Version, c.Arch)
		if err != nil {
			return result, err
		}
		launch := c.Launch && c.Type == "JDK"
		build := c.NeedJDK
		layer, err := b.runtimeLayer(bgctx, ctx, fetcher, "jdk", dep, libcnb.LayerTypes{Build: build, Cache: build, Launch: launch}, launch, c)
		if err != nil {
			return result, err
		}
		result.Layers = append(result.Layers, layer)
		if launch {
			runtimeHome, runtimeDep = layer.Path, dep
		}
	}

	if c.Launch && c.Type == "JRE" {
		dep, err := m.Find(PlanJRE, c.Java.Version, c.Arch)
		if err != nil {
			return result, err
		}
		build := c.JREBuild && !c.NeedJDK
		layer, err := b.runtimeLayer(bgctx, ctx, fetcher, "jre", dep, libcnb.LayerTypes{Build: build, Cache: build, Launch: true}, true, c)
		if err != nil {
			return result, err
		}
		result.Layers = append(result.Layers, layer)
		runtimeHome, runtimeDep = layer.Path, dep
	}

	if !c.Launch {
		return result, nil
	}

	if c.FIPS {
		layer, err := b.fipsLayer(bgctx, ctx, fetcher, m, c, runtimeHome, runtimeDep)
		if err != nil {
			return result, err
		}
		result.Layers = append(result.Layers, layer)
	}

	helper, err := b.helperLayer(ctx, c, runtimeHome)
	if err != nil {
		return result, err
	}
	result.Layers = append(result.Layers, helper)
	return result, nil
}

func sameMetadata(layer libcnb.Layer, want map[string]any) bool {
	if len(layer.Metadata) == 0 {
		return false
	}
	if entries, err := os.ReadDir(layer.Path); err != nil || len(entries) == 0 {
		return false
	}
	got := map[string]any{}
	for k, v := range layer.Metadata {
		got[k] = fmt.Sprint(v)
	}
	normalized := map[string]any{}
	for k, v := range want {
		normalized[k] = fmt.Sprint(v)
	}
	return reflect.DeepEqual(got, normalized)
}

func (b Builder) runtimeLayer(bgctx context.Context, ctx libcnb.BuildContext, f Fetcher, name string, dep manifest.Dependency, types libcnb.LayerTypes, launch bool, c Config) (libcnb.Layer, error) {
	layer, err := ctx.Layers.Layer(name)
	if err != nil {
		return layer, err
	}
	extraCA := ""
	if launch && !c.FIPS {
		extraCA = c.CAHash
	}
	want := map[string]any{"sha256": dep.SHA256, "version": dep.Version, "ca": extraCA}

	if sameMetadata(layer, want) {
		b.logf("Reusing cached %s %s", dep.Name, dep.Version)
	} else {
		b.logf("Installing %s %s", dep.Name, dep.Version)
		if layer, err = layer.Reset(); err != nil {
			return layer, err
		}
		archive, err := f.Fetch(bgctx, dep)
		if err != nil {
			return layer, err
		}
		defer os.Remove(archive)
		if err := ExtractTarGz(archive, layer.Path, 1); err != nil {
			return layer, fmt.Errorf("extracting %s: %w", dep.Name, err)
		}
		if extraCA != "" {
			if err := b.importExtraCAs(layer.Path, "", c.ExtraCAs, nil, ""); err != nil {
				return layer, err
			}
		}
	}

	layer.Metadata = want
	layer.LayerTypes = types
	if types.Build {
		layer.BuildEnvironment.Override("JAVA_HOME", layer.Path)
	}
	if launch {
		layer.LaunchEnvironment.Override("JAVA_HOME", layer.Path)
		layer.LaunchEnvironment.Default("MALLOC_ARENA_MAX", "2")
		layer.LaunchEnvironment.Append("JAVA_TOOL_OPTIONS", " ", "-XX:+ExitOnOutOfMemoryError")
		if !c.FIPS {
			cacerts, err := Cacerts(layer.Path)
			if err != nil {
				return layer, err
			}
			layer.LaunchEnvironment.Default("BPI_JVM_TRUSTSTORE", cacerts)
		}
	}
	return layer, WriteSBOM(layer.SBOMPath(libcnb.CycloneDXJSON), c.BPID, c.BPVersion, []manifest.Dependency{dep})
}

func (b Builder) importExtraCAs(home, dest string, paths []string, jars []string, storeType string) error {
	list, err := certs.Load(paths)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}
	if dest == "" {
		if dest, err = Cacerts(home); err != nil {
			return err
		}
	}
	tmp, err := os.MkdirTemp("", "ca-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	p12 := filepath.Join(tmp, "extra.p12")
	if err := certs.WritePKCS12(list, TruststorePassword, p12); err != nil {
		return err
	}
	b.logf("Adding %d CA certificate(s) from bindings to the truststore", len(list))
	return ImportTruststore(home, p12, dest, storeType, jars)
}

func (b Builder) fipsLayer(bgctx context.Context, ctx libcnb.BuildContext, f Fetcher, m manifest.Manifest, c Config, home string, runtimeDep manifest.Dependency) (libcnb.Layer, error) {
	layer, err := ctx.Layers.Layer(fipsLayerName)
	if err != nil {
		return layer, err
	}
	deps := m.FindAll("bc-fips", c.Arch)
	if len(deps) == 0 {
		return layer, fmt.Errorf("no Bouncy Castle FIPS dependencies in buildpack.toml")
	}
	want := map[string]any{"runtime": runtimeDep.SHA256, "ca": c.CAHash, "selftest": fileSHA(selfTestClass(c.BPPath))}
	for _, d := range deps {
		want[d.PURL] = d.SHA256
	}

	libDir := filepath.Join(layer.Path, "lib")
	securityFile := filepath.Join(layer.Path, "java.security")
	truststore := filepath.Join(layer.Path, "truststore.bcfks")

	if sameMetadata(layer, want) {
		b.logf("Reusing cached FIPS configuration")
	} else {
		b.logf("Configuring FIPS 140-3 mode")
		if layer, err = layer.Reset(); err != nil {
			return layer, err
		}
		for _, d := range deps {
			file, err := f.Fetch(bgctx, d)
			if err != nil {
				return layer, err
			}
			err = CopyFile(file, filepath.Join(libDir, filepath.Base(d.URI)), 0o644)
			os.Remove(file)
			if err != nil {
				return layer, err
			}
			b.logf("  %s %s", d.Name, d.Version)
		}

		original, err := SecurityFile(home)
		if err != nil {
			return layer, err
		}
		raw, err := os.ReadFile(original)
		if err != nil {
			return layer, err
		}
		if err := os.WriteFile(securityFile, FipsSecurityProperties(raw), 0o644); err != nil {
			return layer, err
		}

		cacerts, err := Cacerts(home)
		if err != nil {
			return layer, err
		}
		jars, err := filepath.Glob(filepath.Join(libDir, "*.jar"))
		if err != nil {
			return layer, err
		}
		if err := ConvertTruststore(home, cacerts, truststore, jars); err != nil {
			return layer, err
		}
		if err := b.importExtraCAs(home, truststore, c.ExtraCAs, jars, "BCFKS"); err != nil {
			return layer, err
		}
	}

	jars, err := filepath.Glob(filepath.Join(libDir, "*.jar"))
	if err != nil {
		return layer, err
	}
	toolOptions, launcherOptions := FipsOptions(c.Java.Version, home, layer.Path, jars)

	out, err := SelfTest(home, filepath.Dir(selfTestClass(c.BPPath)), strings.Join(toolOptions, " "), strings.Join(launcherOptions, " "))
	if err != nil {
		return layer, err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(line, "Picked up ") && !strings.HasPrefix(line, "NOTE: Picked up ") {
			b.logf("%s", line)
		}
	}

	layer.Metadata = want
	layer.LayerTypes = libcnb.LayerTypes{Launch: true}
	for _, o := range toolOptions {
		layer.LaunchEnvironment.Append("JAVA_TOOL_OPTIONS", " ", o)
	}
	if len(launcherOptions) > 0 {
		layer.LaunchEnvironment.Append("JDK_JAVA_OPTIONS", " ", strings.Join(launcherOptions, " "))
	}
	layer.LaunchEnvironment.Override("BPI_JVM_TRUSTSTORE", truststore)
	layer.LaunchEnvironment.Override("BPI_JVM_TRUSTSTORE_TYPE", "BCFKS")
	layer.LaunchEnvironment.Override("BPI_JVM_FIPS_JARS", strings.Join(jars, string(os.PathListSeparator)))
	layer.LaunchEnvironment.Override("BPI_JVM_FIPS", "true")
	return layer, WriteSBOM(layer.SBOMPath(libcnb.CycloneDXJSON), c.BPID, c.BPVersion, deps)
}

func FipsOptions(java, home, fipsDir string, jars []string) (tool []string, launcher []string) {
	tool = []string{
		"-Djava.security.properties==" + filepath.Join(fipsDir, "java.security"),
		"-Dorg.bouncycastle.fips.approved_only=true",
		"-Djavax.net.ssl.trustStore=" + filepath.Join(fipsDir, "truststore.bcfks"),
		"-Djavax.net.ssl.trustStoreType=BCFKS",
		"-Djavax.net.ssl.trustStorePassword=" + TruststorePassword,
	}
	if java == "8" {
		tool = append(tool, "-Djava.ext.dirs="+ExtDir(home)+string(os.PathListSeparator)+filepath.Join(fipsDir, "lib"))
		return tool, nil
	}
	launcher = []string{"--module-path=" + strings.Join(jars, string(os.PathListSeparator)), "--add-modules=ALL-MODULE-PATH"}
	return tool, launcher
}

func selfTestClass(bpPath string) string {
	return filepath.Join(bpPath, "selftest", "FipsSelfTest.class")
}

func fileSHA(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func (b Builder) helperLayer(ctx libcnb.BuildContext, c Config, home string) (libcnb.Layer, error) {
	layer, err := ctx.Layers.Layer(helperLayerName)
	if err != nil {
		return layer, err
	}
	if layer, err = layer.Reset(); err != nil {
		return layer, err
	}
	src := filepath.Join(c.BPPath, "bin", HelperName)
	dst := filepath.Join(layer.Path, HelperName)
	if err := CopyFile(src, dst, 0o755); err != nil {
		return layer, fmt.Errorf("installing helper: %w", err)
	}
	if err := os.MkdirAll(layer.Exec.Path, 0o755); err != nil {
		return layer, err
	}
	for _, name := range ExecDHelpers {
		if err := os.Symlink(filepath.Join("..", HelperName), layer.Exec.FilePath(name)); err != nil {
			return layer, err
		}
	}

	layer.LayerTypes = libcnb.LayerTypes{Launch: true}
	layer.Metadata = map[string]any{"helper": fileSHA(src)}
	layer.LaunchEnvironment.Default("BPI_JVM_CLASS_COUNT", strconv.Itoa(classcount.JVM(home)))
	layer.LaunchEnvironment.Default("BPI_APPLICATION_PATH", c.AppPath)
	layer.LaunchEnvironment.Default("BPI_JAVA_VERSION", c.Java.Version)
	b.logf("Installed launch helpers: %s", strings.Join(ExecDHelpers, ", "))
	return layer, nil
}
