package com.vegusa.veg_mv_integration_midd;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.data.jpa.repository.config.EnableJpaRepositories;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
@EnableJpaRepositories
public class VegMvIntegrationMiddApplication
{
    public static void main(String[] args)
    {
        SpringApplication.run(VegMvIntegrationMiddApplication.class, args);
    }
}
