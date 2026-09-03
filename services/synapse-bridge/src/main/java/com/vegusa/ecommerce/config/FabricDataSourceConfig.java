package com.vegusa.ecommerce.config;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

import javax.sql.DataSource;

/**
 * Conexión de solo lectura al SQL analytics endpoint de Microsoft Fabric
 * (lakehouse / warehouse). Habla TDS igual que SQL Server, por lo que reutiliza
 * el driver mssql-jdbc y el flujo AAD (msal4j) ya presentes para Synapse.
 *
 * Autenticación headless vía service principal: en el JDBC URL
 * {@code authentication=ActiveDirectoryServicePrincipal}, con
 * {@code spring.datasource.fabric.username} = client id y
 * {@code spring.datasource.fabric.password} = secret.
 *
 * El token AAD del service principal expira (~1 h); {@code max-lifetime} del pool
 * se mantiene por debajo de ese umbral para que ninguna conexión física
 * sobreviva a su token.
 */
@Configuration
public class FabricDataSourceConfig {

    @Bean(name = "fabricDataSource")
    @ConfigurationProperties(prefix = "spring.datasource.fabric")
    public DataSource fabricDataSource() {
        return DataSourceBuilder.create().build();
    }

    @Bean(name = "fabricJdbcTemplate")
    public JdbcTemplate fabricJdbcTemplate(@Qualifier("fabricDataSource") DataSource dataSource) {
        return new JdbcTemplate(dataSource);
    }
}
