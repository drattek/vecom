package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.InterfaceDS;
import com.vegusa.middleware.entity.InterfaceDSId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface InterfaceDSRepository  extends JpaRepository<InterfaceDS, InterfaceDSId> {
    @Query(value = "select * from interface i where i.InterfaceId = ?1", nativeQuery = true)
    InterfaceDS getInterface(String interfaceId);
}
