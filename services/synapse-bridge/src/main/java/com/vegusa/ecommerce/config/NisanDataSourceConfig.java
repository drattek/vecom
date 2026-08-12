package com.vegusa.ecommerce.config;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

import javax.sql.DataSource;

@Configuration
public class NisanDataSourceConfig {

    @Bean(name = "nissanDataSource")
    @ConfigurationProperties(prefix = "spring.datasource.nissan")
    public DataSource nissanDataSource(){
        return DataSourceBuilder.create().build();
    }

    @Bean(name = "nissanJdbcTemplate")
    public JdbcTemplate nissanJdbcTemplate(@Qualifier("nissanDataSource") DataSource dataSource){
        return new JdbcTemplate(dataSource);
    }
}
