package com.vegusa.middleware.config;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.boot.orm.jpa.EntityManagerFactoryBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.data.jpa.repository.config.EnableJpaRepositories;
import org.springframework.orm.jpa.JpaTransactionManager;
import org.springframework.orm.jpa.LocalContainerEntityManagerFactoryBean;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.EnableTransactionManagement;

import jakarta.persistence.EntityManagerFactory;
import javax.sql.DataSource;

@Configuration
@EnableTransactionManagement
@EnableJpaRepositories(
        entityManagerFactoryRef = "dbVegMiddEntityManagerFactory",
        transactionManagerRef = "dbVegMiddTransactionManager",
        basePackages = {"com.vegusa.middleware.repository.local", "com.vegusa.middleware.integrations.camso.repository", "com.vegusa.middleware.integrations.jumpseller.repository", "com.vegusa.middleware.integrations.mercadolibre.repository", "com.vegusa.middleware.integrations.multivende.repository"}
)
public class MWDataBaseConfig {
    @Primary
    @Bean(name = "vegMiddlewareDataSource")
    @ConfigurationProperties(prefix = "spring.veg-middleware.datasource")
    public DataSource vegMiddlewareDataSource() {
        return DataSourceBuilder.create().build();
    }

    @Primary
    @Bean(name = "dbVegMiddEntityManagerFactory")
    public LocalContainerEntityManagerFactoryBean
    entityManagerFactory(EntityManagerFactoryBuilder builder, @Qualifier("vegMiddlewareDataSource") DataSource dataSource) {
        return builder
                .dataSource(dataSource)
                .packages("com.vegusa.middleware.entity", "com.vegusa.middleware.integrations.camso.entity", "com.vegusa.middleware.integrations.jumpseller.entity", "com.vegusa.middleware.integrations.mercadolibre.entity", "com.vegusa.middleware.integrations.multivende.entity")
                .persistenceUnit("db1")
                .build();
    }

    @Primary
    @Bean(name = "dbVegMiddTransactionManager")
    public PlatformTransactionManager dbVegMiddTransactionManager(
            @Qualifier("dbVegMiddEntityManagerFactory") EntityManagerFactory dbVegMiddEntityManagerFactory) {
        return new JpaTransactionManager(dbVegMiddEntityManagerFactory);
    }

}
