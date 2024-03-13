package com.vegusa.veg_mv_integration_midd;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class VegMvOauth20Application
{
    public static void main(String[] args)
    {
        SpringApplication.run(VegMvOauth20Application.class, args);
    }
}
