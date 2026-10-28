package helper

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func calculator(env map[string]string, cgroup string, t *testing.T) MemoryCalculator {
	files := []string{}
	if cgroup != "" {
		p := filepath.Join(t.TempDir(), "memory.max")
		os.WriteFile(p, []byte(cgroup+"\n"), 0o644)
		files = append(files, p)
	}
	return MemoryCalculator{
		Getenv:      func(k string) string { return env[k] },
		CgroupFiles: files,
		Log:         io.Discard,
		CountApp:    func(string) int { return 10000 },
	}
}

func TestCalculateDefaults(t *testing.T) {
	c := calculator(map[string]string{"BPI_JVM_CLASS_COUNT": "20000", "BPI_APPLICATION_PATH": "/workspace"}, "", t)
	b, flags, err := c.Calculate(1*GiB, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b.Classes != 10500 {
		t.Fatalf("classes %d", b.Classes)
	}
	wantMeta := int64(10500)*5800 + 14*MiB
	if b.Metaspace != wantMeta {
		t.Fatalf("metaspace %d", b.Metaspace)
	}
	nonHeap := GiB/10 + 10*MiB + 240*MiB + wantMeta + 250*MiB
	if b.Heap != GiB-nonHeap {
		t.Fatalf("heap %d want %d", b.Heap, GiB-nonHeap)
	}
	got := strings.Join(flags, " ")
	for _, want := range []string{"-Xmx", "-Xss1M", "-XX:MaxDirectMemorySize=10M", "-XX:ReservedCodeCacheSize=240M", "-XX:MaxMetaspaceSize="} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "-Xms") {
		t.Fatal("must not pin -Xms")
	}
}

func TestCalculateRespectsUserFlags(t *testing.T) {
	c := calculator(map[string]string{"BPL_JVM_LOADED_CLASS_COUNT": "1000"}, "", t)
	b, flags, err := c.Calculate(2*GiB, Options("-Xmx512m -XX:ThreadStackSize=512 -XX:MaxMetaspaceSize=128M"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(flags, " ")
	if strings.Contains(got, "-Xmx") || strings.Contains(got, "-Xss") || strings.Contains(got, "MaxMetaspaceSize") {
		t.Fatalf("user flags overridden: %s", got)
	}
	if b.Heap != 512*MiB || b.Stack != 512*KiB || b.Metaspace != 128*MiB {
		t.Fatalf("%+v", b)
	}

	_, flags, _ = c.Calculate(2*GiB, Options("-XX:MaxRAMPercentage=60"))
	if strings.Contains(strings.Join(flags, " "), "-Xmx") {
		t.Fatal("MaxRAMPercentage must disable heap calculation")
	}
}

func TestCalculateInsufficientMemory(t *testing.T) {
	c := calculator(map[string]string{"BPI_JVM_CLASS_COUNT": "30000"}, "", t)
	if _, _, err := c.Calculate(256*MiB, nil); err == nil || !strings.Contains(err.Error(), "insufficient memory") {
		t.Fatalf("expected insufficient memory, got %v", err)
	}
	c = calculator(map[string]string{"BPI_JVM_CLASS_COUNT": "30000", "BPL_JVM_THREAD_COUNT": "20"}, "", t)
	if _, _, err := c.Calculate(512*MiB, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecute(t *testing.T) {
	env := map[string]string{"JAVA_TOOL_OPTIONS": "-Dfoo=bar", "BPI_JVM_CLASS_COUNT": "1000"}
	out, err := calculator(env, "1073741824", t).Execute()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out["JAVA_TOOL_OPTIONS"], "-Dfoo=bar -Xmx") {
		t.Fatal(out)
	}

	out, err = calculator(env, "max", t).Execute()
	if err != nil || out != nil {
		t.Fatalf("unlimited memory must be a no-op: %v %v", out, err)
	}

	env["BPL_JVM_TOTAL_MEMORY"] = "2G"
	c := calculator(env, "", t)
	total, src, _ := c.totalMemory()
	if total != 2*GiB || src != "BPL_JVM_TOTAL_MEMORY" {
		t.Fatal(total, src)
	}

	env["BPL_JVM_HEAD_ROOM"] = "150"
	if _, err := calculator(env, "", t).Execute(); err == nil {
		t.Fatal("expected headroom validation error")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"1024": 1024, "1k": KiB, "512M": 512 * MiB, "2g": 2 * GiB, "1GB": GiB} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Fatalf("%s: %d %v", in, got, err)
		}
	}
	if _, err := ParseSize("abc"); err == nil {
		t.Fatal("expected error")
	}
	if Flag(1536*KiB) != "1536K" || Flag(2*GiB) != "2G" {
		t.Fatal(Flag(1536*KiB), Flag(2*GiB))
	}
}
