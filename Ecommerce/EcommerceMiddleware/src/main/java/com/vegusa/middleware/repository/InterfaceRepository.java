package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Interface;
import com.vegusa.middleware.entity.InterfaceId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface InterfaceRepository extends JpaRepository<Interface, InterfaceId> {
    @Query(value = "select * from interface where InterfaceId = ?1 and DataAreaId = ?2", nativeQuery = true)
    Interface getInterface(String interfaceId, String dataAreaId);
}
