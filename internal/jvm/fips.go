package jvm

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const TruststorePassword = "changeit"

var fipsProperties = []struct{ key, value string }{
	{"security.provider.1", "org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider"},
	{"security.provider.2", "org.bouncycastle.jsse.provider.BouncyCastleJsseProvider fips:BCFIPS"},
	{"security.provider.3", "sun.security.provider.Sun"},
	{"securerandom.source", "file:/dev/urandom"},
	{"securerandom.strongAlgorithms", "DEFAULT:BCFIPS"},
	{"keystore.type", "BCFKS"},
	{"ssl.KeyManagerFactory.algorithm", "PKIX"},
	{"ssl.TrustManagerFactory.algorithm", "PKIX"},
	{"jdk.tls.disabledAlgorithms", "SSLv3, TLSv1, TLSv1.1, DTLSv1.0, RC4, DES, MD5withRSA, DH keySize < 2048, EC keySize < 224, 3DES_EDE_CBC, anon, NULL, ECDH"},
	{"jdk.certpath.disabledAlgorithms", "MD2, MD5, SHA1, RSA keySize < 2048, DSA keySize < 2048, EC keySize < 224"},
	{"jdk.jar.disabledAlgorithms", "MD2, MD5, RSA keySize < 2048, DSA keySize < 2048, EC keySize < 224"},
}

var providerKey = regexp.MustCompile(`^security\.provider\.\d+$`)

func SecurityFile(home string) (string, error) {
	for _, p := range []string{"conf/security/java.security", "lib/security/java.security", "jre/lib/security/java.security"} {
		f := filepath.Join(home, p)
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("java.security not found under %s", home)
}

func Cacerts(home string) (string, error) {
	for _, p := range []string{"lib/security/cacerts", "jre/lib/security/cacerts"} {
		f := filepath.Join(home, p)
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("cacerts not found under %s", home)
}

func ExtDir(home string) string {
	if _, err := os.Stat(filepath.Join(home, "jre", "lib", "ext")); err == nil {
		return filepath.Join(home, "jre", "lib", "ext")
	}
	return filepath.Join(home, "lib", "ext")
}

func FipsSecurityProperties(original []byte) []byte {
	overridden := map[string]bool{}
	for _, p := range fipsProperties {
		overridden[p.key] = true
	}

	var out bytes.Buffer
	for _, line := range logicalLines(original) {
		key := propertyKey(line)
		if key != "" && (overridden[key] || providerKey.MatchString(key)) {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	out.WriteString("\n")
	for _, p := range fipsProperties {
		fmt.Fprintf(&out, "%s=%s\n", p.key, p.value)
	}
	return out.Bytes()
}

func logicalLines(data []byte) []string {
	var lines []string
	var cur strings.Builder
	s := bufio.NewScanner(bytes.NewReader(data))
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		line := s.Text()
		cur.WriteString(line)
		trimmed := strings.TrimRight(line, " \t")
		if !strings.HasPrefix(strings.TrimSpace(line), "#") && continued(trimmed) {
			cur.WriteByte('\n')
			continue
		}
		lines = append(lines, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

func continued(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

func propertyKey(line string) string {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == '#' || t[0] == '!' {
		return ""
	}
	end := strings.IndexAny(t, "=: \t")
	if end < 0 {
		return t
	}
	return t[:end]
}

func keytool(home string, args ...string) error {
	cmd := exec.Command(filepath.Join(home, "bin", "keytool"), args...)
	cmd.Env = CleanJavaEnv(os.Environ())
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keytool %s: %w\n%s", args[0], err, buf.String())
	}
	return nil
}

func CleanJavaEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		switch k {
		case "JAVA_TOOL_OPTIONS", "JDK_JAVA_OPTIONS", "_JAVA_OPTIONS", "JAVA_OPTS":
			continue
		}
		out = append(out, e)
	}
	return out
}

func ProviderArgs(jars []string) []string {
	return []string{
		"-providerpath", strings.Join(jars, string(os.PathListSeparator)),
		"-provider", "org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider",
	}
}

func ConvertTruststore(home, source, dest string, jars []string) error {
	args := []string{"-importkeystore", "-noprompt",
		"-srckeystore", source, "-srcstorepass", "changeit",
		"-destkeystore", dest, "-deststoretype", "BCFKS", "-deststorepass", TruststorePassword,
	}
	return keytool(home, append(args, ProviderArgs(jars)...)...)
}

func ImportTruststore(home, p12, dest, destType string, jars []string) error {
	args := []string{"-importkeystore", "-noprompt",
		"-srckeystore", p12, "-srcstoretype", "PKCS12", "-srcstorepass", TruststorePassword,
		"-destkeystore", dest, "-deststorepass", TruststorePassword,
	}
	if destType != "" {
		args = append(args, "-deststoretype", destType)
	}
	if destType == "BCFKS" {
		args = append(args, ProviderArgs(jars)...)
	}
	return keytool(home, args...)
}

func SelfTest(home, classDir string, toolOptions, launcherOptions string) (string, error) {
	cmd := exec.Command(filepath.Join(home, "bin", "java"), "-cp", classDir, "FipsSelfTest")
	cmd.Env = append(CleanJavaEnv(os.Environ()), "JAVA_TOOL_OPTIONS="+toolOptions)
	if launcherOptions != "" {
		cmd.Env = append(cmd.Env, "JDK_JAVA_OPTIONS="+launcherOptions)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("FIPS self-test failed: %w\n%s", err, out)
	}
	return string(out), nil
}
