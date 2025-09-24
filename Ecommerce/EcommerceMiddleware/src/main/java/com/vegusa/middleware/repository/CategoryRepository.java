package com.vegusa.middleware.repository;

import com.vegusa.middleware.entity.Category;
import com.vegusa.middleware.entity.CategoryId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

@Repository
public interface CategoryRepository extends JpaRepository<Category, CategoryId> {
    @Query(value = "select * from category where RecId = ?1", nativeQuery = true)
    Category getCategory(Long recId);

    @Query(value = "select * from category where Name = :name", nativeQuery = true)
    Category[] getCategories(@Param("name") String name);

    @Query(value = "SELECT * FROM category where dataAreaId = :dataAreaId", nativeQuery = true)
    Category[] getAllCategories(@Param("dataAreaId") String dataAreaId);
}
