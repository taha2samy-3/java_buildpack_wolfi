package io.github.taha2samy3.sample;

import java.security.Security;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class Application {

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @GetMapping("/")
    public String status() {
        return "java.version=" + System.getProperty("java.version") + "\n"
            + "provider.1=" + Security.getProviders()[0].getName() + "\n"
            + "max.heap=" + Runtime.getRuntime().maxMemory() + "\n";
    }
}
