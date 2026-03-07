package com.vegusa.middleware.config;

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
        entityManagerFactoryRef = "msbServerEntityManagerFactory",
        transactionManagerRef = "msbServerTransactionManager",
        basePackages = {"com.vegusa.middleware.repository.erp"}
)

public class MSBServerConfig {
    @Bean(name = "msbServerDataSource")
    @ConfigurationProperties(prefix = "spring.msb-data-lake.datasource")
    public DataSource msbServerDataSource(){ return DataSourceBuilder.create().build(); }

    @Bean(name = "msbServerEntityManagerFactory")
    public LocalContainerEntityManagerFactoryBean
        barEntityManagerFactory(
            EntityManagerFactoryBuilder builder,
            @Qualifier("msbServerDataSource") DataSource dataSource) {
        return builder.dataSource(dataSource)
                        .packages("com.vegusa.middleware.model.erp")
                        .persistenceUnit("msbServerPU")
                        .build();
    }

    @Bean(name = "msbServerTransactionManager")
    public PlatformTransactionManager msbServerTransactionManager(
            @Qualifier("msbServerEntityManagerFactory") EntityManagerFactory
                    msbServerEntityManagerFactory) {
        return new JpaTransactionManager(msbServerEntityManagerFactory);
    }

}
