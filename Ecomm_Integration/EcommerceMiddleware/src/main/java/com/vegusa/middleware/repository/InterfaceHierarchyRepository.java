package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.InterfaceHierarchy;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface InterfaceHierarchyRepository extends JpaRepository<InterfaceHierarchy, Long> {
    @Query(value = "select InterfaceId from interfacehierarchy ih where ih.DataAreaId = ?1 order by ih.Priority", nativeQuery = true)
    List<String> getInterfaceHierarchy(String dataAreaId);
}
