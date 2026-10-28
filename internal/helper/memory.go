package helper

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/taha2samy-3/java_buildpack_wolfi/internal/classcount"
)

const (
	KiB = int64(1024)
	MiB = 1024 * KiB
	GiB = 1024 * MiB

	defaultCodeCache   = 240 * MiB
	defaultDirect      = 10 * MiB
	defaultStack       = 1 * MiB
	metaspaceBase      = 14 * MiB
	metaspacePerClass  = int64(5800)
	loadedClassPercent = 0.35
	unlimitedThreshold = int64(1) << 60
)

type MemoryCalculator struct {
	Getenv      func(string) string
	CgroupFiles []string
	Log         io.Writer
	CountApp    func(string) int
}

func NewMemoryCalculator() MemoryCalculator {
	return MemoryCalculator{
		Getenv:      os.Getenv,
		CgroupFiles: []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"},
		Log:         os.Stderr,
		CountApp:    classcount.Tree,
	}
}

type Breakdown struct {
	Total, Heap, Headroom, Direct, Metaspace, CodeCache, Stack int64
	Threads, Classes                                           int
}

func (c MemoryCalculator) Execute() (map[string]string, error) {
	total, source, err := c.totalMemory()
	if err != nil {
		return nil, err
	}
	if total <= 0 {
		fmt.Fprintln(c.Log, "Memory calculator: no memory limit detected, JVM ergonomics apply")
		return nil, nil
	}

	opts := Options(c.Getenv("JAVA_TOOL_OPTIONS") + " " + c.Getenv("JDK_JAVA_OPTIONS"))

	b, flags, err := c.Calculate(total, opts)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(c.Log, "Memory calculator (%s %s): heap %s, metaspace %s (%d classes), code cache %s, direct %s, stack %s x %d threads, headroom %s\n",
		source, Size(b.Total), Size(b.Heap), Size(b.Metaspace), b.Classes, Size(b.CodeCache), Size(b.Direct), Size(b.Stack), b.Threads, Size(b.Headroom))
	if len(flags) == 0 {
		return nil, nil
	}
	return map[string]string{"JAVA_TOOL_OPTIONS": Join(c.Getenv("JAVA_TOOL_OPTIONS"), flags...)}, nil
}

func (c MemoryCalculator) Calculate(total int64, opts OptionSet) (Breakdown, []string, error) {
	b := Breakdown{Total: total}
	var flags []string

	headroom, err := c.intEnv("BPL_JVM_HEAD_ROOM", 10)
	if err != nil {
		return b, nil, err
	}
	if headroom < 0 || headroom > 99 {
		return b, nil, fmt.Errorf("BPL_JVM_HEAD_ROOM must be between 0 and 99, got %d", headroom)
	}
	b.Headroom = total * int64(headroom) / 100

	if b.Threads, err = c.intEnv("BPL_JVM_THREAD_COUNT", 250); err != nil {
		return b, nil, err
	}
	if b.Threads < 1 {
		return b, nil, fmt.Errorf("BPL_JVM_THREAD_COUNT must be positive")
	}

	if v, ok := opts.Size("-Xss", "-XX:ThreadStackSize="); ok {
		b.Stack = v
	} else {
		b.Stack = defaultStack
		flags = append(flags, "-Xss"+Flag(b.Stack))
	}

	if v, ok := opts.Size("", "-XX:MaxDirectMemorySize="); ok {
		b.Direct = v
	} else {
		b.Direct = defaultDirect
		flags = append(flags, "-XX:MaxDirectMemorySize="+Flag(b.Direct))
	}

	if v, ok := opts.Size("", "-XX:ReservedCodeCacheSize="); ok {
		b.CodeCache = v
	} else {
		b.CodeCache = defaultCodeCache
		flags = append(flags, "-XX:ReservedCodeCacheSize="+Flag(b.CodeCache))
	}

	if b.Classes, err = c.classes(); err != nil {
		return b, nil, err
	}
	if v, ok := opts.Size("", "-XX:MaxMetaspaceSize="); ok {
		b.Metaspace = v
	} else {
		b.Metaspace = int64(b.Classes)*metaspacePerClass + metaspaceBase
		flags = append(flags, "-XX:MaxMetaspaceSize="+Flag(b.Metaspace))
	}

	nonHeap := b.Headroom + b.Direct + b.CodeCache + b.Metaspace + b.Stack*int64(b.Threads)

	if v, ok := opts.Size("-Xmx", "-XX:MaxHeapSize="); ok {
		b.Heap = v
		if v+nonHeap > total {
			fmt.Fprintf(c.Log, "WARNING: configured heap %s plus non-heap %s exceeds total memory %s\n", Size(v), Size(nonHeap), Size(total))
		}
		return b, flags, nil
	}
	if opts.Has("-XX:MaxRAMPercentage=") || opts.Has("-XX:MaxRAM=") {
		return b, flags, nil
	}

	b.Heap = total - nonHeap
	if b.Heap < 32*MiB {
		return b, nil, fmt.Errorf("insufficient memory: total %s, required non-heap %s (headroom %s, direct %s, code cache %s, metaspace %s, stack %s x %d threads); increase the memory limit or lower BPL_JVM_THREAD_COUNT",
			Size(total), Size(nonHeap), Size(b.Headroom), Size(b.Direct), Size(b.CodeCache), Size(b.Metaspace), Size(b.Stack), b.Threads)
	}
	flags = append([]string{"-Xmx" + Flag(b.Heap)}, flags...)
	return b, flags, nil
}

func (c MemoryCalculator) classes() (int, error) {
	if v := strings.TrimSpace(c.Getenv("BPL_JVM_LOADED_CLASS_COUNT")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("BPL_JVM_LOADED_CLASS_COUNT must be a non-negative integer, got %q", v)
		}
		return n, nil
	}
	jvm, _ := strconv.Atoi(c.Getenv("BPI_JVM_CLASS_COUNT"))
	app := 0
	if p := c.Getenv("BPI_APPLICATION_PATH"); p != "" {
		app = c.CountApp(p)
	}
	return int(float64(jvm+app) * loadedClassPercent), nil
}

func (c MemoryCalculator) intEnv(name string, def int) (int, error) {
	v := strings.TrimSpace(c.Getenv(name))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", name, v)
	}
	return n, nil
}

func (c MemoryCalculator) totalMemory() (int64, string, error) {
	if v := strings.TrimSpace(c.Getenv("BPL_JVM_TOTAL_MEMORY")); v != "" {
		n, err := ParseSize(v)
		if err != nil {
			return 0, "", fmt.Errorf("BPL_JVM_TOTAL_MEMORY: %w", err)
		}
		return n, "BPL_JVM_TOTAL_MEMORY", nil
	}
	for _, f := range c.CgroupFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(data))
		if s == "max" || s == "" {
			return 0, "", nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 || n >= unlimitedThreshold || n >= hostMemory() {
			return 0, "", nil
		}
		return n, "container limit", nil
	}
	return 0, "", nil
}

func hostMemory() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return unlimitedThreshold
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				return kb * KiB
			}
		}
	}
	return unlimitedThreshold
}

func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	mult := int64(1)
	upper := strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(s, "B"), "b"))
	switch upper[len(upper)-1] {
	case 'K':
		mult, upper = KiB, upper[:len(upper)-1]
	case 'M':
		mult, upper = MiB, upper[:len(upper)-1]
	case 'G':
		mult, upper = GiB, upper[:len(upper)-1]
	case 'T':
		mult, upper = 1024*GiB, upper[:len(upper)-1]
	}
	n, err := strconv.ParseInt(upper, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return n * mult, nil
}

func Flag(n int64) string {
	switch {
	case n%GiB == 0:
		return strconv.FormatInt(n/GiB, 10) + "G"
	case n%MiB == 0:
		return strconv.FormatInt(n/MiB, 10) + "M"
	case n%KiB == 0:
		return strconv.FormatInt(n/KiB, 10) + "K"
	}
	return strconv.FormatInt(n/KiB, 10) + "K"
}

func Size(n int64) string {
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.1fG", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%dM", n/MiB)
	}
	return fmt.Sprintf("%dK", n/KiB)
}
