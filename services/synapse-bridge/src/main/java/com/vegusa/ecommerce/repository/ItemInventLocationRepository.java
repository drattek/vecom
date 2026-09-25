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

    public ItemInventLocationRepository(@Qualifier("fabricJdbcTemplate") JdbcTemplate jdbcTemplate) {
        this.jdbcTemplate = jdbcTemplate;
    }

    public List<ItemInventLocationDTO> getAllItemInventory(){
        return jdbcTemplate.query("""
                SELECT TOP 10 * FROM dyn.ItemInventLocation
                WHERE Disponible IS NOT NULL AND Disponible > 0
                """, (rs, rowNum) -> {
                    String almacen = rs.getString("Almacen");
                    return new ItemInventLocationDTO(
                        SourceSystem.ERP,
                        rs.getString("Articulo"),
                        rs.getString("Descripción"),
                        rs.getString("Num.Parte"),
                        rs.getString("Sucursal"),
                        almacen,
                        rs.getString("Name"),
                        rs.getString("Grupo"),
                        availableForWarehouse(almacen, rs.getInt("Disponible")),
                        rs.getBigDecimal("Costo Transaccion"),
                        rs.getBigDecimal("Costo promedio"),
                        rs.getBigDecimal("Costo total"),
                        rs.getString("Dimension"),
                        rs.getString("Cateogría"),
                        rs.getString("Marca"),
                        rs.getString("Address"),
                        rs.getString("Empresa")
                    );
        });
    }

    public List<ItemInventLocationDTO> getLocations(int offset, int pageSize){
        return jdbcTemplate.query("""
                SELECT * FROM dyn.ItemInventLocation
                WHERE Disponible IS NOT NULL AND Disponible > 0
                ORDER BY Articulo
                OFFSET ? ROWS
                FETCH NEXT ? ROWS ONLY
                """, (rs, rowNum) -> {
                    String almacen = rs.getString("Almacen");
                    return new ItemInventLocationDTO(
                        SourceSystem.ERP,
                        rs.getString("Articulo"),
                        rs.getString("Descripción"),
                        rs.getString("Num.Parte"),
                        rs.getString("Sucursal"),
                        almacen,
                        rs.getString("Name"),
                        rs.getString("Grupo"),
                        availableForWarehouse(almacen, rs.getInt("Disponible")),
                        rs.getBigDecimal("Costo Transaccion"),
                        rs.getBigDecimal("Costo promedio"),
                        rs.getBigDecimal("Costo total"),
                        rs.getString("Dimension"),
                        rs.getString("Cateogría"),
                        rs.getString("Marca"),
                        rs.getString("Address"),
                        rs.getString("Empresa")
                    );
        }, offset, pageSize);
    }

    // Los almacenes cuyo nombre incluye "-" no deben distribuirse al ecommerce: en vez de
    // excluirlos de la consulta (lo que dejaría el stock local ya sincronizado colgado en un
    // valor positivo, porque la reconciliación por ausencia solo baja a 0 almacenes que
    // "aparecen" en la corrida), se reportan igual pero con disponible forzado a 0. Así el
    // flujo normal de sync (syncProductStock) corrige el stock local existente sin scripts
    // manuales ni tocar la base de datos por fuera del flujo.
    private static int availableForWarehouse(String almacen, int disponible) {
        return isIgnoredWarehouse(almacen) ? 0 : disponible;
    }

    private static boolean isIgnoredWarehouse(String almacen) {
        return almacen != null && almacen.contains("-");
    }
}
