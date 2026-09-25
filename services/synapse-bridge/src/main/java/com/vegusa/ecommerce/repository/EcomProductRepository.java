package com.vegusa.ecommerce.repository;

import com.vegusa.ecommerce.dto.EcomProductDTO;
import com.vegusa.ecommerce.dto.SourceSystem;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public class EcomProductRepository {
    private final JdbcTemplate jdbcTemplate;

    public EcomProductRepository(@Qualifier("fabricJdbcTemplate") JdbcTemplate jdbcTemplate) {
        this.jdbcTemplate = jdbcTemplate;
    }

    public List<EcomProductDTO> getAllItems(){
        return jdbcTemplate.query("SELECT * FROM dyn.ECOMProducts", (rs, rowNum) -> new EcomProductDTO(
                SourceSystem.ERP,
                rs.getInt("RECID"),
                rs.getString("Articulo"),
                rs.getString("Descripción"),
                rs.getString("Num.Parte"),
                rs.getString("Grupo"),
                rs.getString("Marca")
        ));
    }

    public List<EcomProductDTO> getPagedProducts(int offset, int pageSize){
        return jdbcTemplate.query("""
                SELECT * FROM dyn.ECOMProducts ORDER BY Articulo
                OFFSET ? ROWS
                FETCH NEXT ? ROWS ONLY
                """, (rs, rowNum) -> new EcomProductDTO(
                        SourceSystem.ERP,
                        rs.getInt("RECID"),
                        rs.getString("Articulo"),
                        rs.getString("Descripción"),
                        rs.getString("Num.Parte"),
                        rs.getString("Grupo"),
                        rs.getString("Marca")
        ), offset, pageSize);
    }
}
