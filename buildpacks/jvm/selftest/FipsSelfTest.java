import java.lang.reflect.Method;
import java.security.KeyPairGenerator;
import java.security.KeyStore;
import java.security.Provider;
import java.security.SecureRandom;
import java.security.Security;
import javax.crypto.Cipher;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

public final class FipsSelfTest {

    private static int failures = 0;

    public static void main(String[] args) throws Exception {
        Provider[] providers = Security.getProviders();
        check("BCFIPS is the first provider", providers.length > 0 && "BCFIPS".equals(providers[0].getName()));
        check("BCJSSE is the second provider", providers.length > 1 && "BCJSSE".equals(providers[1].getName()));
        check("SunJCE is not registered", Security.getProvider("SunJCE") == null);

        Class<?> registrar = Class.forName("org.bouncycastle.crypto.CryptoServicesRegistrar");
        Method approved = registrar.getMethod("isInApprovedOnlyMode");
        check("approved-only mode is active", Boolean.TRUE.equals(approved.invoke(null)));

        check("AES is served by BCFIPS", "BCFIPS".equals(Cipher.getInstance("AES/GCM/NoPadding").getProvider().getName()));
        check("SecureRandom is served by BCFIPS", "BCFIPS".equals(new SecureRandom().getProvider().getName()));
        check("RSA-1024 key generation is rejected", rejectsWeakRsa());

        SSLContext tls = SSLContext.getDefault();
        check("default SSLContext is BCJSSE", "BCJSSE".equals(tls.getProvider().getName()));

        TrustManagerFactory tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        tmf.init((KeyStore) null);
        int anchors = 0;
        for (TrustManager tm : tmf.getTrustManagers()) {
            if (tm instanceof X509TrustManager) {
                anchors += ((X509TrustManager) tm).getAcceptedIssuers().length;
            }
        }
        check("BCFKS truststore loads trust anchors (" + anchors + ")", anchors > 0);

        if (failures > 0) {
            System.out.println("FIPS self-test: " + failures + " check(s) failed");
            System.exit(1);
        }
        System.out.println("FIPS self-test: all checks passed on Java " + System.getProperty("java.version"));
    }

    private static boolean rejectsWeakRsa() {
        try {
            KeyPairGenerator kpg = KeyPairGenerator.getInstance("RSA");
            kpg.initialize(1024);
            kpg.generateKeyPair();
            return false;
        } catch (Throwable t) {
            return true;
        }
    }

    private static void check(String name, boolean ok) {
        System.out.println((ok ? "  PASS  " : "  FAIL  ") + name);
        if (!ok) {
            failures++;
        }
    }
}
