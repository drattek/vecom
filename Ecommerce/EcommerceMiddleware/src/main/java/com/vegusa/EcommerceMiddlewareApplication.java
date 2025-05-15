package com.vegusa;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.data.jpa.repository.config.EnableJpaRepositories;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
@EnableJpaRepositories("com.vegusa.middleware.integrations.jumpseller.repository.SyncProductJumpsellerRepository")
public class EcommerceMiddlewareApplication
{
    public static void main(String[] args)
    {
        SpringApplication.run(EcommerceMiddlewareApplication.class, args);
    }
}
