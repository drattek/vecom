package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.InterfaceHierarchy;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface InterfaceHierarchyRepository extends JpaRepository<InterfaceHierarchy, Long> {
    @Query(value = "select InterfaceId from interfacehierarchy where DataAreaId = ?1 order by Priority", nativeQuery = true)
    List<String> getInterfaceHierarchy(String dataAreaId);
}
