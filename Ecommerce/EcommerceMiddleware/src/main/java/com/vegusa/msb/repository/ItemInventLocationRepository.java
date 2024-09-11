package com.vegusa.msb.repository;

import com.vegusa.msb.entity.ItemInventLocation;
import com.vegusa.msb.entity.ItemInventLocationId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ItemInventLocationRepository extends JpaRepository<ItemInventLocation, ItemInventLocationId> {
    @Query(value = "select * from ItemInventLocation where Almacen = ?1 order by Articulo", nativeQuery = true)
    ItemInventLocation[] getItemInventLocation(String warehouse);

    @Query(value = "select distinct Almacen, Name, Address from ItemInventLocation order by Almacen", nativeQuery = true)
    List<Object[]> getWarehouse();

    @Query(value = "select Articulo, [Costo promedio] from ItemInventLocation group by Articulo, [Costo promedio] order by Articulo", nativeQuery = true)
    List<Object[]> getItemCost();

    @Query(value = "select Articulo, Cateogría from ItemInventLocation group by Articulo, Cateogría order by Articulo", nativeQuery = true)
    List<Object[]> getItemCategory();
}
