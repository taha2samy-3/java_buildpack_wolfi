//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type scenario struct {
	name      string
	app       string
	buildEnv  map[string]string
	runEnv    map[string]string
	bindings  map[string]map[string]string
	expect    map[string]string
	contains  []string
	logs      []string
	checkTLS  bool
	maxHeapMB int
}

var fipsExpect = map[string]string{
	"provider.1":      "BCFIPS",
	"approved.only":   "true",
	"rsa1024.allowed": "false",
	"tls.provider":    "BCJSSE",
}

func fips(java string) scenario {
	s := scenario{
		name:     "maven-fips-java" + java,
		app:      "maven",
		buildEnv: map[string]string{"BP_JVM_FIPS_ENABLED": "true"},
		expect:   map[string]string{},
		logs:     []string{"Memory calculator"},
		checkTLS: true,
	}
	if java != "8" {
		s.buildEnv["BP_JVM_VERSION"] = java
	}
	for k, v := range fipsExpect {
		s.expect[k] = v
	}
	if java == "8" {
		s.expect["java.version"] = "1.8.0"
	} else {
		s.expect["java.version"] = java
	}
	return s
}

func scenarios() []scenario {
	return []scenario{
		fips("8"),
		fips("11"),
		fips("17"),
		fips("21"),
		fips("25"),
		{
			name:     "maven-default-java21",
			app:      "maven",
			buildEnv: map[string]string{"BP_JVM_VERSION": "21"},
			expect:   map[string]string{"java.version": "21", "provider.1": "SUN", "approved.only": "absent", "rsa1024.allowed": "true", "tls.provider": "SunJSSE"},
			checkTLS: true,
		},
		{
			name:     "maven-jdk-runtime",
			app:      "maven",
			buildEnv: map[string]string{"BP_JVM_VERSION": "17", "BP_JVM_TYPE": "JDK"},
			expect:   map[string]string{"java.version": "17", "java.home": "/layers/taha2samy-3_wolfi-jvm/jdk"},
		},
		{
			name:      "maven-percentage-memory",
			app:       "maven",
			buildEnv:  map[string]string{"BP_JVM_VERSION": "21"},
			runEnv:    map[string]string{"BPL_JVM_MEMORY_MODE": "percentage", "BPL_JVM_HEAP_PERCENTAGE": "50", "BPL_JVM_PIN_HEAP": "true"},
			contains:  []string{"-Xmx512M", "-Xms512M"},
			maxHeapMB: 512,
		},
		{
			name:     "maven-runtime-ca-binding",
			app:      "maven",
			buildEnv: map[string]string{"BP_JVM_FIPS_ENABLED": "true", "BP_JVM_VERSION": "21"},
			bindings: map[string]map[string]string{"corp-ca": {"type": "ca-certificates", "ca.pem": ""}},
			expect:   fipsExpect,
			logs:     []string{"Added 1 CA certificate(s) to the JVM truststore"},
			checkTLS: true,
		},
		{
			name:     "gradle-spring-boot-fips",
			app:      "gradle",
			buildEnv: map[string]string{"BP_JVM_FIPS_ENABLED": "true"},
			expect:   map[string]string{"java.version": "21", "provider.1": "BCFIPS"},
		},
	}
}

func TestScenarios(t *testing.T) {
	builder := os.Getenv("BUILDER")
	if builder == "" {
		t.Skip("BUILDER is not set")
	}
	for _, s := range scenarios() {
		t.Run(s.name, func(t *testing.T) {
			run(t, builder, s)
		})
	}
}

func run(t *testing.T, builder string, s scenario) {
	image := "local/it-" + s.name + ":latest"
	buildCache, launchCache := "it-"+s.name+"-build", "it-"+s.name+"-launch"
	t.Cleanup(func() {
		exec.Command("docker", "rmi", "-f", image).Run()
		exec.Command("docker", "volume", "rm", "-f", buildCache, launchCache).Run()
	})

	args := []string{"build", image, "--path", filepath.Join("..", "samples", s.app), "--builder", builder, "--pull-policy", "if-not-present", "--clear-cache",
		"--cache", "type=build;format=volume;name=" + buildCache, "--cache", "type=launch;format=volume;name=" + launchCache}
	for k, v := range s.buildEnv {
		args = append(args, "--env", k+"="+v)
	}
	out, err := command(t, 30*time.Minute, "pack", args...)
	if err != nil {
		t.Fatalf("pack build failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "FIPS true") && !strings.Contains(out, "FIPS self-test: all checks passed") {
		t.Fatalf("FIPS self-test did not run:\n%s", out)
	}

	runArgs := []string{"run", "-d", "--memory", "1g", "--read-only", "--tmpfs", "/tmp:rw,exec", "-p", "127.0.0.1::8080", "--cap-drop", "ALL", "--security-opt", "no-new-privileges"}
	for k, v := range s.runEnv {
		runArgs = append(runArgs, "-e", k+"="+v)
	}
	if len(s.bindings) > 0 {
		root := bindings(t, s.bindings)
		runArgs = append(runArgs, "-v", root+":/bindings:ro", "-e", "SERVICE_BINDING_ROOT=/bindings")
	}
	runArgs = append(runArgs, image)
	id, err := command(t, time.Minute, "docker", runArgs...)
	if err != nil {
		t.Fatalf("docker run failed: %v\n%s", err, id)
	}
	id = strings.TrimSpace(id)
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", id).Run() })

	port, err := command(t, time.Minute, "docker", "port", id, "8080/tcp")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + strings.TrimSpace(strings.Split(port, "\n")[0])

	status, err := get(base+"/", 3*time.Minute)
	if err != nil {
		logs, _ := command(t, time.Minute, "docker", "logs", id)
		t.Fatalf("application did not respond: %v\n%s", err, logs)
	}
	t.Logf("status:\n%s", status)
	props := parse(status)

	for k, want := range s.expect {
		if got := props[k]; !strings.HasPrefix(got, want) {
			t.Errorf("%s = %q, want prefix %q", k, got, want)
		}
	}
	for _, want := range s.contains {
		if !strings.Contains(status, want) {
			t.Errorf("status does not contain %q", want)
		}
	}
	if heap, err := strconv.ParseInt(props["max.heap"], 10, 64); err == nil {
		limit := int64(1024)
		if s.maxHeapMB > 0 {
			limit = int64(s.maxHeapMB)
		}
		if heap <= 0 || heap > limit<<20 {
			t.Errorf("max heap %d outside (0, %dMB]", heap, limit)
		}
	}

	if s.checkTLS {
		body, err := get(base+"/tls", time.Minute)
		if err != nil || !strings.Contains(body, "status=200") {
			t.Errorf("outbound TLS failed: %v\n%s", err, body)
		}
	}

	logs, _ := command(t, time.Minute, "docker", "logs", id)
	for _, want := range s.logs {
		if !strings.Contains(logs, want) {
			t.Errorf("logs do not contain %q:\n%s", want, logs)
		}
	}
}

func command(t *testing.T, timeout time.Duration, name string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func get(url string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 30 * time.Second}
	var last error
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return string(body), nil
		}
		last = err
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("timeout: %w", last)
}

func parse(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			out[k] = v
		}
	}
	return out
}

func bindings(t *testing.T, spec map[string]map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, files := range spec {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for file, content := range files {
			if strings.HasSuffix(file, ".pem") && content == "" {
				content = selfSignedCA(t)
			}
			if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func selfSignedCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "Integration Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
