package com.vegusa.veg_mv_integration_midd.veg_middleware.repository;

import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.Company;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.InterfaceDS;
import com.vegusa.veg_mv_integration_midd.veg_middleware.entity.InterfaceDSId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface InterfaceDSRepository  extends JpaRepository<InterfaceDS, InterfaceDSId> {
    @Query(value = "select * from interface i where i.InterfaceId = ?1", nativeQuery = true)
    InterfaceDS getInterface(String interfaceId);
}
