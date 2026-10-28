package helper

import (
	"bytes"
	"strings"
	"testing"
)

func options(env map[string]string) (string, string, error) {
	var log bytes.Buffer
	out, err := JVMOptions{Getenv: func(k string) string { return env[k] }, Log: &log}.Execute()
	return out["JAVA_TOOL_OPTIONS"], log.String(), err
}

func TestJVMOptionsDefaultsAreNoop(t *testing.T) {
	got, _, err := options(map[string]string{})
	if err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestJVMOptionsSecureDefaults(t *testing.T) {
	got, log, err := options(map[string]string{"JAVA_TOOL_OPTIONS": "-Da=b", "BPL_JMX_ENABLED": "true", "BPL_DEBUG_ENABLED": "true"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-Da=b ", "-Dcom.sun.management.jmxremote.host=127.0.0.1", "-Dcom.sun.management.jmxremote.port=5000", "-Dcom.sun.management.jmxremote.rmi.port=5000", "address=127.0.0.1:8000", "suspend=n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if log != "" {
		t.Fatalf("unexpected warning %s", log)
	}
}

func TestJVMOptionsExposureWarns(t *testing.T) {
	got, log, err := options(map[string]string{"BPL_DEBUG_ENABLED": "true", "BPL_DEBUG_HOST": "*", "BPL_DEBUG_SUSPEND": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "address=*:8000") || !strings.Contains(got, "suspend=y") || !strings.Contains(log, "WARNING") {
		t.Fatalf("%s %s", got, log)
	}
	got, _, _ = options(map[string]string{"BPL_DEBUG_ENABLED": "true", "BPL_DEBUG_HOST": "*", "BPI_JAVA_VERSION": "8"})
	if !strings.Contains(got, "address=0.0.0.0:8000") {
		t.Fatal(got)
	}
}

func TestJVMOptionsFeatures(t *testing.T) {
	got, _, err := options(map[string]string{"BPL_JAVA_NMT_ENABLED": "true", "BPL_JFR_ENABLED": "true", "BPL_HEAP_DUMP_PATH": "/tmp/dumps"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-XX:NativeMemoryTracking=summary", "-XX:StartFlightRecording=dumponexit=true", "-XX:HeapDumpPath=/tmp/dumps"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
}

func TestJVMOptionsValidation(t *testing.T) {
	for _, env := range []map[string]string{
		{"BPL_JMX_ENABLED": "yes please"},
		{"BPL_JMX_ENABLED": "true", "BPL_JMX_PORT": "70000"},
		{"BPL_DEBUG_ENABLED": "true", "BPL_DEBUG_HOST": "a,b"},
		{"BPL_JAVA_NMT_ENABLED": "true", "BPL_JAVA_NMT_LEVEL": "full"},
	} {
		if _, _, err := options(env); err == nil {
			t.Fatalf("expected error for %v", env)
		}
	}
}

func TestJVMOptionsJavaOpts(t *testing.T) {
	got, _, err := options(map[string]string{"JAVA_TOOL_OPTIONS": "-Da=b", "JAVA_OPTS": " -Dfoo=bar -XX:+UseZGC "})
	if err != nil || got != "-Da=b -Dfoo=bar -XX:+UseZGC" {
		t.Fatalf("%q %v", got, err)
	}
}
