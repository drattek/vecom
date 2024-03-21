package com.vegusa.veg_mv_integration_midd.veg_middleware.Config;

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
        basePackages = {"com.vegusa.veg_mv_integration_midd.veg_middleware.repository"}
)
public class DBVegMiddlewareConfig
{
    @Primary
    @Bean(name = "vegMiddlewareDataSource")
    @ConfigurationProperties(prefix = "spring.veg-middleware.datasource")
    public DataSource vegMiddlewareDataSource() {
        return DataSourceBuilder.create().build();
    }

    @Primary
    @Bean(name = "dbVegMiddEntityManagerFactory")
    public LocalContainerEntityManagerFactoryBean
    entityManagerFactory(EntityManagerFactoryBuilder builder, @Qualifier("vegMiddlewareDataSource") DataSource dataSource)
    {
        return builder
                .dataSource(dataSource)
                .packages("com.vegusa.veg_mv_integration_midd.veg_middleware.entity")
                .persistenceUnit("db1")
                .build();
    }

    @Primary
    @Bean(name = "dbVegMiddTransactionManager")
    public PlatformTransactionManager dbVegMiddTransactionManager(
            @Qualifier("dbVegMiddEntityManagerFactory") EntityManagerFactory dbVegMiddEntityManagerFactory)
    {
        return new JpaTransactionManager(dbVegMiddEntityManagerFactory);
    }

}
