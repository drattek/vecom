package com.vegusa.msb.repository;

import com.vegusa.msb.entity.DYNProduct;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface DYNProductRepository extends JpaRepository<DYNProduct, Long> {
    @Query(value = "select * from ecomproducts where [Num.Parte] IN (" +
            "'6102021'," +
            "'7310678'," +
            "'6812980'," +
            "'7194871'," +
            "'7152664'," +
            "'7101894'," +
            "'7419388'," +
            "'7218087'," +
            "'7152508'," +
            "'7524457'," +
            "'6679239'," +
            "'6679239'," +
            "'7501035'," +
            "'7117662'," +
            "'7185943'," +
            "'M7015*1499004'," +
            "'7113737.EA'," +
            "'7337715'," +
            "'7324830'," +
            "'7102125'," +
            "'M70191608083'," +
            "'7165404'," +
            "'7279102'," +
            "'7272768'," +
            "'7279101'," +
            "'7272771'," +
            "'7115937'," +
            "'7294305'," +
            "'7297499'," +
            "'7405171'," +
            "'7204288'," +
            "'7272680'," +
            "'7234536'," +
            "'7355674'," +
            "'6731409.EA'," +
            "'7115923'" +
            ") order by articulo", nativeQuery = true)
    DYNProduct[] getDYNProducts();
}
