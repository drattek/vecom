package com.vegusa.msb.config;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.boot.orm.jpa.EntityManagerFactoryBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
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
        entityManagerFactoryRef = "dbMsbDataLakeEntityManagerFactory",
        transactionManagerRef = "dbMsbDataLakeTransactionManager",
        basePackages = {"com.vegusa.msb.repository"}
)

public class MSBDataBaseConfig {
    @Bean(name = "dbMsbDataLakeDataSource")
    @ConfigurationProperties(prefix = "spring.msb-data-lake.datasource")
    public DataSource dbMsbDataLakeDataSource(){ return DataSourceBuilder.create().build(); }

    @Bean(name = "dbMsbDataLakeEntityManagerFactory")
    public LocalContainerEntityManagerFactoryBean
        barEntityManagerFactory(
            EntityManagerFactoryBuilder builder,
            @Qualifier("dbMsbDataLakeDataSource") DataSource dataSource) {
        return builder.dataSource(dataSource)
                        .packages("com.vegusa.msb.entity")
                        .persistenceUnit("db2")
                        .build();
    }

    @Bean(name = "dbMsbDataLakeTransactionManager")
    public PlatformTransactionManager dbMsbDataLakeTransactionManager(
            @Qualifier("dbMsbDataLakeEntityManagerFactory") EntityManagerFactory
                    dbMsbDataLakeEntityManagerFactory) {
        return new JpaTransactionManager(dbMsbDataLakeEntityManagerFactory);
    }

}
