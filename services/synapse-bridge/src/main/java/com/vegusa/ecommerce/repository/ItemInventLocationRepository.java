package com.vegusa.ecommerce.repository;

import com.vegusa.ecommerce.dto.ItemInventLocationDTO;
import com.vegusa.ecommerce.dto.SourceSystem;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public class ItemInventLocationRepository {
    private final JdbcTemplate jdbcTemplate;

    public ItemInventLocationRepository(@Qualifier("erpJdbcTemplate") JdbcTemplate jdbcTemplate) {
        this.jdbcTemplate = jdbcTemplate;
    }

    public List<ItemInventLocationDTO> getAllItemInventory(){
        return jdbcTemplate.query("SELECT TOP 10 * FROM dyn.ItemInventLocation", (rs, rowNum) -> new ItemInventLocationDTO(
                SourceSystem.ERP,
                rs.getString("Articulo"),
                rs.getString("Descripción"),
                rs.getString("Num.Parte"),
                rs.getString("Sucursal"),
                rs.getString("Almacen"),
                rs.getString("Name"),
                rs.getString("Grupo"),
                rs.getInt("Disponible"),
                rs.getBigDecimal("Costo Transaccion"),
                rs.getBigDecimal("Costo promedio"),
                rs.getBigDecimal("Costo total"),
                rs.getString("Dimension"),
                rs.getString("Cateogría"),
                rs.getString("Marca"),
                rs.getString("Address"),
                rs.getString("Empresa")
        ));
    }

    public List<ItemInventLocationDTO> getLocations(int offset, int pageSize){
        return jdbcTemplate.query("""
                SELECT * FROM dyn.ItemInventLocation
                ORDER BY Articulo
                OFFSET ? ROWS
                FETCH NEXT ? ROWS ONLY
                """, (rs, rowNum) -> new ItemInventLocationDTO(
                    SourceSystem.ERP,
                    rs.getString("Articulo"),
                    rs.getString("Descripción"),
                    rs.getString("Num.Parte"),
                    rs.getString("Sucursal"),
                    rs.getString("Almacen"),
                    rs.getString("Name"),
                    rs.getString("Grupo"),
                    rs.getInt("Disponible"),
                    rs.getBigDecimal("Costo Transaccion"),
                    rs.getBigDecimal("Costo promedio"),
                    rs.getBigDecimal("Costo total"),
                    rs.getString("Dimension"),
                    rs.getString("Cateogría"),
                    rs.getString("Marca"),
                    rs.getString("Address"),
                    rs.getString("Empresa")
        ), offset, pageSize);
    }
}
