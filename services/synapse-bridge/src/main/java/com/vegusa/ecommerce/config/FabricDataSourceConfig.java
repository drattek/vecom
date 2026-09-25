package com.vegusa.ecommerce.config;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.jdbc.core.JdbcTemplate;

import javax.sql.DataSource;

/**
 * Conexión de solo lectura al SQL analytics endpoint del lakehouse del Link to
 * Fabric (espejo de D365 F&O, workspace DYN365-Fabric). Es el origen de los datos
 * del ERP: sustituye a Synapse serverless, que se retira. Las vistas dyn.* que
 * leen los repositorios se despliegan con infrastructure/fabric/deploy_views.py.
 *
 * Habla TDS igual que SQL Server, por lo que usa el driver mssql-jdbc y el flujo
 * AAD de msal4j.
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
    @Primary
    @ConfigurationProperties(prefix = "spring.datasource.fabric")
    public DataSource fabricDataSource() {
        return DataSourceBuilder.create().build();
    }

    @Bean(name = "fabricJdbcTemplate")
    @Primary
    public JdbcTemplate fabricJdbcTemplate(@Qualifier("fabricDataSource") DataSource dataSource) {
        return new JdbcTemplate(dataSource);
    }
}
