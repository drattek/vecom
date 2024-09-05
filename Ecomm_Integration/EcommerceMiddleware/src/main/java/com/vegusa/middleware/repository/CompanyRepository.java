package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Company;
import com.vegusa.middleware.entity.CompanyId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface CompanyRepository extends JpaRepository<Company, CompanyId> {
    @Query(value = "select * from company c where c.DataAreaId = ?1", nativeQuery = true)
    Company getCompany(String dataAreaId);
}
