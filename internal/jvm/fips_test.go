package jvm

import (
	"strings"
	"testing"
)

const sample = `#
# comment security.provider.1=ignored
security.provider.1=SUN
security.provider.2=SunRsaSign
security.provider.13=SunPKCS11
securerandom.source=file:/dev/random
jdk.tls.disabledAlgorithms=SSLv3, TLSv1, \
    TLSv1.1, RC4
package.access=sun.,\
               jdk.internal.
networkaddress.cache.negative.ttl=10
`

func TestFipsSecurityProperties(t *testing.T) {
	out := string(FipsSecurityProperties([]byte(sample)))

	for _, gone := range []string{"=SUN\n", "SunRsaSign", "SunPKCS11", "file:/dev/random", "TLSv1, \\\n    TLSv1.1"} {
		if strings.Contains(out, gone) {
			t.Fatalf("expected %q to be removed:\n%s", gone, out)
		}
	}
	for _, kept := range []string{"package.access=sun.,\\\n               jdk.internal.", "networkaddress.cache.negative.ttl=10", "# comment security.provider.1=ignored"} {
		if !strings.Contains(out, kept) {
			t.Fatalf("expected %q to be kept:\n%s", kept, out)
		}
	}
	for _, want := range []string{
		"security.provider.1=org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider\n",
		"security.provider.2=org.bouncycastle.jsse.provider.BouncyCastleJsseProvider fips:BCFIPS\n",
		"security.provider.3=sun.security.provider.Sun\n",
		"keystore.type=BCFKS\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Count(out, "\nsecurity.provider.") != 3 {
		t.Fatalf("unexpected provider entries:\n%s", out)
	}
}

func TestFipsOptions(t *testing.T) {
	tool, launcher := FipsOptions("21", "/l/jre", "/l/fips", []string{"/l/fips/lib/a.jar", "/l/fips/lib/b.jar"})
	joined := strings.Join(tool, " ")
	if !strings.Contains(joined, "-Djava.security.properties==/l/fips/java.security") || strings.Contains(joined, "java.ext.dirs") {
		t.Fatal(joined)
	}
	if launcher[0] != "--module-path=/l/fips/lib/a.jar:/l/fips/lib/b.jar" {
		t.Fatal(launcher)
	}

	tool, launcher = FipsOptions("8", "/l/jre", "/l/fips", nil)
	if launcher != nil || !strings.Contains(strings.Join(tool, " "), "-Djava.ext.dirs=/l/jre/lib/ext:/l/fips/lib") || !strings.Contains(strings.Join(tool, " "), "assumeOriginalHostName=true") {
		t.Fatal(tool, launcher)
	}
}

func TestCleanJavaEnv(t *testing.T) {
	out := CleanJavaEnv([]string{"A=1", "JAVA_TOOL_OPTIONS=x", "JDK_JAVA_OPTIONS=y", "_JAVA_OPTIONS=z", "PATH=/bin"})
	if strings.Join(out, ",") != "A=1,PATH=/bin" {
		t.Fatal(out)
	}
}
