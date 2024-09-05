package com.vegusa;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class EcommerceMiddlewareApplication
{
    public static void main(String[] args)
    {
        SpringApplication.run(EcommerceMiddlewareApplication.class, args);
    }
}
