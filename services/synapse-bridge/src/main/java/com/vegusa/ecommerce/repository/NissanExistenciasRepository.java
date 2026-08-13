package com.vegusa.ecommerce.repository;

import com.vegusa.ecommerce.dto.NissanExistenciasDTO;
import com.vegusa.ecommerce.dto.SourceSystem;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public class NissanExistenciasRepository {
    private final JdbcTemplate jdbcTemplate;

    public NissanExistenciasRepository(@Qualifier("nissanJdbcTemplate") JdbcTemplate jdbcTemplate) {
        this.jdbcTemplate = jdbcTemplate;
    }

    public List<NissanExistenciasDTO> getPagedExistencias(int offset, int pageSize){
        return jdbcTemplate.query("""
                SELECT * FROM RE_VEXISTENCIAS WHERE RELA_EXISTENCIAACTUAL > 0 ORDER BY PROD_CLAVE
                OFFSET ? ROWS
                FETCH NEXT ? ROWS ONLY
                """, (rs, rowNum) -> new NissanExistenciasDTO(
                        SourceSystem.NISSAN,
                        rs.getString("PROD_CLAVE"),
                        rs.getString("PROD_TIPOREFA"),
                        rs.getInt("RELA_EXISTENCIAACTUAL"),
                        rs.getBigDecimal("RELA_COSTOPROMEDIO"),
                        rs.getString("PROD_STATUS"),
                        rs.getString("RELA_UBICACION"),
                        rs.getString("PROD_DESCRIPCION1"),
                        rs.getString("PROD_UNIDAD"),
                        rs.getString("AGEN_NOMAGENCIA"),
                        rs.getBoolean("PROD_ORIGINAL"),
                        rs.getBigDecimal("PROD_PRECIO1"),
                        rs.getBigDecimal("PROD_PRECIO2"),
                        rs.getBigDecimal("PROD_PRECIO3"),
                        rs.getBigDecimal("PROD_PRECIO4"),
                        rs.getBigDecimal("PROD_PRECIO5"),
                        rs.getString("PROD_SUPERSESION")
        ), offset, pageSize);
    }
}
