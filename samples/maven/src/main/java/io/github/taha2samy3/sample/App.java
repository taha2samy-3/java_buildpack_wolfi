package io.github.taha2samy3.sample;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.lang.management.ManagementFactory;
import java.net.InetSocketAddress;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.security.KeyPairGenerator;
import java.security.Security;
import javax.net.ssl.HttpsURLConnection;
import javax.net.ssl.SSLContext;

public final class App {

    public static void main(String[] args) throws IOException {
        int port = Integer.parseInt(System.getenv().getOrDefault("PORT", "8080"));
        HttpServer server = HttpServer.create(new InetSocketAddress(port), 0);
        server.createContext("/", App::status);
        server.createContext("/tls", App::tls);
        server.start();
        System.out.println("listening on " + port);
    }

    private static void status(HttpExchange exchange) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("java.version=").append(System.getProperty("java.version")).append('\n');
        sb.append("java.home=").append(System.getProperty("java.home")).append('\n');
        sb.append("provider.1=").append(Security.getProviders()[0].getName()).append('\n');
        sb.append("approved.only=").append(approvedOnly()).append('\n');
        sb.append("rsa1024.allowed=").append(weakRsaAllowed()).append('\n');
        sb.append("max.heap=").append(Runtime.getRuntime().maxMemory()).append('\n');
        sb.append("jvm.args=").append(ManagementFactory.getRuntimeMXBean().getInputArguments()).append('\n');
        try {
            sb.append("tls.provider=").append(SSLContext.getDefault().getProvider().getName()).append('\n');
        } catch (Exception e) {
            sb.append("tls.provider=error:").append(e).append('\n');
        }
        reply(exchange, 200, sb.toString());
    }

    private static void tls(HttpExchange exchange) throws IOException {
        String query = exchange.getRequestURI().getQuery();
        String target = query != null && query.startsWith("url=") ? query.substring(4) : "https://repo1.maven.org/maven2/";
        try {
            HttpsURLConnection connection = (HttpsURLConnection) new URL(target).openConnection();
            connection.setConnectTimeout(10000);
            connection.setReadTimeout(10000);
            int code = connection.getResponseCode();
            String suite = connection.getCipherSuite();
            try (InputStream in = connection.getInputStream()) {
                while (in.read() != -1) {
                }
            }
            reply(exchange, 200, "status=" + code + "\ncipher=" + suite + "\n");
        } catch (Exception e) {
            reply(exchange, 502, "error=" + e + "\n");
        }
    }

    private static String approvedOnly() {
        try {
            Class<?> registrar = Class.forName("org.bouncycastle.crypto.CryptoServicesRegistrar");
            return String.valueOf(registrar.getMethod("isInApprovedOnlyMode").invoke(null));
        } catch (ClassNotFoundException e) {
            return "absent";
        } catch (Exception e) {
            return "error:" + e;
        }
    }

    private static boolean weakRsaAllowed() {
        try {
            KeyPairGenerator kpg = KeyPairGenerator.getInstance("RSA");
            kpg.initialize(1024);
            kpg.generateKeyPair();
            return true;
        } catch (Throwable t) {
            return false;
        }
    }

    private static void reply(HttpExchange exchange, int code, String body) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "text/plain; charset=utf-8");
        exchange.sendResponseHeaders(code, bytes.length);
        try (OutputStream out = exchange.getResponseBody()) {
            out.write(bytes);
        }
    }
}
