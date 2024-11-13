package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.SystemParameter;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

@Repository
public interface SystemParameterRepository extends JpaRepository<SystemParameter, Long> {

    @Query(value = "select * from SystemParameter where Name = ?1", nativeQuery = true)
    SystemParameter getSystemParameter(String parameterName);

}
