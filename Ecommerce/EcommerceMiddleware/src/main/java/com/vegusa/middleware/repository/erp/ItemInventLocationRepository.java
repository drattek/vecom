package com.vegusa.middleware.repository.erp;

import com.vegusa.middleware.dto.PriceProjection;
import com.vegusa.middleware.integrations.jumpseller.dto.StockProjection;
import com.vegusa.middleware.model.erp.ItemInventLocation;
import com.vegusa.middleware.model.erp.ItemInventLocationId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ItemInventLocationRepository extends JpaRepository<ItemInventLocation, ItemInventLocationId> {
    @Query(value = "select * from ItemInventLocation where Almacen = ?1 order by Articulo", nativeQuery = true)
    ItemInventLocation[] getItemInventLocation(String warehouse);

    @Query(value = "select * from ItemInventLocation where Almacen IN :warehouses and Articulo = :internalCode", nativeQuery = true)
    ItemInventLocation[] getItemStock(@Param("warehouses") List<String> warehouses, @Param("internalCode") String internalCode);

    @Query(value = "select Articulo, SUM(Disponible) as Total from dyn.ItemInventLocation where Almacen IN :warehouses and Articulo IN :items GROUP BY Articulo", nativeQuery = true)
    List<StockProjection> getStock(@Param("warehouses") List<String> warehouses, @Param("items") List<String> items);

    @Query(value = "select distinct Almacen, Name, Address from ItemInventLocation where Almacen not like '%-%' order by Almacen", nativeQuery = true)
    List<Object[]> getWarehouse();

    @Query(value = "select Articulo, [Costo promedio] from ItemInventLocation group by Articulo, [Costo promedio] order by Articulo", nativeQuery = true)
    List<Object[]> getItemCost();

    @Query(value = "SELECT Articulo, [Costo promedio] FROM dyn.ItemInventLocation where Articulo IN :items group by Articulo, [Costo promedio] order by Articulo", nativeQuery = true)
    List<Object[]> getCosts(@Param("items") List<String> items);

    @Query(value = "select Articulo, [Costo promedio] as cost from dyn.ItemInventLocation where Articulo IN :items and [Costo promedio] IS NOT NULL group by Articulo, [Costo promedio] order by Articulo", nativeQuery = true)
    List<PriceProjection> getPrices(@Param("items") List<String> items);
}
