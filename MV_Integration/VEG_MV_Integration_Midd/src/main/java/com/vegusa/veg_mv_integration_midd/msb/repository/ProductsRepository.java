package com.vegusa.veg_mv_integration_midd.msb.repository;

import com.vegusa.veg_mv_integration_midd.msb.entity.VendTable;
import jakarta.persistence.Entity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

import java.util.Collection;

public interface ProductsRepository extends JpaRepository<VendTable, Integer>
{
    @Query(
        value = "SELECT PAYMTERMID FROM VENDTABLE",
        nativeQuery = true)
    Collection<VendTable> test();

}