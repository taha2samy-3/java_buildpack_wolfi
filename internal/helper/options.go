package helper

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type OptionSet []string

func Options(s string) OptionSet {
	return OptionSet(strings.Fields(s))
}

func (o OptionSet) Has(prefix string) bool {
	for _, v := range o {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func (o OptionSet) Size(short, long string) (int64, bool) {
	var found int64
	ok := false
	for _, v := range o {
		switch {
		case short != "" && strings.HasPrefix(v, short):
			if n, err := ParseSize(strings.TrimPrefix(v, short)); err == nil {
				found, ok = n, true
			}
		case strings.HasPrefix(v, long):
			raw := strings.TrimPrefix(v, long)
			if long == "-XX:ThreadStackSize=" {
				if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
					found, ok = n*KiB, true
				}
				continue
			}
			if n, err := ParseSize(raw); err == nil {
				found, ok = n, true
			}
		}
	}
	return found, ok
}

func Join(existing string, flags ...string) string {
	parts := append([]string{}, strings.TrimSpace(existing))
	parts = append(parts, flags...)
	return strings.TrimSpace(strings.Join(parts, " "))
}

type JVMOptions struct {
	Getenv func(string) string
	Log    io.Writer
}

func NewJVMOptions() JVMOptions {
	return JVMOptions{Getenv: os.Getenv, Log: os.Stderr}
}

func (j JVMOptions) enabled(name string) (bool, error) {
	v := strings.TrimSpace(j.Getenv(name))
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", name, v)
	}
	return b, nil
}

func (j JVMOptions) value(name, def string) string {
	if v := strings.TrimSpace(j.Getenv(name)); v != "" {
		return v
	}
	return def
}

func (j JVMOptions) port(name, def string) (string, error) {
	v := j.value(name, def)
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("%s must be a TCP port, got %q", name, v)
	}
	return v, nil
}

func (j JVMOptions) host(name string) (string, error) {
	v := j.value(name, "127.0.0.1")
	if v == "*" {
		v = "0.0.0.0"
	}
	if net.ParseIP(v) == nil && strings.ContainsAny(v, " ,=") {
		return "", fmt.Errorf("%s is not a valid host, got %q", name, v)
	}
	if ip := net.ParseIP(v); ip == nil || !ip.IsLoopback() {
		fmt.Fprintf(j.Log, "WARNING: %s=%s exposes the endpoint beyond the container loopback interface\n", name, v)
	}
	return v, nil
}

func (j JVMOptions) Execute() (map[string]string, error) {
	var flags []string

	if on, err := j.enabled("BPL_JAVA_NMT_ENABLED"); err != nil {
		return nil, err
	} else if on {
		level := j.value("BPL_JAVA_NMT_LEVEL", "summary")
		if level != "summary" && level != "detail" {
			return nil, fmt.Errorf("BPL_JAVA_NMT_LEVEL must be summary or detail, got %q", level)
		}
		flags = append(flags, "-XX:+UnlockDiagnosticVMOptions", "-XX:NativeMemoryTracking="+level, "-XX:+PrintNMTStatistics")
	}

	if on, err := j.enabled("BPL_JMX_ENABLED"); err != nil {
		return nil, err
	} else if on {
		port, err := j.port("BPL_JMX_PORT", "5000")
		if err != nil {
			return nil, err
		}
		host, err := j.host("BPL_JMX_HOST")
		if err != nil {
			return nil, err
		}
		flags = append(flags,
			"-Dcom.sun.management.jmxremote",
			"-Dcom.sun.management.jmxremote.host="+host,
			"-Dcom.sun.management.jmxremote.port="+port,
			"-Dcom.sun.management.jmxremote.rmi.port="+port,
			"-Dcom.sun.management.jmxremote.authenticate=false",
			"-Dcom.sun.management.jmxremote.ssl=false",
			"-Djava.rmi.server.hostname="+host,
		)
	}

	if on, err := j.enabled("BPL_DEBUG_ENABLED"); err != nil {
		return nil, err
	} else if on {
		port, err := j.port("BPL_DEBUG_PORT", "8000")
		if err != nil {
			return nil, err
		}
		host, err := j.host("BPL_DEBUG_HOST")
		if err != nil {
			return nil, err
		}
		suspend, err := j.enabled("BPL_DEBUG_SUSPEND")
		if err != nil {
			return nil, err
		}
		s := "n"
		if suspend {
			s = "y"
		}
		if host == "0.0.0.0" && j.Getenv("BPI_JAVA_VERSION") != "8" {
			host = "*"
		}
		flags = append(flags, fmt.Sprintf("-agentlib:jdwp=transport=dt_socket,server=y,suspend=%s,address=%s:%s", s, host, port))
	}

	if on, err := j.enabled("BPL_JFR_ENABLED"); err != nil {
		return nil, err
	} else if on {
		flags = append(flags, "-XX:StartFlightRecording="+j.value("BPL_JFR_ARGS", "dumponexit=true,filename=/tmp/recording.jfr"))
	}

	if p := strings.TrimSpace(j.Getenv("BPL_HEAP_DUMP_PATH")); p != "" {
		flags = append(flags, "-XX:+HeapDumpOnOutOfMemoryError", "-XX:HeapDumpPath="+p)
	}

	if opts := strings.TrimSpace(j.Getenv("JAVA_OPTS")); opts != "" {
		flags = append(flags, opts)
	}

	if len(flags) == 0 {
		return nil, nil
	}
	return map[string]string{"JAVA_TOOL_OPTIONS": Join(j.Getenv("JAVA_TOOL_OPTIONS"), flags...)}, nil
}
